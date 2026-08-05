package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunCommand_NoArgumentsRequiresInteractiveTerminal(t *testing.T) {
	result := runCommandForTest(t, nil, t.TempDir())
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, runtimeExitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for non-terminal default command")
	}
	for _, expected := range []string{
		"`switchlet` without a command launches the interactive profile picker.",
		"stdin and stdout must be interactive terminals.",
		"switchlet list",
		"switchlet status",
		"switchlet apply <profile-name>",
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestRunCommand_NoArgumentsRejectsNonTerminalOutput(t *testing.T) {
	forceTerminalFileDetection(t, true)

	projectRoot := writeMinimalCommandProject(t)
	input := openTerminalLikeFile(t, os.O_RDONLY)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	programStarted := false

	err := runCommandWithTerminalRunner(nil, projectRoot, func(model tea.Model) (tea.Model, error) {
		programStarted = true
		return model, nil
	}, input, &stdout)
	if err == nil {
		t.Fatal("runCommandWithTerminalRunner returned nil error, want terminal error")
	}
	if writeErr := writeCommandError(err, &stdout, &stderr); writeErr != nil {
		t.Fatalf("writeCommandError returned error: %v", writeErr)
	}
	if exitCodeForError(err) != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", exitCodeForError(err), runtimeExitCode)
	}
	if programStarted {
		t.Fatal("runProgram was called for non-terminal output")
	}
	if !strings.Contains(stderr.String(), "stdin and stdout must be interactive terminals") {
		t.Fatalf("stderr %q does not explain terminal requirement", stderr.String())
	}
}

func TestRunCommand_NoArgumentsRejectsNonTerminalInput(t *testing.T) {
	forceTerminalFileDetection(t, true)

	projectRoot := writeMinimalCommandProject(t)
	output := openTerminalLikeFile(t, os.O_WRONLY)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	programStarted := false

	err := runCommandWithTerminalRunner(nil, projectRoot, func(model tea.Model) (tea.Model, error) {
		programStarted = true
		return model, nil
	}, strings.NewReader(""), output)
	if err == nil {
		t.Fatal("runCommandWithTerminalRunner returned nil error, want terminal error")
	}
	if writeErr := writeCommandError(err, &stdout, &stderr); writeErr != nil {
		t.Fatalf("writeCommandError returned error: %v", writeErr)
	}
	if exitCodeForError(err) != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", exitCodeForError(err), runtimeExitCode)
	}
	if programStarted {
		t.Fatal("runProgram was called for non-terminal input")
	}
	if !strings.Contains(stderr.String(), "stdin and stdout must be interactive terminals") {
		t.Fatalf("stderr %q does not explain terminal requirement", stderr.String())
	}
}

func TestRunCommand_NoArgumentsStartsProgramWithInteractiveTerminals(t *testing.T) {
	forceTerminalFileDetection(t, true)

	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", strings.TrimSpace(`
{
  "services": {
    "backend": {
      "baseUrl": "https://old.example.test"
    }
  }
}
`)+"\n")

	input := openTerminalLikeFile(t, os.O_RDONLY)
	output := openTerminalLikeFile(t, os.O_WRONLY)
	programStarted := false

	err := runCommandWithTerminalRunner(nil, projectRoot, func(model tea.Model) (tea.Model, error) {
		programStarted = true
		if model == nil {
			t.Fatal("runProgram received nil model")
		}

		return model, nil
	}, input, output)
	if err != nil {
		t.Fatalf("runCommandWithTerminalRunner returned error: %v", err)
	}
	if !programStarted {
		t.Fatal("runProgram was not called for default command")
	}
}

func TestRunCommand_ListDoesNotStartProgramAndShowsAvailability(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
  - name: Production
    valueFromEnv: MYAPPLICATION_PRODUCTION_URL
    protected: true
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"list"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", result.exitCode, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for list command")
	}
	if !strings.Contains(result.stdout, "Local") {
		t.Fatalf("stdout %q does not include literal profile", result.stdout)
	}
	if !strings.Contains(result.stdout, "Production [protected, unavailable]") {
		t.Fatalf("stdout %q does not show protected unavailable profile", result.stdout)
	}
	if !strings.Contains(result.stdout, "MYAPPLICATION_PRODUCTION_URL") {
		t.Fatalf("stdout %q does not include unavailable reason", result.stdout)
	}
}

func TestRunCommand_ListJSONReturnsStructuredProfiles(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
  - name: Production
    valueFromEnv: MYAPPLICATION_PRODUCTION_URL
    protected: true
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"list", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Profiles []struct {
			Name              string `json:"name"`
			Protected         bool   `json:"protected"`
			Available         bool   `json:"available"`
			UnavailableReason string `json:"unavailableReason"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal list JSON: %v\noutput: %q", err, result.stdout)
	}
	if len(payload.Profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2", len(payload.Profiles))
	}
	if payload.Profiles[1].Name != "Production" {
		t.Fatalf("profiles[1].Name = %q, want %q", payload.Profiles[1].Name, "Production")
	}
	if !payload.Profiles[1].Protected {
		t.Fatal("profiles[1].Protected = false, want true")
	}
	if payload.Profiles[1].Available {
		t.Fatal("profiles[1].Available = true, want false")
	}
	if !strings.Contains(payload.Profiles[1].UnavailableReason, "MYAPPLICATION_PRODUCTION_URL") {
		t.Fatalf("profiles[1].UnavailableReason = %q, want environment variable name", payload.Profiles[1].UnavailableReason)
	}
}

