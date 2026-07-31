package config_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestLoad_ReturnsErrorForInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		wantError     string
	}{
		{
			name: "missing version",
			configContent: strings.TrimSpace(`
target:
  file: appsettings.Development.json
  jsonPath: database.primary.url

profiles:
  - name: Local
    value: postgres://localhost:5432/myapp
`) + "\n",
			wantError: "version must be set",
		},
		{
			name: "unsupported version",
			configContent: versionTwoConfigWithVersion(4, "appsettings.Development.json", "database.primary.url", `  - name: Local
    value: postgres://localhost:5432/myapp`),
			wantError: "unsupported version 4",
		},
		{
			name: "missing target file",
			configContent: strings.TrimSpace(`
version: 2

target:
  jsonPath: database.primary.url

profiles:
  - name: Local
    value: postgres://localhost:5432/myapp
`) + "\n",
			wantError: "target.file must be set",
		},
		{
			name: "missing connection name",
			configContent: strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json

profiles:
  - name: Local
    value: postgres://localhost:5432/myapp
`) + "\n",
			wantError: "target.connectionName must be set",
		},
		{
			name: "missing JSON path",
			configContent: strings.TrimSpace(`
version: 2

target:
  file: appsettings.Development.json

profiles:
  - name: Local
    value: postgres://localhost:5432/myapp
`) + "\n",
			wantError: "target.jsonPath must be set",
		},
		{
			name: "version one does not support JSON path",
			configContent: strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  jsonPath: ConnectionStrings.DefaultConnection

profiles:
  - name: Local
    value: postgres://localhost:5432/myapp
`) + "\n",
			wantError: "target.jsonPath is not supported in version 1",
		},
		{
			name: "version two does not support connection name",
			configContent: legacyConfig(2, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: postgres://localhost:5432/myapp`),
			wantError: "target.connectionName is not supported in version 2",
		},
		{
			name: "invalid JSON path",
			configContent: versionTwoConfig("appsettings.Development.json", "database..url", `  - name: Local
    value: postgres://localhost:5432/myapp`),
			wantError: "target.jsonPath is invalid",
		},
		{
			name: "legacy connection name cannot contain dots",
			configContent: legacyConfig(1, "appsettings.Development.json", "Primary.Default", `  - name: Local
    value: postgres://localhost:5432/myapp`),
			wantError: "cannot be mapped to a JSON path",
		},
		{
			name: "empty profile list",
			configContent: strings.TrimSpace(`
version: 2

target:
  file: appsettings.Development.json
  jsonPath: database.primary.url

profiles: []
`) + "\n",
			wantError: "at least one profile must be configured",
		},
		{
			name: "empty profile name",
			configContent: versionTwoConfig("appsettings.Development.json", "database.primary.url", `  - name: ""
    value: postgres://localhost:5432/myapp`),
			wantError: "profiles[0].name must be set",
		},
		{
			name: "duplicate profile names",
			configContent: versionTwoConfig("appsettings.Development.json", "database.primary.url", `  - name: Local
    value: postgres://localhost:5432/myapp
  - name: Local
    valueFromEnv: MYAPP_TEST_DATABASE_URL`),
			wantError: `duplicate profile name "Local"`,
		},
		{
			name: "profile with both value fields",
			configContent: versionTwoConfig("appsettings.Development.json", "database.primary.url", `  - name: Local
    value: postgres://localhost:5432/myapp
    valueFromEnv: MYAPP_TEST_DATABASE_URL`),
			wantError: `profile "Local" must define exactly one of value or valueFromEnv`,
		},
		{
			name:          "profile with neither value field",
			configContent: versionTwoConfig("appsettings.Development.json", "database.primary.url", `  - name: Local`),
			wantError:     `profile "Local" must define exactly one of value or valueFromEnv`,
		},
		{
			name: "empty environment variable name",
			configContent: versionTwoConfig("appsettings.Development.json", "database.primary.url", `  - name: Test
    valueFromEnv: "   "`),
			wantError: `profile "Test" valueFromEnv must be set`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			configPath := writeFile(t, projectRoot, ".switchlet.yaml", testCase.configContent)

			_, err := config.Load(configPath)
			if err == nil {
				t.Fatal("Load returned nil error, want validation error")
			}

			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("Load returned error %q, want substring %q", err, testCase.wantError)
			}
		})
	}
}

