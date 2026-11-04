package editor

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestApplyTargetChanges_UpdatesMultipleJSONFiles(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=old;Database=App;"
  }
}
`)+"\n")
	frontendPath := writeTargetFile(t, projectRoot, "frontend/config.json", strings.TrimSpace(`
{
  "api": {
    "url": "https://old.example.test"
  }
}
`)+"\n")

	changes := []TargetChange{
		{
			Target: config.Target{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
			Value:  "Server=new;Database=App;",
		},
		{
			Target: config.Target{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://new.example.test",
		},
	}

	if err := ApplyTargetChanges(changes); err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	databaseRoot := decodeJSONRoot(t, readFile(t, databasePath))
	connectionStrings := databaseRoot["ConnectionStrings"].(map[string]any)
	if connectionStrings["DefaultConnection"] != "Server=new;Database=App;" {
		t.Fatalf("DefaultConnection = %q, want updated value", connectionStrings["DefaultConnection"])
	}

	frontendRoot := decodeJSONRoot(t, readFile(t, frontendPath))
	api := frontendRoot["api"].(map[string]any)
	if api["url"] != "https://new.example.test" {
		t.Fatalf("api.url = %q, want updated value", api["url"])
	}
}

func TestApplyTargetChanges_SingleFileApplySucceedsAndPreservesPermissions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://old"}}`)
	if err := os.Chmod(targetPath, 0o600); err != nil {
		t.Fatalf("set target permissions: %v", err)
	}
	originalInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat target file: %v", err)
	}

	err = ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		Value:  "postgres://new",
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	root := decodeJSONRoot(t, readFile(t, targetPath))
	database := root["database"].(map[string]any)
	if database["url"] != "postgres://new" {
		t.Fatalf("database.url = %q, want updated value", database["url"])
	}
	updatedInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat updated target file: %v", err)
	}
	if updatedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("file permissions = %o, want %o", updatedInfo.Mode().Perm(), originalInfo.Mode().Perm())
	}
}

