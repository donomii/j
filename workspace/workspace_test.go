package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfinesPathsAndSymlinks(t *testing.T) {
	rootDirectory := t.TempDir()
	root, err := NewRoot(rootDirectory)
	if err != nil {
		t.Fatalf("create workspace root %q: %v", rootDirectory, err)
	}

	inside, err := Resolve(root, "nested/new.txt", false)
	if err != nil {
		t.Fatalf("resolve path inside workspace: %v", err)
	}
	expectedInside := filepath.Join(string(root), "nested", "new.txt")
	if inside != expectedInside {
		t.Fatalf("resolved inside path=%q, expected=%q", inside, expectedInside)
	}
	if _, err := Resolve(root, "nested/../still-inside.txt", false); err == nil {
		t.Fatal("lexical parent traversal inside the workspace was accepted")
	}

	outsideDirectory := t.TempDir()
	if _, err := Resolve(root, filepath.Join(outsideDirectory, "outside.txt"), false); err == nil {
		t.Fatalf("absolute path outside workspace was accepted: %q", outsideDirectory)
	}
	symlinkPath := filepath.Join(rootDirectory, "outside-link")
	if err := os.Symlink(outsideDirectory, symlinkPath); err != nil {
		t.Fatalf("create workspace escape symlink %q -> %q: %v", symlinkPath, outsideDirectory, err)
	}
	if _, err := Resolve(root, filepath.Join(symlinkPath, "outside.txt"), false); err == nil {
		t.Fatalf("symlink path outside workspace was accepted: %q", symlinkPath)
	}
}

func TestResolveRequiresExistingPathWhenRequested(t *testing.T) {
	rootDirectory := t.TempDir()
	root, err := NewRoot(rootDirectory)
	if err != nil {
		t.Fatalf("create workspace root %q: %v", rootDirectory, err)
	}
	if _, err := Resolve(root, "missing.txt", true); err == nil {
		t.Fatal("missing required path was accepted")
	}
}