func TestLoad_ReturnsErrorForInvalidVersionThreeConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		wantError     string
	}{
		{
			name: "empty target list",
			configContent: strings.TrimSpace(`
version: 3

targets: []

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://localhost:5432/myapp
`) + "\n",
			wantError: "at least one target must be configured",
		},
		{
			name: "empty target name",
			configContent: versionThreeConfig(`  - name: ""
    file: config.json
    type: json
    jsonPath: database.url`, validVersionThreeProfiles()),
			wantError: "targets[0].name must be set",
		},
		{
			name: "target name has leading whitespace",
			configContent: versionThreeConfig(`  - name: " database"
    file: config.json
    type: json
    jsonPath: database.url`, validVersionThreeProfiles()),
			wantError: "targets[0].name must not contain leading or trailing whitespace",
		},
		{
			name: "duplicate target names",
			configContent: versionThreeConfig(`  - name: database
    file: config.json
    type: json
    jsonPath: database.url
  - name: database
    file: config.json
    type: json
    jsonPath: database.replicaUrl`, validVersionThreeProfiles()),
			wantError: `duplicate target name "database"`,
		},
		{
			name: "missing target file",
			configContent: versionThreeConfig(`  - name: database
    type: json
    jsonPath: database.url`, validVersionThreeProfiles()),
			wantError: "targets[0].file must be set",
		},
		{
			name: "unsupported target type",
			configContent: versionThreeConfig(`  - name: database
    file: config.xml
    type: xml
    jsonPath: database.url`, validVersionThreeProfiles()),
			wantError: `targets[0].type "xml" is not supported`,
		},
		{
			name: "ambiguous target type inference",
			configContent: versionThreeConfig(`  - name: database
    file: config.local
    jsonPath: database.url`, validVersionThreeProfiles()),
			wantError: "targets[0].type must be set because target type cannot be inferred",
		},
		{
			name: "json target missing json path",
			configContent: versionThreeConfig(`  - name: database
    file: config.json
    type: json`, validVersionThreeProfiles()),
			wantError: "targets[0].jsonPath must be set for json targets",
		},
		{
			name: "json target rejects yaml path",
			configContent: versionThreeConfig(`  - name: database
    file: config.json
    type: json
    jsonPath: database.url
    yamlPath: database.url`, validVersionThreeProfiles()),
			wantError: "targets[0].yamlPath is only supported for yaml targets",
		},
		{
			name: "json target rejects toml path",
			configContent: versionThreeConfig(`  - name: database
    file: config.json
    type: json
    jsonPath: database.url
    tomlPath: database.url`, validVersionThreeProfiles()),
			wantError: "targets[0].tomlPath is only supported for toml targets",
		},
		{
			name: "json target rejects key",
			configContent: versionThreeConfig(`  - name: database
    file: config.json
    type: json
    jsonPath: database.url
    key: DATABASE_URL`, validVersionThreeProfiles()),
			wantError: "targets[0].key is only supported for dotenv targets",
		},
		{
			name: "dotenv target missing key",
			configContent: versionThreeConfig(`  - name: frontendApi
    file: .env
    type: dotenv
    key: ""`, `  - name: Local
    values:
      - target: frontendApi
        value: http://localhost:5173`),
			wantError: "targets[0].key must be set for dotenv targets",
		},
		{
			name: "dotenv target rejects invalid key syntax",
			configContent: versionThreeConfig(`  - name: frontendApi
    file: .env
    type: dotenv
    key: 1INVALID`, `  - name: Local
    values:
      - target: frontendApi
        value: http://localhost:5173`),
			wantError: "targets[0].key is invalid",
		},
		{
			name: "dotenv target rejects json path",
			configContent: versionThreeConfig(`  - name: frontendApi
    file: .env
    type: dotenv
    key: VITE_API_URL
    jsonPath: services.api.url`, `  - name: Local
    values:
      - target: frontendApi
        value: http://localhost:5173`),
			wantError: "targets[0].jsonPath is only supported for json targets",
		},
		{
			name: "dotenv target rejects yaml path",
			configContent: versionThreeConfig(`  - name: frontendApi
    file: .env
    type: dotenv
    key: VITE_API_URL
    yamlPath: services.api.url`, `  - name: Local
    values:
      - target: frontendApi
        value: http://localhost:5173`),
			wantError: "targets[0].yamlPath is only supported for yaml targets",
		},
		{
			name: "dotenv target rejects toml path",
			configContent: versionThreeConfig(`  - name: frontendApi
    file: .env
    type: dotenv
    key: VITE_API_URL
    tomlPath: services.api.url`, `  - name: Local
    values:
      - target: frontendApi
        value: http://localhost:5173`),
			wantError: "targets[0].tomlPath is only supported for toml targets",
		},
		{
			name: "yaml target missing yaml path",
			configContent: versionThreeConfig(`  - name: workerQueue
    file: worker/config.yaml
    type: yaml`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`),
			wantError: "targets[0].yamlPath must be set for yaml targets",
		},
		{
			name: "yaml target rejects invalid yaml path",
			configContent: versionThreeConfig(`  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue..endpoint`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`),
			wantError: "targets[0].yamlPath is invalid",
		},
		{
			name: "yaml target rejects path segment with leading whitespace",
			configContent: versionThreeConfig(`  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: "queue. endpoint"`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`),
			wantError: `targets[0].yamlPath is invalid: segment " endpoint" must not contain leading or trailing whitespace`,
		},
		{
			name: "yaml target rejects json path",
			configContent: versionThreeConfig(`  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    jsonPath: queue.endpoint
    yamlPath: queue.endpoint`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`),
			wantError: "targets[0].jsonPath is only supported for json targets",
		},
		{
			name: "yaml target rejects key",
			configContent: versionThreeConfig(`  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue.endpoint
    key: QUEUE_ENDPOINT`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`),
			wantError: "targets[0].key is only supported for dotenv targets",
		},
		{
			name: "yaml target rejects toml path",
			configContent: versionThreeConfig(`  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue.endpoint
    tomlPath: queue.endpoint`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`),
			wantError: "targets[0].tomlPath is only supported for toml targets",
		},
		{
			name: "toml target missing toml path",
			configContent: versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.toml
    type: toml`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`),
			wantError: "targets[0].tomlPath must be set for toml targets",
		},
		{
			name: "toml target rejects invalid toml path",
			configContent: versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services..endpoint`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`),
			wantError: "targets[0].tomlPath is invalid",
		},
		{
			name: "toml target rejects array selector syntax",
			configContent: versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services[0].endpoint`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`),
			wantError: "array selectors are not supported",
		},
		{
			name: "toml target rejects quoted key syntax",
			configContent: versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    tomlPath: 'services."api".endpoint'`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`),
			wantError: "must use unquoted TOML bare-key syntax",
		},
		{
			name: "toml target rejects json path",
			configContent: versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    jsonPath: services.api.endpoint
    tomlPath: services.api.endpoint`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`),
			wantError: "targets[0].jsonPath is only supported for json targets",
		},
		{
			name: "toml target rejects yaml path",
			configContent: versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    yamlPath: services.api.endpoint
    tomlPath: services.api.endpoint`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`),
			wantError: "targets[0].yamlPath is only supported for yaml targets",
		},
		{
			name: "toml target rejects key",
			configContent: versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    key: SERVICE_ENDPOINT
    tomlPath: services.api.endpoint`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`),
			wantError: "targets[0].key is only supported for dotenv targets",
		},
		{
			name: "duplicate target location",
			configContent: versionThreeConfig(`  - name: primaryDatabase
    file: config.json
    type: json
    jsonPath: database.url
  - name: replicaDatabase
    file: config.json
    type: json
    jsonPath: database.url`, `  - name: Local
    values:
      - target: primaryDatabase
        value: postgres://localhost:5432/myapp`),
			wantError: "duplicates target location",
		},
		{
			name: "duplicate yaml target location",
			configContent: versionThreeConfig(`  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue.endpoint
  - name: reportingQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue.endpoint`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`),
			wantError: "duplicates target location",
		},
		{
			name: "duplicate toml target location",
			configContent: versionThreeConfig(`  - name: primaryEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services.api.endpoint
  - name: secondaryEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services.api.endpoint`, `  - name: Local
    values:
      - target: primaryEndpoint
        value: http://localhost:8080`),
			wantError: "duplicates target location",
		},
		{
			name:          "profile values required",
			configContent: versionThreeConfig(validVersionThreeTargets(), `  - name: Local`),
			wantError:     `profile "Local" must include at least one value`,
		},
		{
			name: "unknown profile target reference",
			configContent: versionThreeConfig(validVersionThreeTargets(), `  - name: Local
    values:
      - target: missing
        value: postgres://localhost:5432/myapp`),
			wantError: `profile "Local" values[0].target "missing" is not configured`,
		},
		{
			name: "duplicate profile target values",
			configContent: versionThreeConfig(validVersionThreeTargets(), `  - name: Local
    values:
      - target: database
        value: postgres://localhost:5432/myapp
      - target: database
        value: postgres://localhost:5433/myapp`),
			wantError: `profile "Local" has duplicate value for target "database"`,
		},
		{
			name: "profile value with both value fields",
			configContent: versionThreeConfig(validVersionThreeTargets(), `  - name: Local
    values:
      - target: database
        value: postgres://localhost:5432/myapp
        valueFromEnv: MYAPP_DATABASE_URL`),
			wantError: `profile "Local" value for target "database" must define exactly one of value or valueFromEnv`,
		},
		{
			name: "profile value with neither value field",
			configContent: versionThreeConfig(validVersionThreeTargets(), `  - name: Local
    values:
      - target: database`),
			wantError: `profile "Local" value for target "database" must define exactly one of value or valueFromEnv`,
		},
		{
			name: "profile value empty environment variable name",
			configContent: versionThreeConfig(validVersionThreeTargets(), `  - name: Local
    values:
      - target: database
        valueFromEnv: "   "`),
			wantError: `profile "Local" value for target "database" valueFromEnv must be set`,
		},
		{
			name: "version three rejects top-level profile value",
			configContent: versionThreeConfig(validVersionThreeTargets(), `  - name: Local
    value: postgres://localhost:5432/myapp`),
			wantError: `profile "Local" must define target values under values in version 3`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			configPath := writeFile(t, projectRoot, ".switchlet.yaml", testCase.configContent)

			_, err := config.Load(configPath)
			if err == nil {
				t.Fatal("Load returned nil error, want validation error")
			}

			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("Load returned error %q, want substring %q", err, testCase.wantError)
			}
		})
	}
}