func TestRunCommand_ConfigFlagLoadsExplicitProjectAndResolvesTargetsFromConfigDirectory(t *testing.T) {
	workingDirectory, explicitConfigPath := writeConfigSelectionProjects(t)
	relativeConfigPath, err := filepath.Rel(workingDirectory, explicitConfigPath)
	if err != nil {
		t.Fatalf("make relative config path: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{name: "list", args: []string{"list", "--config", relativeConfigPath}, expected: []string{"Explicit"}},
		{name: "inspect", args: []string{"inspect", "--config", relativeConfigPath, "Explicit"}, expected: []string{"Profile: Explicit"}},
		{name: "apply dry run", args: []string{"apply", "--dry-run", "--config", relativeConfigPath, "Explicit"}, expected: []string{`Dry run successful for profile "Explicit"`, "No changes were written."}},
		{name: "status", args: []string{"status", "--config", relativeConfigPath, "--short"}, expected: []string{"Current profile: Explicit"}},
		{name: "diff", args: []string{"diff", "--config", relativeConfigPath, "Explicit"}, expected: []string{"Switchlet diff", "Already matches", "config/runtime.json"}},
		{name: "doctor", args: []string{"doctor", "--config", relativeConfigPath}, expected: []string{"Switchlet doctor", "using explicit configuration file", "[ok] configuration_loading"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCommandForTest(t, testCase.args, workingDirectory)
			if result.exitCode != 0 {
				t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
			}
			if result.programStarted {
				t.Fatal("runProgram was called for non-interactive command")
			}
			for _, expected := range testCase.expected {
				if !strings.Contains(result.stdout, expected) {
					t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
				}
			}
			if strings.Contains(result.stdout, "Default") {
				t.Fatalf("stdout %q includes discovered project profile instead of explicit config", result.stdout)
			}
		})
	}

	nameResult := runCommandForTest(t, []string{"status", "--name", "--config", relativeConfigPath}, workingDirectory)
	if nameResult.exitCode != 0 {
		t.Fatalf("status --name exitCode = %d, want 0 (stdout: %q, stderr: %q)", nameResult.exitCode, nameResult.stdout, nameResult.stderr)
	}
	if nameResult.stdout != "Explicit\n" {
		t.Fatalf("status --name stdout = %q, want exact profile name", nameResult.stdout)
	}
}

func TestRunCommand_ConfigFlagSelectsProjectForInteractiveStartup(t *testing.T) {
	forceTerminalFileDetection(t, true)

	workingDirectory, explicitConfigPath := writeConfigSelectionProjects(t)
	relativeConfigPath, err := filepath.Rel(workingDirectory, explicitConfigPath)
	if err != nil {
		t.Fatalf("make relative config path: %v", err)
	}
	input := openTerminalLikeFile(t, os.O_RDONLY)
	output := openTerminalLikeFile(t, os.O_WRONLY)
	programStarted := false

	err = runCommandWithTerminalRunner([]string{"--config", relativeConfigPath}, workingDirectory, func(model tea.Model) (tea.Model, error) {
		programStarted = true
		updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 32})
		view := updatedModel.View()
		if !strings.Contains(view, "Explicit") {
			t.Fatalf("View() = %q, want explicit config profile", view)
		}
		if strings.Contains(view, "Default") {
			t.Fatalf("View() = %q, must not show discovered project profile", view)
		}

		return model, nil
	}, input, output)
	if err != nil {
		t.Fatalf("runCommandWithTerminalRunner returned error: %v", err)
	}
	if !programStarted {
		t.Fatal("runProgram was not called for interactive startup")
	}
}

func TestRunCommand_ConfigFlagReportsUsageAndLoadFailures(t *testing.T) {
	workingDirectory := t.TempDir()

	missingValue := runCommandForTest(t, []string{"list", "--config"}, workingDirectory)
	if missingValue.exitCode != usageExitCode {
		t.Fatalf("missing value exitCode = %d, want %d", missingValue.exitCode, usageExitCode)
	}
	if !strings.Contains(missingValue.stderr, "--config requires a path") {
		t.Fatalf("stderr %q does not explain missing config path", missingValue.stderr)
	}

	missingFile := runCommandForTest(t, []string{"list", "--config", "missing.yaml"}, workingDirectory)
	if missingFile.exitCode != runtimeExitCode {
		t.Fatalf("missing file exitCode = %d, want %d", missingFile.exitCode, runtimeExitCode)
	}
	if !strings.Contains(missingFile.stderr, "read configuration file") || !strings.Contains(missingFile.stderr, "missing.yaml") {
		t.Fatalf("stderr %q does not explain missing explicit config file", missingFile.stderr)
	}

	invalidConfigPath := writeFile(t, workingDirectory, "invalid.switchlet.yaml", "version: 3\nprofiles: [\n")
	invalidConfig := runCommandForTest(t, []string{"list", "--config", invalidConfigPath}, workingDirectory)
	if invalidConfig.exitCode != runtimeExitCode {
		t.Fatalf("invalid config exitCode = %d, want %d", invalidConfig.exitCode, runtimeExitCode)
	}
	if !strings.Contains(invalidConfig.stderr, "parse configuration file") {
		t.Fatalf("stderr %q does not explain invalid explicit config file", invalidConfig.stderr)
	}
}

