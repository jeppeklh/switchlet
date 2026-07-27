package editor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
	"gopkg.in/yaml.v3"
)

func TestValidateTarget_AcceptsExistingYAMLPath(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: http://localhost:4566/queue
  retries: 3
`)+"\n")

	err := ValidateTarget(config.Target{
		Name:     "workerQueue",
		File:     targetPath,
		Type:     config.TargetTypeYAML,
		YAMLPath: "queue.endpoint",
	})
	if err != nil {
		t.Fatalf("ValidateTarget returned error: %v", err)
	}
}

func TestApplyTargetChanges_MergesYAMLTargetsInOneFileAndPreservesCommentsOrderAndSemantics(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
# worker settings
serviceUrl: http://old-service.example.test
queue:
  # queue endpoint stays documented
  endpoint: http://old-queue.example.test
  retries: 3
features:
  enabled: true
`)+"\n")
	var writtenTargets []string
	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		writtenTargets = append(writtenTargets, newPath)
		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "service", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "serviceUrl"},
			Value:  "http://new-service.example.test",
		},
		{
			Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			Value:  "http://new-queue.example.test",
		},
	})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}
	if len(writtenTargets) != 1 || writtenTargets[0] != targetPath {
		t.Fatalf("written targets = %#v, want one write to %q", writtenTargets, targetPath)
	}

	updatedContents := readFile(t, targetPath)
	updatedText := string(updatedContents)
	for _, wantComment := range []string{"# worker settings", "# queue endpoint stays documented"} {
		if !strings.Contains(updatedText, wantComment) {
			t.Fatalf("updated YAML %q does not preserve comment %q", updatedText, wantComment)
		}
	}
	for _, orderedPair := range [][2]string{{"serviceUrl:", "queue:"}, {"queue:", "features:"}} {
		leftIndex := strings.Index(updatedText, orderedPair[0])
		rightIndex := strings.Index(updatedText, orderedPair[1])
		if leftIndex < 0 || rightIndex < 0 || leftIndex > rightIndex {
			t.Fatalf("updated YAML does not preserve order %q before %q:\n%s", orderedPair[0], orderedPair[1], updatedText)
		}
	}

	root := decodeYAMLRoot(t, updatedContents)
	if root["serviceUrl"] != "http://new-service.example.test" {
		t.Fatalf("serviceUrl = %q, want updated value", root["serviceUrl"])
	}
	queue := root["queue"].(map[string]any)
	if queue["endpoint"] != "http://new-queue.example.test" {
		t.Fatalf("queue.endpoint = %q, want updated value", queue["endpoint"])
	}
	if queue["retries"] != 3 {
		t.Fatalf("queue.retries = %#v, want unchanged integer", queue["retries"])
	}
	features := root["features"].(map[string]any)
	if features["enabled"] != true {
		t.Fatalf("features.enabled = %#v, want unchanged boolean", features["enabled"])
	}
}

