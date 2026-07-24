package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunCommand_NoArgumentsStartsProgram(t *testing.T) {
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

	result := runCommandForTest(t, nil, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr: %q)", result.exitCode, result.stderr)
	}
	if !result.programStarted {
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
	if !strings.Contains(result.stdout, "Password=****") {
		t.Fatalf("stdout %q does not include masked value", result.stdout)
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
	if payload.Profile.MaskedValue != "Server=test;Database=App;Pwd=****;" {
		t.Fatalf("profile.MaskedValue = %q, want masked value", payload.Profile.MaskedValue)
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
			Kind string `json:"kind"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal inspect JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "profile_not_found" {
		t.Fatalf("error.kind = %q, want %q", payload.Error.Kind, "profile_not_found")
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
	if !strings.Contains(result.stdout, "Applied profile: Local") {
		t.Fatalf("stdout %q does not include apply success", result.stdout)
	}
	if !strings.Contains(result.stdout, targetPath) {
		t.Fatalf("stdout %q does not include target file path", result.stdout)
	}
	if !strings.Contains(result.stdout, "services.backend.baseUrl") {
		t.Fatalf("stdout %q does not include target JSON path", result.stdout)
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
	if !strings.Contains(allowed.stdout, "Applied profile: Production") {
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
	if !strings.Contains(allowed.stdout, "Dry run successful: Production") {
		t.Fatalf("stdout %q does not include dry-run success", allowed.stdout)
	}
	if !strings.Contains(allowed.stdout, targetPath) {
		t.Fatalf("stdout %q does not include target file path", allowed.stdout)
	}
	if !strings.Contains(allowed.stdout, "services.backend.baseUrl") {
		t.Fatalf("stdout %q does not include target JSON path", allowed.stdout)
	}
	if !strings.Contains(allowed.stdout, "No changes were written.") {
		t.Fatalf("stdout %q does not state that no changes were written", allowed.stdout)
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
	if !strings.Contains(payload.Error.Message, "apply requires exactly one profile name") {
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
			Kind string `json:"kind"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("unmarshal config-not-found JSON error: %v\noutput: %q", err, result.stdout)
	}
	if payload.Error.Kind != "config_not_found" {
		t.Fatalf("error.kind = %q, want %q", payload.Error.Kind, "config_not_found")
	}
}

func TestREADME_ContainsInstallationAndCommandExamples(t *testing.T) {
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned ok=false")
	}

	readmePath := filepath.Clean(filepath.Join(filepath.Dir(currentFilePath), "..", "..", "README.md"))
	contents, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README %q: %v", readmePath, err)
	}

	readme := string(contents)
	for _, expected := range []string{
		"go install github.com/jeppeklh/switchlet/cmd/switchlet@latest",
		"switchlet_linux_amd64",
		"switchlet_windows_amd64.exe",
		"switchlet init",
		"switchlet list",
		"switchlet inspect Local",
		"switchlet apply Local",
		"switchlet apply Production --dry-run --allow-protected",
		"Use `--json` on `list`, `inspect`, and `apply`",
		"No changes were written.",
	} {
		if !strings.Contains(readme, expected) {
			t.Fatalf("README %q does not contain %q", readmePath, expected)
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

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}
