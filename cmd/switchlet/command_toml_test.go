package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunCommand_ListVersionThreeTOMLTargetWorks(t *testing.T) {
	t.Setenv("STAGING_SERVICE_ENDPOINT", "")

	projectRoot, _ := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080
  - name: Staging
    protected: true
    values:
      - target: serviceEndpoint
        valueFromEnv: STAGING_SERVICE_ENDPOINT
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
		"STAGING_SERVICE_ENDPOINT",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
}

func TestRunCommand_InspectVersionThreeTOMLReportsTargetContext(t *testing.T) {
	projectRoot, servicePath := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080
`)+"\n")

	result := runCommandForTest(t, []string{"inspect", "Local"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		"Profile: Local",
		"- serviceEndpoint [toml]",
		"file: services/development.toml",
		"tomlPath: services.api.endpoint",
		"masked value: ****",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, servicePath) {
		t.Fatalf("stdout %q must use project-relative path instead of %q", result.stdout, servicePath)
	}
}

func TestRunCommand_ListAndInspectVersionThreeTOMLJSONReportTargetContext(t *testing.T) {
	projectRoot, servicePath := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080
`)+"\n")

	listResult := runCommandForTest(t, []string{"list", "--json"}, projectRoot)
	if listResult.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", listResult.exitCode, listResult.stdout, listResult.stderr)
	}

	var listPayload struct {
		Profiles []struct {
			Name   string `json:"name"`
			Values []struct {
				TargetName   string `json:"targetName"`
				TargetFile   string `json:"targetFile"`
				TargetType   string `json:"targetType"`
				SelectorName string `json:"selectorName"`
				Selector     string `json:"selector"`
				MaskedValue  string `json:"maskedValue"`
			} `json:"values"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(listResult.stdout), &listPayload); err != nil {
		t.Fatalf("unmarshal TOML list JSON: %v\noutput: %q", err, listResult.stdout)
	}
	if len(listPayload.Profiles) != 1 || len(listPayload.Profiles[0].Values) != 1 {
		t.Fatalf("list payload = %#v, want one profile with one TOML value", listPayload)
	}
	listValue := listPayload.Profiles[0].Values[0]
	if listPayload.Profiles[0].Name != "Local" || listValue.TargetName != "serviceEndpoint" || listValue.TargetFile != servicePath || listValue.TargetType != "toml" || listValue.SelectorName != "tomlPath" || listValue.Selector != "services.api.endpoint" {
		t.Fatalf("list TOML value = %#v for profiles %#v, want TOML target context", listValue, listPayload.Profiles)
	}
	if listValue.MaskedValue != "****" {
		t.Fatalf("list masked value = %q, want redacted literal value", listValue.MaskedValue)
	}

	inspectResult := runCommandForTest(t, []string{"inspect", "Local", "--json"}, projectRoot)
	if inspectResult.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", inspectResult.exitCode, inspectResult.stdout, inspectResult.stderr)
	}

	var inspectPayload struct {
		Profile struct {
			Name   string `json:"name"`
			Values []struct {
				TargetName   string `json:"targetName"`
				TargetFile   string `json:"targetFile"`
				TargetType   string `json:"targetType"`
				SelectorName string `json:"selectorName"`
				Selector     string `json:"selector"`
			} `json:"values"`
		} `json:"profile"`
	}
	if err := json.Unmarshal([]byte(inspectResult.stdout), &inspectPayload); err != nil {
		t.Fatalf("unmarshal TOML inspect JSON: %v\noutput: %q", err, inspectResult.stdout)
	}
	if inspectPayload.Profile.Name != "Local" || len(inspectPayload.Profile.Values) != 1 {
		t.Fatalf("inspect payload = %#v, want Local with one TOML value", inspectPayload)
	}
	inspectValue := inspectPayload.Profile.Values[0]
	if inspectValue.TargetName != "serviceEndpoint" || inspectValue.TargetFile != servicePath || inspectValue.TargetType != "toml" || inspectValue.SelectorName != "tomlPath" || inspectValue.Selector != "services.api.endpoint" {
		t.Fatalf("inspect TOML value = %#v, want TOML target context", inspectValue)
	}
}

func TestRunCommand_ApplyVersionThreeTOMLDryRunTextAndJSONWriteNothing(t *testing.T) {
	projectRoot, servicePath := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: serviceEndpoint
        value: https://api.staging.example.test
`)+"\n")
	originalServiceContents := readFileBytes(t, servicePath)

	result := runCommandForTest(t, []string{"apply", "Staging", "--dry-run"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Dry run successful for profile "Staging"`,
		"Would update:",
		"would update services/development.toml",
		"  serviceEndpoint [toml]",
		"  services.api.endpoint",
		"No changes were written.",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "https://api.staging.example.test") {
		t.Fatalf("stdout %q must not contain resolved replacement value", result.stdout)
	}
	if strings.Contains(result.stdout, servicePath) {
		t.Fatalf("stdout %q must use project-relative path instead of %q", result.stdout, servicePath)
	}
	if !bytes.Equal(readFileBytes(t, servicePath), originalServiceContents) {
		t.Fatal("TOML target changed during dry run")
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
		t.Fatalf("unmarshal TOML dry-run JSON: %v\noutput: %q", err, jsonResult.stdout)
	}
	if payload.Result.ProfileName != "Staging" || payload.Result.Status != "dry_run" || !payload.Result.DryRun {
		t.Fatalf("result = %#v, want dry-run Staging", payload.Result)
	}
	if payload.Result.TargetPath != "services.api.endpoint" || payload.Result.TargetFile != servicePath || payload.Result.TargetCount != 1 {
		t.Fatalf("result target context = %#v, want one TOML target", payload.Result)
	}
	if len(payload.Result.Changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1", len(payload.Result.Changes))
	}
	change := payload.Result.Changes[0]
	if change.TargetName != "serviceEndpoint" || change.TargetFile != servicePath || change.TargetType != "toml" || change.SelectorName != "tomlPath" || change.Selector != "services.api.endpoint" {
		t.Fatalf("change = %#v, want serviceEndpoint TOML target", change)
	}
	if strings.Contains(jsonResult.stdout, "https://api.staging.example.test") {
		t.Fatalf("stdout %q must not contain resolved replacement value", jsonResult.stdout)
	}
	if !bytes.Equal(readFileBytes(t, servicePath), originalServiceContents) {
		t.Fatal("TOML target changed during JSON dry run")
	}
}

func TestRunCommand_ApplyVersionThreeTOMLUpdatesTarget(t *testing.T) {
	projectRoot, servicePath := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: serviceEndpoint
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Applied profile "Staging"`,
		"Updated target:",
		"updated services/development.toml",
		"  serviceEndpoint [toml]",
		"  services.api.endpoint",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "https://api.staging.example.test") {
		t.Fatalf("stdout %q must not contain resolved replacement value", result.stdout)
	}
	if strings.Contains(result.stdout, servicePath) {
		t.Fatalf("stdout %q must use project-relative path instead of %q", result.stdout, servicePath)
	}
	if !strings.Contains(string(readFileBytes(t, servicePath)), `endpoint = "https://api.staging.example.test"`) {
		t.Fatalf("TOML target %q was not updated", string(readFileBytes(t, servicePath)))
	}
}

