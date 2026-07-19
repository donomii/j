package editor

import (
	"path/filepath"
	"testing"

	"github.com/donomii/j/workspace"
)

func TestCodeEditorCreatesReadsAndPatchesInsideWorkspace(t *testing.T) {
	rootDirectory := t.TempDir()
	root, err := workspace.NewRoot(rootDirectory)
	if err != nil {
		t.Fatalf("create workspace root %q: %v", rootDirectory, err)
	}
	editor := New(root)
	if err := editor.WriteFile("nested/value.txt", "before\n"); err != nil {
		t.Fatalf("create file inside workspace: %v", err)
	}
	if err := editor.WriteFile("nested/value.txt", "replacement\n"); err == nil {
		t.Fatal("writeFile replaced an existing file")
	}
	if err := editor.ApplyPatch("nested/value.txt", "before", "after"); err != nil {
		t.Fatalf("patch file inside workspace: %v", err)
	}
	content, err := editor.ReadFile("nested/value.txt")
	if err != nil {
		t.Fatalf("read file inside workspace: %v", err)
	}
	if content != "after\n" {
		t.Fatalf("patched content=%q, expected=%q", content, "after\n")
	}
}

func TestCodeEditorRejectsAmbiguousAndOutsideChanges(t *testing.T) {
	rootDirectory := t.TempDir()
	root, err := workspace.NewRoot(rootDirectory)
	if err != nil {
		t.Fatalf("create workspace root %q: %v", rootDirectory, err)
	}
	editor := New(root)
	if err := editor.WriteFile("duplicate.txt", "same same"); err != nil {
		t.Fatalf("create duplicate-content file: %v", err)
	}
	if err := editor.ApplyPatch("duplicate.txt", "same", "changed"); err == nil {
		t.Fatal("ambiguous patch was accepted")
	}
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	if err := editor.WriteFile(outsidePath, "outside"); err == nil {
		t.Fatalf("outside write was accepted: %q", outsidePath)
	}
}
