package github

import (
	"os"
	"testing"

	"github.com/donomii/j/workspace"
)

func TestRepositoryURLValidation(t *testing.T) {
	validated, err := validateRepositoryURL("https://github.com/donomii/j")
	if err != nil {
		t.Fatalf("validate GitHub repository URL: %v", err)
	}
	if validated != "https://github.com/donomii/j.git" {
		t.Fatalf("validated URL=%q, expected canonical GitHub URL", validated)
	}
	invalidURLs := []string{
		"http://github.com/donomii/j",
		"https://example.com/donomii/j",
		"https://github.com/donomii/j/extra",
		"https://github.com/donomii/j/",
		"https://github.com/donomii//j",
		"https://github.com:444/donomii/j",
		"https://github.com/donomii/j?ref=main",
	}
	for _, invalid := range invalidURLs {
		if _, err := validateRepositoryURL(invalid); err == nil {
			t.Fatalf("invalid repository URL was accepted: %q", invalid)
		}
	}
}

func TestCloneRefusesExistingDestinationWithoutChangingIt(t *testing.T) {
	rootDirectory := t.TempDir()
	root, err := workspace.NewRoot(rootDirectory)
	if err != nil {
		t.Fatalf("create workspace root %q: %v", rootDirectory, err)
	}
	destination := "existing"
	if err := os.Mkdir(rootDirectory+"/"+destination, 0755); err != nil {
		t.Fatalf("create existing destination: %v", err)
	}
	service := New("", root)
	if err := service.CloneRepository("https://github.com/donomii/j", destination); err == nil {
		t.Fatal("clone accepted an existing destination")
	}
	if _, err := os.Stat(rootDirectory + "/" + destination); err != nil {
		t.Fatalf("existing destination changed after rejected clone: %v", err)
	}
}
