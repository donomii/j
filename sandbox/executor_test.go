package sandbox

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/donomii/j/workspace"
)

func TestContainerArgumentsPreserveExecutableAndArguments(t *testing.T) {
	rootDirectory := t.TempDir()
	subdirectory := filepath.Join(rootDirectory, "project")
	if err := os.Mkdir(subdirectory, 0755); err != nil {
		t.Fatalf("create project directory %q: %v", subdirectory, err)
	}
	root, err := workspace.NewRoot(rootDirectory)
	if err != nil {
		t.Fatalf("create workspace root %q: %v", rootDirectory, err)
	}
	executor := New(root, true, "node:20-slim")
	actual, err := executor.containerArguments(
		"node",
		[]string{"script.js", "argument with spaces", "; literal"},
		subdirectory,
	)
	if err != nil {
		t.Fatalf("build container arguments: %v", err)
	}
	expected := []string{
		"run", "--rm", "--network=none",
		"-v", string(root) + ":/workspace",
		"-w", "/workspace/project",
		"node:20-slim", "node", "script.js", "argument with spaces", "; literal",
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("container arguments=%q, expected=%q", actual, expected)
	}
}

func TestExecutorRejectsDisabledAndOutsideExecution(t *testing.T) {
	rootDirectory := t.TempDir()
	root, err := workspace.NewRoot(rootDirectory)
	if err != nil {
		t.Fatalf("create workspace root %q: %v", rootDirectory, err)
	}
	if _, err := New(root, false, "node:20-slim").Execute("node", nil, rootDirectory); err == nil {
		t.Fatal("disabled container executor accepted a command")
	}
	if _, err := New(root, true, "node:20-slim").containerArguments("node", nil, t.TempDir()); err == nil {
		t.Fatal("container arguments accepted a working directory outside the workspace")
	}
}
