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
	var existingConfigError config.ExistingConfigError
	if !errors.As(err, &existingConfigError) {
		t.Fatalf("ValidateCreateLocation returned error %T, want ExistingConfigError", err)
	}
	if !strings.Contains(err.Error(), `discovered existing configuration file`) {
		t.Fatalf("ValidateCreateLocation returned error %q, want existing configuration error", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(projectRoot, ".switchlet.yaml")) {
		t.Fatalf("ValidateCreateLocation returned error %q, want existing configuration path", err)
	}
	if existingConfigError.ConfigPath != filepath.Join(projectRoot, ".switchlet.yaml") {
		t.Fatalf("ExistingConfigError.ConfigPath = %q, want parent config path", existingConfigError.ConfigPath)
	}
}

func TestPrepareReplacement_DoesNotModifyExistingConfigurationBeforeCommit(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: existing
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Existing
    values:
      - target: existing
        value: https://existing.example.test
`)+"\n")
	originalContents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read original configuration: %v", err)
	}

	replacement, err := config.PrepareReplacement(
		projectRoot,
		[]config.Target{{Name: "service", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "service.baseUrl"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("https://local.example.test")}}}},
	)
	if err != nil {
		t.Fatalf("PrepareReplacement returned error: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration after PrepareReplacement: %v", err)
	}
	if string(contents) != string(originalContents) {
		t.Fatalf("configuration file was modified before commit: %q", string(contents))
	}
	if replacement.ConfigPath() != configPath {
		t.Fatalf("ConfigPath = %q, want %q", replacement.ConfigPath(), configPath)
	}
	if replacement.Config().Targets[0].Name != "service" {
		t.Fatalf("replacement config target = %#v, want service target", replacement.Config().Targets[0])
	}
}

func TestPrepareReplacementFromSnapshot_DetectsChangedConfigurationContents(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: service
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Local
    values:
      - target: service
        value: https://local.example.test
`)+"\n")
	snapshot, err := config.LoadSnapshot(configPath)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(`
version: 3

targets:
  - name: service
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Changed
    values:
      - target: service
        value: https://changed.example.test
`)+"\n"), 0o644); err != nil {
		t.Fatalf("modify configuration after snapshot: %v", err)
	}

	_, err = config.PrepareReplacementFromSnapshot(snapshot, snapshot.Config.Targets, snapshot.Config.Profiles)
	if err == nil {
		t.Fatal("PrepareReplacementFromSnapshot returned nil error, want stale configuration error")
	}
	if !errors.Is(err, config.ErrConfigChanged) {
		t.Fatalf("PrepareReplacementFromSnapshot returned error %v, want ErrConfigChanged", err)
	}
}

