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
	if !strings.Contains(applyResult.stdout, "Applied profile: Local") {
		t.Fatalf("apply stdout %q does not contain success message", applyResult.stdout)
	}
	if !strings.Contains(applyResult.stdout, targetPath) {
		t.Fatalf("apply stdout %q does not contain target file path", applyResult.stdout)
	}
	if !strings.Contains(string(readFileBytes(t, targetPath)), "https://local.example.test") {
		t.Fatalf("target file %q was not updated by installed binary", string(readFileBytes(t, targetPath)))
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
	if loadedConfig.Version != 2 {
		t.Fatalf("version = %d, want 2", loadedConfig.Version)
	}
	if loadedConfig.Target.JSONPath != "database.primary.url" {
		t.Fatalf("json path = %q, want %q", loadedConfig.Target.JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(loadedConfig.Profiles))
	}
	if loadedConfig.Profiles[0].Value == nil || *loadedConfig.Profiles[0].Value != "postgres://localhost:5432/myapp" {
		t.Fatalf("literal profile value = %#v, want configured literal value", loadedConfig.Profiles[0].Value)
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