func TestApplyTargetChanges_MergesMultipleJSONTargetsInOneFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=old;Database=App;",
    "Reporting": "Server=old;Database=Reporting;"
  },
  "Feature": "unchanged"
}
`)+"\n")

	var writtenTargets []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		writtenTargets = append(writtenTargets, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	changes := []TargetChange{
		{
			Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
			Value:  "Server=new;Database=App;",
		},
		{
			Target: config.Target{Name: "reporting", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "ConnectionStrings.Reporting"},
			Value:  "Server=new;Database=Reporting;",
		},
	}

	if err := ApplyTargetChanges(changes); err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	if len(writtenTargets) != 1 || writtenTargets[0] != targetPath {
		t.Fatalf("written targets = %#v, want one write to %q", writtenTargets, targetPath)
	}

	rootObject := decodeJSONRoot(t, readFile(t, targetPath))
	connectionStrings := rootObject["ConnectionStrings"].(map[string]any)
	if connectionStrings["DefaultConnection"] != "Server=new;Database=App;" {
		t.Fatalf("DefaultConnection = %q, want updated value", connectionStrings["DefaultConnection"])
	}
	if connectionStrings["Reporting"] != "Server=new;Database=Reporting;" {
		t.Fatalf("Reporting = %q, want updated value", connectionStrings["Reporting"])
	}
	if rootObject["Feature"] != "unchanged" {
		t.Fatalf("Feature = %q, want unchanged", rootObject["Feature"])
	}
}

func TestApplyTargetChanges_RejectsDuplicateTargetLocationBeforeWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://old"}}`)
	originalContents := readFile(t, targetPath)

	var writtenTargets []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		writtenTargets = append(writtenTargets, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "primaryDatabase", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://secret-one",
		},
		{
			Target: config.Target{Name: "replicaDatabase", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://secret-two",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want duplicate location failure")
	}
	if !strings.Contains(err.Error(), `target "replicaDatabase"`) || !strings.Contains(err.Error(), `duplicates target location used by target "primaryDatabase"`) {
		t.Fatalf("ApplyTargetChanges returned error %q, want duplicate target-location context", err)
	}
	for _, forbidden := range []string{"secret-one", "secret-two"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if len(writtenTargets) != 0 {
		t.Fatalf("written targets = %#v, want no writes after duplicate location failure", writtenTargets)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed after duplicate target-location failure")
	}
}

func TestApplyTargetChanges_PreparationFailureLeavesEveryFileUnchangedAndHidesSecret(t *testing.T) {
	projectRoot := t.TempDir()
	validPath := writeTargetFile(t, projectRoot, "valid.json", `{"database":{"url":"postgres://old"}}`)
	invalidPath := writeTargetFile(t, projectRoot, "invalid.json", `{"api":{"baseUrl":"https://old.example.test"}}`)
	originalValidContents := readFile(t, validPath)
	originalInvalidContents := readFile(t, invalidPath)
	secretValue := "postgres://user:super-secret@example.test/app"

	changes := []TargetChange{
		{
			Target: config.Target{Name: "database", File: validPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  secretValue,
		},
		{
			Target: config.Target{Name: "api", File: invalidPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://new.example.test",
		},
	}

	err := ApplyTargetChanges(changes)
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want preparation failure")
	}
	if !strings.Contains(err.Error(), `target "api"`) || !strings.Contains(err.Error(), `jsonPath "api.url"`) {
		t.Fatalf("ApplyTargetChanges returned error %q, want target and selector context", err)
	}
	var targetErr TargetError
	if !errors.As(err, &targetErr) {
		t.Fatalf("ApplyTargetChanges returned error %q, want TargetError", err)
	}
	if targetErr.Target.Name != "api" || targetErr.Target.File != invalidPath || targetErr.Target.JSONPath != "api.url" {
		t.Fatalf("TargetError.Target = %#v, want api target context", targetErr.Target)
	}
	if targetErr.Err == nil || !strings.Contains(targetErr.Err.Error(), `missing segment "url"`) {
		t.Fatalf("TargetError.Err = %v, want underlying reason", targetErr.Err)
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
	}

	if !bytes.Equal(readFile(t, validPath), originalValidContents) {
		t.Fatal("valid target file changed after another file failed preparation")
	}
	if !bytes.Equal(readFile(t, invalidPath), originalInvalidContents) {
		t.Fatal("invalid target file changed after preparation failure")
	}
}

func TestReadTargetValue_ReturnsContextualErrorsWithoutLeakingCurrentValuesOrWriting(t *testing.T) {
	projectRoot := t.TempDir()
	missingPath := filepath.Join(projectRoot, "missing.json")

	tests := []struct {
		name           string
		relativePath   string
		targetContents string
		target         config.Target
		wantError      string
		secret         string
	}{
		{
			name:      "missing file",
			target:    config.Target{Name: "database", File: missingPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			wantError: "stat target file",
		},
		{
			name:           "invalid JSON",
			relativePath:   "invalid.json",
			targetContents: `{`,
			target:         config.Target{Name: "database", Type: config.TargetTypeJSON, JSONPath: "database.url"},
			wantError:      "contains invalid JSON",
		},
		{
			name:           "invalid dotenv",
			relativePath:   "frontend/.env.local",
			targetContents: "VITE_API_URL=http://localhost:5173\nnot an assignment\n",
			target:         config.Target{Name: "frontendApi", Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			wantError:      "not a supported KEY=value assignment",
		},
		{
			name:           "invalid YAML",
			relativePath:   "worker/config.yaml",
			targetContents: `{`,
			target:         config.Target{Name: "workerQueue", Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			wantError:      "contains invalid YAML",
		},
		{
			name:           "invalid TOML",
			relativePath:   "worker/config.toml",
			targetContents: `endpoint = "unterminated`,
			target:         config.Target{Name: "workerQueue", Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
			wantError:      "contains invalid TOML",
		},
		{
			name:           "missing JSON selector",
			relativePath:   "config.json",
			targetContents: `{"database":{"password":"super-secret-current"}}`,
			target:         config.Target{Name: "database", Type: config.TargetTypeJSON, JSONPath: "database.url"},
			wantError:      `missing segment "url"`,
			secret:         "super-secret-current",
		},
		{
			name:           "non-string JSON value",
			relativePath:   "number.json",
			targetContents: `{"database":{"url":42,"password":"super-secret-current"}}`,
			target:         config.Target{Name: "database", Type: config.TargetTypeJSON, JSONPath: "database.url"},
			wantError:      `JSON path "database.url" must resolve to a string`,
			secret:         "super-secret-current",
		},
		{
			name:           "duplicate dotenv key",
			relativePath:   "frontend/.env.local",
			targetContents: "VITE_API_URL=super-secret-current\nVITE_API_URL=other-secret-current\n",
			target:         config.Target{Name: "frontendApi", Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			wantError:      "dotenv key appears more than once",
			secret:         "super-secret-current",
		},
		{
			name:           "duplicate YAML key",
			relativePath:   "worker/config.yaml",
			targetContents: "queue:\n  endpoint: super-secret-current\n  endpoint: other-secret-current\n",
			target:         config.Target{Name: "workerQueue", Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			wantError:      `duplicate key "endpoint"`,
			secret:         "super-secret-current",
		},
		{
			name:           "non-string TOML value",
			relativePath:   "worker/config.toml",
			targetContents: "[queue]\nendpoint = 42\nsecret = \"super-secret-current\"\n",
			target:         config.Target{Name: "workerQueue", Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
			wantError:      `TOML path "queue.endpoint" must resolve to a string`,
			secret:         "super-secret-current",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var originalContents []byte
			if testCase.relativePath != "" {
				testCase.target.File = writeTargetFile(t, projectRoot, testCase.relativePath, testCase.targetContents)
				originalContents = readFile(t, testCase.target.File)
			}

			_, err := ReadTargetValue(testCase.target)
			if err == nil {
				t.Fatal("ReadTargetValue returned nil error, want target-read failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("ReadTargetValue returned error %q, want substring %q", err, testCase.wantError)
			}
			if !strings.Contains(err.Error(), `target "`+testCase.target.Name+`"`) || !strings.Contains(err.Error(), `type "`+string(testCase.target.Type)+`"`) {
				t.Fatalf("ReadTargetValue returned error %q, want target name and type context", err)
			}
			selectorName, selector := targetSelectorSummary(testCase.target)
			if !strings.Contains(err.Error(), selectorName+` "`+selector+`"`) {
				t.Fatalf("ReadTargetValue returned error %q, want selector context", err)
			}
			if testCase.secret != "" && strings.Contains(err.Error(), testCase.secret) {
				t.Fatalf("ReadTargetValue leaked current value %q in error %q", testCase.secret, err)
			}

			if testCase.relativePath != "" {
				if !bytes.Equal(readFile(t, testCase.target.File), originalContents) {
					t.Fatal("target file changed during failed current-value read")
				}
				if containsTempFile(t, filepath.Dir(testCase.target.File), tempFilePrefix(testCase.target.File)) {
					t.Fatal("failed current-value read created a temporary file")
				}
			}
		})
	}
}

func TestReadTargetValues_ReadsSameFileGroupOnce(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "url": "postgres://current",
    "replica": "postgres://replica"
  }
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	readCount := 0
	originalReadTargetFile := readTargetFile
	readTargetFile = func(targetPath string) ([]byte, fs.FileInfo, error) {
		readCount++
		return originalReadTargetFile(targetPath)
	}
	t.Cleanup(func() {
		readTargetFile = originalReadTargetFile
	})

	values, err := ReadTargetValues([]config.Target{
		{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		{Name: "replica", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.replica"},
	})
	if err != nil {
		t.Fatalf("ReadTargetValues returned error: %v", err)
	}

	if readCount != 1 {
		t.Fatalf("readCount = %d, want one grouped file read", readCount)
	}
	if values["database"] != "postgres://current" || values["replica"] != "postgres://replica" {
		t.Fatalf("values = %#v, want both current values", values)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during grouped current-value read")
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("grouped current-value read created a temporary file")
	}
}

func TestReadTargetValues_ReturnsTargetSpecificErrorsWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://current","secret":"current-secret"}}`)
	originalContents := readFile(t, targetPath)

	_, err := ReadTargetValues([]config.Target{
		{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		{Name: "missing", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.missing"},
	})
	if err == nil {
		t.Fatal("ReadTargetValues returned nil error, want target-specific failure")
	}
	if !strings.Contains(err.Error(), `target "missing"`) || !strings.Contains(err.Error(), `jsonPath "database.missing"`) || !strings.Contains(err.Error(), `missing segment "missing"`) {
		t.Fatalf("ReadTargetValues returned error %q, want missing target context", err)
	}
	if strings.Contains(err.Error(), "current-secret") {
		t.Fatalf("ReadTargetValues leaked current value in error %q", err)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during failed grouped current-value read")
	}
}

func TestReadTargetValue_RejectsDirectoryTargetPathWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetDirectory := filepath.Join(projectRoot, "config")
	if err := os.MkdirAll(targetDirectory, 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}

	_, err := ReadTargetValue(config.Target{
		Name:     "database",
		File:     targetDirectory,
		Type:     config.TargetTypeJSON,
		JSONPath: "database.url",
	})
	if err == nil {
		t.Fatal("ReadTargetValue returned nil error, want directory target-path failure")
	}
	for _, expected := range []string{`target "database"`, `jsonPath "database.url"`, `is a directory`} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ReadTargetValue returned error %q, want substring %q", err, expected)
		}
	}
	if containsTempFile(t, filepath.Dir(targetDirectory), tempFilePrefix(targetDirectory)) {
		t.Fatal("directory target-path read created a temporary file")
	}
}

func TestReadTargetValue_RejectsSymlinkTargetPathWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	realTargetPath := writeTargetFile(t, projectRoot, "config/runtime.json", `{"database":{"url":"postgres://current"}}`)
	linkPath := createSymlinkOrSkip(t, projectRoot, "config/runtime-link.json", realTargetPath)
	originalContents := readFile(t, realTargetPath)

	_, err := ReadTargetValue(config.Target{
		Name:     "database",
		File:     linkPath,
		Type:     config.TargetTypeJSON,
		JSONPath: "database.url",
	})
	if err == nil {
		t.Fatal("ReadTargetValue returned nil error, want symlink target-path failure")
	}
	for _, expected := range []string{`target "database"`, `jsonPath "database.url"`, `is a symlink`} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ReadTargetValue returned error %q, want substring %q", err, expected)
		}
	}
	if !bytes.Equal(readFile(t, realTargetPath), originalContents) {
		t.Fatal("symlink destination changed during failed current-value read")
	}
	if containsTempFile(t, filepath.Dir(linkPath), tempFilePrefix(linkPath)) {
		t.Fatal("symlink target-path read created a temporary file")
	}
	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat symlink path: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink path mode = %v, want symlink", linkInfo.Mode())
	}
}

func TestReadTargetValues_RejectsMixedTargetTypesInOneFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://current"},"queue":{"endpoint":"current"}}`)
	originalContents := readFile(t, targetPath)

	_, err := ReadTargetValues([]config.Target{
		{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
	})
	if err == nil {
		t.Fatal("ReadTargetValues returned nil error, want mixed target-type failure")
	}
	if !strings.Contains(err.Error(), `target "workerQueue"`) || !strings.Contains(err.Error(), `target file already has "json" reads queued`) {
		t.Fatalf("ReadTargetValues returned error %q, want mixed target-type context", err)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed after mixed target-type read failure")
	}
}

func TestPreviewTargetChanges_ReturnsJSONValidationErrorsWithoutWriting(t *testing.T) {
	tests := []struct {
		name           string
		targetContents string
		jsonPath       string
		wantError      string
	}{
		{
			name:           "invalid JSON",
			targetContents: `{`,
			jsonPath:       "database.url",
			wantError:      "contains invalid JSON",
		},
		{
			name:           "missing JSON path",
			targetContents: `{"database":{"url":"postgres://old"}}`,
			jsonPath:       "database.primary.url",
			wantError:      `missing segment "primary"`,
		},
		{
			name:           "non-string JSON target",
			targetContents: `{"database":{"url":42}}`,
			jsonPath:       "database.url",
			wantError:      `JSON path "database.url" must resolve to a string`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "config.json", testCase.targetContents)
			originalContents := readFile(t, targetPath)

			err := PreviewTargetChanges([]TargetChange{{
				Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: testCase.jsonPath},
				Value:  "postgres://secret-value",
			}})
			if err == nil {
				t.Fatal("PreviewTargetChanges returned nil error, want validation failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("PreviewTargetChanges returned error %q, want substring %q", err, testCase.wantError)
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("PreviewTargetChanges leaked secret in error %q", err)
			}

			if !bytes.Equal(readFile(t, targetPath), originalContents) {
				t.Fatal("target file changed during preview")
			}
		})
	}
}

func TestApplyTargetChanges_RenameFailureLeavesOriginalFileIntactAndCleansTemporaryFiles(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://old"}}`)
	originalContents := readFile(t, targetPath)

	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		return errors.New("rename failed")
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		Value:  "postgres://new",
	}})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want rename failure")
	}
	if !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("ApplyTargetChanges returned error %q, want rename failure", err)
	}

	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed after rename failure")
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("temporary file was not cleaned up after rename failure")
	}
}

