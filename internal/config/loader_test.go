package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestLoad_LoadsAndValidatesConfiguration(t *testing.T) {
	configPath := fixturePath(t, "valid", "basic", ".switchlet.yaml")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	projectRoot := fixturePath(t, "valid", "basic")
	wantTargetPath := filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json")

	if loadedConfig.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", loadedConfig.ConfigPath, configPath)
	}
	if loadedConfig.ProjectRoot != projectRoot {
		t.Fatalf("ProjectRoot = %q, want %q", loadedConfig.ProjectRoot, projectRoot)
	}
	if loadedConfig.Version != 1 {
		t.Fatalf("Version = %d, want 1", loadedConfig.Version)
	}
	if loadedConfig.Target.File != wantTargetPath {
		t.Fatalf("Target.File = %q, want %q", loadedConfig.Target.File, wantTargetPath)
	}
	if loadedConfig.Target.ConnectionName != "DefaultConnection" {
		t.Fatalf("Target.ConnectionName = %q, want %q", loadedConfig.Target.ConnectionName, "DefaultConnection")
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(loadedConfig.Profiles))
	}

	if loadedConfig.Profiles[0].Name != "Local" {
		t.Fatalf("Profiles[0].Name = %q, want %q", loadedConfig.Profiles[0].Name, "Local")
	}
	if loadedConfig.Profiles[0].Value == nil {
		t.Fatal("Profiles[0].Value is nil, want literal value")
	}
	if !strings.Contains(*loadedConfig.Profiles[0].Value, "Database=MyApplication") {
		t.Fatalf("Profiles[0].Value = %q, want literal connection string", *loadedConfig.Profiles[0].Value)
	}

	if loadedConfig.Profiles[1].Name != "Test" {
		t.Fatalf("Profiles[1].Name = %q, want %q", loadedConfig.Profiles[1].Name, "Test")
	}
	if loadedConfig.Profiles[1].ValueFromEnv == nil {
		t.Fatal("Profiles[1].ValueFromEnv is nil, want environment variable name")
	}
	if *loadedConfig.Profiles[1].ValueFromEnv != "MYAPPLICATION_TEST_CONNECTION_STRING" {
		t.Fatalf("Profiles[1].ValueFromEnv = %q, want %q", *loadedConfig.Profiles[1].ValueFromEnv, "MYAPPLICATION_TEST_CONNECTION_STRING")
	}
}

func TestLoad_ResolvesTargetPathRelativeToConfiguration(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, "config/.switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: ../src/MyApplication/appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`)+"\n")
	writeFile(t, projectRoot, "src/MyApplication/appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=MyApplication;"
  }
}
`)+"\n")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	wantTargetPath := filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json")
	if loadedConfig.Target.File != wantTargetPath {
		t.Fatalf("Target.File = %q, want %q", loadedConfig.Target.File, wantTargetPath)
	}
	if loadedConfig.ProjectRoot != filepath.Join(projectRoot, "config") {
		t.Fatalf("ProjectRoot = %q, want %q", loadedConfig.ProjectRoot, filepath.Join(projectRoot, "config"))
	}
}

func TestLoad_ReturnsErrorForInvalidYAML(t *testing.T) {
	configPath := fixturePath(t, "invalid", "invalid-yaml", ".switchlet.yaml")

	_, err := config.Load(configPath)
	if err == nil {
		t.Fatal("Load returned nil error, want parse error")
	}

	if !strings.Contains(err.Error(), "parse configuration file") {
		t.Fatalf("Load returned error %q, want parse configuration file error", err)
	}
}
