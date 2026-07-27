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
	toml "github.com/pelletier/go-toml/v2"
)

func TestValidateTarget_AcceptsExistingTOMLPath(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", strings.TrimSpace(`
[queue]
endpoint = "http://localhost:4566/queue"
retries = 3
`)+"\n")

	err := ValidateTarget(config.Target{
		Name:     "workerQueue",
		File:     targetPath,
		Type:     config.TargetTypeTOML,
		TOMLPath: "queue.endpoint",
	})
	if err != nil {
		t.Fatalf("ValidateTarget returned error: %v", err)
	}
}

func TestApplyTargetChanges_MergesTOMLTargetsInOneFileAndPreservesCommentsOrderAndSemantics(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", strings.TrimSpace(`
# worker settings
serviceUrl = "http://old-service.example.test"
retries = 3

[queue]
# queue endpoint stays documented
endpoint = "http://old-queue.example.test"
enabled = true
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
			Target: config.Target{Name: "service", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "serviceUrl"},
			Value:  "http://new-service.example.test",
		},
		{
			Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
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
			t.Fatalf("updated TOML %q does not preserve comment %q", updatedText, wantComment)
		}
	}
	lineIndexes := map[string]int{}
	for index, line := range strings.Split(strings.TrimSuffix(updatedText, "\n"), "\n") {
		lineIndexes[line] = index
	}
	orderedLines := []string{
		"# worker settings",
		`serviceUrl = "http://new-service.example.test"`,
		"retries = 3",
		"[queue]",
		"# queue endpoint stays documented",
		`endpoint = "http://new-queue.example.test"`,
		"enabled = true",
	}
	for index, line := range orderedLines {
		lineIndex, ok := lineIndexes[line]
		if !ok {
			t.Fatalf("updated TOML does not contain line %q:\n%s", line, updatedText)
		}
		if index > 0 && lineIndex <= lineIndexes[orderedLines[index-1]] {
			t.Fatalf("updated TOML does not preserve line order %q before %q:\n%s", orderedLines[index-1], line, updatedText)
		}
	}

	root := decodeTOMLRoot(t, updatedContents)
	if root["serviceUrl"] != "http://new-service.example.test" {
		t.Fatalf("serviceUrl = %q, want updated value", root["serviceUrl"])
	}
	if root["retries"] != int64(3) {
		t.Fatalf("retries = %#v, want unchanged integer", root["retries"])
	}
	queue := root["queue"].(map[string]any)
	if queue["endpoint"] != "http://new-queue.example.test" {
		t.Fatalf("queue.endpoint = %q, want updated value", queue["endpoint"])
	}
	if queue["enabled"] != true {
		t.Fatalf("queue.enabled = %#v, want unchanged boolean", queue["enabled"])
	}
}

