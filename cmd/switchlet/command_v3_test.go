package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunCommand_ListVersionOneJSONReportsCompatibilityTarget(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=old;Database=App;"
  }
}
`)+"\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: Server=local;Database=App;
`)+"\n")

	result := runCommandForTest(t, []string{"list", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Profiles []struct {
			Name         string `json:"name"`
			Status       string `json:"status"`
			TargetCount  int    `json:"targetCount"`
			TotalTargets int    `json:"totalTargets"`
			Values       []struct {
				TargetName   string `json:"targetName"`
				TargetFile   string `json:"targetFile"`
				TargetType   string `json:"targetType"`
				SelectorName string `json:"selectorName"`
				Selector     string `json:"selector"`
				Status       string `json:"status"`
			} `json:"values"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal list JSON: %v\noutput: %q", err, result.stdout)
	}
	if len(payload.Profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(payload.Profiles))
	}
	profile := payload.Profiles[0]
	if profile.Name != "Local" || profile.Status != "available" {
		t.Fatalf("profile = %#v, want available Local", profile)
	}
	if profile.TargetCount != 1 || profile.TotalTargets != 1 {
		t.Fatalf("target count = %d/%d, want 1/1", profile.TargetCount, profile.TotalTargets)
	}
	if len(profile.Values) != 1 {
		t.Fatalf("len(values) = %d, want 1", len(profile.Values))
	}
	value := profile.Values[0]
	if value.TargetName != "default" {
		t.Fatalf("targetName = %q, want default", value.TargetName)
	}
	if value.TargetFile != targetPath {
		t.Fatalf("targetFile = %q, want %q", value.TargetFile, targetPath)
	}
	if value.TargetType != "json" || value.SelectorName != "jsonPath" || value.Selector != "ConnectionStrings.DefaultConnection" {
		t.Fatalf("value = %#v, want JSON compatibility target", value)
	}
	if value.Status != "available" {
		t.Fatalf("value.Status = %q, want available", value.Status)
	}
}

func TestRunCommand_ListVersionThreeShowsTargetCountsAndPartialProfiles(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	writeFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: backend/appsettings.Development.json
    type: json
    jsonPath: database.url
  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
  - name: Staging
    protected: true
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"list"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", result.exitCode, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for list command")
	}
	if !strings.Contains(result.stdout, "Local [1 target, partial]") {
		t.Fatalf("stdout %q does not include partial target count", result.stdout)
	}
	if !strings.Contains(result.stdout, "Staging [2 targets, protected, unavailable]") {
		t.Fatalf("stdout %q does not include protected unavailable target count", result.stdout)
	}
	if !strings.Contains(result.stdout, "STAGING_DATABASE_URL") {
		t.Fatalf("stdout %q does not include unavailable environment variable", result.stdout)
	}
}

func TestRunCommand_InspectVersionThreeReportsMultiTargetPlanSafely(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "Server=staging;Database=App;Password=super-secret;")

	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    protected: true
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"inspect", "Staging"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", result.exitCode, result.stderr)
	}

	for _, expected := range []string{
		"Profile: Staging",
		"Changes: 2 targets",
		"- database [json]",
		"file: " + databasePath,
		"jsonPath: database.url",
		"environment variable: STAGING_DATABASE_URL",
		"Password=****",
		"- frontendApi [dotenv]",
		"file: " + frontendPath,
		"key: VITE_API_URL",
		"masked value: https://api.staging.example.test",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	if strings.Contains(result.stdout, "super-secret") {
		t.Fatalf("stdout %q must not include the resolved secret", result.stdout)
	}
}

func TestRunCommand_InspectVersionThreeJSONContainsTargetAwareFields(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "Server=staging;Database=App;Pwd=super-secret;")

	projectRoot, databasePath, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"inspect", "Staging", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Profile struct {
			Name        string `json:"name"`
			Status      string `json:"status"`
			TargetCount int    `json:"targetCount"`
			Values      []struct {
				TargetName              string `json:"targetName"`
				TargetFile              string `json:"targetFile"`
				TargetType              string `json:"targetType"`
				SelectorName            string `json:"selectorName"`
				Selector                string `json:"selector"`
				Status                  string `json:"status"`
				EnvironmentVariableName string `json:"environmentVariableName"`
				MaskedValue             string `json:"maskedValue"`
			} `json:"values"`
		} `json:"profile"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal inspect JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Profile.Name != "Staging" || payload.Profile.Status != "available" || payload.Profile.TargetCount != 2 {
		t.Fatalf("profile = %#v, want available Staging with two targets", payload.Profile)
	}
	if len(payload.Profile.Values) != 2 {
		t.Fatalf("len(values) = %d, want 2", len(payload.Profile.Values))
	}
	databaseValue := payload.Profile.Values[0]
	if databaseValue.TargetName != "database" || databaseValue.TargetFile != databasePath || databaseValue.TargetType != "json" {
		t.Fatalf("database value = %#v, want database JSON target", databaseValue)
	}
	if databaseValue.SelectorName != "jsonPath" || databaseValue.Selector != "database.url" {
		t.Fatalf("database selector = %#v, want database.url JSON path", databaseValue)
	}
	if databaseValue.EnvironmentVariableName != "STAGING_DATABASE_URL" {
		t.Fatalf("environmentVariableName = %q, want STAGING_DATABASE_URL", databaseValue.EnvironmentVariableName)
	}
	if databaseValue.MaskedValue != "Server=staging;Database=App;Pwd=****;" {
		t.Fatalf("maskedValue = %q, want masked database value", databaseValue.MaskedValue)
	}
	if strings.Contains(result.stdout, "super-secret") {
		t.Fatalf("stdout %q must not include the resolved secret", result.stdout)
	}
}

func TestRunCommand_ApplyVersionThreeProfileUpdatesAllIncludedTargets(t *testing.T) {
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        value: postgres://staging
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
		"updated " + databasePath,
		"  database [json]",
		"  database.url",
		"updated " + frontendPath,
		"  frontendApi [dotenv]",
		"  VITE_API_URL",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"postgres://staging", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain resolved replacement value %q", result.stdout, forbidden)
		}
	}
	if !strings.Contains(string(readFileBytes(t, databasePath)), "postgres://staging") {
		t.Fatalf("database file %q was not updated", string(readFileBytes(t, databasePath)))
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_API_URL=https://api.staging.example.test") {
		t.Fatalf("dotenv file %q was not updated", string(readFileBytes(t, frontendPath)))
	}
}

