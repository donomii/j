package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donomii/j/types"
	"github.com/donomii/j/workspace"
)

type CodeEditor struct {
	workspaceRoot types.WorkspaceRoot
}

func New(workspaceRoot types.WorkspaceRoot) *CodeEditor {
	return &CodeEditor{workspaceRoot: workspaceRoot}
}

func (e *CodeEditor) ReadFile(path string) (string, error) {
	resolved, err := workspace.Resolve(e.workspaceRoot, path, true)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	data, err := os.ReadFile(resolved)
	return string(data), err
}

func (e *CodeEditor) WriteFile(path, content string) error {
	resolved, err := workspace.Resolve(e.workspaceRoot, path, false)
	if err != nil {
		return fmt.Errorf("create file %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0755); err != nil {
		return fmt.Errorf("create parent directory for %q: %w", resolved, err)
	}
	file, err := os.OpenFile(resolved, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create new file %q without replacing existing data: %w", resolved, err)
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return fmt.Errorf("write new file %q: %w", resolved, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close new file %q: %w", resolved, err)
	}
	return nil
}

func (e *CodeEditor) ApplyPatch(path, search, replace string) error {
	resolved, err := workspace.Resolve(e.workspaceRoot, path, true)
	if err != nil {
		return fmt.Errorf("apply patch to %q: %w", path, err)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return fmt.Errorf("read patch target %q: %w", resolved, err)
	}
	content := string(data)
	count := strings.Count(content, search)
	if count != 1 {
		return fmt.Errorf("apply patch to %q: expected exactly one search block, found %d", resolved, count)
	}
	if err := os.WriteFile(resolved, []byte(strings.Replace(content, search, replace, 1)), 0644); err != nil {
		return fmt.Errorf("write patched file %q: %w", resolved, err)
	}
	return nil
}

func (e *CodeEditor) ListFiles(dir string) ([]string, error) {
	resolved, err := workspace.Resolve(e.workspaceRoot, dir, true)
	if err != nil {
		return nil, fmt.Errorf("list files in %q: %w", dir, err)
	}
	skip := map[string]bool{"node_modules": true, ".git": true, "dist": true}
	var results []string
	err = filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() && skip[info.Name()] {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			results = append(results, path)
		}
		return nil
	})
	return results, err
}
