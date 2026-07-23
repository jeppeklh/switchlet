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
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  },
  "AllowedHosts": "*"
}
`)+"\n")

	application := app.New()
	result, err := application.ApplyProfile(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		config.Profile{Name: "Local", Value: stringPointer("Server=localhost;Database=NewDatabase;")},
	)
	if err != nil {
		t.Fatalf("ApplyProfile returned error: %v", err)
	}

	if result.ProfileName != "Local" {
		t.Fatalf("ProfileName = %q, want %q", result.ProfileName, "Local")
	}
	if result.Source != profile.ValueSourceLiteral {
		t.Fatalf("Source = %q, want %q", result.Source, profile.ValueSourceLiteral)
	}
	if result.TargetPath != targetPath {
		t.Fatalf("TargetPath = %q, want %q", result.TargetPath, targetPath)
	}
	if result.ConnectionName != "DefaultConnection" {
		t.Fatalf("ConnectionName = %q, want %q", result.ConnectionName, "DefaultConnection")
	}

	updatedRoot := decodeJSONRoot(t, readFile(t, targetPath))
	connectionStrings := updatedRoot["ConnectionStrings"].(map[string]any)
	if connectionStrings["DefaultConnection"] != "Server=localhost;Database=NewDatabase;" {
		t.Fatalf("DefaultConnection = %q, want %q", connectionStrings["DefaultConnection"], "Server=localhost;Database=NewDatabase;")
	}
	if updatedRoot["AllowedHosts"] != "*" {
		t.Fatalf("AllowedHosts = %q, want %q", updatedRoot["AllowedHosts"], "*")
	}
}

func TestApplication_ApplyProfile_AppliesEnvironmentProfile(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_CONNECTION_STRING", "Server=test;Database=FromEnvironment;Pwd=secret;")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	application := app.New()
	result, err := application.ApplyProfile(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		config.Profile{Name: "Test", ValueFromEnv: stringPointer("MYAPPLICATION_TEST_CONNECTION_STRING")},
	)
	if err != nil {
		t.Fatalf("ApplyProfile returned error: %v", err)
	}

	if result.Source != profile.ValueSourceEnvironment {
		t.Fatalf("Source = %q, want %q", result.Source, profile.ValueSourceEnvironment)
	}

	updatedRoot := decodeJSONRoot(t, readFile(t, targetPath))
	connectionStrings := updatedRoot["ConnectionStrings"].(map[string]any)
	if connectionStrings["DefaultConnection"] != "Server=test;Database=FromEnvironment;Pwd=secret;" {
		t.Fatalf("DefaultConnection = %q, want resolved environment value", connectionStrings["DefaultConnection"])
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForMissingEnvironmentVariable(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	application := app.New()
	_, err := application.ApplyProfile(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		config.Profile{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_MISSING_CONNECTION_STRING")},
	)
	if err == nil {
		t.Fatal("ApplyProfile returned nil error, want missing environment variable error")
	}
	if !errors.Is(err, profile.ErrEnvironmentVariableNotSet) {
		t.Fatalf("ApplyProfile returned error %v, want ErrEnvironmentVariableNotSet", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after missing environment variable error")
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForEmptyEnvironmentVariable(t *testing.T) {
	t.Setenv("MYAPPLICATION_EMPTY_CONNECTION_STRING", "")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	application := app.New()
	_, err := application.ApplyProfile(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		config.Profile{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_EMPTY_CONNECTION_STRING")},
	)
	if err == nil {
		t.Fatal("ApplyProfile returned nil error, want empty environment variable error")
	}
	if !errors.Is(err, profile.ErrEnvironmentVariableEmpty) {
		t.Fatalf("ApplyProfile returned error %v, want ErrEnvironmentVariableEmpty", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after empty environment variable error")
	}
}

func TestApplication_ApplyProfile_ReturnsErrorForEmptyResolvedValue(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	application := app.New()
	_, err := application.ApplyProfile(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		config.Profile{Name: "Local", Value: stringPointer("")},
	)
	if err == nil {
		t.Fatal("ApplyProfile returned nil error, want empty value error")
	}
	if !strings.Contains(err.Error(), `resolved profile "Local" is empty`) {
		t.Fatalf("ApplyProfile returned error %q, want empty value error", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after empty value error")
	}
}

func TestApplication_ApplyProfile_PropagatesEditorFailureWithoutLeakingSecrets(t *testing.T) {
	t.Setenv("MYAPPLICATION_PRODUCTION_CONNECTION_STRING", "Server=prod;Database=App;Password=super-secret;")

	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", `{`)
	originalContents := readFile(t, targetPath)

	application := app.New()
	_, err := application.ApplyProfile(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		config.Profile{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_PRODUCTION_CONNECTION_STRING")},
	)
	if err == nil {
		t.Fatal("ApplyProfile returned nil error, want editor failure")
	}
	if !strings.Contains(err.Error(), `apply profile "Production"`) {
		t.Fatalf("ApplyProfile returned error %q, want contextual profile error", err)
	}
	if !strings.Contains(err.Error(), `contains invalid JSON`) {
		t.Fatalf("ApplyProfile returned error %q, want editor failure", err)
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("ApplyProfile returned error %q, must not contain secrets", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after editor failure")
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