func TestApplyTargetChanges_ReplacesDottedTOMLKeyWhenValueRangeIsSafe(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.toml", strings.TrimSpace(`
services.api.endpoint = "http://old-api.example.test"
services.api.timeout = 30
`)+"\n")

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "api", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"},
		Value:  "http://new-api.example.test",
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	updatedText := string(readFile(t, targetPath))
	if !strings.Contains(updatedText, `services.api.endpoint = "http://new-api.example.test"`) {
		t.Fatalf("updated TOML does not replace dotted key value:\n%s", updatedText)
	}
	if !strings.Contains(updatedText, `services.api.timeout = 30`) {
		t.Fatalf("updated TOML does not preserve unrelated dotted key:\n%s", updatedText)
	}
}

func TestApplyTargetChanges_QuotesTOMLReplacementString(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.toml", "[queue]\nendpoint = \"old\"\n")
	replacementValue := "line \"one\"\npath\\next"

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "queue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
		Value:  replacementValue,
	}})
	if err != nil {
		t.Fatalf("ApplyTargetChanges returned error: %v", err)
	}

	updatedContents := readFile(t, targetPath)
	updatedText := string(updatedContents)
	if strings.Contains(updatedText, replacementValue) {
		t.Fatalf("updated TOML contains raw replacement value instead of escaped TOML string:\n%s", updatedText)
	}
	root := decodeTOMLRoot(t, updatedContents)
	queue := root["queue"].(map[string]any)
	if queue["endpoint"] != replacementValue {
		t.Fatalf("queue.endpoint = %q, want %q", queue["endpoint"], replacementValue)
	}
}

func TestPreviewTargetChanges_ValidatesTOMLWithoutWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", strings.TrimSpace(`
[queue]
endpoint = "http://old-queue.example.test"
`)+"\n")
	originalContents := readFile(t, targetPath)

	err := PreviewTargetChanges([]TargetChange{{
		Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
		Value:  "http://new-queue.example.test",
	}})
	if err != nil {
		t.Fatalf("PreviewTargetChanges returned error: %v", err)
	}

	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("TOML target changed during preview")
	}
}

func TestApplyTargetChanges_PreservesTOMLFilePermissions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", "[queue]\nendpoint = \"http://old-queue.example.test\"\n")

	if err := os.Chmod(targetPath, 0o600); err != nil {
		t.Fatalf("set file permissions: %v", err)
	}
	originalInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat original file: %v", err)
	}

	err = ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
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

func TestApplyTargetChanges_RejectsDuplicateTOMLTargetLocationBeforeWriting(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", "[queue]\nendpoint = \"http://old-queue.example.test\"\n")
	originalContents := readFile(t, targetPath)

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
			Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
			Value:  "http://secret-queue-one.example.test",
		},
		{
			Target: config.Target{Name: "reportingQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
			Value:  "http://secret-queue-two.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want duplicate TOML target-location failure")
	}
	if !strings.Contains(err.Error(), `target "reportingQueue"`) || !strings.Contains(err.Error(), `duplicates target location used by target "workerQueue"`) || !strings.Contains(err.Error(), `tomlPath "queue.endpoint"`) {
		t.Fatalf("ApplyTargetChanges returned error %q, want duplicate TOML target-location context", err)
	}
	for _, forbidden := range []string{"secret-queue-one", "secret-queue-two"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if len(writtenTargets) != 0 {
		t.Fatalf("written targets = %#v, want no writes after duplicate TOML location failure", writtenTargets)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("TOML file changed after duplicate TOML target-location failure")
	}
}

func TestApplyTargetChanges_TOMLValidationFailuresLeaveFileUnchangedAndHideSecrets(t *testing.T) {
	tests := []struct {
		name           string
		targetContents string
		tomlPath       string
		wantError      string
	}{
		{
			name:           "invalid TOML",
			targetContents: `serviceUrl = "unterminated`,
			tomlPath:       "queue.endpoint",
			wantError:      "contains invalid TOML",
		},
		{
			name:           "duplicate TOML path",
			targetContents: "[queue]\nendpoint = \"http://old.example.test\"\nendpoint = \"http://other.example.test\"\n",
			tomlPath:       "queue.endpoint",
			wantError:      "contains invalid TOML",
		},
		{
			name:           "missing path",
			targetContents: "[queue]\nother = \"http://old.example.test\"\n",
			tomlPath:       "queue.endpoint",
			wantError:      `missing segment "endpoint"`,
		},
		{
			name:           "non-table intermediate scalar",
			targetContents: "queue = \"http://old.example.test\"\n",
			tomlPath:       "queue.endpoint",
			wantError:      `cannot continue through "queue" because it is not a table`,
		},
		{
			name:           "non-string final value",
			targetContents: "[queue]\nretries = 3\n",
			tomlPath:       "queue.retries",
			wantError:      `TOML path "queue.retries" must resolve to a string`,
		},
		{
			name:           "array intermediate",
			targetContents: "services = [{ endpoint = \"http://old.example.test\" }]\n",
			tomlPath:       "services.endpoint",
			wantError:      `cannot continue through "services" because arrays are not supported`,
		},
		{
			name:           "array table path",
			targetContents: "[[services]]\nendpoint = \"http://old.example.test\"\n",
			tomlPath:       "services.endpoint",
			wantError:      `uses unsupported array table at "services"`,
		},
		{
			name:           "inline table member",
			targetContents: "services = { endpoint = \"http://old.example.test\" }\n",
			tomlPath:       "services.endpoint",
			wantError:      `cannot continue through "services" because inline tables are not supported`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", testCase.targetContents)
			originalContents := readFile(t, targetPath)

			err := ApplyTargetChanges([]TargetChange{{
				Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: testCase.tomlPath},
				Value:  "https://secret-value.example.test",
			}})
			if err == nil {
				t.Fatal("ApplyTargetChanges returned nil error, want TOML validation failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("ApplyTargetChanges returned error %q, want substring %q", err, testCase.wantError)
			}
			if !strings.Contains(err.Error(), `target "workerQueue"`) || !strings.Contains(err.Error(), `tomlPath "`+testCase.tomlPath+`"`) {
				t.Fatalf("ApplyTargetChanges returned error %q, want target and TOML path context", err)
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
			}
			if !bytes.Equal(readFile(t, targetPath), originalContents) {
				t.Fatal("TOML file changed after validation failure")
			}
		})
	}
}

