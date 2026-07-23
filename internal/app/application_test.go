package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/profile"
)

func TestApplication_ApplyProfile_AppliesLiteralProfile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  },
  "AllowedHosts": "*"
}
`)+"\n")

	application := app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Local", Value: stringPointer("postgres://new")}},
	)
	result, err := application.ApplyProfileByName("Local")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}

	if result.ProfileName != "Local" {
		t.Fatalf("ProfileName = %q, want %q", result.ProfileName, "Local")
	}
	if result.TargetPath != "database.primary.url" {
		t.Fatalf("TargetPath = %q, want %q", result.TargetPath, "database.primary.url")
	}

	updatedRoot := decodeJSONRoot(t, readFile(t, targetPath))
	database := updatedRoot["database"].(map[string]any)
	primary := database["primary"].(map[string]any)
	if primary["url"] != "postgres://new" {
		t.Fatalf("database.primary.url = %q, want %q", primary["url"], "postgres://new")
	}
	if updatedRoot["AllowedHosts"] != "*" {
		t.Fatalf("AllowedHosts = %q, want %q", updatedRoot["AllowedHosts"], "*")
	}
}

func TestApplication_Profiles_ReturnsResolvedDisplayDataForAvailableProfiles(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_CONNECTION_STRING", "Server=test;Database=App;Password=super-secret;")

	application := app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;Pwd=local-secret;")},
			{Name: "Test", ValueFromEnv: stringPointer("MYAPPLICATION_TEST_CONNECTION_STRING"), Protected: true},
		},
	)

	items := application.Profiles()
	if len(items) != 2 {
		t.Fatalf("len(Profiles()) = %d, want 2", len(items))
	}

	if items[0].Source != app.ProfileSourceLiteral {
		t.Fatalf("Profiles()[0].Source = %q, want %q", items[0].Source, app.ProfileSourceLiteral)
	}
	if items[0].MaskedValue != "Server=localhost;Database=App;Pwd=****;" {
		t.Fatalf("Profiles()[0].MaskedValue = %q, want masked literal value", items[0].MaskedValue)
	}
	if !items[1].Available {
		t.Fatalf("Profiles()[1].Available = false, want true (reason: %q)", items[1].UnavailableReason)
	}
	if !items[1].Protected {
		t.Fatal("Profiles()[1].Protected = false, want true")
	}
	if items[1].Source != app.ProfileSourceEnvironment {
		t.Fatalf("Profiles()[1].Source = %q, want %q", items[1].Source, app.ProfileSourceEnvironment)
	}
	if items[1].EnvironmentVariableName != "MYAPPLICATION_TEST_CONNECTION_STRING" {
		t.Fatalf("Profiles()[1].EnvironmentVariableName = %q, want %q", items[1].EnvironmentVariableName, "MYAPPLICATION_TEST_CONNECTION_STRING")
	}
	if items[1].MaskedValue != "Server=test;Database=App;Password=****;" {
		t.Fatalf("Profiles()[1].MaskedValue = %q, want masked environment value", items[1].MaskedValue)
	}
	if items[1].UnavailableReason != "" {
		t.Fatalf("Profiles()[1].UnavailableReason = %q, want empty string", items[1].UnavailableReason)
	}
}

func TestApplication_Profiles_ReturnsUnavailableResolutionError(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_MISSING_CONNECTION_STRING")}},
	)

	items := application.Profiles()
	if len(items) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(items))
	}
	if items[0].Available {
		t.Fatal("Profiles()[0].Available = true, want false")
	}
	if !strings.Contains(items[0].UnavailableReason, "MYAPPLICATION_MISSING_CONNECTION_STRING") {
		t.Fatalf("Profiles()[0].UnavailableReason = %q, want environment variable name", items[0].UnavailableReason)
	}
	if items[0].MaskedValue != "" {
		t.Fatalf("Profiles()[0].MaskedValue = %q, want empty string", items[0].MaskedValue)
	}
}

func TestApplication_ApplyProfile_AppliesEnvironmentProfile(t *testing.T) {
	t.Setenv("MYAPPLICATION_SERVICE_URL", "https://env.example.test")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Test", ValueFromEnv: stringPointer("MYAPPLICATION_SERVICE_URL")}},
	)
	result, err := application.ApplyProfileByName("Test")
	if err != nil {
		t.Fatalf("ApplyProfileByName returned error: %v", err)
	}
	if result.ProfileName != "Test" {
		t.Fatalf("ProfileName = %q, want %q", result.ProfileName, "Test")
	}
	if result.TargetPath != "serviceUrl" {
		t.Fatalf("TargetPath = %q, want %q", result.TargetPath, "serviceUrl")
	}

	updatedRoot := decodeJSONRoot(t, readFile(t, targetPath))
	if updatedRoot["serviceUrl"] != "https://env.example.test" {
		t.Fatalf("serviceUrl = %q, want resolved environment value", updatedRoot["serviceUrl"])
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForMissingEnvironmentVariable(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_MISSING_CONNECTION_STRING")}},
	)
	_, err := application.ApplyProfileByName("Production")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want missing environment variable error")
	}
	if !errors.Is(err, profile.ErrEnvironmentVariableNotSet) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrEnvironmentVariableNotSet", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after missing environment variable error")
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForEmptyEnvironmentVariable(t *testing.T) {
	t.Setenv("MYAPPLICATION_EMPTY_CONNECTION_STRING", "")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_EMPTY_CONNECTION_STRING")}},
	)
	_, err := application.ApplyProfileByName("Production")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want empty environment variable error")
	}
	if !errors.Is(err, profile.ErrEnvironmentVariableEmpty) {
		t.Fatalf("ApplyProfileByName returned error %v, want ErrEnvironmentVariableEmpty", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after empty environment variable error")
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForEmptyResolvedValue(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("")}},
	)
	_, err := application.ApplyProfileByName("Local")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want empty value error")
	}
	if !strings.Contains(err.Error(), `resolved profile "Local" is empty`) {
		t.Fatalf("ApplyProfileByName returned error %q, want empty value error", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after empty value error")
	}
}

func TestApplication_ApplyProfile_PropagatesEditorFailureWithoutLeakingSecrets(t *testing.T) {
	t.Setenv("MYAPPLICATION_PRODUCTION_CONNECTION_STRING", "Server=prod;Database=App;Password=super-secret;")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{`)
	originalContents := readFile(t, targetPath)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "serviceUrl"},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_PRODUCTION_CONNECTION_STRING")}},
	)
	_, err := application.ApplyProfileByName("Production")
	if err == nil {
		t.Fatal("ApplyProfileByName returned nil error, want editor failure")
	}
	if !strings.Contains(err.Error(), `apply profile "Production"`) {
		t.Fatalf("ApplyProfileByName returned error %q, want contextual profile error", err)
	}
	if !strings.Contains(err.Error(), `contains invalid JSON`) {
		t.Fatalf("ApplyProfileByName returned error %q, want editor failure", err)
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("ApplyProfileByName returned error %q, must not contain secrets", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after editor failure")
	}
}

func TestApplication_ValidateStartup_ReturnsErrorForMissingTargetPath(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{}}`)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.url"},
		nil,
	)

	err := application.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup returned nil error, want target-path validation error")
	}
	if !strings.Contains(err.Error(), `does not contain JSON path "database.primary.url"`) {
		t.Fatalf("ValidateStartup returned error %q, want missing target path error", err)
	}
}

func TestApplication_ValidateStartup_ReturnsErrorForNonStringTargetValue(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"primary":{"port":5432}}}`)

	application := app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.port"},
		nil,
	)

	err := application.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup returned nil error, want non-string target error")
	}
	if !strings.Contains(err.Error(), `JSON path "database.primary.port" must resolve to a string`) {
		t.Fatalf("ValidateStartup returned error %q, want non-string target error", err)
	}
}

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

	var decodedRoot map[string]any
	if err := json.Unmarshal(contents, &decodedRoot); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	return decodedRoot
}

func stringPointer(value string) *string {
	return &value
}
