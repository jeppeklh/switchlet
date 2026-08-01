package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunCommand_ListVersionThreeYAMLTargetWorks(t *testing.T) {
	projectRoot, _ := writeVersionThreeYAMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: workerQueue
        value: local-queue
  - name: Staging
    protected: true
    values:
      - target: workerQueue
        valueFromEnv: STAGING_WORKER_QUEUE
`)+"\n")

	result := runCommandForTest(t, []string{"list"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for list command")
	}
	for _, expected := range []string{
		"Local",
		"Staging [protected, unavailable]",
		"STAGING_WORKER_QUEUE",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
}

func TestRunCommand_InspectVersionThreeYAMLReportsTargetContext(t *testing.T) {
	projectRoot, workerPath := writeVersionThreeYAMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: workerQueue
        value: local-queue
`)+"\n")

	result := runCommandForTest(t, []string{"inspect", "Local"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		"Profile: Local",
		"- workerQueue [yaml]",
		"file: worker/config.yaml",
		"yamlPath: queue.endpoint",
		"masked value: local-queue",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, workerPath) {
		t.Fatalf("stdout %q must use project-relative path instead of %q", result.stdout, workerPath)
	}
}

func TestRunCommand_ApplyVersionThreeYAMLDryRunTextAndJSONWriteNothing(t *testing.T) {
	projectRoot, workerPath := writeVersionThreeYAMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: workerQueue
        value: staging-queue
`)+"\n")
	originalWorkerContents := readFileBytes(t, workerPath)

	result := runCommandForTest(t, []string{"apply", "Staging", "--dry-run"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Dry run successful for profile "Staging"`,
		"Planned target:",
		"would update worker/config.yaml",
		"  workerQueue [yaml]",
		"  queue.endpoint",
		"No changes were written.",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "staging-queue") {
		t.Fatalf("stdout %q must not contain resolved replacement value", result.stdout)
	}
	if strings.Contains(result.stdout, workerPath) {
		t.Fatalf("stdout %q must use project-relative path instead of %q", result.stdout, workerPath)
	}
	if !bytes.Equal(readFileBytes(t, workerPath), originalWorkerContents) {
		t.Fatal("YAML target changed during dry run")
	}

	jsonResult := runCommandForTest(t, []string{"apply", "Staging", "--dry-run", "--json"}, projectRoot)
	if jsonResult.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", jsonResult.exitCode, jsonResult.stdout, jsonResult.stderr)
	}

	var payload struct {
		Result struct {
			ProfileName string `json:"profileName"`
			Status      string `json:"status"`
			TargetPath  string `json:"targetPath"`
			TargetFile  string `json:"targetFile"`
			TargetCount int    `json:"targetCount"`
			DryRun      bool   `json:"dryRun"`
			Changes     []struct {
				TargetName   string `json:"targetName"`
				TargetFile   string `json:"targetFile"`
				TargetType   string `json:"targetType"`
				SelectorName string `json:"selectorName"`
				Selector     string `json:"selector"`
			} `json:"changes"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(jsonResult.stdout), &payload); err != nil {
		t.Fatalf("unmarshal YAML dry-run JSON: %v\noutput: %q", err, jsonResult.stdout)
	}
	if payload.Result.ProfileName != "Staging" || payload.Result.Status != "dry_run" || !payload.Result.DryRun {
		t.Fatalf("result = %#v, want dry-run Staging", payload.Result)
	}
	if payload.Result.TargetPath != "queue.endpoint" || payload.Result.TargetFile != workerPath || payload.Result.TargetCount != 1 {
		t.Fatalf("result target context = %#v, want one YAML target", payload.Result)
	}
	if len(payload.Result.Changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1", len(payload.Result.Changes))
	}
	change := payload.Result.Changes[0]
	if change.TargetName != "workerQueue" || change.TargetFile != workerPath || change.TargetType != "yaml" || change.SelectorName != "yamlPath" || change.Selector != "queue.endpoint" {
		t.Fatalf("change = %#v, want workerQueue YAML target", change)
	}
	if strings.Contains(jsonResult.stdout, "staging-queue") {
		t.Fatalf("stdout %q must not contain resolved replacement value", jsonResult.stdout)
	}
	if !bytes.Equal(readFileBytes(t, workerPath), originalWorkerContents) {
		t.Fatal("YAML target changed during JSON dry run")
	}
}

func TestRunCommand_ApplyVersionThreeYAMLUpdatesTarget(t *testing.T) {
	projectRoot, workerPath := writeVersionThreeYAMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: workerQueue
        value: staging-queue
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Applied profile "Staging"`,
		"Updated target:",
		"updated worker/config.yaml",
		"  workerQueue [yaml]",
		"  queue.endpoint",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "staging-queue") {
		t.Fatalf("stdout %q must not contain resolved replacement value", result.stdout)
	}
	if strings.Contains(result.stdout, workerPath) {
		t.Fatalf("stdout %q must use project-relative path instead of %q", result.stdout, workerPath)
	}
	if !strings.Contains(string(readFileBytes(t, workerPath)), "endpoint: staging-queue") {
		t.Fatalf("YAML target %q was not updated", string(readFileBytes(t, workerPath)))
	}
}