func TestRunCommand_ApplyVersionThreeDryRunTextListsPlannedTargetsAndWritesNothing(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "Server=staging;Database=App;Password=super-secret;")
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"apply", "Staging", "--dry-run"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	for _, expected := range []string{
		`Dry run successful for profile "Staging"`,
		"Planned targets:",
		"would update " + databasePath,
		"  database [json]",
		"  database.url",
		"would update " + frontendPath,
		"  frontendApi [dotenv]",
		"  VITE_API_URL",
		"No changes were written.",
	} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
		}
	}
	for _, forbidden := range []string{"super-secret", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain resolved replacement value %q", result.stdout, forbidden)
		}
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during dry run")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during dry run")
	}
}

func TestRunCommand_ApplyVersionThreeUnavailableProfileIdentifiesTargetAndEnvironmentVariable(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}

	for _, expected := range []string{
		`Profile "Staging" is unavailable.`,
		"Unavailable values:",
		"- database",
		"environment variable: STAGING_DATABASE_URL",
		"Run `switchlet inspect Staging` to review profile values.",
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if strings.Contains(result.stderr, "https://api.staging.example.test") {
		t.Fatalf("stderr %q must not contain resolved literal value", result.stderr)
	}
}

func TestRunCommand_ApplyVersionThreeTargetPreparationErrorShowsSafeTargetContext(t *testing.T) {
	projectRoot, _, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: frontendApi
        value: "https://secret-value.example.test\nNEXT=value"
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}

	for _, expected := range []string{
		`Could not prepare target "frontendApi".`,
		"File:\n" + frontendPath,
		"Type:\ndotenv",
		"Selector:\nVITE_API_URL",
		"Reason:\nreplacement value must not contain newline characters",
		"Run `switchlet inspect Staging` to review planned targets.",
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if strings.Contains(result.stderr, "secret-value") {
		t.Fatalf("stderr %q must not contain resolved replacement value", result.stderr)
	}

	jsonResult := runCommandForTest(t, []string{"apply", "Staging", "--json"}, projectRoot)
	if jsonResult.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", jsonResult.exitCode, runtimeExitCode)
	}
	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(jsonResult.stdout), &payload); err != nil {
		t.Fatalf("unmarshal target-preparation JSON error: %v\noutput: %q", err, jsonResult.stdout)
	}
	if payload.Error.Kind != "runtime" {
		t.Fatalf("error.kind = %q, want runtime", payload.Error.Kind)
	}
	if !strings.Contains(payload.Error.Message, `Could not prepare target "frontendApi".`) || strings.Contains(payload.Error.Message, "secret-value") {
		t.Fatalf("error.message = %q, want safe target context", payload.Error.Message)
	}
}