func TestPrepareReplacementFromSnapshot_CommitsReplacementWithCurrentPermissions(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: service
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Local
    values:
      - target: service
        value: https://local.example.test
`)+"\n")
	if err := os.Chmod(configPath, 0o640); err != nil {
		t.Fatalf("chmod configuration file: %v", err)
	}
	snapshot, err := config.LoadSnapshot(configPath)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}

	replacement, err := config.PrepareReplacementFromSnapshot(
		snapshot,
		snapshot.Config.Targets,
		[]config.Profile{{Name: "Staging", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("https://staging.example.test")}}}},
	)
	if err != nil {
		t.Fatalf("PrepareReplacementFromSnapshot returned error: %v", err)
	}
	if err := replacement.Commit(); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	configInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat replacement configuration: %v", err)
	}
	if configInfo.Mode().Perm() != 0o640 {
		t.Fatalf("configuration permissions = %o, want 640", configInfo.Mode().Perm())
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read replacement configuration: %v", err)
	}
	if !strings.Contains(string(contents), "name: Staging") || strings.Contains(string(contents), "name: Local") {
		t.Fatalf("configuration contents = %q, want committed snapshot replacement", string(contents))
	}
}

func TestPrepareReplacementFromSnapshot_PreservesVersionThreeCommentsWhenSafe(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
# Switchlet project
version: 3

# Managed targets
targets:
  # Service target
  - name: service
    file: config.json # keep file comment
    type: json
    jsonPath: service.baseUrl

# Profiles
profiles:
  # Local profile
  - name: Local
    values:
      - target: service
        value: https://local.example.test # keep local comment
  # Staging profile
  - name: Staging
    protected: true
    values:
      - target: service
        valueFromEnv: STAGING_SERVICE_URL # keep env comment
`)+"\n")

	snapshot, err := config.LoadSnapshot(configPath)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}

	replacement, err := config.PrepareReplacementFromSnapshot(snapshot, snapshot.Config.Targets, []config.Profile{
		{Name: "Local", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("https://updated.example.test")}}},
		{Name: "Staging", Protected: true, Values: []config.ProfileValue{{Target: "service", ValueFromEnv: stringPointer("STAGING_SERVICE_URL")}}},
	})
	if err != nil {
		t.Fatalf("PrepareReplacementFromSnapshot returned error: %v", err)
	}
	if err := replacement.Commit(); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read replacement configuration: %v", err)
	}
	contentText := string(contents)
	for _, expected := range []string{
		"# Switchlet project",
		"# Managed targets",
		"# Service target",
		"# keep file comment",
		"# Profiles",
		"# Local profile",
		"# keep local comment",
		"# Staging profile",
		"# keep env comment",
	} {
		if !strings.Contains(contentText, expected) {
			t.Fatalf("replacement configuration = %q, want preserved comment %q", contentText, expected)
		}
	}
	if !strings.Contains(contentText, "value: https://updated.example.test") || strings.Contains(contentText, "value: https://local.example.test") {
		t.Fatalf("replacement configuration = %q, want updated Local value", contentText)
	}

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error after comment-preserving save: %v", err)
	}
	if len(loadedConfig.Profiles) != 2 || loadedConfig.Profiles[0].Values[0].Value == nil || *loadedConfig.Profiles[0].Values[0].Value != "https://updated.example.test" {
		t.Fatalf("loaded profiles = %#v, want updated Local profile after save", loadedConfig.Profiles)
	}
}

func TestPrepareReplacementFromSnapshot_NormalizesCompatibilityConfigurationToVersionThree(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config.json
  jsonPath: service.baseUrl

profiles:
  - name: Local
    value: https://local.example.test
`)+"\n")
	snapshot, err := config.LoadSnapshot(configPath)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if snapshot.OriginalVersion != 2 {
		t.Fatalf("OriginalVersion = %d, want 2", snapshot.OriginalVersion)
	}

	replacement, err := config.PrepareReplacementFromSnapshot(snapshot, snapshot.Config.Targets, snapshot.Config.Profiles)
	if err != nil {
		t.Fatalf("PrepareReplacementFromSnapshot returned error: %v", err)
	}
	if replacement.Config().Version != 3 {
		t.Fatalf("replacement version = %d, want 3", replacement.Config().Version)
	}
	if err := replacement.Commit(); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read replacement configuration: %v", err)
	}
	if !strings.Contains(string(contents), "version: 3") || !strings.Contains(string(contents), "targets:") || !strings.Contains(string(contents), "name: default") {
		t.Fatalf("configuration contents = %q, want normalized Version 3 configuration", string(contents))
	}
}

func TestPrepareReplacement_CommitsReplacementWithExistingPermissions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: existing
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Existing
    values:
      - target: existing
        value: https://existing.example.test
`)+"\n")
	if err := os.Chmod(configPath, 0o640); err != nil {
		t.Fatalf("chmod configuration file: %v", err)
	}
	originalInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat original configuration: %v", err)
	}

	replacement, err := config.PrepareReplacement(
		projectRoot,
		[]config.Target{{Name: "service", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "service.baseUrl"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("https://local.example.test")}}}},
	)
	if err != nil {
		t.Fatalf("PrepareReplacement returned error: %v", err)
	}
	if err := replacement.Commit(); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read replacement configuration: %v", err)
	}
	if !strings.Contains(string(contents), "name: service") || strings.Contains(string(contents), "name: existing") {
		t.Fatalf("configuration contents = %q, want committed replacement", string(contents))
	}
	configInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat replacement configuration: %v", err)
	}
	if configInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("configuration permissions = %o, want %o", configInfo.Mode().Perm(), originalInfo.Mode().Perm())
	}
}

