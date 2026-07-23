package config_test

import (
	"fmt"
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
			configContent: versionTwoConfigWithVersion(3, "appsettings.Development.json", "database.primary.url", `  - name: Local
    value: postgres://localhost:5432/myapp`),
			wantError: "unsupported version 3",
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
