package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommand_DiffPatchEmitsManagedPatchWithoutWriting(t *testing.T) {
	const workerEnv = "SWITCHLET_TEST_STAGING_WORKER_QUEUE_ENDPOINT"
	t.Setenv(workerEnv, "")

	projectRoot, targetPaths := writeMixedCurrentStateCommandProject(t, workerEnv)
	originalContents := readTargetContents(t, targetPaths)
	configPath := filepath.Join(projectRoot, ".switchlet.yaml")
	originalConfig := readFileBytes(t, configPath)

	result := runCommandForTest(t, []string{"diff", "Staging", "--patch"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for diff --patch command")
	}

	for _, expected := range []string{
		"# Switchlet managed patch: Staging",
		"# values: shown for changed managed targets",
		"# protected: true",
		"# complete: false",
		"# targets: 4 included, 0 omitted, 4 configured",
		"# read-only: true",
		"diff --switchlet backend/appsettings.Development.json",
		"@@ -1 +1 @@ database [json] jsonPath: database.url",
		`- current: "postgres://local-secret"`,
		`+ profile: "postgres://staging-secret"`,
		"diff --switchlet worker/config.yaml",
		"@@ -1 +1 @@ workerQueue [yaml] yamlPath: queue.endpoint",
		" unavailable",
		" environment: " + workerEnv,
		` reason: profile "Staging" value for target "workerQueue" environment variable "` + workerEnv + `" is empty: environment variable is empty`,
		"diff --switchlet services/development.toml",
		"@@ -1 +1 @@ serviceEndpoint [toml] tomlPath: services.api.endpoint",
		`- current: "http://localhost:8080/secret"`,
		`+ profile: "https://services.staging.example.test/secret"`,
		"diff --switchlet frontend/.env.local",
		"@@ -1 +1 @@ frontendApi [dotenv] key: VITE_API_URL",
		" already matches",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{
		"http://localhost:4566/queue-secret",
		"http://localhost:5173/secret",
		"VITE_FEATURES=local",
	} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain unmanaged or unchanged value %q", result.stdout, forbidden)
		}
	}
	for _, absolutePath := range targetPaths {
		if strings.Contains(result.stdout, absolutePath) {
			t.Fatalf("stdout %q must use project-relative paths instead of %q", result.stdout, absolutePath)
		}
	}
	assertTargetContentsUnchanged(t, targetPaths, originalContents)
	if !bytes.Equal(readFileBytes(t, configPath), originalConfig) {
		t.Fatal(".switchlet.yaml changed during diff --patch")
	}
}

func TestRunCommand_DiffPatchReportsOmittedTargetsSafely(t *testing.T) {
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Database Only
    values:
      - target: database
        value: postgres://staging
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"diff", "Database Only", "--patch"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		"# Switchlet managed patch: Database Only",
		"# values: shown for changed managed targets",
		"# targets: 1 included, 1 omitted, 2 configured",
		"diff --switchlet backend/appsettings.Development.json",
		"@@ -1 +1 @@ database [json] jsonPath: database.url",
		`- current: "postgres://old"`,
		`+ profile: "postgres://staging"`,
		"# Omitted targets",
		"# frontendApi [dotenv]",
		"# file: frontend/.env.local",
		"# key: VITE_API_URL",
		"# unchanged by selected profile",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "http://localhost:5173") {
		t.Fatalf("stdout %q must not reveal omitted target value", result.stdout)
	}
	for _, absolutePath := range []string{databasePath, frontendPath} {
		if strings.Contains(result.stdout, absolutePath) {
			t.Fatalf("stdout %q must use project-relative paths instead of %q", result.stdout, absolutePath)
		}
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during diff --patch")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during diff --patch")
	}
}

func TestRunCommand_DiffPatchOutputRemainsPlainWithNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Database Only
    values:
      - target: database
        value: postgres://staging
`)+"\n")

	patchResult := runCommandForTest(t, []string{"diff", "Database Only", "--patch"}, projectRoot)
	if patchResult.exitCode != 0 {
		t.Fatalf("diff --patch exitCode = %d, want 0 (stdout: %q, stderr: %q)", patchResult.exitCode, patchResult.stdout, patchResult.stderr)
	}

	noColorResult := runCommandForTest(t, []string{"diff", "Database Only", "--patch", "--no-color"}, projectRoot)
	if noColorResult.exitCode != 0 {
		t.Fatalf("diff --patch --no-color exitCode = %d, want 0 (stdout: %q, stderr: %q)", noColorResult.exitCode, noColorResult.stdout, noColorResult.stderr)
	}
	if noColorResult.stdout != patchResult.stdout {
		t.Fatalf("diff --patch --no-color stdout = %q, want unchanged %q", noColorResult.stdout, patchResult.stdout)
	}
	if containsANSIStyling(noColorResult.stdout) {
		t.Fatalf("patch stdout %q contains ANSI styling", noColorResult.stdout)
	}
}

func TestRunCommand_DiffPatchAndJSONConflictReturnsUsageError(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://old
      - target: frontendApi
        value: http://localhost:5173
`)+"\n")

	result := runCommandForTest(t, []string{"diff", "Local", "--json", "--patch"}, projectRoot)
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty string for JSON usage error", result.stderr)
	}

	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal conflict JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "usage" || !strings.Contains(payload.Error.Message, "diff --patch cannot be combined with --json") {
		t.Fatalf("error = %#v, want JSON usage conflict", payload.Error)
	}
}