func TestRunCommand_ConfigFlagDoesNotLoadForUtilityCommands(t *testing.T) {
	workingDirectory := t.TempDir()
	missingConfigPath := filepath.Join(workingDirectory, "missing.yaml")

	versionResult := runCommandForTest(t, []string{"--config", missingConfigPath, "version"}, workingDirectory)
	if versionResult.exitCode != 0 || !strings.HasPrefix(versionResult.stdout, "switchlet ") {
		t.Fatalf("version result = %#v, want success without loading config", versionResult)
	}

	completionResult := runCommandForTest(t, []string{"--config", missingConfigPath, "completion", "bash"}, workingDirectory)
	if completionResult.exitCode != 0 || !strings.Contains(completionResult.stdout, "# bash completion for switchlet") {
		t.Fatalf("completion result = %#v, want script without loading config", completionResult)
	}

	for _, testCase := range []struct {
		name string
		args []string
	}{
		{name: "init", args: []string{"init", "--config", missingConfigPath}},
		{name: "version", args: []string{"version", "--config", missingConfigPath}},
		{name: "completion", args: []string{"completion", "--config", missingConfigPath, "bash"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCommandForTest(t, testCase.args, workingDirectory)
			if result.exitCode != usageExitCode {
				t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, usageExitCode, result.stdout, result.stderr)
			}
			if !strings.Contains(result.stderr, `unsupported flag "--config"`) {
				t.Fatalf("stderr %q does not reject subcommand --config", result.stderr)
			}
		})
	}
}

func TestRunCommand_ConfigFlagPreservesDashPrefixedProfileAfterDelimiter(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config/runtime.json
    type: json
    jsonPath: database.url

profiles:
  - name: "-Local"
    values:
      - target: database
        value: postgres://local
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"database":{"url":"postgres://old"}}`)

	result := runCommandForTest(t, []string{"inspect", "--config", configPath, "--", "-Local"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "Profile: -Local") {
		t.Fatalf("stdout %q does not inspect dash-prefixed profile", result.stdout)
	}
}

func TestRunCommand_JSONOutputsIncludeSchemaVersion(t *testing.T) {
	projectRoot := writeMinimalCommandProject(t)

	tests := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"list", "--json"}},
		{name: "inspect", args: []string{"inspect", "Local", "--json"}},
		{name: "apply", args: []string{"apply", "Local", "--dry-run", "--json"}},
		{name: "status", args: []string{"status", "--json"}},
		{name: "diff", args: []string{"diff", "Local", "--json"}},
		{name: "doctor", args: []string{"doctor", "--json"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCommandForTest(t, testCase.args, projectRoot)
			if result.exitCode != 0 {
				t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
			}
			assertJSONSchemaVersion(t, result.stdout)
		})
	}
}

func TestRunCommand_JSONErrorsIncludeSchemaVersion(t *testing.T) {
	projectRoot := writeMinimalCommandProject(t)

	tests := []struct {
		name string
		args []string
	}{
		{name: "runtime", args: []string{"inspect", "Missing", "--json"}},
		{name: "usage", args: []string{"status", "--short", "--json"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCommandForTest(t, testCase.args, projectRoot)
			if result.exitCode == 0 {
				t.Fatalf("exitCode = 0, want failure (stdout: %q, stderr: %q)", result.stdout, result.stderr)
			}
			if result.stderr != "" {
				t.Fatalf("stderr = %q, want JSON error on stdout", result.stderr)
			}
			assertJSONSchemaVersion(t, result.stdout)
		})
	}
}

func TestRunCommand_ListHelpWritesSubcommandUsageWithoutLoadingConfiguration(t *testing.T) {
	result := runCommandForTest(t, []string{"list", "--help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for list help")
	}
	if !strings.Contains(result.stdout, "switchlet list [--json]") {
		t.Fatalf("stdout %q does not contain list usage", result.stdout)
	}
}

func TestRunCommand_InspectDoesNotStartProgramAndShowsMaskedValue(t *testing.T) {
	t.Setenv("MYAPPLICATION_PRODUCTION_URL", "Server=prod;Database=App;Password=super-secret;")

	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Production
    valueFromEnv: MYAPPLICATION_PRODUCTION_URL
    protected: true
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"inspect", "Production"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", result.exitCode, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for inspect command")
	}
	if !strings.Contains(result.stdout, "Profile: Production") {
		t.Fatalf("stdout %q does not include profile name", result.stdout)
	}
	if !strings.Contains(result.stdout, "Environment variable: MYAPPLICATION_PRODUCTION_URL") {
		t.Fatalf("stdout %q does not include environment variable name", result.stdout)
	}
	if !strings.Contains(result.stdout, "Masked value:\n****") {
		t.Fatalf("stdout %q does not include redacted value", result.stdout)
	}
	if strings.Contains(result.stdout, "super-secret") {
		t.Fatalf("stdout %q must not include the secret value", result.stdout)
	}
}

func TestRunCommand_InspectJSONReturnsStructuredProfile(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_URL", "Server=test;Database=App;Pwd=secret;")

	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Test
    valueFromEnv: MYAPPLICATION_TEST_URL
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"inspect", "Test", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Profile struct {
			Name                    string `json:"name"`
			Available               bool   `json:"available"`
			Source                  string `json:"source"`
			EnvironmentVariableName string `json:"environmentVariableName"`
			MaskedValue             string `json:"maskedValue"`
		} `json:"profile"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal inspect JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Profile.Name != "Test" {
		t.Fatalf("profile.Name = %q, want %q", payload.Profile.Name, "Test")
	}
	if !payload.Profile.Available {
		t.Fatal("profile.Available = false, want true")
	}
	if payload.Profile.Source != "environment" {
		t.Fatalf("profile.Source = %q, want %q", payload.Profile.Source, "environment")
	}
	if payload.Profile.EnvironmentVariableName != "MYAPPLICATION_TEST_URL" {
		t.Fatalf("profile.EnvironmentVariableName = %q, want %q", payload.Profile.EnvironmentVariableName, "MYAPPLICATION_TEST_URL")
	}
	if payload.Profile.MaskedValue != "****" {
		t.Fatalf("profile.MaskedValue = %q, want redacted value", payload.Profile.MaskedValue)
	}
}

