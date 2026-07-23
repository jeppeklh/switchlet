package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestValidateCreateLocation_ReturnsErrorWhenConfigurationExistsInParentDirectory(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "project")
	nestedDirectory := filepath.Join(projectRoot, "src", "MyApplication")

	if err := os.MkdirAll(nestedDirectory, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=App;"
`)+"\n")

	err := config.ValidateCreateLocation(nestedDirectory)
	if err == nil {
		t.Fatal("ValidateCreateLocation returned nil error, want existing configuration error")
	}
	if !strings.Contains(err.Error(), `discovered existing configuration file`) {
		t.Fatalf("ValidateCreateLocation returned error %q, want existing configuration error", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(projectRoot, ".switchlet.yaml")) {
		t.Fatalf("ValidateCreateLocation returned error %q, want existing configuration path", err)
	}
}

func TestCreate_WritesConfigurationAndLoadsItBack(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "src/MyApplication/appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	configPath, loadedConfig, err := config.Create(
		projectRoot,
		config.Target{File: targetPath, JSONPath: "ConnectionStrings.DefaultConnection"},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Production", ValueFromEnv: stringPointer("MYAPPLICATION_PRODUCTION_CONNECTION_STRING"), Protected: true},
		},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	wantConfigPath := filepath.Join(projectRoot, ".switchlet.yaml")
	if configPath != wantConfigPath {
		t.Fatalf("Create returned config path %q, want %q", configPath, wantConfigPath)
	}
	if loadedConfig.Version != 1 {
		t.Fatalf("loaded version = %d, want 1", loadedConfig.Version)
	}
	if loadedConfig.Target.File != targetPath {
		t.Fatalf("loaded target file = %q, want %q", loadedConfig.Target.File, targetPath)
	}
	if loadedConfig.Target.JSONPath != "ConnectionStrings.DefaultConnection" {
		t.Fatalf("loaded JSON path = %q, want %q", loadedConfig.Target.JSONPath, "ConnectionStrings.DefaultConnection")
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(loaded profiles) = %d, want 2", len(loadedConfig.Profiles))
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "file: src/MyApplication/appsettings.Development.json") {
		t.Fatalf("configuration file contents %q do not contain relative target path", string(contents))
	}
	if !strings.Contains(string(contents), "connectionName: DefaultConnection") {
		t.Fatalf("configuration file contents %q do not contain legacy connection name", string(contents))
	}
	if !strings.Contains(string(contents), "valueFromEnv: MYAPPLICATION_PRODUCTION_CONNECTION_STRING") {
		t.Fatalf("configuration file contents %q do not contain environment-backed profile", string(contents))
	}
	if !strings.Contains(string(contents), "protected: true") {
		t.Fatalf("configuration file contents %q do not contain protected profile", string(contents))
	}
	if !strings.HasSuffix(string(contents), "\n") {
		t.Fatal("configuration file does not end with a trailing newline")
	}
}

func TestCreate_WritesRelativePathOutsideProjectRootWhenPossible(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "project")
	sharedRoot := filepath.Join(workspaceRoot, "shared")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	targetPath := writeFile(t, sharedRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	configPath, _, err := config.Create(
		projectRoot,
		config.Target{File: targetPath, JSONPath: "ConnectionStrings.DefaultConnection"},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "file: ../shared/appsettings.Development.json") {
		t.Fatalf("configuration file contents %q do not contain relative parent path", string(contents))
	}
}

func TestCreate_RemovesConfigurationFileWhenGeneratedConfigurationIsInvalid(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	configPath, _, err := config.Create(
		projectRoot,
		config.Target{File: targetPath, JSONPath: "ConnectionStrings.DefaultConnection"},
		[]config.Profile{{
			Name:         "Broken",
			Value:        stringPointer("Server=localhost;Database=App;"),
			ValueFromEnv: stringPointer("MYAPPLICATION_CONNECTION_STRING"),
		}},
	)
	if err == nil {
		t.Fatal("Create returned nil error, want validation error")
	}
	if !strings.Contains(err.Error(), `must define exactly one of value or valueFromEnv`) {
		t.Fatalf("Create returned error %q, want profile validation error", err)
	}

	_, statErr := os.Stat(configPath)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error", statErr)
	}
}

func TestCreate_ReturnsErrorForNonConnectionStringsTargetPath(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	_, _, err := config.Create(
		projectRoot,
		config.Target{File: targetPath, JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Local", Value: stringPointer("postgres://localhost:5432/myapp")}},
	)
	if err == nil {
		t.Fatal("Create returned nil error, want unsupported init target error")
	}
	if !strings.Contains(err.Error(), "current init workflow can create only ConnectionStrings targets") {
		t.Fatalf("Create returned error %q, want unsupported init target error", err)
	}
}
