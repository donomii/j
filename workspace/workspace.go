package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donomii/j/types"
)

func NewRoot(path string) (types.WorkspaceRoot, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path %q: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve workspace symlinks for %q: %w", abs, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect workspace %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory", resolved)
	}
	return types.WorkspaceRoot(filepath.Clean(resolved)), nil
}

func Resolve(root types.WorkspaceRoot, selectedPath string, mustExist bool) (string, error) {
	if selectedPath == "" {
		return "", fmt.Errorf("workspace path is empty")
	}
	if hasParentTraversal(selectedPath) {
		return "", fmt.Errorf("workspace path %q contains parent traversal", selectedPath)
	}
	rootPath := string(root)
	candidate := selectedPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootPath, candidate)
	}
	candidate = filepath.Clean(candidate)
	resolved, err := resolveExistingPrefix(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path %q: %w", selectedPath, err)
	}
	if !contains(rootPath, resolved) {
		return "", fmt.Errorf("path %q resolves outside approved workspace %q", selectedPath, rootPath)
	}
	if mustExist {
		if _, err := os.Stat(resolved); err != nil {
			return "", fmt.Errorf("required workspace path %q is unavailable: %w", selectedPath, err)
		}
	}
	return resolved, nil
}

func hasParentTraversal(path string) bool {
	for _, component := range strings.FieldsFunc(path, func(separator rune) bool { return separator == '/' || separator == '\\' }) {
		if component == ".." {
			return true
		}
	}
	return false
}

func Relative(root types.WorkspaceRoot, path string) (string, error) {
	resolved, err := Resolve(root, path, true)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(string(root), resolved)
	if err != nil {
		return "", fmt.Errorf("make workspace-relative path for %q: %w", resolved, err)
	}
	return relative, nil
}

func contains(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func resolveExistingPrefix(path string) (string, error) {
	existing := path
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", err
		}
		existing = parent
	}
	resolvedPrefix, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	suffix, err := filepath.Rel(existing, path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(resolvedPrefix, suffix)), nil
}
