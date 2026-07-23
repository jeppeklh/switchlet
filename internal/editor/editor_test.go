package editor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestUpdateConnectionString_ReplacesConfiguredConnectionAndPreservesOtherValues(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "Logging": {
    "LogLevel": {
      "Default": "Information"
    }
  },
  "MaxItems": 9007199254740993,
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;",
    "Reporting": "Server=localhost;Database=Reporting;"
  },
  "AllowedHosts": "*"
}
`)+"\n")

	replacementValue := "Server=test;Database=NewDatabase;User Id=test;Password=secret;"
	if err := UpdateConnectionString(targetPath, "DefaultConnection", replacementValue); err != nil {
		t.Fatalf("UpdateConnectionString returned error: %v", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.HasSuffix(updatedContents, []byte("\n")) {
		t.Fatal("updated file does not end with a trailing newline")
	}

	rootObject := decodeJSONRoot(t, updatedContents)
	connectionStrings := rootObject["ConnectionStrings"].(map[string]any)
	if connectionStrings["DefaultConnection"] != replacementValue {
		t.Fatalf("DefaultConnection = %q, want %q", connectionStrings["DefaultConnection"], replacementValue)
	}
	if connectionStrings["Reporting"] != "Server=localhost;Database=Reporting;" {
		t.Fatalf("Reporting = %q, want %q", connectionStrings["Reporting"], "Server=localhost;Database=Reporting;")
	}

	if rootObject["AllowedHosts"] != "*" {
		t.Fatalf("AllowedHosts = %q, want %q", rootObject["AllowedHosts"], "*")
	}

	logging := rootObject["Logging"].(map[string]any)
	logLevel := logging["LogLevel"].(map[string]any)
	if logLevel["Default"] != "Information" {
		t.Fatalf("LogLevel.Default = %q, want %q", logLevel["Default"], "Information")
	}

	maxItems, ok := rootObject["MaxItems"].(json.Number)
	if !ok {
		t.Fatalf("MaxItems has type %T, want json.Number", rootObject["MaxItems"])
	}
	if maxItems.String() != "9007199254740993" {
		t.Fatalf("MaxItems = %q, want %q", maxItems.String(), "9007199254740993")
	}
}

func TestUpdateConnectionString_ReturnsErrorForInvalidTargetJSON(t *testing.T) {
	tests := []struct {
		name           string
		targetContents string
		wantError      string
	}{
		{
			name:           "invalid JSON",
			targetContents: `{`,
			wantError:      `contains invalid JSON`,
		},
		{
			name:           "non-object root",
			targetContents: `[]`,
			wantError:      `must contain a JSON object at the root`,
		},
		{
			name:           "missing ConnectionStrings",
			targetContents: `{"Logging":{}}`,
			wantError:      `must contain a ConnectionStrings object`,
		},
		{
			name:           "non-object ConnectionStrings",
			targetContents: `{"ConnectionStrings":"invalid"}`,
			wantError:      `ConnectionStrings must be an object`,
		},
		{
			name:           "missing configured connection string",
			targetContents: `{"ConnectionStrings":{"Reporting":"Server=localhost;Database=Reporting;"}}`,
			wantError:      `does not contain connection string "DefaultConnection"`,
		},
		{
			name:           "non-string configured connection value",
			targetContents: `{"ConnectionStrings":{"DefaultConnection":42}}`,
			wantError:      `connection string "DefaultConnection" must be a string`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", testCase.targetContents)

			originalContents := readFile(t, targetPath)
			err := UpdateConnectionString(targetPath, "DefaultConnection", "Server=test;Database=NewDatabase;")
			if err == nil {
				t.Fatal("UpdateConnectionString returned nil error, want validation error")
			}

			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("UpdateConnectionString returned error %q, want substring %q", err, testCase.wantError)
			}

			updatedContents := readFile(t, targetPath)
			if !bytes.Equal(updatedContents, originalContents) {
				t.Fatal("target file changed after validation failure")
			}
		})
	}
}
