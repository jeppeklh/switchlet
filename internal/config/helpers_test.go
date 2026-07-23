package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}

	pathParts := append([]string{filepath.Dir(currentFile), "..", "..", "testdata"}, parts...)
	return filepath.Join(pathParts...)
}

func copyTree(t *testing.T, sourceRoot string, destinationRoot string) {
	t.Helper()

	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destinationRoot, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		return os.WriteFile(targetPath, contents, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture tree: %v", err)
	}
}

func writeFile(t *testing.T, rootDir string, relativePath string, contents string) string {
	t.Helper()

	fullPath := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create parent directories for %q: %v", fullPath, err)
	}

	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %q: %v", fullPath, err)
	}

	return fullPath
}

func stringPointer(value string) *string {
	return &value
}