func TestRunCommand_ApplyVersionThreeMixedJSONYAMLTOMLDotenvProfileUpdatesAllTargets(t *testing.T) {
	projectRoot, databasePath, workerPath, servicePath, frontendPath := writeVersionThreeMixedTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        value: postgres-staging
      - target: workerQueue
        value: staging-queue
      - target: serviceEndpoint
        value: https://api.staging.example.test
      - target: frontendApi
        value: https://frontend.staging.example.test
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
		"updated services/development.toml",
		"  serviceEndpoint [toml]",
		"  services.api.endpoint",
		"updated frontend/.env.local",
		"  frontendApi [dotenv]",
		"  VITE_API_URL",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres-staging", "staging-queue", "https://api.staging.example.test", "https://frontend.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain resolved replacement value %q", result.stdout, forbidden)
		}
	}
	for _, absolutePath := range []string{databasePath, workerPath, servicePath, frontendPath} {
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
	if !strings.Contains(string(readFileBytes(t, servicePath)), `endpoint = "https://api.staging.example.test"`) {
		t.Fatalf("TOML target %q was not updated", string(readFileBytes(t, servicePath)))
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_API_URL=https://frontend.staging.example.test") {
		t.Fatalf("dotenv target %q was not updated", string(readFileBytes(t, frontendPath)))
	}
}