func TestApplyTargetChanges_PreflightFailurePreventsEveryReplacementAndCleansTemporaryFiles(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/config.json", `{"api":{"url":"https://old.example.test"}}`)
	originalDatabaseContents := readFile(t, databasePath)
	originalFrontendContents := readFile(t, frontendPath)

	var writtenTargets []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		writtenTargets = append(writtenTargets, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	originalSyncFile := syncFile
	syncFile = func(file *os.File) error {
		if filepath.Dir(file.Name()) == filepath.Dir(frontendPath) {
			return errors.New("preflight sync failed")
		}

		return originalSyncFile(file)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
		syncFile = originalSyncFile
	})

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://new-secret",
		},
		{
			Target: config.Target{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://secret-value.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want preflight failure")
	}
	var preflightErr PreflightError
	if !errors.As(err, &preflightErr) {
		t.Fatalf("ApplyTargetChanges returned error %q, want PreflightError", err)
	}
	if preflightErr.TargetFile != frontendPath {
		t.Fatalf("PreflightError.TargetFile = %q, want %q", preflightErr.TargetFile, frontendPath)
	}
	for _, expected := range []string{"preflight target file", "preflight sync failed"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, expected)
		}
	}
	for _, forbidden := range []string{"new-secret", "secret-value"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if len(writtenTargets) != 0 {
		t.Fatalf("writtenTargets = %#v, want no replacements after preflight failure", writtenTargets)
	}
	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after preflight failure")
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed after preflight failure")
	}
	for _, targetPath := range []string{databasePath, frontendPath} {
		if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
			t.Fatalf("preflight temporary file for %q was not cleaned up", targetPath)
		}
	}
}

