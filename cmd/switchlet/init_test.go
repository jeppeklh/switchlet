package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

func TestRunCommand_HelpWritesUsage(t *testing.T) {
	var output bytes.Buffer

	err := runCommand([]string{"help"}, t.TempDir(), func(model tea.Model) error {
		t.Fatal("runProgram should not be called for help")
		return nil
	}, strings.NewReader(""), &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), "switchlet init") {
		t.Fatalf("help output %q does not mention init", output.String())
	}
	if !strings.Contains(output.String(), "switchlet list") {
		t.Fatalf("help output %q does not mention list", output.String())
	}
	if !strings.Contains(output.String(), "switchlet inspect <profile-name>") {
		t.Fatalf("help output %q does not mention inspect", output.String())
	}
	if !strings.Contains(output.String(), "switchlet apply <profile-name>") {
		t.Fatalf("help output %q does not mention apply", output.String())
	}
}

func TestRunCommand_InitCreatesConfiguration(t *testing.T) {
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

	input := strings.NewReader(strings.Join([]string{
		"config.json",
		"database.primary.url",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"y",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	loadedConfig, err := config.Load(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loadedConfig.Version != 2 {
		t.Fatalf("Version = %d, want 2", loadedConfig.Version)
	}
	if loadedConfig.Target.JSONPath != "database.primary.url" {
		t.Fatalf("JSON path = %q, want %q", loadedConfig.Target.JSONPath, "database.primary.url")
	}
	if len(loadedConfig.Profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(loadedConfig.Profiles))
	}
	if loadedConfig.Profiles[0].Name != "Local" {
		t.Fatalf("profile name = %q, want %q", loadedConfig.Profiles[0].Name, "Local")
	}
	if loadedConfig.Profiles[0].Value == nil || *loadedConfig.Profiles[0].Value != "postgres://localhost:5432/myapp" {
		t.Fatalf("literal profile value = %#v, want configured literal value", loadedConfig.Profiles[0].Value)
	}

	contents, err := os.ReadFile(filepath.Join(projectRoot, ".switchlet.yaml"))
	if err != nil {
		t.Fatalf("read configuration file: %v", err)
	}
	if !strings.Contains(string(contents), "jsonPath: database.primary.url") {
		t.Fatalf("configuration file contents %q do not contain version 2 JSON path", string(contents))
	}
	if strings.Contains(string(contents), "connectionName:") {
		t.Fatalf("configuration file contents %q must not contain legacy connectionName", string(contents))
	}
	if !strings.Contains(output.String(), "Created configuration:") {
		t.Fatalf("init output %q does not report created configuration", output.String())
	}
}

func TestRunCommand_InitReturnsErrorWhenConfigurationExistsInParentDirectory(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "project")
	nestedDirectory := filepath.Join(projectRoot, "src", "MyApplication")

	if err := os.MkdirAll(nestedDirectory, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=App;"
`)+"\n")

	var output bytes.Buffer
	err := runCommand([]string{"init"}, nestedDirectory, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, strings.NewReader(""), &output)
	if err == nil {
		t.Fatal("runCommand returned nil error, want existing configuration error")
	}
	if !strings.Contains(err.Error(), "discovered existing configuration file") {
		t.Fatalf("runCommand returned error %q, want existing configuration error", err)
	}
}

func TestRunCommand_InitCancellationDoesNotWriteConfiguration(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "service": {
    "baseUrl": "https://old.example.test"
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"config.json",
		"service.baseUrl",
		"Local",
		"1",
		"https://new.example.test",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error", statErr)
	}
	if !strings.Contains(output.String(), "Initialization cancelled.") {
		t.Fatalf("init output %q does not report cancellation", output.String())
	}
}

func TestRunInit_RemovesCreatedConfigurationWhenFinalValidationFails(t *testing.T) {
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

	input := strings.NewReader(strings.Join([]string{
		"config.json",
		"database.primary.url",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"y",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runInit(projectRoot, input, &output, initDependencies{
		validateCreateLocation: config.ValidateCreateLocation,
		validateStringTarget:   editor.ValidateStringTarget,
		createConfig:           config.Create,
		validateCreatedConfig: func(loadedConfig config.Config) error {
			return errors.New("target validation failed")
		},
		removeFile: os.Remove,
	})
	if err == nil {
		t.Fatal("runInit returned nil error, want final validation error")
	}
	if !strings.Contains(err.Error(), "target validation failed") {
		t.Fatalf("runInit returned error %q, want final validation error", err)
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error", statErr)
	}
}

func TestRunInit_RePromptsAfterInvalidTargetPath(t *testing.T) {
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

	input := strings.NewReader(strings.Join([]string{
		"missing.json",
		"database.primary.url",
		"config.json",
		"database.primary.url",
		"Local",
		"2",
		"MYAPP_DATABASE_URL",
		"y",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), `Error: stat target file`) {
		t.Fatalf("init output %q does not report invalid target path", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}

func TestRunInit_RePromptsAfterMissingTargetJSONPath(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "replica": {
      "url": "postgres://replica"
    }
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"config.json",
		"database.primary.url",
		"config.json",
		"database.replica.url",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), `does not contain JSON path "database.primary.url"`) {
		t.Fatalf("init output %q does not report missing JSON path", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}

func TestRunInit_RePromptsAfterNonStringTargetValue(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "port": 5432,
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	input := strings.NewReader(strings.Join([]string{
		"config.json",
		"database.primary.port",
		"config.json",
		"database.primary.url",
		"Local",
		"1",
		"postgres://localhost:5432/myapp",
		"n",
		"n",
		"n",
	}, "\n") + "\n")
	var output bytes.Buffer

	err := runCommand([]string{"init"}, projectRoot, func(model tea.Model) error {
		t.Fatal("runProgram should not be called for init")
		return nil
	}, input, &output)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if !strings.Contains(output.String(), `JSON path "database.primary.port" must resolve to a string`) {
		t.Fatalf("init output %q does not report non-string target value", output.String())
	}

	_, statErr := os.Stat(filepath.Join(projectRoot, ".switchlet.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configuration file stat error = %v, want not-exist error after cancellation", statErr)
	}
}
