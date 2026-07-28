package editor

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestPreviewManagedTargetChanges_ReadsAndPreparesAllFormatsWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	jsonPath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"},"unrelated":"json-unrelated-secret"}`)
	dotenvPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\nUNRELATED=dotenv-unrelated-secret\n")
	yamlPath := writeTargetFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: http://old-queue.example.test\nunrelated: yaml-unrelated-secret\n")
	tomlPath := writeTargetFile(t, projectRoot, "services/development.toml", "[service]\nendpoint = \"http://old-service.example.test\"\nunrelated = \"toml-unrelated-secret\"\n")
	if err := os.Chmod(tomlPath, 0o600); err != nil {
		t.Fatalf("set TOML permissions: %v", err)
	}

	targetPaths := []string{jsonPath, dotenvPath, yamlPath, tomlPath}
	originalContents := map[string][]byte{}
	originalModes := map[string]fs.FileMode{}
	for _, targetPath := range targetPaths {
		originalContents[targetPath] = readFile(t, targetPath)
		info, err := os.Stat(targetPath)
		if err != nil {
			t.Fatalf("stat target file %q: %v", targetPath, err)
		}
		originalModes[targetPath] = info.Mode().Perm()
	}

	var writtenTargets []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		writtenTargets = append(writtenTargets, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	preview, err := PreviewManagedTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: jsonPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://new",
		},
		{
			Target: config.Target{Name: "frontendApi", File: dotenvPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			Value:  "https://api.example.test",
		},
		{
			Target: config.Target{Name: "workerQueue", File: yamlPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			Value:  "http://new-queue.example.test",
		},
		{
			Target: config.Target{Name: "serviceEndpoint", File: tomlPath, Type: config.TargetTypeTOML, TOMLPath: "service.endpoint"},
			Value:  "http://new-service.example.test",
		},
	})
	if err != nil {
		t.Fatalf("PreviewManagedTargetChanges returned error: %v", err)
	}

	wantFiles := []struct {
		path          string
		targetType    config.TargetType
		targetName    string
		selectorName  string
		selector      string
		originalValue string
		proposedValue string
	}{
		{jsonPath, config.TargetTypeJSON, "database", "jsonPath", "database.url", "postgres://old", "postgres://new"},
		{dotenvPath, config.TargetTypeDotenv, "frontendApi", "key", "VITE_API_URL", "http://localhost:5173", "https://api.example.test"},
		{yamlPath, config.TargetTypeYAML, "workerQueue", "yamlPath", "queue.endpoint", "http://old-queue.example.test", "http://new-queue.example.test"},
		{tomlPath, config.TargetTypeTOML, "serviceEndpoint", "tomlPath", "service.endpoint", "http://old-service.example.test", "http://new-service.example.test"},
	}
	if len(preview.Files) != len(wantFiles) {
		t.Fatalf("len(Files) = %d, want %d", len(preview.Files), len(wantFiles))
	}
	for index, wantFile := range wantFiles {
		filePreview := preview.Files[index]
		if filePreview.TargetFile != wantFile.path || filePreview.TargetType != wantFile.targetType || len(filePreview.Hunks) != 1 {
			t.Fatalf("Files[%d] = %#v, want one %s hunk for %q", index, filePreview, wantFile.targetType, wantFile.path)
		}
		hunk := filePreview.Hunks[0]
		if hunk.Target.Name != wantFile.targetName || hunk.SelectorName != wantFile.selectorName || hunk.Selector != wantFile.selector || hunk.OriginalValue != wantFile.originalValue || hunk.ProposedValue != wantFile.proposedValue {
			t.Fatalf("hunk[%d] = %#v, want %s %s from %q to %q", index, hunk, wantFile.targetName, wantFile.selector, wantFile.originalValue, wantFile.proposedValue)
		}
	}
	if len(writtenTargets) != 0 {
		t.Fatalf("written targets = %#v, want no writes during managed preview", writtenTargets)
	}
	for _, targetPath := range targetPaths {
		if !bytes.Equal(readFile(t, targetPath), originalContents[targetPath]) {
			t.Fatalf("target file %q changed during managed preview", targetPath)
		}
		info, err := os.Stat(targetPath)
		if err != nil {
			t.Fatalf("stat target file %q after preview: %v", targetPath, err)
		}
		if info.Mode().Perm() != originalModes[targetPath] {
			t.Fatalf("target file %q permissions = %o, want %o", targetPath, info.Mode().Perm(), originalModes[targetPath])
		}
		if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
			t.Fatalf("managed preview created a temporary file for %q", targetPath)
		}
	}
}