func TestApplyTargetChanges_RejectsTOMLReplacementWithInvalidUTF8(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", "[queue]\nendpoint = \"http://old-queue.example.test\"\n")
	originalContents := readFile(t, targetPath)

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
		Value:  string([]byte{0xff, 0xfe}),
	}})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want invalid UTF-8 failure")
	}
	if !strings.Contains(err.Error(), "replacement value must be valid UTF-8") {
		t.Fatalf("ApplyTargetChanges returned error %q, want invalid UTF-8 error", err)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("TOML file changed after invalid UTF-8 failure")
	}
}

func TestInspectTOMLStringTargets_ReturnsHierarchicalNodes(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", strings.TrimSpace(`
serviceUrl = "https://service.example.test"
services.worker.baseUrl = "https://worker.example.test"
services.worker.retries = 3

[queue]
endpoint = "http://queue.example.test"
retries = 3

[features]
defaultMode = "local"
enabled = true
`)+"\n")

	nodes, err := InspectTOMLStringTargets(targetPath)
	if err != nil {
		t.Fatalf("InspectTOMLStringTargets returned error: %v", err)
	}

	wantNodes := []TOMLStringTargetNode{
		{
			Name:       "serviceUrl",
			TOMLPath:   "serviceUrl",
			Selectable: true,
		},
		{
			Name:     "services",
			TOMLPath: "services",
			Children: []TOMLStringTargetNode{{
				Name:     "worker",
				TOMLPath: "services.worker",
				Children: []TOMLStringTargetNode{{
					Name:       "baseUrl",
					TOMLPath:   "services.worker.baseUrl",
					Selectable: true,
				}},
			}},
		},
		{
			Name:     "queue",
			TOMLPath: "queue",
			Children: []TOMLStringTargetNode{{
				Name:       "endpoint",
				TOMLPath:   "queue.endpoint",
				Selectable: true,
			}},
		},
		{
			Name:     "features",
			TOMLPath: "features",
			Children: []TOMLStringTargetNode{{
				Name:       "defaultMode",
				TOMLPath:   "features.defaultMode",
				Selectable: true,
			}},
		},
	}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", nodes, wantNodes)
	}
}

func TestInspectTOMLStringTargets_SkipsUnsupportedPathShapeKeysAndUnsupportedStructures(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", strings.TrimSpace(`
[queue]
endpoint = "http://queue.example.test"
"endpoint.with.dot" = "http://hidden.example.test"
" leadingSpace" = "http://hidden.example.test"
"trailingSpace " = "http://hidden.example.test"
inline = { endpoint = "http://hidden.example.test" }
replicas = ["http://hidden.example.test"]

[[workers]]
endpoint = "http://hidden.example.test"

[workers.metadata]
visible = "http://hidden.example.test"

[nested]
visible = "local"
`)+"\n")

	nodes, err := InspectTOMLStringTargets(targetPath)
	if err != nil {
		t.Fatalf("InspectTOMLStringTargets returned error: %v", err)
	}

	wantNodes := []TOMLStringTargetNode{
		{
			Name:     "queue",
			TOMLPath: "queue",
			Children: []TOMLStringTargetNode{{
				Name:       "endpoint",
				TOMLPath:   "queue.endpoint",
				Selectable: true,
			}},
		},
		{
			Name:     "nested",
			TOMLPath: "nested",
			Children: []TOMLStringTargetNode{{
				Name:       "visible",
				TOMLPath:   "nested.visible",
				Selectable: true,
			}},
		},
	}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", nodes, wantNodes)
	}
}

func TestInspectTOMLStringTargets_ReturnsErrorWhenNoSelectablePathsExist(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", strings.TrimSpace(`
[queue]
retries = 3
services = ["https://worker.example.test"]
`)+"\n")

	_, err := InspectTOMLStringTargets(targetPath)
	if err == nil {
		t.Fatal("InspectTOMLStringTargets returned nil error, want no-selectable-paths error")
	}
	if !strings.Contains(err.Error(), "does not contain any existing string-valued TOML paths") {
		t.Fatalf("InspectTOMLStringTargets returned error %q, want no-selectable-paths error", err)
	}
}

func TestDiscoverTargetFileCandidates_IncludesInspectableTOMLFiles(t *testing.T) {
	projectRoot := t.TempDir()

	writeTargetFile(t, projectRoot, "root.json", `{"serviceUrl":"https://old.example.test"}`)
	writeTargetFile(t, projectRoot, "worker.yaml", "queue:\n  endpoint: https://old.example.test\n")
	writeTargetFile(t, projectRoot, "settings.toml", `serviceUrl = "https://old.example.test"`)
	writeTargetFile(t, projectRoot, "numbers.toml", "retries = 3\n")
	writeTargetFile(t, projectRoot, "inline-only.toml", `service = { url = "https://old.example.test" }`)

	candidates, err := DiscoverTargetFileCandidates(projectRoot)
	if err != nil {
		t.Fatalf("DiscoverTargetFileCandidates returned error: %v", err)
	}

	gotRelativePaths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		gotRelativePaths = append(gotRelativePaths, candidate.RelativePath)
	}

	wantRelativePaths := []string{
		"root.json",
		"settings.toml",
		"worker.yaml",
	}
	if !reflect.DeepEqual(gotRelativePaths, wantRelativePaths) {
		t.Fatalf("relative paths = %#v, want %#v", gotRelativePaths, wantRelativePaths)
	}
	for _, candidate := range candidates {
		if candidate.RelativePath == "settings.toml" && candidate.Type != config.TargetTypeTOML {
			t.Fatalf("settings.toml candidate type = %q, want %q", candidate.Type, config.TargetTypeTOML)
		}
	}
}

func TestApplyTargetChanges_JSONPlusTOMLPreparationFailureLeavesEveryFileUnchanged(t *testing.T) {
	projectRoot := t.TempDir()
	jsonPath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	tomlPath := writeTargetFile(t, projectRoot, "worker/config.toml", "[queue]\nendpoint = \"http://old-queue.example.test\"\n")
	originalJSONContents := readFile(t, jsonPath)
	originalTOMLContents := readFile(t, tomlPath)

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "database", File: jsonPath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			Value:  "postgres://secret-database",
		},
		{
			Target: config.Target{Name: "workerQueue", File: tomlPath, Type: config.TargetTypeTOML, TOMLPath: "queue.missing"},
			Value:  "http://secret-queue.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want TOML preparation failure")
	}
	if !strings.Contains(err.Error(), `target "workerQueue"`) || !strings.Contains(err.Error(), `tomlPath "queue.missing"`) {
		t.Fatalf("ApplyTargetChanges returned error %q, want TOML target context", err)
	}
	for _, forbidden := range []string{"secret-database", "secret-queue"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if !bytes.Equal(readFile(t, jsonPath), originalJSONContents) {
		t.Fatal("JSON file changed after TOML preparation failure")
	}
	if !bytes.Equal(readFile(t, tomlPath), originalTOMLContents) {
		t.Fatal("TOML file changed after TOML preparation failure")
	}
}

func TestApplyTargetChanges_YAMLPlusTOMLPreparationFailureLeavesEveryFileUnchanged(t *testing.T) {
	projectRoot := t.TempDir()
	yamlPath := writeTargetFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: http://old-queue.example.test\n")
	tomlPath := writeTargetFile(t, projectRoot, "backend/config.toml", "[api]\nurl = \"https://old.example.test\"\n")
	originalYAMLContents := readFile(t, yamlPath)
	originalTOMLContents := readFile(t, tomlPath)

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "workerQueue", File: yamlPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			Value:  "http://secret-queue.example.test",
		},
		{
			Target: config.Target{Name: "api", File: tomlPath, Type: config.TargetTypeTOML, TOMLPath: "api.missing"},
			Value:  "https://secret-api.example.test",
		},
	})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want TOML preparation failure")
	}
	if !strings.Contains(err.Error(), `target "api"`) || !strings.Contains(err.Error(), `tomlPath "api.missing"`) {
		t.Fatalf("ApplyTargetChanges returned error %q, want TOML target context", err)
	}
	for _, forbidden := range []string{"secret-queue", "secret-api"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("ApplyTargetChanges leaked secret %q in error %q", forbidden, err)
		}
	}
	if !bytes.Equal(readFile(t, yamlPath), originalYAMLContents) {
		t.Fatal("YAML file changed after TOML preparation failure")
	}
	if !bytes.Equal(readFile(t, tomlPath), originalTOMLContents) {
		t.Fatal("TOML file changed after TOML preparation failure")
	}
}

func TestApplyTargetChanges_TOMLPlusDotenvPreparationFailureLeavesEveryFileUnchanged(t *testing.T) {
	projectRoot := t.TempDir()
	tomlPath := writeTargetFile(t, projectRoot, "worker/config.toml", "[queue]\nendpoint = \"http://old-queue.example.test\"\n")
	dotenvPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalTOMLContents := readFile(t, tomlPath)
	originalDotenvContents := readFile(t, dotenvPath)

	err := ApplyTargetChanges([]TargetChange{
		{
			Target: config.Target{Name: "workerQueue", File: tomlPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
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
	if !bytes.Equal(readFile(t, tomlPath), originalTOMLContents) {
		t.Fatal("TOML file changed after dotenv preparation failure")
	}
	if !bytes.Equal(readFile(t, dotenvPath), originalDotenvContents) {
		t.Fatal("dotenv file changed after dotenv preparation failure")
	}
}

func TestApplyTargetChanges_WriteFailureAfterTOMLSuccessReportsPartialStateAndCleansTemporaryFile(t *testing.T) {
	projectRoot := t.TempDir()
	tomlPath := writeTargetFile(t, projectRoot, "worker/config.toml", "[queue]\nendpoint = \"http://old-queue.example.test\"\n")
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
			Target: config.Target{Name: "workerQueue", File: tomlPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
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

	tomlRoot := decodeTOMLRoot(t, readFile(t, tomlPath))
	queue := tomlRoot["queue"].(map[string]any)
	if queue["endpoint"] != "http://new-queue.example.test" {
		t.Fatalf("queue.endpoint = %q, want first TOML file to have been replaced before second failure", queue["endpoint"])
	}
	if !bytes.Equal(readFile(t, jsonPath), originalJSONContents) {
		t.Fatal("second target file changed after its replacement failed")
	}
	if containsTempFile(t, filepath.Dir(jsonPath), tempFilePrefix(jsonPath)) {
		t.Fatal("temporary file was not cleaned up after second-file rename failure")
	}
}

func TestApplyTargetChanges_TOMLRenameFailureLeavesOriginalFileIntactAndCleansTemporaryFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.toml", "[queue]\nendpoint = \"http://old-queue.example.test\"\n")
	originalContents := readFile(t, targetPath)

	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		return errors.New("rename failed")
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := ApplyTargetChanges([]TargetChange{{
		Target: config.Target{Name: "workerQueue", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "queue.endpoint"},
		Value:  "http://secret-queue.example.test",
	}})
	if err == nil {
		t.Fatal("ApplyTargetChanges returned nil error, want TOML rename failure")
	}
	if !strings.Contains(err.Error(), "rename failed") || !strings.Contains(err.Error(), targetPath) {
		t.Fatalf("ApplyTargetChanges returned error %q, want TOML rename failure context", err)
	}
	if strings.Contains(err.Error(), "secret-queue") {
		t.Fatalf("ApplyTargetChanges leaked secret in error %q", err)
	}

	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("TOML target changed after rename failure")
	}
	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("temporary file was not cleaned up after TOML rename failure")
	}
}

func decodeTOMLRoot(t *testing.T, contents []byte) map[string]any {
	t.Helper()

	var decodedRoot map[string]any
	if err := toml.Unmarshal(contents, &decodedRoot); err != nil {
		t.Fatalf("decode TOML: %v", err)
	}

	return decodedRoot
}