func TestPreparedChangesWrite_RejectsSymlinkTargetPathBeforeReplacement(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config/runtime.json", `{"database":{"url":"postgres://old"}}`)
	preparedChanges, err := PrepareTargetChanges([]TargetChange{{
		Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		Value:  "postgres://new",
	}})
	if err != nil {
		t.Fatalf("PrepareTargetChanges returned error: %v", err)
	}

	replacementTargetPath := writeTargetFile(t, projectRoot, "config/replacement.json", `{"database":{"url":"postgres://replacement"}}`)
	originalReplacementContents := readFile(t, replacementTargetPath)
	if err := os.Remove(targetPath); err != nil {
		t.Fatalf("remove original target file: %v", err)
	}
	if err := os.Symlink(replacementTargetPath, targetPath); err != nil {
		t.Skipf("symlink creation not supported in this environment: %v", err)
	}

	err = preparedChanges.Write()
	if err == nil {
		t.Fatal("PreparedChanges.Write returned nil error, want symlink preflight failure")
	}
	var preflightErr PreflightError
	if !errors.As(err, &preflightErr) {
		t.Fatalf("PreparedChanges.Write returned error %q, want PreflightError", err)
	}
	if preflightErr.TargetFile != targetPath {
		t.Fatalf("PreflightError.TargetFile = %q, want %q", preflightErr.TargetFile, targetPath)
	}
	if !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("PreparedChanges.Write returned error %q, want symlink failure", err)
	}
	if !bytes.Equal(readFile(t, replacementTargetPath), originalReplacementContents) {
		t.Fatal("symlink destination changed after prepared write preflight failure")
	}
	linkInfo, err := os.Lstat(targetPath)
	if err != nil {
		t.Fatalf("lstat target path: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target path mode = %v, want symlink", linkInfo.Mode())
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("symlink preflight failure left a temporary file behind")
	}
}

func TestPreparedChangesWrite_RejectsTargetFileChangedAfterPreparationWithoutLeakingValues(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config/runtime.json", `{"database":{"url":"postgres://original-secret"}}`)
	preparedChanges, err := PrepareTargetChanges([]TargetChange{{
		Target: config.Target{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
		Value:  "postgres://replacement-secret",
	}})
	if err != nil {
		t.Fatalf("PrepareTargetChanges returned error: %v", err)
	}

	externalContents := []byte(`{"database":{"url":"postgres://external-secret"}}`)
	if err := os.WriteFile(targetPath, externalContents, 0o644); err != nil {
		t.Fatalf("overwrite target file after preparation: %v", err)
	}

	err = preparedChanges.Write()
	if err == nil {
		t.Fatal("PreparedChanges.Write returned nil error, want stale-file failure")
	}
	var staleErr StaleTargetFileError
	if !errors.As(err, &staleErr) {
		t.Fatalf("PreparedChanges.Write returned error %q, want StaleTargetFileError", err)
	}
	if staleErr.TargetFile != targetPath {
		t.Fatalf("StaleTargetFileError.TargetFile = %q, want %q", staleErr.TargetFile, targetPath)
	}
	if staleErr.Err != nil {
		t.Fatalf("StaleTargetFileError.Err = %v, want nil byte-mismatch reason", staleErr.Err)
	}
	for _, forbidden := range []string{"postgres://original-secret", "postgres://external-secret", "postgres://replacement-secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("PreparedChanges.Write leaked value %q in error %q", forbidden, err)
		}
	}
	if !bytes.Equal(readFile(t, targetPath), externalContents) {
		t.Fatal("stale target file was overwritten after preparation mismatch")
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("stale-file failure left a temporary file behind")
	}
}