func TestRunCommand_ApplyVersionThreeMixedJSONYAMLTOMLDotenvProfileDryRunWritesNothing(t *testing.T) {
	projectRoot, databasePath, workerPath, servicePath, frontendPath := writeVersionThreeMixedTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        value: postgres-staging
      - target: workerQueue
        value: staging-queue
      - target: serviceEndpoint
        value: https://api.staging.example.test
      - target: frontendApi
        value: https://frontend.staging.example.test
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalWorkerContents := readFileBytes(t, workerPath)
	originalServiceContents := readFileBytes(t, servicePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"apply", "Staging", "--dry-run"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Dry run successful for profile "Staging"`,
		"Would update:",
		"would update backend/appsettings.Development.json",
		"  database [json]",
		"  database.url",
		"would update worker/config.yaml",
		"  workerQueue [yaml]",
		"  queue.endpoint",
		"would update services/development.toml",
		"  serviceEndpoint [toml]",
		"  services.api.endpoint",
		"would update frontend/.env.local",
		"  frontendApi [dotenv]",
		"  VITE_API_URL",
		"No changes were written.",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres-staging", "staging-queue", "https://api.staging.example.test", "https://frontend.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain resolved replacement value %q", result.stdout, forbidden)
		}
	}
	for _, absolutePath := range []string{databasePath, workerPath, servicePath, frontendPath} {
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
	if !bytes.Equal(readFileBytes(t, servicePath), originalServiceContents) {
		t.Fatal("TOML target changed during mixed dry run")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("dotenv target changed during mixed dry run")
	}
}

func TestRunCommand_ApplyVersionThreeTOMLPartialProfilesModifyOnlyIncludedTargets(t *testing.T) {
	projectRoot, databasePath, workerPath, servicePath, frontendPath := writeVersionThreeMixedTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Service Endpoint Only
    values:
      - target: serviceEndpoint
        value: https://api.partial.example.test
  - name: Frontend Only
    values:
      - target: frontendApi
        value: https://frontend-only.example.test
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalWorkerContents := readFileBytes(t, workerPath)
	originalServiceContents := readFileBytes(t, servicePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	tomlResult := runCommandForTest(t, []string{"apply", "Service Endpoint Only"}, projectRoot)
	if tomlResult.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", tomlResult.exitCode, tomlResult.stdout, tomlResult.stderr)
	}
	if !strings.Contains(tomlResult.stdout, "serviceEndpoint [toml]") || strings.Contains(tomlResult.stdout, "https://api.partial.example.test") {
		t.Fatalf("stdout %q should identify TOML target without resolved value", tomlResult.stdout)
	}
	updatedServiceContents := readFileBytes(t, servicePath)
	if bytes.Equal(updatedServiceContents, originalServiceContents) || !strings.Contains(string(updatedServiceContents), `endpoint = "https://api.partial.example.test"`) {
		t.Fatalf("TOML target %q was not updated", string(updatedServiceContents))
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after TOML-only partial profile")
	}
	if !bytes.Equal(readFileBytes(t, workerPath), originalWorkerContents) {
		t.Fatal("YAML target changed after TOML-only partial profile")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("dotenv target changed after TOML-only partial profile")
	}

	frontendResult := runCommandForTest(t, []string{"apply", "Frontend Only"}, projectRoot)
	if frontendResult.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", frontendResult.exitCode, frontendResult.stdout, frontendResult.stderr)
	}
	if !strings.Contains(frontendResult.stdout, "frontendApi [dotenv]") || strings.Contains(frontendResult.stdout, "https://frontend-only.example.test") {
		t.Fatalf("stdout %q should identify dotenv target without resolved value", frontendResult.stdout)
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed after frontend-only partial profile")
	}
	if !bytes.Equal(readFileBytes(t, workerPath), originalWorkerContents) {
		t.Fatal("YAML target changed after frontend-only partial profile")
	}
	if !bytes.Equal(readFileBytes(t, servicePath), updatedServiceContents) {
		t.Fatal("TOML target changed after profile that omitted TOML")
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_API_URL=https://frontend-only.example.test") {
		t.Fatalf("dotenv target %q was not updated", string(readFileBytes(t, frontendPath)))
	}
}

func TestRunCommand_ApplyVersionThreeProtectedTOMLProfileRequiresOptIn(t *testing.T) {
	projectRoot, servicePath := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Production
    protected: true
    values:
      - target: serviceEndpoint
        value: https://api.production.example.test
`)+"\n")
	originalServiceContents := readFileBytes(t, servicePath)

	refused := runCommandForTest(t, []string{"apply", "Production"}, projectRoot)
	if refused.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", refused.exitCode, runtimeExitCode)
	}
	if !strings.Contains(refused.stderr, `profile "Production" is protected`) {
		t.Fatalf("stderr %q does not contain protected-profile error", refused.stderr)
	}
	if !bytes.Equal(readFileBytes(t, servicePath), originalServiceContents) {
		t.Fatal("TOML target changed after protected-profile refusal")
	}

	allowed := runCommandForTest(t, []string{"apply", "Production", "--allow-protected"}, projectRoot)
	if allowed.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", allowed.exitCode, allowed.stdout, allowed.stderr)
	}
	if !strings.Contains(allowed.stdout, "serviceEndpoint [toml]") || strings.Contains(allowed.stdout, "https://api.production.example.test") {
		t.Fatalf("stdout %q should identify TOML target without resolved value", allowed.stdout)
	}
	if !strings.Contains(string(readFileBytes(t, servicePath)), `endpoint = "https://api.production.example.test"`) {
		t.Fatalf("TOML target %q was not updated", string(readFileBytes(t, servicePath)))
	}
}