func TestLoad_InfersVersionThreeTargetTypesFromUnambiguousFileNames(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config/appsettings.json
    jsonPath: ConnectionStrings.DefaultConnection
  - name: frontendApi
    file: frontend/.env.local
    key: VITE_API_URL
  - name: workerQueue
    file: worker/config.yaml
    yamlPath: queue.endpoint
  - name: workerMode
    file: worker/settings.yml
    yamlPath: worker.mode
  - name: serviceEndpoint
    file: services/development.toml
    tomlPath: services.api.endpoint

profiles:
  - name: Local
    values:
      - target: database
        value: Server=localhost;Database=App;
      - target: frontendApi
        value: http://localhost:5173
      - target: workerQueue
        value: http://localhost:4566/queue
      - target: workerMode
        value: local
      - target: serviceEndpoint
        value: http://localhost:8080
`)+"\n")

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loadedConfig.Targets[0].Type != config.TargetTypeJSON {
		t.Fatalf("Targets[0].Type = %q, want %q", loadedConfig.Targets[0].Type, config.TargetTypeJSON)
	}
	if loadedConfig.Targets[1].Type != config.TargetTypeDotenv {
		t.Fatalf("Targets[1].Type = %q, want %q", loadedConfig.Targets[1].Type, config.TargetTypeDotenv)
	}
	if loadedConfig.Targets[2].Type != config.TargetTypeYAML {
		t.Fatalf("Targets[2].Type = %q, want %q", loadedConfig.Targets[2].Type, config.TargetTypeYAML)
	}
	if loadedConfig.Targets[3].Type != config.TargetTypeYAML {
		t.Fatalf("Targets[3].Type = %q, want %q", loadedConfig.Targets[3].Type, config.TargetTypeYAML)
	}
	if loadedConfig.Targets[4].Type != config.TargetTypeTOML {
		t.Fatalf("Targets[4].Type = %q, want %q", loadedConfig.Targets[4].Type, config.TargetTypeTOML)
	}
	if loadedConfig.Targets[2].YAMLPath != "queue.endpoint" {
		t.Fatalf("Targets[2].YAMLPath = %q, want %q", loadedConfig.Targets[2].YAMLPath, "queue.endpoint")
	}
	if loadedConfig.Targets[4].TOMLPath != "services.api.endpoint" {
		t.Fatalf("Targets[4].TOMLPath = %q, want %q", loadedConfig.Targets[4].TOMLPath, "services.api.endpoint")
	}
}

func TestValidateVersionThreeDraft_ValidatesAndNormalizesDraftData(t *testing.T) {
	projectRoot := t.TempDir()

	loadedConfig, err := config.ValidateVersionThreeDraft(
		projectRoot,
		[]config.Target{{Name: "database", File: "config.json", Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://localhost:5432/myapp")}}}},
	)
	if err != nil {
		t.Fatalf("ValidateVersionThreeDraft returned error: %v", err)
	}

	if loadedConfig.Version != 3 {
		t.Fatalf("Version = %d, want 3", loadedConfig.Version)
	}
	if loadedConfig.Targets[0].File != filepath.Join(projectRoot, "config.json") {
		t.Fatalf("target file = %q, want project-relative path resolved", loadedConfig.Targets[0].File)
	}

	_, err = config.ValidateVersionThreeDraft(
		projectRoot,
		[]config.Target{{Name: "database", File: "config.json", Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Broken", Values: []config.ProfileValue{{Target: "missing", Value: stringPointer("postgres://localhost:5432/myapp")}}}},
	)
	if err == nil {
		t.Fatal("ValidateVersionThreeDraft returned nil error, want invalid draft error")
	}
	if !strings.Contains(err.Error(), `profile "Broken" values[0].target "missing" is not configured`) {
		t.Fatalf("ValidateVersionThreeDraft returned error %q, want target reference error", err)
	}
}

func TestLoad_AcceptsExplicitVersionThreeYAMLTargetForUnusualFileName(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", versionThreeConfig(`  - name: workerQueue
    file: worker/config.local
    type: yaml
    yamlPath: queue.endpoint`, `  - name: Local
    values:
      - target: workerQueue
        value: http://localhost:4566/queue`))

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loadedConfig.Targets[0].Type != config.TargetTypeYAML {
		t.Fatalf("Targets[0].Type = %q, want %q", loadedConfig.Targets[0].Type, config.TargetTypeYAML)
	}
	if loadedConfig.Targets[0].YAMLPath != "queue.endpoint" {
		t.Fatalf("Targets[0].YAMLPath = %q, want %q", loadedConfig.Targets[0].YAMLPath, "queue.endpoint")
	}
}

