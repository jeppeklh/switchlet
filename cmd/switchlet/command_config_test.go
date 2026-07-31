package main

import (
	"strings"
	"testing"
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