func TestRunCommand_ApplyVersionThreeMixedJSONYAMLDotenvProfileUpdatesAllTargets(t *testing.T) {
	projectRoot, databasePath, workerPath, frontendPath := writeVersionThreeMixedCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        value: postgres-staging
      - target: workerQueue
        value: staging-queue
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Applied profile "Staging"`,
		"Updated targets:",
		"updated backend/appsettings.Development.json",
		"  database [json]",
		"  database.url",
		"updated worker/config.yaml",
		"  workerQueue [yaml]",
		"  queue.endpoint",
		"updated frontend/.env.local",
		"  frontendApi [dotenv]",
		"  VITE_API_URL",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres-staging", "staging-queue", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain resolved replacement value %q", result.stdout, forbidden)
		}
	}
	for _, absolutePath := range []string{databasePath, workerPath, frontendPath} {
		if strings.Contains(result.stdout, absolutePath) {
			t.Fatalf("stdout %q must use project-relative paths instead of %q", result.stdout, absolutePath)
		}
	}
	if !strings.Contains(string(readFileBytes(t, databasePath)), "postgres-staging") {
		t.Fatalf("database target %q was not updated", string(readFileBytes(t, databasePath)))
	}
	if !strings.Contains(string(readFileBytes(t, workerPath)), "endpoint: staging-queue") {
		t.Fatalf("YAML target %q was not updated", string(readFileBytes(t, workerPath)))
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_API_URL=https://api.staging.example.test") {
		t.Fatalf("dotenv target %q was not updated", string(readFileBytes(t, frontendPath)))
	}
}

func TestRunCommand_ApplyVersionThreeMixedJSONYAMLDotenvProfileDryRunWritesNothing(t *testing.T) {
	projectRoot, databasePath, workerPath, frontendPath := writeVersionThreeMixedCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        value: postgres-staging
      - target: workerQueue
        value: staging-queue
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalWorkerContents := readFileBytes(t, workerPath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"apply", "Staging", "--dry-run"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Dry run successful for profile "Staging"`,
		"Planned targets:",
		"would update backend/appsettings.Development.json",
		"  database [json]",
		"  database.url",
		"would update worker/config.yaml",
		"  workerQueue [yaml]",
		"  queue.endpoint",
		"would update frontend/.env.local",
		"  frontendApi [dotenv]",
		"  VITE_API_URL",
		"No changes were written.",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres-staging", "staging-queue", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain resolved replacement value %q", result.stdout, forbidden)
		}
	}
	for _, absolutePath := range []string{databasePath, workerPath, frontendPath} {
		if strings.Contains(result.stdout, absolutePath) {
			t.Fatalf("stdout %q must use project-relative paths instead of %q", result.stdout, absolutePath)
		}
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during mixed dry run")
	}
	if !bytes.Equal(readFileBytes(t, workerPath), originalWorkerContents) {
		t.Fatal("YAML target changed during mixed dry run")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("dotenv target changed during mixed dry run")
	}
}

func TestRunCommand_ApplyVersionThreeYAMLPartialProfilesModifyOnlyIncludedTargets(t *testing.T) {
	projectRoot, databasePath, workerPath, frontendPath := writeVersionThreeMixedCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Worker Queue Only
    values:
      - target: workerQueue
        value: yaml-only-queue
  - name: Frontend Only
    values:
      - target: frontendApi
        value: https://api.frontend-only.example.test
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalWorkerContents := readFileBytes(t, workerPath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	yamlResult := runCommandForTest(t, []string{"apply", "Worker Queue Only"}, projectRoot)
	if yamlResult.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", yamlResult.exitCode, yamlResult.stdout, yamlResult.stderr)
	}
	if !strings.Contains(yamlResult.stdout, "workerQueue [yaml]") || strings.Contains(yamlResult.stdout, "yaml-only-queue") {
		t.Fatalf("stdout %q should identify YAML target without resolved value", yamlResult.stdout)
	}
	updatedWorkerContents := readFileBytes(t, workerPath)
	if bytes.Equal(updatedWorkerContents, originalWorkerContents) || !strings.Contains(string(updatedWorkerContents), "endpoint: yaml-only-queue") {
		t.Fatalf("YAML target %q was not updated", string(updatedWorkerContents))
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after YAML-only partial profile")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("dotenv target changed after YAML-only partial profile")
	}

	frontendResult := runCommandForTest(t, []string{"apply", "Frontend Only"}, projectRoot)
	if frontendResult.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", frontendResult.exitCode, frontendResult.stdout, frontendResult.stderr)
	}
	if !strings.Contains(frontendResult.stdout, "frontendApi [dotenv]") || strings.Contains(frontendResult.stdout, "https://api.frontend-only.example.test") {
		t.Fatalf("stdout %q should identify dotenv target without resolved value", frontendResult.stdout)
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after frontend-only partial profile")
	}
	if !bytes.Equal(readFileBytes(t, workerPath), updatedWorkerContents) {
		t.Fatal("YAML target changed after profile that omitted YAML")
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_API_URL=https://api.frontend-only.example.test") {
		t.Fatalf("dotenv target %q was not updated", string(readFileBytes(t, frontendPath)))
	}
}