func TestLoad_AcceptsExplicitVersionThreeTOMLTargetForUnusualFileName(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", versionThreeConfig(`  - name: serviceEndpoint
    file: services/development.local
    type: toml
    tomlPath: services.api.endpoint`, `  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080`))

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loadedConfig.Targets[0].Type != config.TargetTypeTOML {
		t.Fatalf("Targets[0].Type = %q, want %q", loadedConfig.Targets[0].Type, config.TargetTypeTOML)
	}
	if loadedConfig.Targets[0].TOMLPath != "services.api.endpoint" {
		t.Fatalf("Targets[0].TOMLPath = %q, want %q", loadedConfig.Targets[0].TOMLPath, "services.api.endpoint")
	}
}

func TestLoad_AllowsDuplicateSelectorAcrossDifferentTargetTypes(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", versionThreeConfig(`  - name: jsonConfig
    file: config/shared.data
    type: json
    jsonPath: service.url
  - name: yamlConfig
    file: config/shared.data
    type: yaml
    yamlPath: service.url
  - name: tomlConfig
    file: config/shared.data
    type: toml
    tomlPath: service.url`, `  - name: Local
    values:
      - target: jsonConfig
        value: http://localhost:8080
      - target: yamlConfig
        value: http://localhost:8080
      - target: tomlConfig
        value: http://localhost:8080`))

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(loadedConfig.Targets) != 3 {
		t.Fatalf("len(Targets) = %d, want 3", len(loadedConfig.Targets))
	}
}

