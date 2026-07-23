package editor

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestListConnectionStringNames_ReturnsSortedStringValuedConnectionNames(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "Reporting": "Server=localhost;Database=Reporting;",
    "DefaultConnection": "Server=localhost;Database=App;",
    "RetryCount": 3
  }
}
`)+"\n")

	connectionNames, err := ListConnectionStringNames(targetPath)
	if err != nil {
		t.Fatalf("ListConnectionStringNames returned error: %v", err)
	}

	wantConnectionNames := []string{"DefaultConnection", "Reporting"}
	if !reflect.DeepEqual(connectionNames, wantConnectionNames) {
		t.Fatalf("connection names = %#v, want %#v", connectionNames, wantConnectionNames)
	}
}

func TestListConnectionStringNames_ReturnsErrorWhenNoStringValuedConnectionsExist(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", `{"ConnectionStrings":{"RetryCount":3}}`)

	_, err := ListConnectionStringNames(targetPath)
	if err == nil {
		t.Fatal("ListConnectionStringNames returned nil error, want no-string-connections error")
	}
	if !strings.Contains(err.Error(), "does not contain any string-valued connection strings") {
		t.Fatalf("ListConnectionStringNames returned error %q, want no-string-connections error", err)
	}
}

func TestUpdateStringValue_ReplacesTopLevelStringAndPreservesOtherValues(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "serviceUrl": "https://old.example.test",
  "MaxItems": 9007199254740993,
  "AllowedHosts": "*",
  "featureFlags": {
    "beta": true
  }
}
`)+"\n")

	replacementValue := "https://new.example.test"
	if err := UpdateStringValue(targetPath, "serviceUrl", replacementValue); err != nil {
		t.Fatalf("UpdateStringValue returned error: %v", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.HasSuffix(updatedContents, []byte("\n")) {
		t.Fatal("updated file does not end with a trailing newline")
	}

	rootObject := decodeJSONRoot(t, updatedContents)
	if rootObject["serviceUrl"] != replacementValue {
		t.Fatalf("serviceUrl = %q, want %q", rootObject["serviceUrl"], replacementValue)
	}
	if rootObject["AllowedHosts"] != "*" {
		t.Fatalf("AllowedHosts = %q, want %q", rootObject["AllowedHosts"], "*")
	}

	featureFlags := rootObject["featureFlags"].(map[string]any)
	if featureFlags["beta"] != true {
		t.Fatalf("featureFlags.beta = %v, want true", featureFlags["beta"])
	}

	maxItems, ok := rootObject["MaxItems"].(json.Number)
	if !ok {
		t.Fatalf("MaxItems has type %T, want json.Number", rootObject["MaxItems"])
	}
	if maxItems.String() != "9007199254740993" {
		t.Fatalf("MaxItems = %q, want %q", maxItems.String(), "9007199254740993")
	}
}

func TestUpdateStringValue_ReplacesNestedStringAndPreservesOtherValues(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old",
      "poolSize": 10
    },
    "replica": {
      "url": "postgres://replica"
    }
  },
  "environment": "development"
}
`)+"\n")

	replacementValue := "postgres://new"
	if err := UpdateStringValue(targetPath, "database.primary.url", replacementValue); err != nil {
		t.Fatalf("UpdateStringValue returned error: %v", err)
	}

	rootObject := decodeJSONRoot(t, readFile(t, targetPath))
	database := rootObject["database"].(map[string]any)
	primary := database["primary"].(map[string]any)
	replica := database["replica"].(map[string]any)

	if primary["url"] != replacementValue {
		t.Fatalf("database.primary.url = %q, want %q", primary["url"], replacementValue)
	}
	if primary["poolSize"] != json.Number("10") {
		t.Fatalf("database.primary.poolSize = %#v, want %#v", primary["poolSize"], json.Number("10"))
	}
	if replica["url"] != "postgres://replica" {
		t.Fatalf("database.replica.url = %q, want %q", replica["url"], "postgres://replica")
	}
	if rootObject["environment"] != "development" {
		t.Fatalf("environment = %q, want %q", rootObject["environment"], "development")
	}
}

func TestUpdateStringValue_ReturnsErrorForInvalidJSONPath(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"serviceUrl":"https://old.example.test"}`)

	err := UpdateStringValue(targetPath, "serviceUrl..current", "https://new.example.test")
	if err == nil {
		t.Fatal("UpdateStringValue returned nil error, want invalid path error")
	}
	if !strings.Contains(err.Error(), `invalid JSON path "serviceUrl..current"`) {
		t.Fatalf("UpdateStringValue returned error %q, want invalid path error", err)
	}
}

func TestUpdateStringValue_ReturnsErrorForInvalidTargetJSON(t *testing.T) {
	tests := []struct {
		name           string
		targetContents string
		jsonPath       string
		wantError      string
	}{
		{
			name:           "invalid JSON",
			targetContents: `{`,
			jsonPath:       `database.primary.url`,
			wantError:      `contains invalid JSON`,
		},
		{
			name:           "non-object root",
			targetContents: `[]`,
			jsonPath:       `database.primary.url`,
			wantError:      `must contain a JSON object at the root`,
		},
		{
			name:           "missing path segment",
			targetContents: `{"database":{"replica":{"url":"postgres://replica"}}}`,
			jsonPath:       `database.primary.url`,
			wantError:      `does not contain JSON path "database.primary.url": missing segment "primary"`,
		},
		{
			name:           "non-object intermediate segment",
			targetContents: `{"database":"postgres://old"}`,
			jsonPath:       `database.primary.url`,
			wantError:      `cannot continue through "database" because it is not an object`,
		},
		{
			name:           "non-string target value",
			targetContents: `{"database":{"primary":{"url":42}}}`,
			jsonPath:       `database.primary.url`,
			wantError:      `JSON path "database.primary.url" must resolve to a string`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "config.json", testCase.targetContents)

			originalContents := readFile(t, targetPath)
			err := UpdateStringValue(targetPath, testCase.jsonPath, "postgres://new")
			if err == nil {
				t.Fatal("UpdateStringValue returned nil error, want validation error")
			}

			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("UpdateStringValue returned error %q, want substring %q", err, testCase.wantError)
			}

			updatedContents := readFile(t, targetPath)
			if !bytes.Equal(updatedContents, originalContents) {
				t.Fatal("target file changed after validation failure")
			}
		})
	}
}