func TestApplyTargetChanges_WriteFailureRestoresEarlierFilesInReverseOrder(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/config.json", `{"api":{"url":"https://old.example.test"}}`)
	workerPath := writeTargetFile(t, projectRoot, "worker/config.json", `{"queue":{"url":"https://queue.old.example.test"}}`)
	originalContents := map[string][]byte{
		databasePath: readFile(t, databasePath),
		frontendPath: readFile(t, frontendPath),
		workerPath:   readFile(t, workerPath),
	}
	if err := os.Chmod(databasePath, 0o600); err != nil {
		t.Fatalf("set database permissions: %v", err)
	}
	originalDatabaseInfo, err := os.Stat(databasePath)
	if err != nil {
		t.Fatalf("stat database target: %v", err)
	}

	var replacementOrder []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		replacementOrder = append(replacementOrder, newPath)
		if newPath == workerPath {
			return errors.New("worker rename failed")
		}

		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err = ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://new-secret",
		},
		{
			Target: config.Target{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://secret-value.example.test",
		},
		{
			Target: config.Target{Name: "workerQueue", File: workerPath, Type: config.TargetTypeJSON, JSONPath: "queue.url"},
			Value:  "https://queue.secret.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want third-file write failure")
	}
	var recoveryErr RecoveryError
	if !errors.As(err, &recoveryErr) {
		t.Fatalf("ApplyTargetChanges returned error %q, want RecoveryError", err)
	}
	if recoveryErr.FailedFile != workerPath {
		t.Fatalf("RecoveryError.FailedFile = %q, want %q", recoveryErr.FailedFile, workerPath)
	}
	wantRestored := []string{frontendPath, databasePath}
	if !stringSlicesEqual(recoveryErr.RestoredFiles, wantRestored) {
		t.Fatalf("RecoveryError.RestoredFiles = %#v, want %#v", recoveryErr.RestoredFiles, wantRestored)
	}
	if len(recoveryErr.UnrestoredFiles) != 0 {
		t.Fatalf("RecoveryError.UnrestoredFiles = %#v, want none", recoveryErr.UnrestoredFiles)
	}
	wantReplacementOrder := []string{databasePath, frontendPath, workerPath, frontendPath, databasePath}
	if !stringSlicesEqual(replacementOrder, wantReplacementOrder) {
		t.Fatalf("replacementOrder = %#v, want reverse restoration order %#v", replacementOrder, wantReplacementOrder)
	}
	for _, expected := range []string{"after 2 file(s) were already replaced", "restored prior replacements", "worker rename failed"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, expected)
		}
	}
	for _, forbidden := range []string{"new-secret", "secret-value", "queue.secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	for targetPath, originalContent := range originalContents {
		if !bytes.Equal(readFile(t, targetPath), originalContent) {
			t.Fatalf("target file %q was not restored to original contents", targetPath)
		}
		if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
			t.Fatalf("temporary file for %q was not cleaned up", targetPath)
		}
	}
	updatedDatabaseInfo, err := os.Stat(databasePath)
	if err != nil {
		t.Fatalf("stat restored database target: %v", err)
	}
	if updatedDatabaseInfo.Mode().Perm() != originalDatabaseInfo.Mode().Perm() {
		t.Fatalf("database permissions = %o, want restored permissions %o", updatedDatabaseInfo.Mode().Perm(), originalDatabaseInfo.Mode().Perm())
	}
}

func TestPreparedChangesWrite_StaleLaterFileRestoresEarlierReplacements(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/config.json", `{"api":{"url":"https://old.example.test"}}`)
	originalDatabaseContents := readFile(t, databasePath)

	preparedChanges, err := PrepareTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://replacement-secret",
		},
		{
			Target: config.Target{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://replacement-secret.example.test",
		},
	})
	if err != nil {
		t.Fatalf("PrepareTargetChanges returned error: %v", err)
	}

	externalFrontendContents := []byte(`{"api":{"url":"https://external-secret.example.test"}}`)
	if err := os.WriteFile(frontendPath, externalFrontendContents, 0o644); err != nil {
		t.Fatalf("overwrite later target file after preparation: %v", err)
	}

	var replacementOrder []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		replacementOrder = append(replacementOrder, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err = preparedChanges.Write()
	if err == nil {
		t.Fatal("PreparedChanges.Write returned nil error, want stale later-file failure")
	}
	var recoveryErr RecoveryError
	if !errors.As(err, &recoveryErr) {
		t.Fatalf("PreparedChanges.Write returned error %q, want RecoveryError", err)
	}
	if recoveryErr.FailedFile != frontendPath {
		t.Fatalf("RecoveryError.FailedFile = %q, want %q", recoveryErr.FailedFile, frontendPath)
	}
	if !stringSlicesEqual(recoveryErr.RestoredFiles, []string{databasePath}) {
		t.Fatalf("RecoveryError.RestoredFiles = %#v, want %#v", recoveryErr.RestoredFiles, []string{databasePath})
	}
	if len(recoveryErr.UnrestoredFiles) != 0 {
		t.Fatalf("RecoveryError.UnrestoredFiles = %#v, want none", recoveryErr.UnrestoredFiles)
	}
	var staleErr StaleTargetFileError
	if !errors.As(err, &staleErr) {
		t.Fatalf("PreparedChanges.Write returned error %q, want stale cause", err)
	}
	if staleErr.TargetFile != frontendPath {
		t.Fatalf("StaleTargetFileError.TargetFile = %q, want %q", staleErr.TargetFile, frontendPath)
	}
	for _, forbidden := range []string{"postgres://replacement-secret", "https://replacement-secret.example.test", "https://external-secret.example.test"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("PreparedChanges.Write leaked value %q in error %q", forbidden, err)
		}
	}
	if !stringSlicesEqual(replacementOrder, []string{databasePath, databasePath}) {
		t.Fatalf("replacementOrder = %#v, want first replacement plus restoration", replacementOrder)
	}
	if !bytes.Equal(readFile(t, databasePath), originalDatabaseContents) {
		t.Fatal("earlier replaced target file was not restored after stale later file")
	}
	if !bytes.Equal(readFile(t, frontendPath), externalFrontendContents) {
		t.Fatal("stale later target file should remain on-disk external contents")
	}
	for _, targetPath := range []string{databasePath, frontendPath} {
		if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
			t.Fatalf("temporary file for %q was not cleaned up", targetPath)
		}
	}
}