func TestRunCommand_ApplyVersionThreeUnavailableTOMLProfileIdentifiesTargetAndEnvironmentVariable(t *testing.T) {
	t.Setenv("STAGING_SERVICE_ENDPOINT", "")

	projectRoot, servicePath := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: serviceEndpoint
        valueFromEnv: STAGING_SERVICE_ENDPOINT
`)+"\n")
	originalServiceContents := readFileBytes(t, servicePath)

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{
		`Profile "Staging" is unavailable.`,
		"Unavailable values:",
		"- serviceEndpoint",
		"environment variable: STAGING_SERVICE_ENDPOINT",
		"Run `switchlet inspect Staging` to review profile values.",
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if !bytes.Equal(readFileBytes(t, servicePath), originalServiceContents) {
		t.Fatal("TOML target changed after unavailable profile")
	}
}

func TestRunCommand_ApplyVersionThreeTOMLValidationErrorIncludesSelectorContext(t *testing.T) {
	projectRoot := t.TempDir()
	servicePath := writeFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
retries = 3
`)+"\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services.api.endpoint

profiles:
  - name: Staging
    values:
      - target: serviceEndpoint
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{
		`target "serviceEndpoint"`,
		servicePath,
		`tomlPath "services.api.endpoint"`,
		`missing segment "endpoint"`,
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if strings.Contains(result.stderr, "https://api.staging.example.test") {
		t.Fatalf("stderr %q must not contain resolved replacement value", result.stderr)
	}
}

func TestRunCommand_VersionThreeTOMLStartupValidationErrorsUseCommandOutputMode(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		jsonOutput bool
	}{
		{name: "list text", args: []string{"list"}},
		{name: "inspect json", args: []string{"inspect", "Local", "--json"}, jsonOutput: true},
		{name: "apply json", args: []string{"apply", "Local", "--json"}, jsonOutput: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			servicePath := writeFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
retries = 3
`)+"\n")
			writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services.api.endpoint

profiles:
  - name: Local
    values:
      - target: serviceEndpoint
        value: https://api.local.example.test
`)+"\n")

			result := runCommandForTest(t, testCase.args, projectRoot)
			if result.exitCode != runtimeExitCode {
				t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, runtimeExitCode, result.stdout, result.stderr)
			}

			if testCase.jsonOutput {
				if result.stderr != "" {
					t.Fatalf("stderr = %q, want empty string for JSON command error", result.stderr)
				}

				var payload struct {
					Error struct {
						Kind    string `json:"kind"`
						Message string `json:"message"`
					} `json:"error"`
				}
				if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
					t.Fatalf("unmarshal TOML startup JSON error: %v\noutput: %q", err, result.stdout)
				}
				if payload.Error.Kind != "runtime" {
					t.Fatalf("error.kind = %q, want runtime", payload.Error.Kind)
				}
				assertTOMLStartupValidationMessage(t, payload.Error.Message, servicePath)
				return
			}

			if result.stdout != "" {
				t.Fatalf("stdout = %q, want empty string for text command error", result.stdout)
			}
			assertTOMLStartupValidationMessage(t, result.stderr, servicePath)
		})
	}
}

func TestRunCommand_TOMLNoProfileUsageStillReturnsUsageExitCode(t *testing.T) {
	projectRoot, _ := writeVersionThreeTOMLCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: serviceEndpoint
        value: http://localhost:8080
`)+"\n")

	result := runCommandForTest(t, []string{"apply"}, projectRoot)
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	if !strings.Contains(result.stderr, "No profile specified.") || !strings.Contains(result.stderr, "Local") {
		t.Fatalf("stderr %q does not contain no-profile guidance", result.stderr)
	}
}

func writeVersionThreeTOMLCommandProject(t *testing.T, profilesYAML string) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	servicePath := writeFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services.api.endpoint

`)+"\n"+profilesYAML)

	return projectRoot, servicePath
}

func writeVersionThreeMixedTOMLCommandProject(t *testing.T, profilesYAML string) (string, string, string, string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	databasePath := writeFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres-old"}}`)
	workerPath := writeFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")
	servicePath := writeFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
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
  - name: serviceEndpoint
    file: services/development.toml
    type: toml
    tomlPath: services.api.endpoint
  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

`)+"\n"+profilesYAML)

	return projectRoot, databasePath, workerPath, servicePath, frontendPath
}

func assertTOMLStartupValidationMessage(t *testing.T, message string, servicePath string) {
	t.Helper()

	for _, expected := range []string{
		"validate configured targets",
		`target "serviceEndpoint"`,
		servicePath,
		`tomlPath "services.api.endpoint"`,
		`missing segment "endpoint"`,
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message %q does not contain %q", message, expected)
		}
	}
	if strings.Contains(message, "https://api.local.example.test") {
		t.Fatalf("message %q must not contain resolved replacement value", message)
	}
}
