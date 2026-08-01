package main

import (
	"strings"
	"testing"
)

func TestRunCommand_CompletionPrintsStaticScriptsWithoutLoadingConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		shell    string
		expected []string
	}{
		{
			name:  "bash",
			shell: "bash",
			expected: []string{
				"# bash completion for switchlet",
				"_switchlet_completion",
				"complete -F _switchlet_completion switchlet",
				"--dry-run",
				"--patch",
				"bash zsh fish",
			},
		},
		{
			name:  "zsh",
			shell: "zsh",
			expected: []string{
				"#compdef switchlet",
				"apply:Apply one configured profile",
				"--allow-protected[Allow non-interactive apply for a protected profile]",
				"_values 'shell' bash zsh fish",
			},
		},
		{
			name:  "fish",
			shell: "fish",
			expected: []string{
				"# fish completion for switchlet",
				"complete -c switchlet -n '__fish_use_subcommand' -a \"completion\"",
				"complete -c switchlet -n '__fish_seen_subcommand_from init' -l overwrite",
				"complete -c switchlet -n '__fish_seen_subcommand_from diff' -l patch",
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCommandForTest(t, []string{"completion", testCase.shell}, t.TempDir())
			if result.exitCode != 0 {
				t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
			}
			if result.programStarted {
				t.Fatal("completion command started the terminal program")
			}

			for _, expected := range testCase.expected {
				if !strings.Contains(result.stdout, expected) {
					t.Fatalf("stdout %q does not contain %q", result.stdout, expected)
				}
			}
		})
	}
}

func TestRunCommand_CompletionRejectsUnsupportedShell(t *testing.T) {
	result := runCommandForTest(t, []string{"completion", "powershell"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{`unsupported completion shell "powershell"`, "Supported shells: bash, zsh, fish", "switchlet completion <shell>"} {
		if !strings.Contains(result.stderr, expected) {
			t.Fatalf("stderr %q does not contain %q", result.stderr, expected)
		}
	}
}

func TestRunCommand_HelpListsCompletionCommand(t *testing.T) {
	result := runCommandForTest(t, []string{"help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "switchlet completion <shell>") {
		t.Fatalf("stdout %q does not list completion command", result.stdout)
	}
}

func TestRunCommand_CompletionHelpDoesNotLoadConfiguration(t *testing.T) {
	result := runCommandForTest(t, []string{"completion", "--help"}, t.TempDir())
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("completion help started the terminal program")
	}
	if !strings.Contains(result.stdout, "Supported shells: bash, zsh, fish") {
		t.Fatalf("stdout %q does not include supported shells", result.stdout)
	}
}