func TestPreviewTargetChanges_ValidatesYAMLWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: http://old-queue.example.test
`)+"\n")
	originalContents := readFile(t, targetPath)

	err := PreviewTargetChanges([]TargetChange{{
		Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
		Value:  "http://new-queue.example.test",
	}})
	if err != nil {
		t.Fatalf("PreviewTargetChanges returned error: %v", err)
	}

	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("YAML target changed during preview")
	}
}

func TestApplyTargetChanges_PreservesYAMLFilePermissions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: http://old-queue.example.test\n")

	if err := os.Chmod(targetPath, 0o600); err != nil {
		t.Fatalf("set file permissions: %v", err)
	}
	originalInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat original file: %v", err)
	}

	err = ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
		Value:  "http://new-queue.example.test",
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	updatedInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat updated file: %v", err)
	}
	if updatedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("file permissions = %o, want %o", updatedInfo.Mode().Perm(), originalInfo.Mode().Perm())
	}
}

func TestApplyTargetChanges_YAMLValidationFailuresLeaveFileUnchangedAndHideSecrets(t *testing.T) {
	tests := []struct {
		name           string
		targetContents string
		yamlPath       string
		wantError      string
	}{
		{
			name:           "invalid YAML",
			targetContents: `{`,
			yamlPath:       "queue.endpoint",
			wantError:      "contains invalid YAML",
		},
		{
			name:           "multiple documents",
			targetContents: "queue:\n  endpoint: http://old.example.test\n---\nqueue:\n  endpoint: http://other.example.test\n",
			yamlPath:       "queue.endpoint",
			wantError:      "multiple YAML documents are not supported",
		},
		{
			name:           "non-mapping root",
			targetContents: "- http://old.example.test\n",
			yamlPath:       "queue.endpoint",
			wantError:      "must contain a YAML mapping at the root",
		},
		{
			name:           "missing path",
			targetContents: "queue:\n  other: http://old.example.test\n",
			yamlPath:       "queue.endpoint",
			wantError:      `missing segment "endpoint"`,
		},
		{
			name:           "non-mapping intermediate",
			targetContents: "queue: http://old.example.test\n",
			yamlPath:       "queue.endpoint",
			wantError:      `cannot continue through "queue" because it is not a mapping`,
		},
		{
			name:           "non-string final value",
			targetContents: "queue:\n  retries: 3\n",
			yamlPath:       "queue.retries",
			wantError:      `YAML path "queue.retries" must resolve to a scalar string`,
		},
		{
			name:           "duplicate key on selected path",
			targetContents: "queue:\n  endpoint: http://old.example.test\n  endpoint: http://other.example.test\n",
			yamlPath:       "queue.endpoint",
			wantError:      `duplicate key "endpoint"`,
		},
		{
			name:           "sequence index path",
			targetContents: "queue:\n  - endpoint: http://old.example.test\n",
			yamlPath:       "queue.0.endpoint",
			wantError:      `cannot continue through "queue" because it is not a mapping`,
		},
		{
			name: "merge key path",
			targetContents: strings.TrimSpace(`
defaults: &defaults
  endpoint: http://default.example.test
queue:
  <<: *defaults
`) + "\n",
			yamlPath:  "queue.endpoint",
			wantError: "unsupported YAML merge key",
		},
		{
			name: "alias path",
			targetContents: strings.TrimSpace(`
defaults: &defaults
  endpoint: http://default.example.test
queue: *defaults
`) + "\n",
			yamlPath:  "queue.endpoint",
			wantError: "cannot use aliases",
		},
		{
			name:           "anchored target value",
			targetContents: "queue:\n  endpoint: &endpoint http://old.example.test\n",
			yamlPath:       "queue.endpoint",
			wantError:      "cannot use anchored nodes",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", testCase.targetContents)
			originalContents := readFile(t, targetPath)

			err := ApplyTargetChanges([]TargetChange{{
				Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: testCase.yamlPath},
				Value:  "https://secret-value.example.test",
			}})
			if err == nil {
				t.Fatal("ApplyTargetChanges returned nil error, want YAML validation failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, testCase.wantError)
			}
			if !strings.Contains(err.Error(), `target "workerQueue"`) || !strings.Contains(err.Error(), `yamlPath "`+testCase.yamlPath+`"`) {
				t.Fatalf("ApplyTargetChanges returned error %q, want target and YAML path context", err)
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
			}
			if !bytes.Equal(readFile(t, targetPath), originalContents) {
				t.Fatal("YAML file changed after validation failure")
			}
		})
	}
}

func TestInspectYAMLStringTargets_ReturnsHierarchicalNodes(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
services:
  worker:
    baseUrl: https://worker.example.test
    replicas:
      - https://replica.example.test
queue:
  endpoint: http://queue.example.test
  retries: 3
features:
  defaultMode: local
  enabled: true
`)+"\n")

	nodes, err := InspectYAMLStringTargets(targetPath)
	if err != nil {
		t.Fatalf("InspectYAMLStringTargets returned error: %v", err)
	}

	wantNodes := []YAMLStringTargetNode{
		{
			Name:     "services",
			YAMLPath: "services",
			Children: []YAMLStringTargetNode{{
				Name:     "worker",
				YAMLPath: "services.worker",
				Children: []YAMLStringTargetNode{{
					Name:       "baseUrl",
					YAMLPath:   "services.worker.baseUrl",
					Selectable: true,
				}},
			}},
		},
		{
			Name:     "queue",
			YAMLPath: "queue",
			Children: []YAMLStringTargetNode{{
				Name:       "endpoint",
				YAMLPath:   "queue.endpoint",
				Selectable: true,
			}},
		},
		{
			Name:     "features",
			YAMLPath: "features",
			Children: []YAMLStringTargetNode{{
				Name:       "defaultMode",
				YAMLPath:   "features.defaultMode",
				Selectable: true,
			}},
		},
	}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", nodes, wantNodes)
	}
}

