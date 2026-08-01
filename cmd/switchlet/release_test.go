package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestInstalledBinary_SmokeWorkflow(t *testing.T) {
	repoRoot, binaryPath := installBinaryForReleaseTest(t)

	helpResult := runExternalCommand(t, repoRoot, nil, binaryPath, "help")
	if helpResult.exitCode != 0 {
		t.Fatalf("switchlet help exitCode = %d, want 0\nstdout: %q\nstderr: %q", helpResult.exitCode, helpResult.stdout, helpResult.stderr)
	}
	if !strings.Contains(helpResult.stdout, "switchlet list [--json]") {
		t.Fatalf("help stdout %q does not mention list usage", helpResult.stdout)
	}
	if !strings.Contains(helpResult.stdout, "full-screen terminal UI") {
		t.Fatalf("help stdout %q does not mention full-screen terminal UI", helpResult.stdout)
	}
	if !strings.Contains(helpResult.stdout, "Enter/y to confirm") {
		t.Fatalf("help stdout %q does not mention the protected confirmation keys", helpResult.stdout)
	}
	if !strings.Contains(helpResult.stdout, "switchlet version") {
		t.Fatalf("help stdout %q does not mention version command", helpResult.stdout)
	}

	versionResult := runExternalCommand(t, repoRoot, nil, binaryPath, "version")
	flagVersionResult := runExternalCommand(t, repoRoot, nil, binaryPath, "--version")
	if versionResult.exitCode != 0 {
		t.Fatalf("switchlet version exitCode = %d, want 0\nstdout: %q\nstderr: %q", versionResult.exitCode, versionResult.stdout, versionResult.stderr)
	}
	if flagVersionResult.exitCode != 0 {
		t.Fatalf("switchlet --version exitCode = %d, want 0\nstdout: %q\nstderr: %q", flagVersionResult.exitCode, flagVersionResult.stdout, flagVersionResult.stderr)
	}
	if versionResult.stdout == "" || !strings.HasPrefix(versionResult.stdout, "switchlet ") {
		t.Fatalf("version stdout %q does not contain a concise switchlet version line", versionResult.stdout)
	}
	if flagVersionResult.stdout != versionResult.stdout {
		t.Fatalf("--version stdout = %q, want %q", flagVersionResult.stdout, versionResult.stdout)
	}

	projectRoot := t.TempDir()
	targetPath := writeFile(t, projectRoot, "config/runtime.json", strings.TrimSpace(`
{
  "services": {
    "backend": {
      "baseUrl": "https://old.example.test"
    }
  }
}
`)+"\n")
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: https://local.example.test
  - name: Production
    valueFromEnv: MYAPPLICATION_PRODUCTION_URL
    protected: true
`)+"\n")

	commandEnv := append(os.Environ(), "MYAPPLICATION_PRODUCTION_URL=Server=prod;Database=App;Password=super-secret;")

	listResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "list")
	if listResult.exitCode != 0 {
		t.Fatalf("switchlet list exitCode = %d, want 0\nstdout: %q\nstderr: %q", listResult.exitCode, listResult.stdout, listResult.stderr)
	}
	if !strings.Contains(listResult.stdout, "Local") {
		t.Fatalf("list stdout %q does not contain Local profile", listResult.stdout)
	}
	if !strings.Contains(listResult.stdout, "Production [protected]") {
		t.Fatalf("list stdout %q does not contain protected profile", listResult.stdout)
	}

	inspectResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "inspect", "Production")
	if inspectResult.exitCode != 0 {
		t.Fatalf("switchlet inspect exitCode = %d, want 0\nstdout: %q\nstderr: %q", inspectResult.exitCode, inspectResult.stdout, inspectResult.stderr)
	}
	if !strings.Contains(inspectResult.stdout, "Password=****") {
		t.Fatalf("inspect stdout %q does not contain masked secret", inspectResult.stdout)
	}
	if strings.Contains(inspectResult.stdout, "super-secret") {
		t.Fatalf("inspect stdout %q must not contain unmasked secret", inspectResult.stdout)
	}

	originalContents := readFileBytes(t, targetPath)
	dryRunResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "apply", "Local", "--dry-run")
	if dryRunResult.exitCode != 0 {
		t.Fatalf("switchlet apply --dry-run exitCode = %d, want 0\nstdout: %q\nstderr: %q", dryRunResult.exitCode, dryRunResult.stdout, dryRunResult.stderr)
	}
	if !strings.Contains(dryRunResult.stdout, "No changes were written.") {
		t.Fatalf("dry-run stdout %q does not contain no-write message", dryRunResult.stdout)
	}
	if !bytes.Equal(readFileBytes(t, targetPath), originalContents) {
		t.Fatal("target file changed during installed-binary dry run")
	}

	applyResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "apply", "Local")
	if applyResult.exitCode != 0 {
		t.Fatalf("switchlet apply exitCode = %d, want 0\nstdout: %q\nstderr: %q", applyResult.exitCode, applyResult.stdout, applyResult.stderr)
	}
	if !strings.Contains(applyResult.stdout, `Applied profile "Local"`) {
		t.Fatalf("apply stdout %q does not contain success message", applyResult.stdout)
	}
	if !strings.Contains(applyResult.stdout, "updated config/runtime.json") {
		t.Fatalf("apply stdout %q does not contain updated target marker", applyResult.stdout)
	}
	if strings.Contains(applyResult.stdout, targetPath) {
		t.Fatalf("apply stdout %q must use project-relative target path instead of %q", applyResult.stdout, targetPath)
	}
	if strings.Contains(applyResult.stdout, "https://local.example.test") {
		t.Fatalf("apply stdout %q must not contain resolved replacement value", applyResult.stdout)
	}
	if !strings.Contains(string(readFileBytes(t, targetPath)), "https://local.example.test") {
		t.Fatalf("target file %q was not updated by installed binary", string(readFileBytes(t, targetPath)))
	}
}

func TestInstalledBinary_VersionThreeMultiTargetWorkflow(t *testing.T) {
	_, binaryPath := installBinaryForReleaseTest(t)

	projectRoot := t.TempDir()
	databasePath := writeFile(t, projectRoot, "backend/appsettings.Development.json", strings.TrimSpace(`
{
  "database": {
    "url": "postgres://old"
  }
}
`)+"\n")
	frontendPath := writeFile(t, projectRoot, "frontend/.env.local", strings.TrimSpace(`
VITE_API_URL=http://localhost:5173
VITE_FEATURES=local
`)+"\n")
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
  - name: Database Only
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

	commandEnv := append(os.Environ(), "STAGING_DATABASE_URL=Server=staging;Database=App;Password=super-secret;")

	listResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "list")
	if listResult.exitCode != 0 {
		t.Fatalf("switchlet list exitCode = %d, want 0\nstdout: %q\nstderr: %q", listResult.exitCode, listResult.stdout, listResult.stderr)
	}
	for _, expected := range []string{"Database Only [1 target, partial]", "Staging [2 targets, protected]"} {
		if !strings.Contains(listResult.stdout, expected) {
			t.Fatalf("list stdout %q does not contain %q", listResult.stdout, expected)
		}
	}

	inspectResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "inspect", "Staging")
	if inspectResult.exitCode != 0 {
		t.Fatalf("switchlet inspect exitCode = %d, want 0\nstdout: %q\nstderr: %q", inspectResult.exitCode, inspectResult.stdout, inspectResult.stderr)
	}
	for _, expected := range []string{"- database [json]", "jsonPath: database.url", "Password=****", "- frontendApi [dotenv]", "key: VITE_API_URL"} {
		if !strings.Contains(inspectResult.stdout, expected) {
			t.Fatalf("inspect stdout %q does not contain %q", inspectResult.stdout, expected)
		}
	}
	if strings.Contains(inspectResult.stdout, "super-secret") {
		t.Fatalf("inspect stdout %q must not contain unmasked secret", inspectResult.stdout)
	}

	originalDatabaseContents := readFileBytes(t, databasePath)
	originalFrontendContents := readFileBytes(t, frontendPath)
	dryRunResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "apply", "Database Only", "--dry-run", "--json")
	if dryRunResult.exitCode != 0 {
		t.Fatalf("switchlet apply --dry-run --json exitCode = %d, want 0\nstdout: %q\nstderr: %q", dryRunResult.exitCode, dryRunResult.stdout, dryRunResult.stderr)
	}
	if !strings.Contains(dryRunResult.stdout, `"status":"dry_run"`) || !strings.Contains(dryRunResult.stdout, `"targetName":"database"`) {
		t.Fatalf("dry-run JSON stdout %q does not contain target-aware dry-run result", dryRunResult.stdout)
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) {
		t.Fatal("database target changed during version 3 dry run")
	}
	if !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("frontend target changed during version 3 dry run")
	}

	protectedResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "apply", "Staging")
	if protectedResult.exitCode != runtimeExitCode {
		t.Fatalf("protected apply exitCode = %d, want %d\nstdout: %q\nstderr: %q", protectedResult.exitCode, runtimeExitCode, protectedResult.stdout, protectedResult.stderr)
	}
	if !strings.Contains(protectedResult.stderr, "protected profile requires explicit opt-in") {
		t.Fatalf("protected apply stderr %q does not contain opt-in guidance", protectedResult.stderr)
	}
	if !bytes.Equal(readFileBytes(t, databasePath), originalDatabaseContents) || !bytes.Equal(readFileBytes(t, frontendPath), originalFrontendContents) {
		t.Fatal("targets changed after protected profile refusal")
	}

	applyResult := runExternalCommand(t, projectRoot, commandEnv, binaryPath, "apply", "Staging", "--allow-protected")
	if applyResult.exitCode != 0 {
		t.Fatalf("switchlet apply --allow-protected exitCode = %d, want 0\nstdout: %q\nstderr: %q", applyResult.exitCode, applyResult.stdout, applyResult.stderr)
	}
	for _, expected := range []string{`Applied profile "Staging"`, "Updated targets:", "updated backend/appsettings.Development.json", "  database [json]", "updated frontend/.env.local", "  frontendApi [dotenv]"} {
		if !strings.Contains(applyResult.stdout, expected) {
			t.Fatalf("apply stdout %q does not contain %q", applyResult.stdout, expected)
		}
	}
	for _, absolutePath := range []string{databasePath, frontendPath} {
		if strings.Contains(applyResult.stdout, absolutePath) {
			t.Fatalf("apply stdout %q must use project-relative paths instead of %q", applyResult.stdout, absolutePath)
		}
	}
	for _, forbidden := range []string{"super-secret", "https://api.staging.example.test"} {
		if strings.Contains(applyResult.stdout, forbidden) {
			t.Fatalf("apply stdout %q must not contain resolved replacement value %q", applyResult.stdout, forbidden)
		}
	}
	if !strings.Contains(string(readFileBytes(t, databasePath)), "Server=staging;Database=App;Password=super-secret;") {
		t.Fatalf("database file %q was not updated by version 3 apply", string(readFileBytes(t, databasePath)))
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_API_URL=https://api.staging.example.test") {
		t.Fatalf("frontend file %q was not updated by version 3 apply", string(readFileBytes(t, frontendPath)))
	}
	if !strings.Contains(string(readFileBytes(t, frontendPath)), "VITE_FEATURES=local") {
		t.Fatalf("frontend file %q did not preserve unrelated dotenv entry", string(readFileBytes(t, frontendPath)))
	}
}

func TestInstalledBinary_InitFallbackCreatesConfigurationAndGitignore(t *testing.T) {
	_, binaryPath := installBinaryForReleaseTest(t)

	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	input := strings.Join([]string{
		"1",
		"1",
		"1",
		"1",
		"database",
		"n",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"",
		"",
	}, "\n") + "\n"

	initResult := runExternalCommandWithInput(t, projectRoot, nil, input, binaryPath, "init")
	if initResult.exitCode != 0 {
		t.Fatalf("switchlet init exitCode = %d, want 0\nstdout: %q\nstderr: %q", initResult.exitCode, initResult.stdout, initResult.stderr)
	}
	if !strings.Contains(initResult.stdout, "Created configuration:") {
		t.Fatalf("init stdout %q does not report configuration creation", initResult.stdout)
	}
	if !strings.Contains(initResult.stdout, "Updated project .gitignore to ignore .switchlet.yaml.") {
		t.Fatalf("init stdout %q does not report gitignore protection", initResult.stdout)
	}
	if strings.Contains(initResult.stdout, "postgres://old") {
		t.Fatalf("init stdout %q must not expose the existing target value", initResult.stdout)
	}

	loadedConfig, err := config.Load(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("load created configuration: %v", err)
	}
	if loadedConfig.Version != 3 {
		t.Fatalf("version = %d, want 3", loadedConfig.Version)
	}
	if len(loadedConfig.Targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(loadedConfig.Targets))
	}
	if loadedConfig.Targets[0].Name != "database" {
		t.Fatalf("target name = %q, want database", loadedConfig.Targets[0].Name)
	}
	if loadedConfig.Targets[0].JSONPath != "database.primary.url" {
		t.Fatalf("json path = %q, want %q", loadedConfig.Targets[0].JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(loadedConfig.Profiles))
	}
	if len(loadedConfig.Profiles[0].Values) != 1 {
		t.Fatalf("len(profile values) = %d, want 1", len(loadedConfig.Profiles[0].Values))
	}
	profileValue := loadedConfig.Profiles[0].Values[0]
	if profileValue.Target != "database" || profileValue.Value == nil || *profileValue.Value != "postgres://localhost:5432/myapp" {
		t.Fatalf("profile value = %#v, want configured database literal value", profileValue)
	}

	gitignoreContents, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if string(gitignoreContents) != ".switchlet.yaml\n" {
		t.Fatalf(".gitignore contents = %q, want %q", string(gitignoreContents), ".switchlet.yaml\n")
	}
}

func installBinaryForReleaseTest(t *testing.T) (string, string) {
	t.Helper()

	repoRoot := repositoryRoot(t)
	installRoot := t.TempDir()
	gobinPath := filepath.Join(installRoot, "bin")
	if err := os.MkdirAll(gobinPath, 0o755); err != nil {
		t.Fatalf("create GOBIN %q: %v", gobinPath, err)
	}

	installResult := runExternalCommand(t, repoRoot, append(os.Environ(), "GOBIN="+gobinPath), "go", "install", "./cmd/switchlet")
	if installResult.exitCode != 0 {
		t.Fatalf("go install exitCode = %d, want 0\nstdout: %q\nstderr: %q", installResult.exitCode, installResult.stdout, installResult.stderr)
	}

	binaryName := "switchlet"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(gobinPath, binaryName)
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("stat installed binary %q: %v", binaryPath, err)
	}

	return repoRoot, binaryPath
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned ok=false")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFilePath), "..", ".."))
}

type externalCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runExternalCommand(t *testing.T, workingDirectory string, environment []string, name string, arguments ...string) externalCommandResult {
	t.Helper()
	return runExternalCommandWithInput(t, workingDirectory, environment, "", name, arguments...)
}

func runExternalCommandWithInput(t *testing.T, workingDirectory string, environment []string, input string, name string, arguments ...string) externalCommandResult {
	t.Helper()

	command := exec.Command(name, arguments...)
	command.Dir = workingDirectory
	if environment != nil {
		command.Env = environment
	}
	if input != "" {
		command.Stdin = strings.NewReader(input)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	result := externalCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		return result
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.exitCode = exitError.ExitCode()
		return result
	}

	t.Fatalf("run command %q %q: %v\nstdout: %q\nstderr: %q", name, strings.Join(arguments, " "), err, result.stdout, result.stderr)
	return externalCommandResult{}
}