func TestRunCommand_ApplyVersionThreeDryRunJSONDoesNotWriteTargets(t *testing.T) {
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Database Only
    values:
      - target: database
        value: postgres://dry-run
`)+"\n")
	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)

	result := runCommandForTest(t, []string{"apply", "Database Only", "--dry-run", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Result struct {
			ProfileName string `json:"profileName"`
			Status      string `json:"status"`
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
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal dry-run JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.ProfileName != "Database Only" || payload.Result.Status != "dry_run" || !payload.Result.DryRun {
		t.Fatalf("result = %#v, want dry-run Database Only", payload.Result)
	}
	if payload.Result.TargetCount != 1 || len(payload.Result.Changes) != 1 {
		t.Fatalf("target count/changes = %d/%d, want 1/1", payload.Result.TargetCount, len(payload.Result.Changes))
	}
	change := payload.Result.Changes[0]
	if change.TargetName != "database" || change.TargetFile != databasePath || change.TargetType != "json" || change.SelectorName != "jsonPath" || change.Selector != "database.url" {
		t.Fatalf("change = %#v, want database JSON path change", change)
	}
	if strings.Contains(result.stdout, "postgres://dry-run") {
		t.Fatalf("stdout %q must not contain resolved replacement value", result.stdout)
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during dry run")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during dry run")
	}
}

func TestRunCommand_ApplyVersionThreeJSONDoesNotExposeReplacementValues(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "Server=staging;Database=App;Password=super-secret;")
	projectRoot, databasePath, frontendPath := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Staging
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.test
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Staging", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Result struct {
			ProfileName string `json:"profileName"`
			Status      string `json:"status"`
			TargetCount int    `json:"targetCount"`
			DryRun      bool   `json:"dryRun"`
			Changes     []struct {
				TargetName string `json:"targetName"`
				TargetFile string `json:"targetFile"`
				TargetType string `json:"targetType"`
				Selector   string `json:"selector"`
			} `json:"changes"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal apply JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.ProfileName != "Staging" || payload.Result.Status != "applied" || payload.Result.DryRun {
		t.Fatalf("result = %#v, want applied Staging", payload.Result)
	}
	if payload.Result.TargetCount != 2 || len(payload.Result.Changes) != 2 {
		t.Fatalf("target count/changes = %d/%d, want 2/2", payload.Result.TargetCount, len(payload.Result.Changes))
	}
	if payload.Result.Changes[0].TargetName != "database" || payload.Result.Changes[0].TargetFile != databasePath || payload.Result.Changes[0].TargetType != "json" || payload.Result.Changes[0].Selector != "database.url" {
		t.Fatalf("database change = %#v, want database JSON target", payload.Result.Changes[0])
	}
	if payload.Result.Changes[1].TargetName != "frontendApi" || payload.Result.Changes[1].TargetFile != frontendPath || payload.Result.Changes[1].TargetType != "dotenv" || payload.Result.Changes[1].Selector != "VITE_API_URL" {
		t.Fatalf("frontend change = %#v, want frontend dotenv target", payload.Result.Changes[1])
	}
	for _, forbidden := range []string{"super-secret", "https://api.staging.example.test"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain resolved replacement value %q", result.stdout, forbidden)
		}
	}
	if !strings.Contains(string(readFileBytes(t, databasePath)), "Server=staging;Database=App;Password=super-secret;") {
		t.Fatalf("database file %q was not updated", string(readFileBytes(t, databasePath)))
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_API_URL=https://api.staging.example.test") {
		t.Fatalf("dotenv file %q was not updated", string(readFileBytes(t, frontendPath)))
	}
}

func writeVersionThreeCommandProject(t *testing.T, profilesYAML string) (string, string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	databasePath := writeFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\nVITE_FEATURES=local\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: backend/appsettings.Development.json
    type: json
    jsonPath: database.url
  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

`)+"\n"+profilesYAML)

	return projectRoot, databasePath, frontendPath
}