func TestInspectYAMLStringTargets_ReturnsErrorWhenNoSelectablePathsExist(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  retries: 3
services:
  - https://worker.example.test
`)+"\n")

	_, err := InspectYAMLStringTargets(targetPath)
	if err == nil {
		t.Fatal("InspectYAMLStringTargets returned nil error, want no-selectable-paths error")
	}
	if !strings.Contains(err.Error(), "does not contain any existing string-valued YAML paths") {
		t.Fatalf("InspectYAMLStringTargets returned error %q, want no-selectable-paths error", err)
	}
}

func TestApplyTargetChanges_JSONPlusYAMLPreparationFailureLeavesEveryFileUnchanged(t *testing.T) {
	projectRoot := t.TempDir()
	jsonPath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	yamlPath := writeTargetFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: http://old-queue.example.test\n")
	originalJSONContents := readFile(t, jsonPath)
	originalYAMLContents := readFile(t, yamlPath)

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: jsonPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://secret-database",
		},
		{
			Target: config.Target{Name: "workerQueue", File: yamlPath, Type: config.TargetTypeYAML, YAMLPath: "queue.missing"},
			Value:  "http://secret-queue.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want YAML preparation failure")
	}
	if !strings.Contains(err.Error(), `target "workerQueue"`) || !strings.Contains(err.Error(), `yamlPath "queue.missing"`) {
		t.Fatalf("ApplyTargetChanges returned error %q, want YAML target context", err)
	}
	for _, forbidden := range []string{"secret-database", "secret-queue"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if !bytes.Equal(readFile(t, jsonPath), originalJSONContents) {
		t.Fatal("JSON file changed after YAML preparation failure")
	}
	if !bytes.Equal(readFile(t, yamlPath), originalYAMLContents) {
		t.Fatal("YAML file changed after YAML preparation failure")
	}
}

func TestApplyTargetChanges_YAMLPlusDotenvPreparationFailureLeavesEveryFileUnchanged(t *testing.T) {
	projectRoot := t.TempDir()
	yamlPath := writeTargetFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: http://old-queue.example.test\n")
	dotenvPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalYAMLContents := readFile(t, yamlPath)
	originalDotenvContents := readFile(t, dotenvPath)

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "workerQueue", File: yamlPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			Value:  "http://secret-queue.example.test",
		},
		{
			Target: config.Target{Name: "frontendApi", File: dotenvPath, Type: config.TargetTypeDotenv, Key: "MISSING_KEY"},
			Value:  "https://secret-api.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want dotenv preparation failure")
	}
	if !strings.Contains(err.Error(), `target "frontendApi"`) || !strings.Contains(err.Error(), `key "MISSING_KEY"`) {
		t.Fatalf("ApplyTargetChanges returned error %q, want dotenv target context", err)
	}
	for _, forbidden := range []string{"secret-queue", "secret-api"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if !bytes.Equal(readFile(t, yamlPath), originalYAMLContents) {
		t.Fatal("YAML file changed after dotenv preparation failure")
	}
	if !bytes.Equal(readFile(t, dotenvPath), originalDotenvContents) {
		t.Fatal("dotenv file changed after dotenv preparation failure")
	}
}

func TestApplyTargetChanges_WriteFailureAfterYAMLSuccessReportsPartialStateAndCleansTemporaryFile(t *testing.T) {
	projectRoot := t.TempDir()
	yamlPath := writeTargetFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: http://old-queue.example.test\n")
	jsonPath := writeTargetFile(t, projectRoot, "backend/config.json", `{"api":{"url":"https://old.example.test"}}`)
	originalJSONContents := readFile(t, jsonPath)

	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		if newPath == jsonPath {
			return errors.New("rename failed")
		}

		return originalReplaceFile(oldPath, newPath)
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "workerQueue", File: yamlPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			Value:  "http://new-queue.example.test",
		},
		{
			Target: config.Target{Name: "api", File: jsonPath, Type: config.TargetTypeJSON, JSONPath: "api.url"},
			Value:  "https://secret-api.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want second-file write failure")
	}
	for _, expected := range []string{"after 1 file(s) were already replaced", "target files may now be partially updated", "rename failed"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, expected)
		}
	}
	if strings.Contains(err.Error(), "secret-api") {
		t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
	}

	yamlRoot := decodeYAMLRoot(t, readFile(t, yamlPath))
	queue := yamlRoot["queue"].(map[string]any)
	if queue["endpoint"] != "http://new-queue.example.test" {
		t.Fatalf("queue.endpoint = %q, want first YAML file to have been replaced before second failure", queue["endpoint"])
	}
	if !bytes.Equal(readFile(t, jsonPath), originalJSONContents) {
		t.Fatal("second target file changed after its replacement failed")
	}
	if containsTempFile(t, filepath.Dir(jsonPath), tempFilePrefix(jsonPath)) {
		t.Fatal("temporary file was not cleaned up after second-file rename failure")
	}
}

func decodeYAMLRoot(t *testing.T, contents []byte) map[string]any {
	t.Helper()

	var decodedRoot map[string]any
	if err := yaml.Unmarshal(contents, &decodedRoot); err != nil {
		t.Fatalf("decode YAML: %v", err)
	}

	return decodedRoot
}
