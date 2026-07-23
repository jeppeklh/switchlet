package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestLoad_ReturnsErrorForInvalidConfiguration(t *testing.T) {
	const validTarget = `
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=MyApplication;"
  }
}
`

	tests := []struct {
		name          string
		configContent string
		targetPath    string
		targetContent *string
		wantError     string
	}{
		{
			name: "missing version",
			configContent: strings.TrimSpace(`
target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`) + "\n",
			targetContent: stringPointer(validTarget),
			wantError:     "version must be set",
		},
		{
			name: "unsupported version",
			configContent: baseConfig(2, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(validTarget),
			wantError:     "unsupported version 2",
		},
		{
			name: "missing target file",
			configContent: strings.TrimSpace(`
version: 1

target:
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`) + "\n",
			targetContent: stringPointer(validTarget),
			wantError:     "target.file must be set",
		},
		{
			name: "missing connection name",
			configContent: strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`) + "\n",
			targetContent: stringPointer(validTarget),
			wantError:     "target.connectionName must be set",
		},
		{
			name: "empty profile list",
			configContent: strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles: []
`) + "\n",
			targetContent: stringPointer(validTarget),
			wantError:     "at least one profile must be configured",
		},
		{
			name: "empty profile name",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: ""
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(validTarget),
			wantError:     "profiles[0].name must be set",
		},
		{
			name: "duplicate profile names",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"
  - name: Local
    valueFromEnv: MYAPPLICATION_TEST_CONNECTION_STRING`),
			targetContent: stringPointer(validTarget),
			wantError:     `duplicate profile name "Local"`,
		},
		{
			name: "profile with both value fields",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"
    valueFromEnv: MYAPPLICATION_TEST_CONNECTION_STRING`),
			targetContent: stringPointer(validTarget),
			wantError:     `profile "Local" must define exactly one of value or valueFromEnv`,
		},
		{
			name:          "profile with neither value field",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local`),
			targetContent: stringPointer(validTarget),
			wantError:     `profile "Local" must define exactly one of value or valueFromEnv`,
		},
		{
			name: "empty environment variable name",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Test
    valueFromEnv: "   "`),
			targetContent: stringPointer(validTarget),
			wantError:     `profile "Test" valueFromEnv must be set`,
		},
		{
			name: "missing target file on disk",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			wantError: `target file`,
		},
		{
			name: "invalid target json",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(`{`),
			wantError:     `contains invalid JSON`,
		},
		{
			name: "non-object target root",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(`[]`),
			wantError:     `must contain a JSON object at the root`,
		},
		{
			name: "missing ConnectionStrings object",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(`{"Logging":{}}`),
			wantError:     `must contain a ConnectionStrings object`,
		},
		{
			name: "non-object ConnectionStrings value",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(`{"ConnectionStrings":"invalid"}`),
			wantError:     `ConnectionStrings must be an object`,
		},
		{
			name: "missing configured connection string",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(`{"ConnectionStrings":{"AnotherConnection":"Server=localhost;Database=MyApplication;"}}`),
			wantError:     `does not contain connection string "DefaultConnection"`,
		},
		{
			name: "non-string configured connection value",
			configContent: baseConfig(1, "appsettings.Development.json", "DefaultConnection", `  - name: Local
    value: "Server=localhost;Database=MyApplication;"`),
			targetContent: stringPointer(`{"ConnectionStrings":{"DefaultConnection":42}}`),
			wantError:     `connection string "DefaultConnection" must be a string`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			configPath := writeFile(t, projectRoot, ".switchlet.yaml", testCase.configContent)

			if testCase.targetContent != nil {
				targetPath := testCase.targetPath
				if targetPath == "" {
					targetPath = "appsettings.Development.json"
				}

				writeFile(t, projectRoot, targetPath, *testCase.targetContent)
			}

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

func baseConfig(version int, targetFile string, connectionName string, profilesBlock string) string {
	return fmt.Sprintf(strings.TrimSpace(`
version: %d

target:
  file: %s
  connectionName: %s

profiles:
%s
`)+"\n", version, targetFile, connectionName, profilesBlock)
}