func TestApplyTargetChanges_FailedRestorationReportsUncertainPartialState(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/config.json", `{"api":{"url":"https://old.example.test"}}`)
	originalFrontendContents := readFile(t, frontendPath)

	databaseReplacementCount := 0
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		if newPath == frontendPath {
			return errors.New("frontend rename failed")
		}
		if newPath == databasePath {
			databaseReplacementCount++
			if databaseReplacementCount == 2 {
				return errors.New("database restore failed")
			}
		}

		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://new-secret",
		},
		{
			Target: config.Target{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://secret-value.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want restoration failure")
	}
	var recoveryErr RecoveryError
	if !errors.As(err, &recoveryErr) {
		t.Fatalf("ApplyTargetChanges returned error %q, want RecoveryError", err)
	}
	if !stringSlicesEqual(recoveryErr.UnrestoredFiles, []string{databasePath}) {
		t.Fatalf("RecoveryError.UnrestoredFiles = %#v, want database", recoveryErr.UnrestoredFiles)
	}
	if recoveryErr.RestoreErr == nil || !strings.Contains(recoveryErr.RestoreErr.Error(), "database restore failed") {
		t.Fatalf("RecoveryError.RestoreErr = %v, want restore failure", recoveryErr.RestoreErr)
	}
	for _, expected := range []string{"restoration failed", "target files may now be partially updated", "frontend rename failed"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, expected)
		}
	}
	for _, forbidden := range []string{"new-secret", "secret-value"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}

	databaseRoot := decodeJSONRoot(t, readFile(t, databasePath))
	database := databaseRoot["database"].(map[string]any)
	if database["url"] != "postgres://new-secret" {
		t.Fatalf("database.url = %q, want updated value after failed restoration", database["url"])
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("second target file changed after its replacement failed")
	}
	for _, targetPath := range []string{databasePath, frontendPath} {
		if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
			t.Fatalf("temporary file for %q was not cleaned up", targetPath)
		}
	}
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

func TestApplyTargetChanges_DirectorySyncFailureRestoresEveryReplacedFile(t *testing.T) {
	for _, fileCount := range []int{1, 2} {
		t.Run(fmt.Sprintf("%d files", fileCount), func(t *testing.T) {
			root := t.TempDir()
			changes := make([]TargetChange, 0, fileCount)
			for index := 0; index < fileCount; index++ {
				path := writeTargetFile(t, root, fmt.Sprintf("%d.json", index), `{"url":"old"}`)
				changes = append(changes, TargetChange{Target: config.Target{Name: fmt.Sprint(index), File: path, Type: config.TargetTypeJSON, JSONPath: "url"}, Value: "new"})
			}
			originalSync := syncDirectory
			t.Cleanup(func() { syncDirectory = originalSync })
			calls := 0
			syncFailure := errors.New("directory sync failed after replacement")
			syncDirectory = func(path string) error {
				calls++
				// Every file is preflighted before replacements begin.
				if calls == 2*fileCount {
					return syncFailure
				}
				return originalSync(path)
			}

			err := ApplyTargetChanges(changes)
			var recovery RecoveryError
			if !errors.As(err, &recovery) || !errors.Is(err, syncFailure) {
				t.Fatalf("apply error = %v, want recoverable directory sync failure", err)
			}
			if len(recovery.ReplacedFiles) != fileCount || len(recovery.RestoredFiles) != fileCount || len(recovery.UnrestoredFiles) != 0 {
				t.Fatalf("recovery = %#v, want all replacements restored", recovery)
			}
			for _, change := range changes {
				if got := string(readFile(t, change.Target.File)); got != `{"url":"old"}` {
					t.Fatalf("file contents = %q, want original contents", got)
				}
				if containsTempFile(t, root, tempFilePrefix(change.Target.File)) {
					t.Fatal("temporary file remains after recovery")
				}
			}
		})
	}
}

func TestApplyTargetChanges_PostReplacementSyncFailureReportsFailedRestoration(t *testing.T) {
	root := t.TempDir()
	path := writeTargetFile(t, root, "config.json", `{"url":"old"}`)
	originalSync, originalReplace := syncDirectory, replaceFile
	t.Cleanup(func() { syncDirectory, replaceFile = originalSync, originalReplace })
	replaced := false
	replaceFile = func(source, target string) error {
		if replaced {
			return errors.New("restoration failed")
		}
		if err := originalReplace(source, target); err != nil {
			return err
		}
		replaced = true
		return nil
	}
	syncDirectory = func(directory string) error {
		if replaced {
			return errors.New("directory sync failed")
		}
		return originalSync(directory)
	}
	err := ApplyTargetChanges([]TargetChange{{Target: config.Target{Name: "url", File: path, Type: config.TargetTypeJSON, JSONPath: "url"}, Value: "new"}})
	var recovery RecoveryError
	if !errors.As(err, &recovery) || len(recovery.UnrestoredFiles) != 1 || recovery.UnrestoredFiles[0] != path {
		t.Fatalf("apply error = %v, want failed file included in uncertain recovery", err)
	}
	if !strings.Contains(err.Error(), "partially updated") || string(readFile(t, path)) != `{"url":"new"}` {
		t.Fatalf("error = %v, want truthful partial-state report", err)
	}
}

func TestApplyTargetChanges_AlreadyMatchingValuesDoNotTouchFiles(t *testing.T) {
	tests := []struct {
		name, contents string
		target         config.Target
	}{
		{"json", "{  \"url\" : \"\\u006fld\" }\r\n", config.Target{Type: config.TargetTypeJSON, JSONPath: "url"}},
		{"yaml", "service:\r\n    url: 'old'  # unchanged\r\n", config.Target{Type: config.TargetTypeYAML, YAMLPath: "service.url"}},
		{"toml", "url = 'old' # unchanged\r\n", config.Target{Type: config.TargetTypeTOML, TOMLPath: "url"}},
		{"dotenv", "URL='old'\r\n", config.Target{Type: config.TargetTypeDotenv, Key: "URL"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.target.File = writeTargetFile(t, root, "config", test.contents)
			test.target.Name = "url"
			before, err := os.Stat(test.target.File)
			if err != nil {
				t.Fatal(err)
			}
			originalSync := syncFile
			t.Cleanup(func() { syncFile = originalSync })
			syncFile = func(*os.File) error {
				t.Fatal("already matching apply attempted a preflight or file write")
				return nil
			}
			if err := ApplyTargetChanges([]TargetChange{{Target: test.target, Value: "old"}}); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(test.target.File)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || string(readFile(t, test.target.File)) != test.contents {
				t.Fatal("already matching apply changed file identity, modification time, or bytes")
			}
		})
	}
}

