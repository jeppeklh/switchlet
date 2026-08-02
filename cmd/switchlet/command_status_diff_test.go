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
		"Switchlet status",
		"Current profile",
		"Local",
		"Matched targets",
		"> database [json]",
		"backend/appsettings.Development.json",
		"jsonPath          database.url",
		"> frontendApi [dotenv]",
		"frontend/.env.local",
		"key               VITE_API_URL",
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
	for _, absolutePath := range []string{databasePath, frontendPath} {
		if strings.Contains(result.stdout, absolutePath) {
			t.Fatalf("stdout %q must use project-relative paths instead of %q", result.stdout, absolutePath)
		}
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during status")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during status")
	}
}

func TestRunCommand_StatusShortReportsExactMatch(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
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

	result := runCommandForTest(t, []string{"status", "--short"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.stdout != "Current profile: Local\n" {
		t.Fatalf("stdout = %q, want concise current profile", result.stdout)
	}
	for _, forbidden := range []string{"postgres://old", "http://localhost:5173", "postgres://staging", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
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
		"Switchlet status",
		"State",
		"no complete profile match",
		"Partial matches",
		"> Database Only  1/1 included match; 1 omitted",
		"Closest profiles",
		"> Local  1/2 targets match",
		"> Staging  0/2 targets match",
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

func TestRunCommand_StatusShortReportsNoCompleteMatch(t *testing.T) {
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
`)+"\n")

	result := runCommandForTest(t, []string{"status", "--short"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.stdout != "Current profile: none\n" {
		t.Fatalf("stdout = %q, want concise unmatched output", result.stdout)
	}
	for _, forbidden := range []string{"postgres://old", "https://api.local.example.test"} {
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
		"Switchlet status",
		"State",
		"multiple complete profiles match",
		"Matches",
		"> Local",
		"> Local Copy",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
}

func TestRunCommand_StatusShortReportsAmbiguousCompleteMatches(t *testing.T) {
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

	result := runCommandForTest(t, []string{"status", "--short"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.stdout != "Current profile: ambiguous (Local, Local Copy)\n" {
		t.Fatalf("stdout = %q, want concise ambiguous output", result.stdout)
	}
	for _, forbidden := range []string{"postgres://old", "http://localhost:5173"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
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
		"Switchlet status",
		"Current profile",
		"Local",
		"Matched targets",
		"> database [json]",
		"backend/appsettings.Development.json",
		"jsonPath          database.url",
		"> workerQueue [yaml]",
		"worker/config.yaml",
		"yamlPath          queue.endpoint",
		"> serviceEndpoint [toml]",
		"services/development.toml",
		"tomlPath          services.api.endpoint",
		"> frontendApi [dotenv]",
		"frontend/.env.local",
		"key               VITE_API_URL",
		"Unavailable profiles",
		"> Staging [protected] / workerQueue [yaml]",
		"environment       " + workerEnv,
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
	for _, absolutePath := range targetPaths {
		if strings.Contains(result.stdout, absolutePath) {
			t.Fatalf("stdout %q must use project-relative paths instead of %q", result.stdout, absolutePath)
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

func TestRunCommand_StatusShortCannotBeCombinedWithJSON(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	result := runCommandForTest(t, []string{"status", "--short", "--json"}, projectRoot)
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, usageExitCode, result.stdout, result.stderr)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want JSON usage error on stdout", result.stderr)
	}
	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal status JSON usage error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "usage" || !strings.Contains(payload.Error.Message, "status --short cannot be combined with --json") {
		t.Fatalf("error = %#v, want status --short usage error", payload.Error)
	}
}

func TestRunCommand_NoColorFlagRemovesANSIFromStatusAndDiffText(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
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
        value: http://localhost:5173
`)+"\n")

	statusResult := runCommandForTest(t, []string{"--no-color", "status"}, projectRoot)
	if statusResult.exitCode != 0 {
		t.Fatalf("status exitCode = %d, want 0 (stdout: %q, stderr: %q)", statusResult.exitCode, statusResult.stdout, statusResult.stderr)
	}
	if containsANSIStyling(statusResult.stdout) {
		t.Fatalf("status stdout %q contains ANSI styling despite --no-color", statusResult.stdout)
	}

	diffResult := runCommandForTest(t, []string{"diff", "Staging", "--no-color"}, projectRoot)
	if diffResult.exitCode != 0 {
		t.Fatalf("diff exitCode = %d, want 0 (stdout: %q, stderr: %q)", diffResult.exitCode, diffResult.stdout, diffResult.stderr)
	}
	if containsANSIStyling(diffResult.stdout) {
		t.Fatalf("diff stdout %q contains ANSI styling despite --no-color", diffResult.stdout)
	}
}

func TestRunCommand_RedirectedCommandOutputIsPlainByDefault(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
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
	if containsANSIStyling(result.stdout) {
		t.Fatalf("stdout %q contains ANSI styling despite redirected command output", result.stdout)
	}
}

func TestRunCommand_NoColorEnvironmentRemovesANSIFromStatusAndDiffText(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
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
        value: http://localhost:5173
`)+"\n")

	statusResult := runCommandForTest(t, []string{"status"}, projectRoot)
	if statusResult.exitCode != 0 {
		t.Fatalf("status exitCode = %d, want 0 (stdout: %q, stderr: %q)", statusResult.exitCode, statusResult.stdout, statusResult.stderr)
	}
	if containsANSIStyling(statusResult.stdout) {
		t.Fatalf("status stdout %q contains ANSI styling despite NO_COLOR", statusResult.stdout)
	}

	diffResult := runCommandForTest(t, []string{"diff", "Staging"}, projectRoot)
	if diffResult.exitCode != 0 {
		t.Fatalf("diff exitCode = %d, want 0 (stdout: %q, stderr: %q)", diffResult.exitCode, diffResult.stdout, diffResult.stderr)
	}
	if containsANSIStyling(diffResult.stdout) {
		t.Fatalf("diff stdout %q contains ANSI styling despite NO_COLOR", diffResult.stdout)
	}
}

func TestRunCommand_NoColorDoesNotChangeStatusJSONOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	jsonResult := runCommandForTest(t, []string{"status", "--json"}, projectRoot)
	if jsonResult.exitCode != 0 {
		t.Fatalf("status --json exitCode = %d, want 0 (stdout: %q, stderr: %q)", jsonResult.exitCode, jsonResult.stdout, jsonResult.stderr)
	}

	noColorResult := runCommandForTest(t, []string{"--no-color", "status", "--json"}, projectRoot)
	if noColorResult.exitCode != 0 {
		t.Fatalf("--no-color status --json exitCode = %d, want 0 (stdout: %q, stderr: %q)", noColorResult.exitCode, noColorResult.stdout, noColorResult.stderr)
	}
	if noColorResult.stdout != jsonResult.stdout {
		t.Fatalf("--no-color JSON stdout = %q, want unchanged %q", noColorResult.stdout, jsonResult.stdout)
	}
	if containsANSIStyling(noColorResult.stdout) {
		t.Fatalf("JSON stdout %q contains ANSI styling", noColorResult.stdout)
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
		"Switchlet diff",
		"Staging",
		"Protection        not protected",
		"Would update",
		"> database [json]",
		"backend/appsettings.Development.json",
		"jsonPath          database.url",
		"Already matches",
		"> frontendApi [dotenv]",
		"frontend/.env.local",
		"key               VITE_API_URL",
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
	for _, absolutePath := range []string{databasePath, frontendPath} {
		if strings.Contains(result.stdout, absolutePath) {
			t.Fatalf("stdout %q must use project-relative paths instead of %q", result.stdout, absolutePath)
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
		"Switchlet diff",
		"Database Only",
		"Unavailable",
		"> database [json]",
		"environment       MISSING_DATABASE_URL",
		`environment variable "MISSING_DATABASE_URL" is not set`,
		"Omitted targets",
		"> frontendApi [dotenv]",
		"frontend/.env.local",
		"key               VITE_API_URL",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "postgres://old") || strings.Contains(result.stdout, "http://localhost:5173") {
		t.Fatalf("stdout %q must not contain raw current values", result.stdout)
	}
	if strings.Contains(result.stdout, frontendPath) {
		t.Fatalf("stdout %q must use project-relative path instead of %q", result.stdout, frontendPath)
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
	if !strings.Contains(result.stdout, "Protection        protected") || !strings.Contains(result.stdout, "Already matches") {
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

func TestRunCommand_StatusExpectReturnsZeroForExpectedExactCurrentProfile(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
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

	result := runCommandForTest(t, []string{"status", "--expect", "Local"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	for _, expected := range []string{"Switchlet status", "Expectation", "expected", "Local", "result", "matched"} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
}

func TestRunCommand_StatusExpectAcceptsDashPrefixedProfileName(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: "-Local"
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	for _, args := range [][]string{
		{"status", "--expect", "-Local"},
		{"status", "--expect=-Local"},
	} {
		result := runCommandForTest(t, args, projectRoot)
		if result.exitCode != 0 {
			t.Fatalf("%v exitCode = %d, want 0 (stdout: %q, stderr: %q)", args, result.exitCode, result.stdout, result.stderr)
		}
		if !strings.Contains(result.stdout, "matched") || !strings.Contains(result.stdout, "-Local") {
			t.Fatalf("%v stdout %q does not contain matched -Local expectation", args, result.stdout)
		}
		if result.stderr != "" {
			t.Fatalf("%v stderr = %q, want empty", args, result.stderr)
		}
	}
}

func TestRunCommand_StatusExpectTreatsFlagNamesAsExpectedProfiles(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		profileName string
		args        []string
	}{
		{name: "help", profileName: "--help", args: []string{"status", "--expect", "--help"}},
		{name: "json", profileName: "--json", args: []string{"status", "--expect", "--json"}},
		{name: "json equals", profileName: "--json", args: []string{"status", "--expect=--json"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: "`+testCase.profileName+`"
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

			result := runCommandForTest(t, testCase.args, projectRoot)
			if result.exitCode != 0 {
				t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
			}
			if result.stderr != "" {
				t.Fatalf("stderr = %q, want empty", result.stderr)
			}
			if !strings.Contains(result.stdout, "matched") || !strings.Contains(result.stdout, testCase.profileName) {
				t.Fatalf("stdout %q does not contain matched %s expectation", result.stdout, testCase.profileName)
			}
			if strings.HasPrefix(strings.TrimSpace(result.stdout), "{") {
				t.Fatalf("stdout %q is JSON output, want text expectation report", result.stdout)
			}
		})
	}
}

func TestRunCommand_StatusExpectReturnsNonZeroForUnexpectedStates(t *testing.T) {
	tests := []struct {
		name         string
		profilesYAML string
		expected     string
		wantText     string
	}{
		{
			name: "no complete match",
			profilesYAML: strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: https://api.local.example.test
`) + "\n",
			expected: "Local",
			wantText: "no complete profile matches",
		},
		{
			name: "different complete match",
			profilesYAML: strings.TrimSpace(`
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
`) + "\n",
			expected: "Staging",
			wantText: "current complete profile is \"Local\"",
		},
		{
			name: "ambiguous complete match",
			profilesYAML: strings.TrimSpace(`
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
`) + "\n",
			expected: "Local",
			wantText: "current state is ambiguous",
		},
		{
			name: "missing expected profile",
			profilesYAML: strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`) + "\n",
			expected: "Missing",
			wantText: "expected profile \"Missing\" is not configured",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot, _, _ := writeVersionThreeCommandProject(t, testCase.profilesYAML)
			result := runCommandForTest(t, []string{"status", "--expect", testCase.expected}, projectRoot)
			if result.exitCode != runtimeExitCode {
				t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, runtimeExitCode, result.stdout, result.stderr)
			}
			if result.stderr != "" {
				t.Fatalf("stderr = %q, want empty assertion stderr", result.stderr)
			}
			for _, expected := range []string{"Expectation", "not matched", testCase.wantText} {
				if !strings.Contains(result.stdout, expected) {
					t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
				}
			}
			for _, forbidden := range []string{"postgres://old", "postgres://staging", "https://api.staging.example.test", "https://api.local.example.test"} {
				if strings.Contains(result.stdout, forbidden) {
					t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
				}
			}
		})
	}
}

func TestRunCommand_StatusExpectJSONIncludesAssertionResult(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	result := runCommandForTest(t, []string{"status", "--expect", "Missing", "--json"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty string for JSON assertion result", result.stderr)
	}

	var payload struct {
		Result struct {
			Assertion struct {
				ExpectedProfile  string   `json:"expectedProfile"`
				Matched          bool     `json:"matched"`
				ObservedStatus   string   `json:"observedStatus"`
				ObservedProfiles []string `json:"observedProfiles"`
				Message          string   `json:"message"`
			} `json:"assertion"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal status expectation JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.Assertion.ExpectedProfile != "Missing" || payload.Result.Assertion.Matched || payload.Result.Assertion.ObservedStatus != "matched" || !strings.Contains(payload.Result.Assertion.Message, "not configured") {
		t.Fatalf("assertion = %#v, want missing-profile assertion failure", payload.Result.Assertion)
	}
	if len(payload.Result.Assertion.ObservedProfiles) != 1 || payload.Result.Assertion.ObservedProfiles[0] != "Local" {
		t.Fatalf("observed profiles = %#v, want Local", payload.Result.Assertion.ObservedProfiles)
	}
}

func TestRunCommand_StatusExpectRejectsShortOutput(t *testing.T) {
	result := runCommandForTest(t, []string{"status", "--expect", "Local", "--short"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	if !strings.Contains(result.stderr, "status --expect cannot be combined with --short") {
		t.Fatalf("stderr %q does not contain --expect/--short usage error", result.stderr)
	}
}

func TestRunCommand_DiffExitCodeReportsChangeStatusWithoutWriting(t *testing.T) {
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
        value: http://localhost:5173
  - name: Env Missing
    values:
      - target: database
        valueFromEnv: MISSING_DATABASE_URL
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	alreadyMatching := runCommandForTest(t, []string{"diff", "Local", "--exit-code"}, projectRoot)
	if alreadyMatching.exitCode != 0 {
		t.Fatalf("already matching exitCode = %d, want 0 (stdout: %q, stderr: %q)", alreadyMatching.exitCode, alreadyMatching.stdout, alreadyMatching.stderr)
	}
	if !strings.Contains(alreadyMatching.stdout, "Already matches") {
		t.Fatalf("already matching stdout %q does not contain diff report", alreadyMatching.stdout)
	}

	wouldUpdate := runCommandForTest(t, []string{"diff", "Staging", "--exit-code"}, projectRoot)
	if wouldUpdate.exitCode != runtimeExitCode {
		t.Fatalf("would update exitCode = %d, want %d (stdout: %q, stderr: %q)", wouldUpdate.exitCode, runtimeExitCode, wouldUpdate.stdout, wouldUpdate.stderr)
	}
	if wouldUpdate.stderr != "" || !strings.Contains(wouldUpdate.stdout, "Would update") {
		t.Fatalf("would update stdout/stderr = %q/%q, want report on stdout only", wouldUpdate.stdout, wouldUpdate.stderr)
	}

	unavailable := runCommandForTest(t, []string{"diff", "Env Missing", "--exit-code"}, projectRoot)
	if unavailable.exitCode != runtimeExitCode {
		t.Fatalf("unavailable exitCode = %d, want %d (stdout: %q, stderr: %q)", unavailable.exitCode, runtimeExitCode, unavailable.stdout, unavailable.stderr)
	}
	if unavailable.stderr != "" || !strings.Contains(unavailable.stdout, "Unavailable") || !strings.Contains(unavailable.stdout, "MISSING_DATABASE_URL") {
		t.Fatalf("unavailable stdout/stderr = %q/%q, want unavailable report on stdout only", unavailable.stdout, unavailable.stderr)
	}

	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during diff --exit-code")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during diff --exit-code")
	}
}

func TestRunCommand_DiffExitCodeAppliesToJSONAndPatchOutput(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        value: postgres://staging
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	jsonResult := runCommandForTest(t, []string{"diff", "Staging", "--exit-code", "--json"}, projectRoot)
	if jsonResult.exitCode != runtimeExitCode {
		t.Fatalf("json exitCode = %d, want %d", jsonResult.exitCode, runtimeExitCode)
	}
	if jsonResult.stderr != "" {
		t.Fatalf("json stderr = %q, want empty", jsonResult.stderr)
	}
	var payload struct {
		Result struct {
			WouldUpdate []targetDescriptorTestJSON `json:"wouldUpdate"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(jsonResult.stdout), &payload); err != nil {
		t.Fatalf("unmarshal diff --exit-code JSON: %v\noutput: %q", err, jsonResult.stdout)
	}
	if len(payload.Result.WouldUpdate) != 1 || payload.Result.WouldUpdate[0].TargetName != "database" {
		t.Fatalf("wouldUpdate = %#v, want database", payload.Result.WouldUpdate)
	}

	patchResult := runCommandForTest(t, []string{"diff", "Staging", "--patch", "--exit-code"}, projectRoot)
	if patchResult.exitCode != runtimeExitCode {
		t.Fatalf("patch exitCode = %d, want %d", patchResult.exitCode, runtimeExitCode)
	}
	if patchResult.stderr != "" || !strings.Contains(patchResult.stdout, "# Switchlet managed patch: Staging") || !strings.Contains(patchResult.stdout, "- current: redacted") {
		t.Fatalf("patch stdout/stderr = %q/%q, want patch report only", patchResult.stdout, patchResult.stderr)
	}
}

func TestRunCommand_DoctorReportsHealthyProject(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	result := runCommandForTest(t, []string{"doctor"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for doctor command")
	}
	for _, expected := range []string{
		"Switchlet doctor",
		"[ok] configuration_discovery",
		"[ok] configuration_loading",
		"[ok] startup_target_validation",
		"[ok] profile_availability",
		"[ok] current_state_comparison",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres://old", "http://localhost:5173"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
}

func TestRunCommand_DoctorReportsMissingConfigAsFailure(t *testing.T) {
	result := runCommandForTest(t, []string{"doctor"}, t.TempDir())
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want doctor report on stdout only", result.stderr)
	}
	for _, expected := range []string{"Switchlet doctor", "[failed] configuration_discovery", "No .switchlet.yaml found.", "Run `switchlet init`"} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
}

func TestRunCommand_DoctorReportsInvalidConfigAsJSONFailure(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", "version: 3\nprofiles: [\n")

	result := runCommandForTest(t, []string{"doctor", "--json"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}

	var payload struct {
		Result struct {
			Command string `json:"command"`
			Status  string `json:"status"`
			Healthy bool   `json:"healthy"`
			Checks  []struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"checks"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal doctor JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.Command != "doctor" || payload.Result.Status != "failed" || payload.Result.Healthy || len(payload.Result.Checks) != 2 {
		t.Fatalf("doctor result = %#v, want failed doctor result with discovery and loading checks", payload.Result)
	}
	loadingCheck := payload.Result.Checks[1]
	if loadingCheck.Name != "configuration_loading" || loadingCheck.Status != "failed" || !strings.Contains(loadingCheck.Message, "parse configuration file") {
		t.Fatalf("loading check = %#v, want parse failure", loadingCheck)
	}
}

func TestRunCommand_DoctorReportsTargetValidationFailureWithoutLeakingValues(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"password":"current-secret"}}`)
	originalContents := readFileBytes(t, targetPath)
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

	result := runCommandForTest(t, []string{"doctor"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, runtimeExitCode, result.stdout, result.stderr)
	}
	for _, expected := range []string{"[failed] startup_target_validation", "database [json]", "backend/appsettings.Development.json", "database.url", "missing segment \"url\"", "[skipped] current_state_comparison"} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"current-secret", "resolved-secret"} {
		if strings.Contains(result.stdout, forbidden) || strings.Contains(result.stderr, forbidden) {
			t.Fatalf("doctor output must not contain raw value %q (stdout: %q, stderr: %q)", forbidden, result.stdout, result.stderr)
		}
	}
	if !bytes.Equal(readFileBytes(t, targetPath), originalContents) {
		t.Fatal("target file changed during doctor target validation")
	}
}

func TestRunCommand_DoctorReportsUnavailableEnvironmentProfilesAsWarning(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
  - name: Staging
    protected: true
    values:
      - target: database
        valueFromEnv: SWITCHLET_TEST_MISSING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"doctor", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 for warnings-only doctor result (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Result struct {
			Healthy bool   `json:"healthy"`
			Status  string `json:"status"`
			Checks  []struct {
				Name                string `json:"name"`
				Status              string `json:"status"`
				UnavailableProfiles []struct {
					ProfileName string `json:"profileName"`
					Protected   bool   `json:"protected"`
					Values      []struct {
						TargetName          string `json:"targetName"`
						EnvironmentVariable string `json:"environmentVariable"`
						Reason              string `json:"reason"`
					} `json:"values"`
				} `json:"unavailableProfiles"`
			} `json:"checks"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal doctor warning JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.Status != "warning" {
		t.Fatalf("status = %q, want warning", payload.Result.Status)
	}
	if payload.Result.Healthy {
		t.Fatal("healthy = true, want false for warning doctor result")
	}
	foundAvailabilityCheck := false
	for _, check := range payload.Result.Checks {
		if check.Name != "profile_availability" {
			continue
		}

		foundAvailabilityCheck = true
		if check.Status != "warning" || len(check.UnavailableProfiles) != 1 || check.UnavailableProfiles[0].ProfileName != "Staging" || !check.UnavailableProfiles[0].Protected {
			t.Fatalf("availability check = %#v, want protected Staging warning", check)
		}
		unavailableValues := check.UnavailableProfiles[0].Values
		if len(unavailableValues) != 1 || unavailableValues[0].TargetName != "database" || unavailableValues[0].EnvironmentVariable != "SWITCHLET_TEST_MISSING_DATABASE_URL" || !strings.Contains(unavailableValues[0].Reason, "SWITCHLET_TEST_MISSING_DATABASE_URL") {
			t.Fatalf("unavailable values = %#v, want database environment value", unavailableValues)
		}
	}
	if !foundAvailabilityCheck {
		t.Fatalf("checks = %#v, want profile_availability warning", payload.Result.Checks)
	}
	for _, forbidden := range []string{"postgres://old", "http://localhost:5173", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain raw value %q", result.stdout, forbidden)
		}
	}
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
