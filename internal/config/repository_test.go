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
	targetPath := writeFile(t, projectRoot, "config/development.json", strings.TrimSpace(`
{
	  "database": {
	    "primary": {
	      "url": "postgres://old"
	    }
	  }
}
`)+"\n")

	configPath, loadedConfig, err := config.Create(
		projectRoot,
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.primary.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://localhost:5432/myapp")}}},
			{Name: "Production", Values: []config.ProfileValue{{Target: "database", ValueFromEnv: stringPointer("MYAPPLICATION_PRODUCTION_CONNECTION_STRING")}}, Protected: true},
		},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	wantConfigPath := filepath.Join(projectRoot, ".switchlet.yaml")
	if configPath != wantConfigPath {
		t.Fatalf("Create returned config path %q, want %q", configPath, wantConfigPath)
	}
	if loadedConfig.Version != 3 {
		t.Fatalf("loaded version = %d, want 3", loadedConfig.Version)
	}
	if len(loadedConfig.Targets) != 1 {
		t.Fatalf("len(loaded targets) = %d, want 1", len(loadedConfig.Targets))
	}
	if loadedConfig.Targets[0].File != targetPath {
		t.Fatalf("loaded target file = %q, want %q", loadedConfig.Targets[0].File, targetPath)
	}
	if loadedConfig.Targets[0].JSONPath != "database.primary.url" {
		t.Fatalf("loaded JSON path = %q, want %q", loadedConfig.Targets[0].JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 2 {
		t.Fatalf("len(loaded profiles) = %d, want 2", len(loadedConfig.Profiles))
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "file: config/development.json") {
		t.Fatalf("configuration file contents %q do not contain relative target path", string(contents))
	}
	if !strings.Contains(string(contents), "name: database") {
		t.Fatalf("configuration file contents %q do not contain target name", string(contents))
	}
	if !strings.Contains(string(contents), "type: json") {
		t.Fatalf("configuration file contents %q do not contain target type", string(contents))
	}
	if !strings.Contains(string(contents), "jsonPath: database.primary.url") {
		t.Fatalf("configuration file contents %q do not contain JSON path", string(contents))
	}
	if strings.Contains(string(contents), "connectionName:") {
		t.Fatalf("configuration file contents %q must not contain legacy connection name", string(contents))
	}
	if !strings.Contains(string(contents), "valueFromEnv: MYAPPLICATION_PRODUCTION_CONNECTION_STRING") {
		t.Fatalf("configuration file contents %q do not contain environment-backed profile", string(contents))
	}
	if !strings.Contains(string(contents), "target: database") {
		t.Fatalf("configuration file contents %q do not contain profile target reference", string(contents))
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
	targetPath := writeFile(t, sharedRoot, "config.json", strings.TrimSpace(`
{
	  "service": {
	    "baseUrl": "https://old.example.test"
	  }
}
`)+"\n")

	configPath, _, err := config.Create(
		projectRoot,
		[]config.Target{{Name: "service", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "service.baseUrl"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("https://new.example.test")}}}},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "file: ../shared/config.json") {
		t.Fatalf("configuration file contents %q do not contain relative parent path", string(contents))
	}
}

func TestCreate_RemovesConfigurationFileWhenGeneratedConfigurationIsInvalid(t *testing.T) {
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

	configPath, _, err := config.Create(
		projectRoot,
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.primary.url"}},
		[]config.Profile{{
			Name: "Broken",
			Values: []config.ProfileValue{{
				Target:       "database",
				Value:        stringPointer("postgres://localhost:5432/myapp"),
				ValueFromEnv: stringPointer("MYAPPLICATION_CONNECTION_STRING"),
			}},
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

func TestCreate_WritesConfigurationForGenericJSONTarget(t *testing.T) {
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

	configPath, loadedConfig, err := config.Create(
		projectRoot,
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.primary.url"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://localhost:5432/myapp")}}}},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if loadedConfig.Version != 3 {
		t.Fatalf("loaded version = %d, want 3", loadedConfig.Version)
	}
	if loadedConfig.Targets[0].JSONPath != "database.primary.url" {
		t.Fatalf("loaded JSON path = %q, want %q", loadedConfig.Targets[0].JSONPath, "database.primary.url")
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "jsonPath: database.primary.url") {
		t.Fatalf("configuration file contents %q do not contain generic JSON path", string(contents))
	}
}

func TestEnsureConfigIgnored_CreatesGitignoreWhenMissing(t *testing.T) {
	projectRoot := t.TempDir()

	changed, err := config.EnsureConfigIgnored(projectRoot)
	if err != nil {
		t.Fatalf("EnsureConfigIgnored returned error: %v", err)
	}
	if !changed {
		t.Fatal("EnsureConfigIgnored changed = false, want true when creating .gitignore")
	}

	contents, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if string(contents) != ".switchlet.yaml\n" {
		t.Fatalf(".gitignore contents = %q, want %q", string(contents), ".switchlet.yaml\n")
	}
}

func TestEnsureConfigIgnored_AppendsEntryWithoutDuplicatingExistingContent(t *testing.T) {
	projectRoot := t.TempDir()
	gitignorePath := writeFile(t, projectRoot, ".gitignore", strings.TrimSpace(`
node_modules/
dist/
`)+"\n")

	changed, err := config.EnsureConfigIgnored(projectRoot)
	if err != nil {
		t.Fatalf("EnsureConfigIgnored returned error: %v", err)
	}
	if !changed {
		t.Fatal("EnsureConfigIgnored changed = false, want true when appending .switchlet.yaml")
	}

	contents, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if string(contents) != "node_modules/\ndist/\n.switchlet.yaml\n" {
		t.Fatalf(".gitignore contents = %q, want appended ignore entry", string(contents))
	}

	changed, err = config.EnsureConfigIgnored(projectRoot)
	if err != nil {
		t.Fatalf("second EnsureConfigIgnored returned error: %v", err)
	}
	if changed {
		t.Fatal("EnsureConfigIgnored changed = true, want false when .switchlet.yaml is already ignored")
	}

	contents, err = os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore after second call: %v", err)
	}
	if strings.Count(string(contents), ".switchlet.yaml") != 1 {
		t.Fatalf(".gitignore contents = %q, want exactly one .switchlet.yaml entry", string(contents))
	}
}

func TestEnsureConfigIgnored_PreservesExistingLineEndingsWhenUpdating(t *testing.T) {
	projectRoot := t.TempDir()
	gitignorePath := writeFile(t, projectRoot, ".gitignore", "bin/\r\ndist/")

	changed, err := config.EnsureConfigIgnored(projectRoot)
	if err != nil {
		t.Fatalf("EnsureConfigIgnored returned error: %v", err)
	}
	if !changed {
		t.Fatal("EnsureConfigIgnored changed = false, want true when appending to existing .gitignore")
	}

	contents, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if string(contents) != "bin/\r\ndist/\r\n.switchlet.yaml\r\n" {
		t.Fatalf(".gitignore contents = %q, want preserved CRLF line endings", string(contents))
	}
}