func TestPreparedChangesWrite_AlreadyMatchingFileStillRejectsStaleContents(t *testing.T) {
	root := t.TempDir()
	path := writeTargetFile(t, root, "config.json", `{"url":"old"}`)
	prepared, err := PrepareTargetChanges([]TargetChange{{Target: config.Target{Name: "url", File: path, Type: config.TargetTypeJSON, JSONPath: "url"}, Value: "old"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"url":"external"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stale StaleTargetFileError
	if err := prepared.Write(); !errors.As(err, &stale) {
		t.Fatalf("write error = %v, want stale-file refusal", err)
	}
	if string(readFile(t, path)) != `{"url":"external"}` {
		t.Fatal("external edit was overwritten")
	}
}

func TestApplyTargetChanges_MixedChangesPreserveMatchingFilesAndTokens(t *testing.T) {
	root := t.TempDir()
	matching := writeTargetFile(t, root, "matching.yaml", "url: old # keep\n")
	changed := writeTargetFile(t, root, "changed.json", `{"keep":"\u006fld","url":"old"}`)
	originalReplace := replaceFile
	t.Cleanup(func() { replaceFile = originalReplace })
	var replacements []string
	replaceFile = func(source, target string) error {
		replacements = append(replacements, target)
		return originalReplace(source, target)
	}
	if err := ApplyTargetChanges([]TargetChange{
		{Target: config.Target{Name: "matching", File: matching, Type: config.TargetTypeYAML, YAMLPath: "url"}, Value: "old"},
		{Target: config.Target{Name: "keep", File: changed, Type: config.TargetTypeJSON, JSONPath: "keep"}, Value: "old"},
		{Target: config.Target{Name: "url", File: changed, Type: config.TargetTypeJSON, JSONPath: "url"}, Value: "new"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(replacements) != 1 || replacements[0] != changed {
		t.Fatalf("replacements = %v, want changed file only", replacements)
	}
	if string(readFile(t, changed)) != `{"keep":"\u006fld","url":"new"}` {
		t.Fatal("unchanged JSON string token was rewritten")
	}
}

func TestApplyTargetChanges_SkippedFileIsNotIncludedInRecovery(t *testing.T) {
	root := t.TempDir()
	matching := writeTargetFile(t, root, "matching.json", `{"url":"old"}`)
	changed := writeTargetFile(t, root, "changed.json", `{"url":"old"}`)
	originalReplace := replaceFile
	t.Cleanup(func() { replaceFile = originalReplace })
	replaceFile = func(source, target string) error {
		if target != changed {
			t.Fatal("attempted to replace or restore an unchanged file")
		}
		return errors.New("replacement failed")
	}
	err := ApplyTargetChanges([]TargetChange{
		{Target: config.Target{Name: "matching", File: matching, Type: config.TargetTypeJSON, JSONPath: "url"}, Value: "old"},
		{Target: config.Target{Name: "changed", File: changed, Type: config.TargetTypeJSON, JSONPath: "url"}, Value: "new"},
	})
	var recovery RecoveryError
	if err == nil || errors.As(err, &recovery) {
		t.Fatalf("error = %v, want write failure without recovery", err)
	}
	if string(readFile(t, matching)) != `{"url":"old"}` || string(readFile(t, changed)) != `{"url":"old"}` {
		t.Fatal("file changed despite failed first replacement")
	}
}

func TestApplyTargetChanges_TypedScalarsRoundTripAndPreserveSurroundingBytes(t *testing.T) {
	for _, format := range []config.TargetType{config.TargetTypeJSON, config.TargetTypeYAML, config.TargetTypeTOML, config.TargetTypeDotenv} {
		t.Run(string(format), func(t *testing.T) {
			root := t.TempDir()
			sources := map[config.TargetType]string{
				config.TargetTypeJSON:   "{ \"flag\": true, \"port\": 1, \"ratio\": 0.5, \"keep\": \"8080\" }\r\n",
				config.TargetTypeYAML:   "flag: true # keep\r\nport: 1\r\nratio: 0.5\r\nkeep: '8080'\r\n",
				config.TargetTypeTOML:   "flag = true # keep\r\nport = 1\r\nratio = 0.5\r\nkeep = '8080'\r\n",
				config.TargetTypeDotenv: "flag=true # keep\r\nport=1\r\nratio=0.5\r\nkeep='8080'\r\n",
			}
			path := writeTargetFile(t, root, "settings", sources[format])
			changes := []TargetChange{}
			for index, kind := range []config.ValueType{config.ValueTypeBoolean, config.ValueTypeInteger, config.ValueTypeNumber} {
				name := []string{"flag", "port", "ratio"}[index]
				target := config.Target{Name: name, File: path, Type: format, ValueType: kind}
				switch format {
				case config.TargetTypeJSON:
					target.JSONPath = name
				case config.TargetTypeYAML:
					target.YAMLPath = name
				case config.TargetTypeTOML:
					target.TOMLPath = name
				case config.TargetTypeDotenv:
					target.Key = name
				}
				changes = append(changes, TargetChange{Target: target, Value: []string{"false", "0", "0.12345678901234567890123456789"}[index]})
			}
			if err := ApplyTargetChanges(changes); err != nil {
				t.Fatal(err)
			}
			for _, change := range changes {
				actual, err := ReadTargetValue(change.Target)
				if err != nil || actual != change.Value {
					t.Fatalf("value=%q error=%v", actual, err)
				}
			}
			got := string(readFile(t, path))
			if !strings.Contains(got, "keep") || !strings.Contains(got, "8080") || !strings.Contains(got, "\r\n") {
				t.Fatalf("surrounding content lost: %q", got)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := ApplyTargetChanges(changes); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
				t.Fatal("matching typed apply rewrote file")
			}
		})
	}
}

func TestApplyTargetChanges_TypedValidationFailurePreventsEveryWrite(t *testing.T) {
	root := t.TempDir()
	good := writeTargetFile(t, root, "good.json", `{"flag":true}`)
	bad := writeTargetFile(t, root, "bad.yaml", "port: '8080'\n")
	err := ApplyTargetChanges([]TargetChange{
		{Target: config.Target{Name: "flag", File: good, Type: config.TargetTypeJSON, JSONPath: "flag", ValueType: config.ValueTypeBoolean}, Value: "false"},
		{Target: config.Target{Name: "port", File: bad, Type: config.TargetTypeYAML, YAMLPath: "port", ValueType: config.ValueTypeInteger}, Value: "0"},
	})
	if err == nil {
		t.Fatal("accepted string in an integer target")
	}
	if string(readFile(t, good)) != `{"flag":true}` || string(readFile(t, bad)) != "port: '8080'\n" {
		t.Fatal("validation failure changed files")
	}
}

func TestApplyTargetChanges_EquivalentNumbersDoNotRewriteSource(t *testing.T) {
	root := t.TempDir()
	path := writeTargetFile(t, root, "settings.json", `{"number":1.250e2}`)
	if err := ApplyTargetChanges([]TargetChange{{Target: config.Target{Name: "number", File: path, Type: config.TargetTypeJSON, JSONPath: "number", ValueType: config.ValueTypeNumber}, Value: "125.0"}}); err != nil {
		t.Fatal(err)
	}
	if string(readFile(t, path)) != `{"number":1.250e2}` {
		t.Fatal("equivalent numeric spelling was rewritten")
	}
}

func TestReadTargetValue_TypedNativeNumericSpellings(t *testing.T) {
	for _, test := range []struct {
		format   config.TargetType
		contents string
		kind     config.ValueType
		want     string
	}{
		{config.TargetTypeYAML, "value: 0x10\n", config.ValueTypeInteger, "16"},
		{config.TargetTypeYAML, "value: .5\n", config.ValueTypeNumber, "0.5"},
		{config.TargetTypeYAML, "value: TRUE\n", config.ValueTypeBoolean, "true"},
		{config.TargetTypeTOML, "value = +1_000\n", config.ValueTypeInteger, "1000"},
		{config.TargetTypeTOML, "value = 0x10\n", config.ValueTypeInteger, "16"},
		{config.TargetTypeTOML, "value = +1.25\n", config.ValueTypeNumber, "1.25"},
	} {
		t.Run(test.contents, func(t *testing.T) {
			path := writeTargetFile(t, t.TempDir(), "settings", test.contents)
			target := config.Target{Name: "value", File: path, Type: test.format, ValueType: test.kind, YAMLPath: "value"}
			if test.format == config.TargetTypeTOML {
				target.YAMLPath = ""
				target.TOMLPath = "value"
			}
			value, err := ReadTargetValue(target)
			if err != nil || value != test.want {
				t.Fatalf("value=%q error=%v", value, err)
			}
		})
	}
}

func TestSnapshotRestoreFailureRecoversPreviouslyRestoredFiles(t *testing.T) {
	root := t.TempDir()
	first := writeTargetFile(t, root, "first.json", `{"value":"before"}`)
	second := writeTargetFile(t, root, "second.json", `{"value":"before"}`)
	prepared, err := PrepareTargetChanges([]TargetChange{
		{Target: config.Target{Name: "first", File: first, Type: config.TargetTypeJSON, JSONPath: "value"}, Value: "after"},
		{Target: config.Target{Name: "second", File: second, Type: config.TargetTypeJSON, JSONPath: "value"}, Value: "after"},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshots := prepared.Snapshots()
	if err := prepared.Write(); err != nil {
		t.Fatal(err)
	}
	restoration, err := PrepareRestore(snapshots)
	if err != nil {
		t.Fatal(err)
	}
	originalReplace := replaceFile
	t.Cleanup(func() { replaceFile = originalReplace })
	replaceFile = func(source, target string) error {
		if target == second {
			return errors.New("simulated restore failure")
		}
		return originalReplace(source, target)
	}
	var recovery RecoveryError
	if err := restoration.Write(); !errors.As(err, &recovery) {
		t.Fatalf("restoration error=%v", err)
	}
	if err := restoration.VerifyOriginal(); err != nil {
		t.Fatalf("failed undo did not restore pre-undo state: %v", err)
	}
}

func TestApplyTargetChanges_FailureRecoveryPreservesExternalEdits(t *testing.T) {
	root := t.TempDir()
	first := writeTargetFile(t, root, "first.json", `{"value":"before"}`)
	second := writeTargetFile(t, root, "second.json", `{"value":"before"}`)
	originalReplace := replaceFile
	t.Cleanup(func() { replaceFile = originalReplace })
	replaceFile = func(source, target string) error {
		if target == second {
			if err := os.WriteFile(first, []byte(`{"value":"external"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			return errors.New("simulated write failure")
		}
		return originalReplace(source, target)
	}
	err := ApplyTargetChanges([]TargetChange{
		{Target: config.Target{Name: "first", File: first, Type: config.TargetTypeJSON, JSONPath: "value"}, Value: "after"},
		{Target: config.Target{Name: "second", File: second, Type: config.TargetTypeJSON, JSONPath: "value"}, Value: "after"},
	})
	var recovery RecoveryError
	if !errors.As(err, &recovery) || len(recovery.UnrestoredFiles) != 1 {
		t.Fatalf("unexpected recovery error: %v", err)
	}
	if string(readFile(t, first)) != `{"value":"external"}` {
		t.Fatal("failure recovery overwrote an external edit")
	}
}