func TestRunCommand_InspectJSONReturnsStructuredProfileNotFoundError(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"inspect", "Missing", "--json"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}

	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal inspect JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "profile_not_found" {
		t.Fatalf("error.kind = %q, want %q", payload.Error.Kind, "profile_not_found")
	}
	if !strings.Contains(payload.Error.Message, `Profile "Missing" does not exist.`) || !strings.Contains(payload.Error.Message, "Available profiles:\n- Local") {
		t.Fatalf("error.message = %q, want profile-not-found guidance", payload.Error.Message)
	}
}

func TestRunCommand_ApplyMissingProfileJSONIncludesAvailableProfilesAndSuggestion(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
  - name: Staging
    value: https://staging.example.test
  - name: Production
    value: https://production.example.test
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"apply", "Stagng", "--json"}, projectRoot)
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
		t.Fatalf("unmarshal apply JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "profile_not_found" {
		t.Fatalf("error.kind = %q, want %q", payload.Error.Kind, "profile_not_found")
	}
	for _, expected := range []string{
		`Profile "Stagng" does not exist.`,
		"Available profiles:\n- Local\n- Staging\n- Production",
		`Did you mean "Staging"?`,
	} {
		if !strings.Contains(payload.Error.Message, expected) {
			t.Fatalf("error.message = %q, want profile-not-found guidance %q", payload.Error.Message, expected)
		}
	}
	for _, forbidden := range []string{"http://localhost:8080", "https://staging.example.test", "https://production.example.test"} {
		if strings.Contains(payload.Error.Message, forbidden) {
			t.Fatalf("error.message = %q must not contain resolved value %q", payload.Error.Message, forbidden)
		}
	}
}

func TestRunCommand_InspectMissingProfileListsAvailableProfilesAndSuggestion(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
  - name: Staging
    value: https://staging.example.test
  - name: Production
    value: https://production.example.test
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"inspect", "Stagng"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{
		`Profile "Stagng" does not exist.`,
		"Available profiles:",
		"- Local",
		"- Staging",
		"- Production",
		`Did you mean "Staging"?`,
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestRunCommand_ApplyMissingProfileOmitsAmbiguousSuggestion(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: AbcX
    value: https://x.example.test
  - name: AbcY
    value: https://y.example.test
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"apply", "Abc"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{`Profile "Abc" does not exist.`, "- AbcX", "- AbcY"} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if strings.Contains(result.stderr, "Did you mean") {
		t.Fatalf("stderr %q must not include an ambiguous suggestion", result.stderr)
	}
}

func TestRunCommand_ApplyWithoutProfileShowsConfiguredProfileGuidance(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
  - name: Staging
    protected: true
    values:
      - target: database
        value: postgres://staging
      - target: frontendApi
        value: https://api.staging.example.test
  - name: Database Only
    values:
      - target: database
        value: postgres://database-only
`)+"\n")

	result := runCommandForTest(t, []string{"apply"}, projectRoot)
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{
		"No profile specified.",
		"Available profiles:",
		"- Local [partial]",
		"- Staging [protected]",
		"- Database Only [partial]",
		"Try:\nswitchlet apply Local --dry-run",
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if strings.Contains(result.stderr, "postgres://") || strings.Contains(result.stderr, "https://api.staging.example.test") {
		t.Fatalf("stderr %q must not contain resolved replacement values", result.stderr)
	}
}

func TestRunCommand_InspectWithoutProfileShowsConfiguredProfileGuidance(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Database Only
    values:
      - target: database
        value: postgres://database-only
`)+"\n")

	result := runCommandForTest(t, []string{"inspect"}, projectRoot)
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{
		"No profile specified.",
		"Available profiles:",
		"- Database Only [partial]",
		"Try:\nswitchlet inspect \"Database Only\"",
	} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestRunCommand_ProfileCommandsWithoutProfileJSONShowConfiguredGuidance(t *testing.T) {
	projectRoot, _, _ := writeVersionThreeCommandProject(t, strings.TrimSpace(`
profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local-secret
  - name: Production
    protected: true
    values:
      - target: database
        value: postgres://production-secret
      - target: frontendApi
        value: https://api.production.example.test
`)+"\n")

	tests := []struct {
		name    string
		args    []string
		wantTry string
	}{
		{name: "apply", args: []string{"apply", "--json"}, wantTry: "switchlet apply Local --dry-run"},
		{name: "inspect", args: []string{"inspect", "--json"}, wantTry: "switchlet inspect Local"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCommandForTest(t, testCase.args, projectRoot)
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
				t.Fatalf("unmarshal JSON usage error: %v\noutput: %q", err, result.stdout)
			}
			if payload.Error.Kind != "usage" {
				t.Fatalf("error.kind = %q, want usage", payload.Error.Kind)
			}
			for _, expected := range []string{
				"No profile specified.",
				"Available profiles:\n- Local [partial]\n- Production [protected]",
				"Try:\n" + testCase.wantTry,
			} {
				if !strings.Contains(payload.Error.Message, expected) {
					t.Fatalf("error.message = %q, want guidance %q", payload.Error.Message, expected)
				}
			}
			for _, forbidden := range []string{"postgres://local-secret", "postgres://production-secret", "https://api.production.example.test"} {
				if strings.Contains(payload.Error.Message, forbidden) {
					t.Fatalf("error.message = %q must not contain resolved value %q", payload.Error.Message, forbidden)
				}
			}
		})
	}
}