func TestPrepareReplacement_RefusesParentConfiguration(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "project")
	nestedDirectory := filepath.Join(projectRoot, "src", "MyApplication")
	if err := os.MkdirAll(nestedDirectory, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	targetPath := writeFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: service
    file: config.json
    type: json
    jsonPath: service.baseUrl

profiles:
  - name: Local
    values:
      - target: service
        value: https://local.example.test
`)+"\n")

	_, err := config.PrepareReplacement(
		nestedDirectory,
		[]config.Target{{Name: "service", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "service.baseUrl"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("https://local.example.test")}}}},
	)
	if err == nil {
		t.Fatal("PrepareReplacement returned nil error, want parent configuration refusal")
	}
	if !strings.Contains(err.Error(), "parent directory") {
		t.Fatalf("PrepareReplacement returned error %q, want parent-directory refusal", err)
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

func TestCreate_WritesConfigurationForYAMLTarget(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: http://localhost:4566/old-queue
`)+"\n")

	configPath, loadedConfig, err := config.Create(
		projectRoot,
		[]config.Target{{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "workerQueue", Value: stringPointer("http://localhost:4566/queue")}}}},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if loadedConfig.Version != 3 {
		t.Fatalf("loaded version = %d, want 3", loadedConfig.Version)
	}
	if loadedConfig.Targets[0].Type != config.TargetTypeYAML {
		t.Fatalf("loaded target type = %q, want %q", loadedConfig.Targets[0].Type, config.TargetTypeYAML)
	}
	if loadedConfig.Targets[0].YAMLPath != "queue.endpoint" {
		t.Fatalf("loaded YAML path = %q, want %q", loadedConfig.Targets[0].YAMLPath, "queue.endpoint")
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "file: worker/config.yaml") {
		t.Fatalf("configuration file contents %q do not contain relative YAML target path", string(contents))
	}
	if !strings.Contains(string(contents), "type: yaml") {
		t.Fatalf("configuration file contents %q do not contain YAML target type", string(contents))
	}
	if !strings.Contains(string(contents), "yamlPath: queue.endpoint") {
		t.Fatalf("configuration file contents %q do not contain YAML path", string(contents))
	}
	if strings.Contains(string(contents), "jsonPath:") {
		t.Fatalf("configuration file contents %q must not contain JSON path for YAML target", string(contents))
	}
	if strings.Contains(string(contents), "key:") {
		t.Fatalf("configuration file contents %q must not contain dotenv key for YAML target", string(contents))
	}
}

func TestCreate_WritesConfigurationForTOMLTarget(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://localhost:7000"
`)+"\n")

	configPath, loadedConfig, err := config.Create(
		projectRoot,
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "serviceEndpoint", Value: stringPointer("http://localhost:8080")}}}},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if loadedConfig.Version != 3 {
		t.Fatalf("loaded version = %d, want 3", loadedConfig.Version)
	}
	if loadedConfig.Targets[0].Type != config.TargetTypeTOML {
		t.Fatalf("loaded target type = %q, want %q", loadedConfig.Targets[0].Type, config.TargetTypeTOML)
	}
	if loadedConfig.Targets[0].TOMLPath != "services.api.endpoint" {
		t.Fatalf("loaded TOML path = %q, want %q", loadedConfig.Targets[0].TOMLPath, "services.api.endpoint")
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "file: services/development.toml") {
		t.Fatalf("configuration file contents %q do not contain relative TOML target path", string(contents))
	}
	if !strings.Contains(string(contents), "type: toml") {
		t.Fatalf("configuration file contents %q do not contain TOML target type", string(contents))
	}
	if !strings.Contains(string(contents), "tomlPath: services.api.endpoint") {
		t.Fatalf("configuration file contents %q do not contain TOML path", string(contents))
	}
	if strings.Contains(string(contents), "jsonPath:") {
		t.Fatalf("configuration file contents %q must not contain JSON path for TOML target", string(contents))
	}
	if strings.Contains(string(contents), "yamlPath:") {
		t.Fatalf("configuration file contents %q must not contain YAML path for TOML target", string(contents))
	}
	if strings.Contains(string(contents), "key:") {
		t.Fatalf("configuration file contents %q must not contain dotenv key for TOML target", string(contents))
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
