package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

func TestApplicationApplyProfile_ReturnsStaleTargetFileErrorWithoutLeakingSecrets(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeAppTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://original-secret"}}`)
	externalContents := []byte(`{"database":{"url":"postgres://external-secret"}}`)

	application := New(
		config.Target{File: targetPath, JSONPath: "database.url"},
		[]config.Profile{{Name: "Local", Value: appStringPointer("postgres://replacement-secret")}},
	)
	application.applyTargetChanges = func(changes []editor.TargetChange) error {
		preparedChanges, err := editor.PrepareTargetChanges(changes)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, externalContents, 0o644); err != nil {
			return err
		}

		return preparedChanges.Write()
	}

	_, err := application.ApplyProfileByName("Local")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want stale-file failure")
	}
	if !strings.Contains(err.Error(), `apply profile "Local"`) {
		t.Fatalf("ApplyProfileByName returned error %q, want contextual profile error", err)
	}
	var staleErr editor.StaleTargetFileError
	if !errors.As(err, &staleErr) {
		t.Fatalf("ApplyProfileByName returned error %q, want StaleTargetFileError", err)
	}
	if staleErr.TargetFile != targetPath {
		t.Fatalf("StaleTargetFileError.TargetFile = %q, want %q", staleErr.TargetFile, targetPath)
	}
	for _, forbidden := range []string{"postgres://original-secret", "postgres://external-secret", "postgres://replacement-secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyProfileByName leaked value %q in error %q", forbidden, err)
		}
	}
	if !bytes.Equal(readAppFile(t, targetPath), externalContents) {
		t.Fatal("stale target file was overwritten during failed apply")
	}
	if containsAppTempFile(t, filepath.Dir(targetPath), ".config.json.switchlet-") {
		t.Fatal("stale apply left a temporary file behind")
	}
}

func writeAppTargetFile(t *testing.T, rootDir string, relativePath string, contents string) string {
	t.Helper()

	fullPath := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create parent directories for %q: %v", fullPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write target file %q: %v", fullPath, err)
	}

	return fullPath
}

func readAppFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}

func containsAppTempFile(t *testing.T, directory string, prefix string) bool {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read directory %q: %v", directory, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			return true
		}
	}

	return false
}

func appStringPointer(value string) *string {
	return &value
}