func TestPreviewManagedTargetChanges_MergesSameFileTargetsIntoOnePreviewFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "url": "postgres://old",
    "replica": "postgres://old-replica"
  },
  "unrelated": "unrelated-file-secret"
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	preview, err := PreviewManagedTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://new",
		},
		{
			Target: config.Target{Name: "replica", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.replica"},
			Value:  "postgres://new-replica",
		},
	})
	if err != nil {
		t.Fatalf("PreviewManagedTargetChanges returned error: %v", err)
	}

	if len(preview.Files) != 1 {
		t.Fatalf("len(Files) = %d, want one same-file preview", len(preview.Files))
	}
	filePreview := preview.Files[0]
	if filePreview.TargetFile != targetPath || filePreview.TargetType != config.TargetTypeJSON || len(filePreview.Hunks) != 2 {
		t.Fatalf("file preview = %#v, want two JSON hunks in one file", filePreview)
	}
	if filePreview.Hunks[0].Target.Name != "database" || filePreview.Hunks[0].OriginalValue != "postgres://old" || filePreview.Hunks[0].ProposedValue != "postgres://new" {
		t.Fatalf("first hunk = %#v, want database change", filePreview.Hunks[0])
	}
	if filePreview.Hunks[1].Target.Name != "replica" || filePreview.Hunks[1].OriginalValue != "postgres://old-replica" || filePreview.Hunks[1].ProposedValue != "postgres://new-replica" {
		t.Fatalf("second hunk = %#v, want replica change", filePreview.Hunks[1])
	}
	for _, hunk := range filePreview.Hunks {
		for _, previewValue := range []string{hunk.Selector, hunk.OriginalValue, hunk.ProposedValue} {
			if strings.Contains(previewValue, "unrelated-file-secret") {
				t.Fatalf("managed hunk exposed unrelated file content: %#v", hunk)
			}
		}
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during same-file managed preview")
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("same-file managed preview created a temporary file")
	}
}

func TestPreviewManagedTargetChanges_PreparationFailureReturnsNoPreviewOrWrites(t *testing.T) {
	projectRoot := t.TempDir()
	jsonPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://old"}}`)
	dotenvPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalJSONContents := readFile(t, jsonPath)
	originalDotenvContents := readFile(t, dotenvPath)

	var writtenTargets []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		writtenTargets = append(writtenTargets, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	preview, err := PreviewManagedTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: jsonPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://new-secret",
		},
		{
			Target: config.Target{Name: "frontendApi", File: dotenvPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			Value:  "https://secret.example.test\nNEXT=value",
		},
	})
	if err == nil {
		t.Fatal("PreviewManagedTargetChanges returned nil error, want dotenv preparation failure")
	}
	if len(preview.Files) != 0 {
		t.Fatalf("preview = %#v, want no preview after preparation failure", preview)
	}
	if !strings.Contains(err.Error(), `target "frontendApi"`) || !strings.Contains(err.Error(), `key "VITE_API_URL"`) || !strings.Contains(err.Error(), "replacement value must not contain newline characters") {
		t.Fatalf("PreviewManagedTargetChanges returned error %q, want target, selector, and newline context", err)
	}
	for _, forbidden := range []string{"postgres://new-secret", "https://secret.example.test"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("PreviewManagedTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if len(writtenTargets) != 0 {
		t.Fatalf("written targets = %#v, want no writes after preview preparation failure", writtenTargets)
	}
	if !bytes.Equal(readFile(t, jsonPath), originalJSONContents) {
		t.Fatal("JSON file changed after preview preparation failure")
	}
	if !bytes.Equal(readFile(t, dotenvPath), originalDotenvContents) {
		t.Fatal("dotenv file changed after preview preparation failure")
	}
	for _, targetPath := range []string{jsonPath, dotenvPath} {
		if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
			t.Fatalf("preview preparation failure created a temporary file for %q", targetPath)
		}
	}
}

func TestPreviewManagedTargetChanges_InvalidTargetReturnsNoPreviewOrWrites(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"password":"current-secret"}}`)
	originalContents := readFile(t, targetPath)

	preview, err := PreviewManagedTargetChanges([]TargetChange{{
		Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		Value:  "postgres://new-secret",
	}})
	if err == nil {
		t.Fatal("PreviewManagedTargetChanges returned nil error, want invalid target failure")
	}
	if len(preview.Files) != 0 {
		t.Fatalf("preview = %#v, want no preview after invalid target failure", preview)
	}
	if !strings.Contains(err.Error(), `target "database"`) || !strings.Contains(err.Error(), `jsonPath "database.url"`) || !strings.Contains(err.Error(), `missing segment "url"`) {
		t.Fatalf("PreviewManagedTargetChanges returned error %q, want target and selector context", err)
	}
	for _, forbidden := range []string{"current-secret", "postgres://new-secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("PreviewManagedTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed after invalid target preview failure")
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("invalid target preview failure created a temporary file")
	}
}
