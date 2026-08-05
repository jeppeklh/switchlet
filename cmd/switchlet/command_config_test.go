package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/tui/configeditor"
)

func TestRunCommand_ConfigHelpWritesUsageWithoutLaunchingEditor(t *testing.T) {
	result := runCommandForTest(t, []string{"config", "--help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for config help")
	}
	if !strings.Contains(result.stdout, "switchlet config") {
		t.Fatalf("stdout %q does not contain config usage", result.stdout)
	}
	for _, forbidden := range []string{"Phase", "scaffolding", "later phases"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Fatalf("stdout %q must not contain internal config help wording %q", result.stdout, forbidden)
		}
	}
	for _, expected := range []string{"in-memory draft", "review and save", "interactive terminals"} {
		if !strings.Contains(result.stdout, expected) {
			t.Fatalf("stdout %q does not contain user-facing config help wording %q", result.stdout, expected)
		}
	}
}

func TestRunCommand_HelpConfigWritesUsage(t *testing.T) {
	result := runCommandForTest(t, []string{"help", "config"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "switchlet config") || !strings.Contains(result.stdout, "interactive configuration editor") {
		t.Fatalf("stdout %q does not contain config help", result.stdout)
	}
}

func TestRunCommand_ConfigRejectsPositionalArguments(t *testing.T) {
	result := runCommandForTest(t, []string{"config", "Local"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, usageExitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stderr, "config does not accept a positional argument") {
		t.Fatalf("stderr %q does not contain positional guidance", result.stderr)
	}
}

func TestRunCommand_ConfigRequiresInteractiveTerminal(t *testing.T) {
	result := runCommandForTest(t, []string{"config"}, t.TempDir())
	if result.exitCode != runtimeExitCode {
		t.Fatalf("exitCode = %d, want %d (stdout: %q, stderr: %q)", result.exitCode, runtimeExitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("runProgram was called for non-terminal config command")
	}
	if !strings.Contains(result.stderr, "interactive-only") || !strings.Contains(result.stderr, "stdin and stdout") {
		t.Fatalf("stderr %q does not explain interactive-only config editing", result.stderr)
	}
}

func TestConfigEditorModelForWorkingDirectoryUsesExplicitConfigPath(t *testing.T) {
	workingDirectory, explicitConfigPath := writeConfigSelectionProjects(t)
	relativeConfigPath, err := filepath.Rel(workingDirectory, explicitConfigPath)
	if err != nil {
		t.Fatalf("make relative config path: %v", err)
	}

	model, err := configEditorModelForWorkingDirectory(workingDirectory, configeditor.Options{}, projectLoadOptions{ConfigPath: relativeConfigPath})
	if err != nil {
		t.Fatalf("configEditorModelForWorkingDirectory returned error: %v", err)
	}
	view := model.View()
	if !strings.Contains(view, "Explicit") {
		t.Fatalf("View() = %q, want explicit config profile", view)
	}
	if strings.Contains(view, "Default") {
		t.Fatalf("View() = %q, must not show discovered project profile", view)
	}
}
