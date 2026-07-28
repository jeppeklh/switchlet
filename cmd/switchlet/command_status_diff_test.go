package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunCommand_StatusReportsExactMatchWithoutWritingOrLeakingValues(t *testing.T) {
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
  - name: Staging
    values:
      - target: database
        value: postgres://staging
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"status"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for status command")
	}

	for _, expected := range []string{
		"Current profile: Local",
		"Matched targets:",
		"- database [json]",
		"file: " + databasePath,
		"jsonPath: database.url",
		"- frontendApi [dotenv]",
		"file: " + frontendPath,
		"key: VITE_API_URL",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres://old", "http://localhost:5173", "postgres://staging", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during status")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during status")
	}
}

func TestRunCommand_StatusReportsNoCompleteMatchWithPartialAndClosestProfiles(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Database Only
    values:
      - target: database
        value: postgres://old
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: https://api.local.example.test
  - name: Staging
    values:
      - target: database
        value: postgres://staging
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"status"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		"Current configuration does not match any complete profile.",
		"Partial matches:",
		"- Database Only: 1 of 1 included targets match; 1 targets omitted",
		"Closest profiles:",
		"- Local: 1 of 2 targets match",
		"- Staging: 0 of 2 targets match",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres://old", "https://api.local.example.test", "postgres://staging", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
}

