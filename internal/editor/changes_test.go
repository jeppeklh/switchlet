package editor

import (
	"bytes"
	"errors"
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

func TestApplyTargetChanges_WriteFailureAfterEarlierFileReportsPartialStateAndCleansTemporaryFile(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/config.json", `{"api":{"url":"https://old.example.test"}}`)
	originalFrontendContents := readFile(t, frontendPath)

	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		if newPath == frontendPath {
			return errors.New("rename failed")
		}

		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://new",
		},
		{
			Target: config.Target{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://secret-value.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want second-file write failure")
	}
	for _, expected := range []string{"after 1 file(s) were already replaced", "target files may now be partially updated", "rename failed"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, expected)
		}
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
	}

	databaseRoot := decodeJSONRoot(t, readFile(t, databasePath))
	database := databaseRoot["database"].(map[string]any)
	if database["url"] != "postgres://new" {
		t.Fatalf("database.url = %q, want first file to have been replaced before second failure", database["url"])
	}
	if !bytes.Equal(readFile(t, frontendPath), originalFrontendContents) {
		t.Fatal("second target file changed after its replacement failed")
	}
	if containsTempFile(t, filepath.Dir(frontendPath), tempFilePrefix(frontendPath)) {
		t.Fatal("temporary file was not cleaned up after second-file rename failure")
	}
}