func TestParseTOMLPath_ValidatesBareKeySelectorContract(t *testing.T) {
	tests := []struct {
		name         string
		tomlPath     string
		wantSegments []string
		wantError    string
	}{
		{
			name:         "letters digits underscores and hyphens",
			tomlPath:     "services.api-v1.endpoint_url",
			wantSegments: []string{"services", "api-v1", "endpoint_url"},
		},
		{
			name:      "empty segment",
			tomlPath:  "services..endpoint",
			wantError: "path must contain non-empty dot-separated segments",
		},
		{
			name:      "leading whitespace in segment",
			tomlPath:  "services. api.endpoint",
			wantError: `segment " api" must not contain leading or trailing whitespace`,
		},
		{
			name:      "wildcard selector",
			tomlPath:  "services.*.endpoint",
			wantError: "wildcard selectors are not supported",
		},
		{
			name:      "array selector",
			tomlPath:  "services[0].endpoint",
			wantError: "array selectors are not supported",
		},
		{
			name:      "quoted key selector",
			tomlPath:  `services."api".endpoint`,
			wantError: "must use unquoted TOML bare-key syntax",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			segments, err := config.ParseTOMLPath(testCase.tomlPath)
			if testCase.wantError != "" {
				if err == nil {
					t.Fatal("ParseTOMLPath returned nil error, want validation error")
				}
				if !strings.Contains(err.Error(), testCase.wantError) {
					t.Fatalf("ParseTOMLPath returned error %q, want substring %q", err, testCase.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseTOMLPath returned error: %v", err)
			}
			if fmt.Sprint(segments) != fmt.Sprint(testCase.wantSegments) {
				t.Fatalf("segments = %#v, want %#v", segments, testCase.wantSegments)
			}
		})
	}
}

func legacyConfig(version int, targetFile string, connectionName string, profilesBlock string) string {
	return fmt.Sprintf(strings.TrimSpace(`
version: %d

target:
  file: %s
  connectionName: %s

profiles:
%s
`)+"\n", version, targetFile, connectionName, profilesBlock)
}

func versionTwoConfig(targetFile string, jsonPath string, profilesBlock string) string {
	return versionTwoConfigWithVersion(2, targetFile, jsonPath, profilesBlock)
}

func versionTwoConfigWithVersion(version int, targetFile string, jsonPath string, profilesBlock string) string {
	return fmt.Sprintf(strings.TrimSpace(`
version: %d

target:
  file: %s
  jsonPath: %s

profiles:
%s
`)+"\n", version, targetFile, jsonPath, profilesBlock)
}

func versionThreeConfig(targetsBlock string, profilesBlock string) string {
	return fmt.Sprintf(strings.TrimSpace(`
version: 3

targets:
%s

profiles:
%s
`)+"\n", targetsBlock, profilesBlock)
}

func validVersionThreeTargets() string {
	return `  - name: database
    file: config.json
    type: json
    jsonPath: database.url`
}

func validVersionThreeProfiles() string {
	return `  - name: Local
    values:
      - target: database
        value: postgres://localhost:5432/myapp`
}