func TestRunCommand_StatusReportsAmbiguousCompleteMatches(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
  - name: Local Copy
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	result := runCommandForTest(t, []string{"status"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		"Current configuration matches multiple complete profiles.",
		"Matches:",
		"- Local",
		"- Local Copy",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
}

func TestRunCommand_StatusReportsMixedTargetExactMatchAndUnavailableProfileWithoutWriting(t *testing.T) {
	const workerEnv = "SWITCHLET_TEST_STAGING_WORKER_QUEUE_ENDPOINT"
	t.Setenv(workerEnv, "")

	projectRoot, targetPaths := writeMixedCurrentStateCommandProject(t, workerEnv)
	originalContents := readTargetContents(t, targetPaths)

	result := runCommandForTest(t, []string{"status"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for status command")
	}

	for _, expected := range []string{
		"Current profile: Local",
		"Matched targets:",
		"- database [json]",
		"file: " + targetPaths["database"],
		"jsonPath: database.url",
		"- workerQueue [yaml]",
		"file: " + targetPaths["workerQueue"],
		"yamlPath: queue.endpoint",
		"- serviceEndpoint [toml]",
		"file: " + targetPaths["serviceEndpoint"],
		"tomlPath: services.api.endpoint",
		"- frontendApi [dotenv]",
		"file: " + targetPaths["frontendApi"],
		"key: VITE_API_URL",
		"Unavailable profiles:",
		"- Staging [protected] / workerQueue [yaml]",
		"environment variable: " + workerEnv,
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{
		"postgres://local-secret",
		"http://localhost:4566/queue-secret",
		"http://localhost:8080/secret",
		"http://localhost:5173/secret",
		"postgres://staging-secret",
		"https://services.staging.example.test/secret",
		"https://api.staging.example.test/secret",
	} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
	assertTargetContentsUnchanged(t, targetPaths, originalContents)
}

func TestRunCommand_StatusJSONReturnsParseableSecretSafeResult(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	result := runCommandForTest(t, []string{"status", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Result struct {
			Command        string `json:"command"`
			Status         string `json:"status"`
			CurrentProfile string `json:"currentProfile"`
			TargetCount    int    `json:"targetCount"`
			Complete       bool   `json:"complete"`
			MatchedTargets []struct {
				TargetName   string `json:"targetName"`
				TargetType   string `json:"targetType"`
				SelectorName string `json:"selectorName"`
				Selector     string `json:"selector"`
			} `json:"matchedTargets"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal status JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.Command != "status" || payload.Result.Status != "matched" || payload.Result.CurrentProfile != "Local" || payload.Result.TargetCount != 2 || !payload.Result.Complete {
		t.Fatalf("result = %#v, want matched Local status", payload.Result)
	}
	if len(payload.Result.MatchedTargets) != 2 || payload.Result.MatchedTargets[0].TargetName != "database" || payload.Result.MatchedTargets[1].TargetName != "frontendApi" {
		t.Fatalf("matchedTargets = %#v, want database and frontendApi", payload.Result.MatchedTargets)
	}
	for _, forbidden := range []string{"postgres://old", "http://localhost:5173"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
}

func TestRunCommand_DiffReportsWouldUpdateAndAlreadyMatchingTargetsWithoutWriting(t *testing.T) {
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        value: postgres://staging
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"diff", "Staging"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for diff command")
	}

	for _, expected := range []string{
		`Diff for profile "Staging"`,
		"Protection: Not protected",
		"Would update:",
		"- database [json]",
		"file: " + databasePath,
		"jsonPath: database.url",
		"Already matches:",
		"- frontendApi [dotenv]",
		"file: " + frontendPath,
		"key: VITE_API_URL",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres://staging", "http://localhost:5173"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during diff")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during diff")
	}
}

func TestRunCommand_DiffReportsUnavailableValuesAndOmittedTargets(t *testing.T) {
	projectRoot, _, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Database Only
    values:
      - target: database
        valueFromEnv: MISSING_DATABASE_URL
`)+"\n")

	result := runCommandForTest(t, []string{"diff", "Database Only"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Diff for profile "Database Only"`,
		"Unavailable:",
		"- database [json]",
		"environment variable: MISSING_DATABASE_URL",
		`environment variable "MISSING_DATABASE_URL" is not set`,
		"Omitted targets:",
		"- frontendApi [dotenv]",
		"file: " + frontendPath,
		"key: VITE_API_URL",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "postgres://old") || strings.Contains(result.stdout, "http://localhost:5173") {
		t.Fatalf("stdout %q must not contain raw current values", result.stdout)
	}
}

func TestRunCommand_DiffJSONReturnsParseableSecretSafeResult(t *testing.T) {
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    protected: true
    values:
      - target: database
        value: postgres://staging
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	result := runCommandForTest(t, []string{"diff", "Staging", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Result struct {
			Command     string `json:"command"`
			ProfileName string `json:"profileName"`
			Protected   bool   `json:"protected"`
			Complete    bool   `json:"complete"`
			WouldUpdate []struct {
				TargetName string `json:"targetName"`
				TargetFile string `json:"targetFile"`
				TargetType string `json:"targetType"`
				Selector   string `json:"selector"`
			} `json:"wouldUpdate"`
			AlreadyMatches []struct {
				TargetName string `json:"targetName"`
				TargetFile string `json:"targetFile"`
				TargetType string `json:"targetType"`
				Selector   string `json:"selector"`
			} `json:"alreadyMatches"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal diff JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.Command != "diff" || payload.Result.ProfileName != "Staging" || !payload.Result.Protected || !payload.Result.Complete {
		t.Fatalf("result = %#v, want complete protected Staging diff", payload.Result)
	}
	if len(payload.Result.WouldUpdate) != 1 || payload.Result.WouldUpdate[0].TargetName != "database" || payload.Result.WouldUpdate[0].TargetFile != databasePath || payload.Result.WouldUpdate[0].TargetType != "json" || payload.Result.WouldUpdate[0].Selector != "database.url" {
		t.Fatalf("wouldUpdate = %#v, want database JSON target", payload.Result.WouldUpdate)
	}
	if len(payload.Result.AlreadyMatches) != 1 || payload.Result.AlreadyMatches[0].TargetName != "frontendApi" || payload.Result.AlreadyMatches[0].TargetFile != frontendPath || payload.Result.AlreadyMatches[0].TargetType != "dotenv" || payload.Result.AlreadyMatches[0].Selector != "VITE_API_URL" {
		t.Fatalf("alreadyMatches = %#v, want frontend dotenv target", payload.Result.AlreadyMatches)
	}
	for _, forbidden := range []string{"postgres://staging", "http://localhost:5173"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
}

func TestRunCommand_DiffProtectedProfileDoesNotRequireAllowProtected(t *testing.T) {
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Production
    protected: true
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"diff", "Production"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "Protection: Protected") || !strings.Contains(result.stdout, "Already matches:") {
		t.Fatalf("stdout %q does not include protected read-only diff context", result.stdout)
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during protected diff")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during protected diff")
	}
}

func TestRunCommand_DiffJSONReportsMixedTargetCategoriesWithoutWritingOrLeakingValues(t *testing.T) {
	const workerEnv = "SWITCHLET_TEST_STAGING_WORKER_QUEUE_ENDPOINT"
	t.Setenv(workerEnv, "")

	projectRoot, targetPaths := writeMixedCurrentStateCommandProject(t, workerEnv)
	originalContents := readTargetContents(t, targetPaths)

	result := runCommandForTest(t, []string{"diff", "Staging", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty string for JSON output", result.stderr)
	}

	var payload struct {
		Result struct {
			Command        string                     `json:"command"`
			ProfileName    string                     `json:"profileName"`
			Protected      bool                       `json:"protected"`
			Complete       bool                       `json:"complete"`
			WouldUpdate    []targetDescriptorTestJSON `json:"wouldUpdate"`
			AlreadyMatches []targetDescriptorTestJSON `json:"alreadyMatches"`
			Unavailable    []struct {
				TargetName          string `json:"targetName"`
				TargetFile          string `json:"targetFile"`
				TargetType          string `json:"targetType"`
				SelectorName        string `json:"selectorName"`
				Selector            string `json:"selector"`
				EnvironmentVariable string `json:"environmentVariable"`
				Reason              string `json:"reason"`
			} `json:"unavailable"`
			OmittedTargets []targetDescriptorTestJSON `json:"omittedTargets"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal diff JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.Command != "diff" || payload.Result.ProfileName != "Staging" || !payload.Result.Protected || payload.Result.Complete {
		t.Fatalf("result = %#v, want incomplete protected Staging diff", payload.Result)
	}
	if len(payload.Result.WouldUpdate) != 2 {
		t.Fatalf("len(wouldUpdate) = %d, want 2", len(payload.Result.WouldUpdate))
	}
	assertTargetDescriptor(t, payload.Result.WouldUpdate[0], "database", targetPaths["database"], "json", "jsonPath", "database.url")
	assertTargetDescriptor(t, payload.Result.WouldUpdate[1], "serviceEndpoint", targetPaths["serviceEndpoint"], "toml", "tomlPath", "services.api.endpoint")
	if len(payload.Result.AlreadyMatches) != 1 {
		t.Fatalf("len(alreadyMatches) = %d, want 1", len(payload.Result.AlreadyMatches))
	}
	assertTargetDescriptor(t, payload.Result.AlreadyMatches[0], "frontendApi", targetPaths["frontendApi"], "dotenv", "key", "VITE_API_URL")
	if len(payload.Result.Unavailable) != 1 {
		t.Fatalf("len(unavailable) = %d, want 1", len(payload.Result.Unavailable))
	}
	unavailable := payload.Result.Unavailable[0]
	if unavailable.TargetName != "workerQueue" || unavailable.TargetFile != targetPaths["workerQueue"] || unavailable.TargetType != "yaml" || unavailable.SelectorName != "yamlPath" || unavailable.Selector != "queue.endpoint" || unavailable.EnvironmentVariable != workerEnv || !strings.Contains(unavailable.Reason, workerEnv) {
		t.Fatalf("unavailable = %#v, want workerQueue YAML environment value", unavailable)
	}
	if len(payload.Result.OmittedTargets) != 0 {
		t.Fatalf("omittedTargets = %#v, want none for complete Staging profile", payload.Result.OmittedTargets)
	}
	for _, forbidden := range []string{
		"postgres://local-secret",
		"http://localhost:4566/queue-secret",
		"http://localhost:8080/secret",
		"http://localhost:5173/secret",
		"postgres://staging-secret",
		"https://services.staging.example.test/secret",
		"https://api.staging.example.test/secret",
	} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
	assertTargetContentsUnchanged(t, targetPaths, originalContents)
}

func TestRunCommand_StatusTargetReadFailureReturnsSecretSafeJSONError(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"password":"current-secret"}}`)
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: backend/appsettings.Development.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://resolved-secret
`)+"\n")

	result := runCommandForTest(t, []string{"status", "--json"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty string for JSON error", result.stderr)
	}

	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal status JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "runtime" {
		t.Fatalf("error.kind = %q, want runtime", payload.Error.Kind)
	}
	for _, expected := range []string{
		`target "database"`,
		targetPath,
		`type "json"`,
		`jsonPath "database.url"`,
		`missing segment "url"`,
	} {
		if !strings.Contains(payload.Error.Message, expected) {
			t.Fatalf("error.message %q does not contain %q", payload.Error.Message, expected)
		}
	}
	for _, forbidden := range []string{"current-secret", "resolved-secret"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
}

func TestRunCommand_DiffMissingProfileListsAvailableProfiles(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
  - name: Staging
    values:
      - target: database
        value: postgres://staging
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"diff", "Stagng"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{
		`Profile "Stagng" does not exist.`,
		"Available profiles:",
		"- Local",
		"- Staging",
		`Did you mean "Staging"?`,
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	for _, forbidden := range []string{"postgres://old", "postgres://staging", "https://api.staging.example.test"} {
		if strings.Contains(result.stderr, forbidden) {
			t.Fatalf("stderr %q must not contain raw value %q", result.stderr, forbidden)
		}
	}
}

func TestRunCommand_StatusAndDiffUsageFailuresReturnExitCodeTwo(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
`)+"\n")

	tests := []struct {
		name     string
		args     []string
		wantText string
	}{
		{name: "status extra argument", args: []string{"status", "Local"}, wantText: "status does not accept a profile name"},
		{name: "status unsupported flag", args: []string{"status", "--jsoon"}, wantText: `Did you mean "--json"?`},
		{name: "diff missing profile", args: []string{"diff"}, wantText: "No profile specified."},
		{name: "diff extra argument", args: []string{"diff", "Local", "Extra"}, wantText: "diff requires exactly one profile name"},
		{name: "diff unsupported flag", args: []string{"diff", "Local", "--jsoon"}, wantText: `Did you mean "--json"?`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCommandForTest(t, testCase.args, projectRoot)
			if result.exitCode != usageExitCode {
				t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
			}
			if !strings.Contains(result.stderr, testCase.wantText) {
				t.Fatalf("stderr %q does not contain %q", result.stderr, testCase.wantText)
			}
		})
	}
}

type targetDescriptorTestJSON struct {
	TargetName   string `json:"targetName"`
	TargetFile   string `json:"targetFile"`
	TargetType   string `json:"targetType"`
	SelectorName string `json:"selectorName"`
	Selector     string `json:"selector"`
}

func writeMixedCurrentStateCommandProject(t *testing.T, workerEnv string) (string, map[string]string) {
	t.Helper()

	projectRoot := t.TempDir()
	targetPaths := map[string]string{
		"database":        writeFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://local-secret"}}`),
		"workerQueue":     writeFile(t, projectRoot, "worker/config.yaml", "queue:\n  endpoint: http://localhost:4566/queue-secret\n"),
		"serviceEndpoint": writeFile(t, projectRoot, "services/development.toml", "[services.api]\nendpoint = \"http://localhost:8080/secret\"\n"),
		"frontendApi":     writeFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173/secret\nVITE_FEATURES=local\n"),
	}
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

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local-secret
      - target: workerQueue
        value: http://localhost:4566/queue-secret
      - target: serviceEndpoint
        value: http://localhost:8080/secret
      - target: frontendApi
        value: http://localhost:5173/secret

  - name: Staging
    protected: true
    values:
      - target: database
        value: postgres://staging-secret
      - target: workerQueue
        valueFromEnv: `+workerEnv+`
      - target: serviceEndpoint
        value: https://services.staging.example.test/secret
      - target: frontendApi
        value: http://localhost:5173/secret

  - name: Service Endpoint Only
    values:
      - target: serviceEndpoint
        value: http://localhost:8080/secret
`)+"\n")

	return projectRoot, targetPaths
}

func readTargetContents(t *testing.T, targetPaths map[string]string) map[string][]byte {
	t.Helper()

	contents := make(map[string][]byte, len(targetPaths))
	for name, path := range targetPaths {
		contents[name] = readFileBytes(t, path)
	}

	return contents
}

func assertTargetContentsUnchanged(t *testing.T, targetPaths map[string]string, originalContents map[string][]byte) {
	t.Helper()

	for name, path := range targetPaths {
		if !bytes.Equal(readFileBytes(t, path), originalContents[name]) {
			t.Fatalf("target %q changed during read-only command", name)
		}
	}
}

func assertTargetDescriptor(t *testing.T, descriptor targetDescriptorTestJSON, targetName string, targetFile string, targetType string, selectorName string, selector string) {
	t.Helper()

	if descriptor.TargetName != targetName || descriptor.TargetFile != targetFile || descriptor.TargetType != targetType || descriptor.SelectorName != selectorName || descriptor.Selector != selector {
		t.Fatalf("descriptor = %#v, want %s/%s/%s/%s/%s", descriptor, targetName, targetFile, targetType, selectorName, selector)
	}
}