func TestRunCommand_ApplyVersionThreeProtectedYAMLProfileRequiresOptIn(t *testing.T) {
	projectRoot, workerPath := writeVersionThreeYAMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Production
    protected: true
    values:
      - target: workerQueue
        value: production-queue
`)+"\n")
	originalWorkerContents := readFileBytes(t, workerPath)

	refused := runCommandForTest(t, []string{"apply", "Production"}, projectRoot)
	if refused.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", refused.exitCode, runtimeExitCode)
	}
	if !strings.Contains(refused.stderr, `profile "Production" is protected`) {
		t.Fatalf("stderr %q does not contain protected-profile error", refused.stderr)
	}
	if !bytes.Equal(readFileBytes(t, workerPath), originalWorkerContents) {
		t.Fatal("YAML target changed after protected-profile refusal")
	}

	allowed := runCommandForTest(t, []string{"apply", "Production", "--allow-protected"}, projectRoot)
	if allowed.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", allowed.exitCode, allowed.stdout, allowed.stderr)
	}
	if !strings.Contains(allowed.stdout, "workerQueue [yaml]") || strings.Contains(allowed.stdout, "production-queue") {
		t.Fatalf("stdout %q should identify YAML target without resolved value", allowed.stdout)
	}
	if !strings.Contains(string(readFileBytes(t, workerPath)), "endpoint: production-queue") {
		t.Fatalf("YAML target %q was not updated", string(readFileBytes(t, workerPath)))
	}
}

func TestRunCommand_ApplyVersionThreeUnavailableYAMLProfileIdentifiesTargetAndEnvironmentVariable(t *testing.T) {
	projectRoot, workerPath := writeVersionThreeYAMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: workerQueue
        valueFromEnv: STAGING_WORKER_QUEUE
`)+"\n")
	originalWorkerContents := readFileBytes(t, workerPath)

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{
		`Profile "Staging" is unavailable.`,
		"Unavailable values:",
		"- workerQueue",
		"environment variable: STAGING_WORKER_QUEUE",
		"Run `switchlet inspect Staging` to review profile values.",
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if !bytes.Equal(readFileBytes(t, workerPath), originalWorkerContents) {
		t.Fatal("YAML target changed after unavailable profile")
	}
}

func TestRunCommand_ApplyVersionThreeYAMLValidationErrorIncludesSelectorContext(t *testing.T) {
	projectRoot := t.TempDir()
	workerPath := writeFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  retries: 3
`)+"\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue.endpoint

profiles:
  - name: Staging
    values:
      - target: workerQueue
        value: staging-queue
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{
		`target "workerQueue"`,
		workerPath,
		`yamlPath "queue.endpoint"`,
		`missing segment "endpoint"`,
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if strings.Contains(result.stderr, "staging-queue") {
		t.Fatalf("stderr %q must not contain resolved replacement value", result.stderr)
	}
}

func writeVersionThreeYAMLCommandProject(t *testing.T, profilesYAML string) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	workerPath := writeFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue.endpoint

`)+"\n"+profilesYAML)

	return projectRoot, workerPath
}

func writeVersionThreeMixedCommandProject(t *testing.T, profilesYAML string) (string, string, string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	databasePath := writeFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres-old"}}`)
	workerPath := writeFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")
	frontendPath := writeFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\nVITE_FEATURES=local\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: backend/appsettings.Development.json
    type: json
    jsonPath: database.url
  - name: workerQueue
    file: worker/config.yaml
    type: yaml
    yamlPath: queue.endpoint
  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

`)+"\n"+profilesYAML)

	return projectRoot, databasePath, workerPath, frontendPath
}
