package runner

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultModelURL = "https://huggingface.co/bartowski/Qwen2.5-Coder-3B-Instruct-GGUF/resolve/main/Qwen2.5-Coder-3B-Instruct-Q4_K_M.gguf"
	ServerPort      = 8765
)

func cacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "j-agent")
}

// EnsureModel downloads the GGUF model if not already cached and returns its path.
func EnsureModel(modelURL string) (string, error) {
	dir := cacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := filepath.Base(modelURL)
	if i := strings.IndexByte(name, '?'); i >= 0 {
		name = name[:i]
	}
	dest := filepath.Join(dir, name)
	if _, err := os.Stat(dest); err == nil {
		fmt.Printf("Model cached at %s\n", dest)
		return dest, nil
	}
	fmt.Printf("Downloading model (%s)...\n", name)
	return dest, downloadFile(modelURL, dest)
}

// EnsureServer downloads and extracts the llama-server binary if not already cached.
func EnsureServer() (string, error) {
	dir := cacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName = "llama-server.exe"
	}
	dest := filepath.Join(dir, binName)
	sentinel := dest + ".extracted"
	if _, err := os.Stat(dest); err == nil {
		if _, err := os.Stat(sentinel); err == nil {
			fmt.Printf("llama-server cached at %s\n", dest)
			return dest, nil
		}
		// Binary exists but was extracted without dylibs — delete and re-extract.
		os.Remove(dest)
	}

	assetURL, err := findAsset()
	if err != nil {
		return "", fmt.Errorf("find llama-server release asset: %w", err)
	}
	fmt.Printf("Downloading llama-server...\n")

	archivePath := dest + archiveExt(assetURL)
	if err := downloadFile(assetURL, archivePath); err != nil {
		return "", err
	}
	defer os.Remove(archivePath)

	if err := extractAll(archivePath, dir); err != nil {
		return "", fmt.Errorf("extract archive: %w", err)
	}
	if err := os.Chmod(dest, 0755); err != nil {
		return "", err
	}
	createSonameSymlinks(dir)
	os.WriteFile(sentinel, []byte("ok"), 0644)
	return dest, nil
}

func archiveExt(url string) string {
	if strings.HasSuffix(url, ".tar.gz") {
		return ".tar.gz"
	}
	return ".zip"
}

func apiGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "j-agent/1.0")
	return http.DefaultClient.Do(req)
}

func findAsset() (string, error) {
	resp, err := apiGet("https://api.github.com/repos/ggerganov/llama.cpp/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// GitHub API sometimes returns a JSON redirect instead of an HTTP redirect.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", err
	}
	if redirectURL, ok := raw["url"]; ok && raw["message"] != nil {
		var u string
		json.Unmarshal(redirectURL, &u)
		resp2, err := apiGet(u)
		if err != nil {
			return "", err
		}
		defer resp2.Body.Close()
		if err := json.NewDecoder(resp2.Body).Decode(&raw); err != nil {
			return "", err
		}
	}

	assetsRaw, ok := raw["assets"]
	if !ok {
		return "", fmt.Errorf("no assets in release response")
	}
	var assets []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	if err := json.Unmarshal(assetsRaw, &assets); err != nil {
		return "", err
	}

	var wantKey string
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		wantKey = "macos-arm64"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		wantKey = "macos-x64"
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		wantKey = "ubuntu-x64"
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		wantKey = "ubuntu-arm64"
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	for _, asset := range assets {
		lower := strings.ToLower(asset.Name)
		if !strings.Contains(lower, wantKey) {
			continue
		}
		// Skip GPU-specific and kleidiai variants; prefer plain CPU/Metal build
		if strings.Contains(lower, "cuda") || strings.Contains(lower, "rocm") ||
			strings.Contains(lower, "vulkan") || strings.Contains(lower, "kleidiai") ||
			strings.Contains(lower, "openvino") || strings.Contains(lower, "sycl") {
			continue
		}
		return asset.BrowserDownloadURL, nil
	}
	return "", fmt.Errorf("no matching release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// extractAll extracts every regular file from the archive flat into destDir (no subdirectories).
func extractAll(archivePath, destDir string) error {
	if strings.HasSuffix(archivePath, ".tar.gz") {
		return extractAllTarGz(archivePath, destDir)
	}
	return extractAllZip(archivePath, destDir)
}

func extractAllTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, filepath.Base(hdr.Name))
		switch hdr.Typeflag {
		case tar.TypeReg:
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0644)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			os.Remove(dest)
			if err := os.Symlink(hdr.Linkname, dest); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractAllZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, filepath.Base(f.Name))
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0644)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	total := resp.ContentLength
	var done int64
	buf := make([]byte, 64*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			done += int64(n)
			if total > 0 {
				fmt.Printf("\r  %.1f%% (%d / %d MB)", float64(done)/float64(total)*100, done>>20, total>>20)
			} else {
				fmt.Printf("\r  %d MB", done>>20)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			f.Close()
			return err
		}
	}
	fmt.Println()
	f.Close()
	return os.Rename(tmp, dest)
}

