package editor

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTargetFile(t *testing.T, rootDir string, relativePath string, contents string) string {
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

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}

func decodeJSONRoot(t *testing.T, contents []byte) map[string]any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()

	var decodedRoot any
	if err := decoder.Decode(&decodedRoot); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	var extraValue any
	if err := decoder.Decode(&extraValue); err != io.EOF {
		if err == nil {
			t.Fatal("decode JSON: multiple JSON values are not allowed")
		}

		t.Fatalf("decode JSON: %v", err)
	}

	rootObject, ok := decodedRoot.(map[string]any)
	if !ok {
		t.Fatalf("decoded JSON root has type %T, want object", decodedRoot)
	}

	return rootObject
}

func containsTempFile(t *testing.T, directoryPath string, prefix string) bool {
	t.Helper()

	directoryEntries, err := os.ReadDir(directoryPath)
	if err != nil {
		t.Fatalf("read directory %q: %v", directoryPath, err)
	}

	for _, entry := range directoryEntries {
		if strings.HasPrefix(entry.Name(), prefix) {
			return true
		}
	}

	return false
}