func TestRunCommand_ApplyWithoutProfileFallsBackToUsageWhenConfigCannotLoad(t *testing.T) {
	result := runCommandForTest(t, []string{"apply"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{"No profile specified.", "Usage:", "switchlet apply <profile-name>"} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
	if strings.Contains(result.stderr, "configuration file not found") {
		t.Fatalf("stderr %q should stay focused on the usage mistake", result.stderr)
	}
}

func TestRunCommand_VersionPrintsWithoutLoadingConfiguration(t *testing.T) {
	commandResult := runCommandForTest(t, []string{"version"}, t.TempDir())
	flagResult := runCommandForTest(t, []string{"--version"}, t.TempDir())

	if commandResult.exitCode != 0 {
		t.Fatalf("switchlet version exitCode = %d, want 0 (stdout: %q, stderr: %q)", commandResult.exitCode, commandResult.stdout, commandResult.stderr)
	}
	if flagResult.exitCode != 0 {
		t.Fatalf("switchlet --version exitCode = %d, want 0 (stdout: %q, stderr: %q)", flagResult.exitCode, flagResult.stdout, flagResult.stderr)
	}
	if commandResult.programStarted || flagResult.programStarted {
		t.Fatal("version command started the terminal program")
	}
	if commandResult.stdout == "" || !strings.HasPrefix(commandResult.stdout, "switchlet ") {
		t.Fatalf("version stdout = %q, want concise switchlet version line", commandResult.stdout)
	}
	if flagResult.stdout != commandResult.stdout {
		t.Fatalf("--version stdout = %q, want %q", flagResult.stdout, commandResult.stdout)
	}
}

func TestRunCommand_VersionUsesBuildOverride(t *testing.T) {
	originalBuildVersion := buildVersion
	buildVersion = "v0.20.0-test"
	t.Cleanup(func() { buildVersion = originalBuildVersion })

	result := runCommandForTest(t, []string{"version"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.stdout != "switchlet v0.20.0-test\n" {
		t.Fatalf("stdout = %q, want build override version", result.stdout)
	}
}

func TestRunCommand_HelpListsVersionCommand(t *testing.T) {
	result := runCommandForTest(t, []string{"help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "switchlet version") {
		t.Fatalf("stdout %q does not list version command", result.stdout)
	}
}

func TestRunCommand_HelpAlignsStatusUsageRow(t *testing.T) {
	result := runCommandForTest(t, []string{"help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	expected := "switchlet status [flags]                       Compare current managed values with configured profiles"
	if !strings.Contains(result.stdout, expected) {
		t.Fatalf("stdout %q does not contain aligned status usage row %q", result.stdout, expected)
	}
}

func TestRunCommand_HelpListsDoctorCommand(t *testing.T) {
	result := runCommandForTest(t, []string{"help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "switchlet doctor [--json]") {
		t.Fatalf("stdout %q does not list doctor command", result.stdout)
	}
}

func TestRunCommand_DoctorHelpDoesNotLoadConfiguration(t *testing.T) {
	result := runCommandForTest(t, []string{"doctor", "--help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("doctor help started the terminal program")
	}
	if !strings.Contains(result.stdout, "switchlet doctor [--json]") || !strings.Contains(result.stdout, "project health checks") {
		t.Fatalf("stdout %q does not contain doctor help", result.stdout)
	}
}

func TestDefaultCommandOutputOptionsUsePlainOutputWhenOutputIsRedirected(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	var redirectedOutput bytes.Buffer
	redirectedOptions := defaultCommandOutputOptions(&redirectedOutput)
	if !redirectedOptions.NoColor {
		t.Fatal("NoColor = false for redirected output, want plain command output")
	}

	characterDeviceOutput := openTerminalLikeFile(t, os.O_WRONLY)
	characterDeviceOptions := defaultCommandOutputOptions(characterDeviceOutput)
	if !characterDeviceOptions.NoColor {
		t.Fatal("NoColor = false for non-terminal character device output, want plain command output")
	}

	forceTerminalFileDetection(t, true)
	terminalOutput := openTerminalLikeFile(t, os.O_WRONLY)
	terminalOptions := defaultCommandOutputOptions(terminalOutput)
	if terminalOptions.NoColor {
		t.Fatal("NoColor = true for terminal output without NO_COLOR, want styled output allowed")
	}
}

func TestParseArgumentsSupportsDashPrefixedPositionalsAfterDelimiter(t *testing.T) {
	jsonOutput := false
	positionals, err := parseArguments([]string{"--json", "--", "-Local"}, map[string]*bool{"--json": &jsonOutput}, nil)
	if err != nil {
		t.Fatalf("parseArguments returned error: %v", err)
	}
	if !jsonOutput {
		t.Fatal("jsonOutput = false, want true")
	}
	if len(positionals) != 1 || positionals[0] != "-Local" {
		t.Fatalf("positionals = %#v, want [-Local]", positionals)
	}
}

func TestRunCommand_UnknownCommandSuggestsVersion(t *testing.T) {
	result := runCommandForTest(t, []string{"versoin"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{`unknown command "versoin"`, `Did you mean "version"?`} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestDisplayProjectPathKeepsOutsideProjectPathAbsolute(t *testing.T) {
	projectRoot := filepath.Join(t.TempDir(), "project")
	outsidePath := filepath.Join(t.TempDir(), "runtime.json")

	if got := displayProjectPath(projectRoot, outsidePath); got != outsidePath {
		t.Fatalf("displayProjectPath() = %q, want outside path %q", got, outsidePath)
	}
}

func TestRunCommand_UnknownCommandSuggestsNearestCommand(t *testing.T) {
	result := runCommandForTest(t, []string{"aply"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{`unknown command "aply"`, `Did you mean "apply"?`, "switchlet help [command]"} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestRunCommand_UnsupportedFlagSuggestsNearestFlag(t *testing.T) {
	result := runCommandForTest(t, []string{"apply", "Local", "--dryrun"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{`apply: unsupported flag "--dryrun"`, `Did you mean "--dry-run"?`, "switchlet apply <profile-name>"} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestRunCommand_UnsupportedFlagSuggestsNoColorFlag(t *testing.T) {
	result := runCommandForTest(t, []string{"status", "--no-colr"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{`status: unsupported flag "--no-colr"`, `Did you mean "--no-color"`, "switchlet status [--json] [--short] [--expect <profile-name>]"} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestRunCommand_NoColorFlagRemovesANSIFromCommandError(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	result := runCommandForTest(t, []string{"list", "--no-color"}, t.TempDir())
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	if containsANSIStyling(result.stderr) {
		t.Fatalf("stderr %q contains ANSI styling despite --no-color", result.stderr)
	}
	if !strings.Contains(result.stderr, "No .switchlet.yaml found.") {
		t.Fatalf("stderr %q does not contain config-not-found guidance", result.stderr)
	}
}

func TestRunCommand_NoColorEnvironmentRemovesANSIFromCommandError(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	result := runCommandForTest(t, []string{"list"}, t.TempDir())
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	if containsANSIStyling(result.stderr) {
		t.Fatalf("stderr %q contains ANSI styling despite NO_COLOR", result.stderr)
	}
}

func TestRunCommand_ApplyUpdatesTargetFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Local"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, `Applied profile "Local"`) {
		t.Fatalf("stdout %q does not include apply success", result.stdout)
	}
	if !strings.Contains(result.stdout, "Updated target:") || !strings.Contains(result.stdout, "updated config/runtime.json") {
		t.Fatalf("stdout %q does not include updated target marker", result.stdout)
	}
	if !strings.Contains(result.stdout, "default [json]") {
		t.Fatalf("stdout %q does not include target name and type", result.stdout)
	}
	if strings.Contains(result.stdout, targetPath) {
		t.Fatalf("stdout %q must use project-relative target path instead of %q", result.stdout, targetPath)
	}
	if !strings.Contains(result.stdout, "services.backend.baseUrl") {
		t.Fatalf("stdout %q does not include target JSON path", result.stdout)
	}
	if strings.Contains(result.stdout, "http://localhost:8080") {
		t.Fatalf("stdout %q must not include resolved replacement value", result.stdout)
	}
	if !strings.Contains(string(readFileBytes(t, targetPath)), "http://localhost:8080") {
		t.Fatalf("target file %q was not updated", string(readFileBytes(t, targetPath)))
	}
}

func TestRunCommand_ApplyProtectedProfileRequiresExplicitOptIn(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)
	originalContents := readFileBytes(t, targetPath)
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Production
    value: https://prod.example.test
    protected: true
`)+"\n")

	refused := runCommandForTest(t, []string{"apply", "Production", "--json"}, projectRoot)
	if refused.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", refused.exitCode, runtimeExitCode)
	}
	if refused.stderr != "" {
		t.Fatalf("stderr = %q, want empty string for JSON error", refused.stderr)
	}

	var refusedPayload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(refused.stdout), &refusedPayload); err != nil {
		t.Fatalf("unmarshal protected apply JSON error: %v\noutput: %q", err, refused.stdout)
	}
	if refusedPayload.Error.Kind != "protected_profile" {
		t.Fatalf("error.kind = %q, want %q", refusedPayload.Error.Kind, "protected_profile")
	}
	if !bytes.Equal(readFileBytes(t, targetPath), originalContents) {
		t.Fatal("target file changed after protected-profile refusal")
	}

	allowed := runCommandForTest(t, []string{"apply", "Production", "--allow-protected"}, projectRoot)
	if allowed.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", allowed.exitCode, allowed.stderr)
	}
	if !strings.Contains(allowed.stdout, `Applied profile "Production"`) {
		t.Fatalf("stdout %q does not include protected apply success", allowed.stdout)
	}
	if !strings.Contains(string(readFileBytes(t, targetPath)), "https://prod.example.test") {
		t.Fatalf("target file %q was not updated", string(readFileBytes(t, targetPath)))
	}
}

func TestRunCommand_ApplyUnavailableProfileReturnsStructuredJSONError(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Production
    valueFromEnv: MYAPPLICATION_PRODUCTION_URL
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	result := runCommandForTest(t, []string{"apply", "Production", "--json"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}

	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal unavailable apply JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "profile_unavailable" {
		t.Fatalf("error.kind = %q, want %q", payload.Error.Kind, "profile_unavailable")
	}
	if !strings.Contains(payload.Error.Message, "MYAPPLICATION_PRODUCTION_URL") {
		t.Fatalf("error.message = %q, want environment variable name", payload.Error.Message)
	}
}

func TestRunCommand_ApplyProtectedDryRunRequiresOptInAndDoesNotWrite(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)
	originalContents := readFileBytes(t, targetPath)
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Production
    value: https://prod.example.test
    protected: true
`)+"\n")

	refused := runCommandForTest(t, []string{"apply", "Production", "--dry-run", "--json"}, projectRoot)
	if refused.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", refused.exitCode, runtimeExitCode)
	}

	var refusedPayload struct {
		Error struct {
			Kind string `json:"kind"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(refused.stdout), &refusedPayload); err != nil {
		t.Fatalf("unmarshal protected dry-run JSON error: %v\noutput: %q", err, refused.stdout)
	}
	if refusedPayload.Error.Kind != "protected_profile" {
		t.Fatalf("error.kind = %q, want %q", refusedPayload.Error.Kind, "protected_profile")
	}
	if !bytes.Equal(readFileBytes(t, targetPath), originalContents) {
		t.Fatal("target file changed after protected dry-run refusal")
	}

	allowed := runCommandForTest(t, []string{"apply", "Production", "--dry-run", "--allow-protected"}, projectRoot)
	if allowed.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", allowed.exitCode, allowed.stderr)
	}
	if !strings.Contains(allowed.stdout, `Dry run successful for profile "Production"`) {
		t.Fatalf("stdout %q does not include dry-run success", allowed.stdout)
	}
	if !strings.Contains(allowed.stdout, "Would update:") || !strings.Contains(allowed.stdout, "would update config/runtime.json") {
		t.Fatalf("stdout %q does not include planned target marker", allowed.stdout)
	}
	if !strings.Contains(allowed.stdout, "default [json]") {
		t.Fatalf("stdout %q does not include target name and type", allowed.stdout)
	}
	if strings.Contains(allowed.stdout, targetPath) {
		t.Fatalf("stdout %q must use project-relative target path instead of %q", allowed.stdout, targetPath)
	}
	if !strings.Contains(allowed.stdout, "services.backend.baseUrl") {
		t.Fatalf("stdout %q does not include target JSON path", allowed.stdout)
	}
	if !strings.Contains(allowed.stdout, "No changes were written.") {
		t.Fatalf("stdout %q does not state that no changes were written", allowed.stdout)
	}
	if strings.Contains(allowed.stdout, "https://prod.example.test") {
		t.Fatalf("stdout %q must not include resolved replacement value", allowed.stdout)
	}
	if !bytes.Equal(readFileBytes(t, targetPath), originalContents) {
		t.Fatal("target file changed during protected dry run")
	}
}

func TestRunCommand_ApplyDryRunJSONDoesNotWriteTargetFile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)
	originalContents := readFileBytes(t, targetPath)
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
`)+"\n")

	result := runCommandForTest(t, []string{"apply", "Local", "--dry-run", "--json"}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}

	var payload struct {
		Result struct {
			ProfileName string `json:"profileName"`
			TargetPath  string `json:"targetPath"`
			TargetFile  string `json:"targetFile"`
			DryRun      bool   `json:"dryRun"`
			Preview     struct {
				Complete    bool `json:"complete"`
				WouldUpdate []struct {
					TargetName string `json:"targetName"`
					TargetFile string `json:"targetFile"`
					Selector   string `json:"selector"`
				} `json:"wouldUpdate"`
			} `json:"preview"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal apply JSON: %v\noutput: %q", err, result.stdout)
	}
	if payload.Result.ProfileName != "Local" {
		t.Fatalf("result.ProfileName = %q, want %q", payload.Result.ProfileName, "Local")
	}
	if payload.Result.TargetPath != "services.backend.baseUrl" {
		t.Fatalf("result.TargetPath = %q, want %q", payload.Result.TargetPath, "services.backend.baseUrl")
	}
	if payload.Result.TargetFile != targetPath {
		t.Fatalf("result.TargetFile = %q, want %q", payload.Result.TargetFile, targetPath)
	}
	if !payload.Result.DryRun {
		t.Fatal("result.DryRun = false, want true")
	}
	if !payload.Result.Preview.Complete || len(payload.Result.Preview.WouldUpdate) != 1 {
		t.Fatalf("preview = %#v, want one complete would-update target", payload.Result.Preview)
	}
	previewTarget := payload.Result.Preview.WouldUpdate[0]
	if previewTarget.TargetName != "default" || previewTarget.TargetFile != targetPath || previewTarget.Selector != "services.backend.baseUrl" {
		t.Fatalf("preview wouldUpdate = %#v, want default target", previewTarget)
	}
	if strings.Contains(result.stdout, "http://localhost:8080") {
		t.Fatalf("stdout %q must not contain resolved replacement value", result.stdout)
	}
	if !bytes.Equal(readFileBytes(t, targetPath), originalContents) {
		t.Fatal("target file changed during dry run")
	}
}

func TestRunCommand_ApplyEditorFailureDoesNotLeakSecrets(t *testing.T) {
	t.Setenv("MYAPPLICATION_PRODUCTION_URL", "Server=prod;Database=App;Password=super-secret;")

	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Production
    valueFromEnv: MYAPPLICATION_PRODUCTION_URL
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{`)

	result := runCommandForTest(t, []string{"apply", "Production"}, projectRoot)
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	if !strings.Contains(result.stderr, `contains invalid JSON`) {
		t.Fatalf("stderr %q does not include editor failure", result.stderr)
	}
	if strings.Contains(result.stderr, "super-secret") {
		t.Fatalf("stderr %q must not include the secret value", result.stderr)
	}

	jsonResult := runCommandForTest(t, []string{"apply", "Production", "--json"}, projectRoot)
	if jsonResult.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", jsonResult.exitCode, runtimeExitCode)
	}
	if strings.Contains(jsonResult.stdout, "super-secret") {
		t.Fatalf("stdout %q must not include the secret value", jsonResult.stdout)
	}
}

func TestRunCommand_UsageErrorsReturnExitCodeTwoAndStructuredJSON(t *testing.T) {
	projectRoot := t.TempDir()

	result := runCommandForTest(t, []string{"apply", "--json"}, projectRoot)
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
		t.Fatalf("unmarshal usage JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "usage" {
		t.Fatalf("error.kind = %q, want %q", payload.Error.Kind, "usage")
	}
	if !strings.Contains(payload.Error.Message, "No profile specified.") {
		t.Fatalf("error.message = %q, want usage guidance", payload.Error.Message)
	}
}

func TestRunCommand_ListJSONReturnsStructuredConfigNotFoundError(t *testing.T) {
	result := runCommandForTest(t, []string{"list", "--json"}, t.TempDir())
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}

	var payload struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal config-not-found JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "config_not_found" {
		t.Fatalf("error.kind = %q, want %q", payload.Error.Kind, "config_not_found")
	}
	if !strings.Contains(payload.Error.Message, "Run `switchlet init`") {
		t.Fatalf("error.message = %q, want init guidance", payload.Error.Message)
	}
}

func TestRunCommand_ListWithoutConfigSuggestsInit(t *testing.T) {
	result := runCommandForTest(t, []string{"list"}, t.TempDir())
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, runtimeExitCode)
	}
	for _, expected := range []string{"No .switchlet.yaml found.", "Run `switchlet init` to create one"} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

type commandResult struct {
	stdout         string
	stderr         string
	exitCode       int
	programStarted bool
}

func runCommandForTest(t *testing.T, args []string, workingDirectory string) commandResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	programStarted := false

	err := runCommand(args, workingDirectory, func(model tea.Model) error {
		programStarted = true
		if model == nil {
			t.Fatal("runProgram received nil model")
		}

		return nil
	}, strings.NewReader(""), &stdout)

	result := commandResult{
		stdout:         stdout.String(),
		stderr:         stderr.String(),
		programStarted: programStarted,
	}
	if err == nil {
		return result
	}

	if writeErr := writeCommandError(err, &stdout, &stderr); writeErr != nil {
		t.Fatalf("writeCommandError returned error: %v", writeErr)
	}

	result.stdout = stdout.String()
	result.stderr = stderr.String()
	result.exitCode = exitCodeForError(err)

	return result
}

func writeMinimalCommandProject(t *testing.T) string {
	t.Helper()

	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", `{"services":{"backend":{"baseUrl":"https://old.example.test"}}}`)

	return projectRoot
}

func writeConfigSelectionProjects(t *testing.T) (workingDirectory string, explicitConfigPath string) {
	t.Helper()

	workspace := t.TempDir()
	discoveredRoot := filepath.Join(workspace, "discovered")
	explicitRoot := filepath.Join(workspace, "explicit")
	workingDirectory = filepath.Join(discoveredRoot, "sub")

	writeFile(t, discoveredRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config/runtime.json
    type: json
    jsonPath: database.url

profiles:
  - name: Default
    values:
      - target: database
        value: postgres://default
`)+"\n")
	writeFile(t, discoveredRoot, "config/runtime.json", `{"database":{"url":"postgres://default"}}`)
	writeFile(t, discoveredRoot, "sub/.keep", "")

	explicitConfigPath = writeFile(t, explicitRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config/runtime.json
    type: json
    jsonPath: database.url

profiles:
  - name: Explicit
    values:
      - target: database
        value: postgres://explicit
`)+"\n")
	writeFile(t, explicitRoot, "config/runtime.json", `{"database":{"url":"postgres://explicit"}}`)

	return workingDirectory, explicitConfigPath
}

func assertJSONSchemaVersion(t *testing.T, stdout string) {
	t.Helper()

	var payload struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal JSON output: %v\noutput: %q", err, stdout)
	}
	if payload.SchemaVersion != commandJSONSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", payload.SchemaVersion, commandJSONSchemaVersion)
	}
}

func openTerminalLikeFile(t *testing.T, flag int) *os.File {
	t.Helper()

	file, err := os.OpenFile(os.DevNull, flag, 0)
	if err != nil {
		t.Fatalf("open terminal-like file: %v", err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close terminal-like file: %v", err)
		}
	})

	return file
}

func forceTerminalFileDetection(t *testing.T, terminal bool) {
	t.Helper()

	original := isTerminalFile
	isTerminalFile = func(file *os.File) bool { return terminal }
	t.Cleanup(func() {
		isTerminalFile = original
	})
}

func containsANSIStyling(value string) bool {
	return strings.Contains(value, "\x1b[")
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}