// Start launches llama-server with the given model. Returns the server's base URL and a stop function.
func Start(serverBin, modelPath string) (string, func(), error) {
	// macOS quarantines binaries downloaded from the internet; strip the attribute before running.
	if runtime.GOOS == "darwin" {
		exec.Command("xattr", "-d", "com.apple.quarantine", serverBin).Run()
	}

	serverURL := fmt.Sprintf("http://localhost:%d", ServerPort)

	logPath := filepath.Join(cacheDir(), "llama-server.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", nil, fmt.Errorf("open llama-server log: %w", err)
	}

	cmd := exec.Command(serverBin,
		"-m", modelPath,
		"--port", fmt.Sprintf("%d", ServerPort),
		"--ctx-size", "4096",
		"-n", "-1",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return "", nil, fmt.Errorf("start llama-server: %w", err)
	}

	// Watch for early exit in the background.
	died := make(chan error, 1)
	go func() { died <- cmd.Wait() }()

	stop := func() {
		if cmd.Process != nil {
			cmd.Process.Signal(os.Interrupt)
		}
	}

	fmt.Printf("Loading model (log: %s)\n", logPath)
	client := &http.Client{Timeout: 2 * time.Second}
	for range 120 {
		select {
		case err := <-died:
			logFile.Close()
			tail, _ := os.ReadFile(logPath)
			return "", nil, fmt.Errorf("llama-server exited early (%v):\n%s", err, lastN(string(tail), 20))
		default:
		}
		time.Sleep(time.Second)
		fmt.Print(".")
		resp, err := client.Get(serverURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Println(" ready!")
				return serverURL, stop, nil
			}
		}
	}
	stop()
	logFile.Close()
	tail, _ := os.ReadFile(logPath)
	return "", nil, fmt.Errorf("llama-server not ready after 120s. Last log:\n%s", lastN(string(tail), 20))
}

// createSonameSymlinks creates foo.MAJOR.dylib -> foo.MAJOR.MINOR.PATCH.dylib symlinks
// for every versioned dylib in dir, so that @rpath/foo.MAJOR.dylib resolves correctly.
func createSonameSymlinks(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".dylib") || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		// Match libfoo.MAJOR.MINOR.PATCH.dylib
		base := strings.TrimSuffix(name, ".dylib")
		parts := strings.Split(base, ".")
		if len(parts) < 2 {
			continue
		}
		// soname is everything up to and including MAJOR
		soname := strings.Join(parts[:len(parts)-2], ".") + ".dylib"
		if soname == name {
			continue
		}
		link := filepath.Join(dir, soname)
		os.Remove(link)
		os.Symlink(name, link)
	}
}

// lastN returns the last n lines of s.
func lastN(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
