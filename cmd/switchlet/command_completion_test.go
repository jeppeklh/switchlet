package main

import (
	"strings"
	"testing"
)

func TestRunCommand_CompletionPrintsScriptsWithoutLoadingConfiguration(t *testing.T) {
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
				"inspect|apply|diff",
				"while IFS= read -r candidate",
				"COMPREPLY+=(\"$candidate\")",
				"compopt -o filenames",
				"switchlet __complete-profile-names",
				"complete -F _switchlet_completion switchlet",
				"--dry-run",
				"--patch",
				"--no-color",
				"bash zsh fish",
			},
		},
		{
			name:  "zsh",
			shell: "zsh",
			expected: []string{
				"#compdef switchlet",
				"_switchlet_profile_names",
				"inspect|apply|diff",
				"profiles=(\"${(@f)output}\")",
				"compadd -a profiles",
				"switchlet __complete-profile-names",
				"apply:Apply one configured profile",
				"--allow-protected[Allow non-interactive apply for a protected profile]",
				"--no-color[Disable styled command output]",
				"_values 'shell' bash zsh fish",
			},
		},
		{
			name:  "fish",
			shell: "fish",
			expected: []string{
				"# fish completion for switchlet",
				"__switchlet_complete_profiles",
				"case inspect apply diff",
				"-a '(__switchlet_complete_profiles)'",
				"switchlet __complete-profile-names",
				"__switchlet_profile_completion_needed",
				"complete -c switchlet -n '__fish_use_subcommand' -a \"completion\"",
				"complete -c switchlet -n '__fish_seen_subcommand_from init' -l overwrite",
				"complete -c switchlet -n '__fish_seen_subcommand_from diff' -l patch",
				"complete -c switchlet -l no-color -d 'Disable styled command output'",
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

func TestRunCommand_ProfileCompletionListsConfiguredProfilesOnly(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: missing-target-file.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local Dev
    values:
      - target: database
        value: postgres://local
  - name: "QA $Special"
    values:
      - target: database
        valueFromEnv: SWITCHLET_TEST_COMPLETION_ENV
`)+"\n")

	result := runCommandForTest(t, []string{profileCompletionCommandName}, projectRoot)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.programStarted {
		t.Fatal("profile completion command started the terminal program")
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}

	want := "Local Dev\nQA $Special\n"
	if result.stdout != want {
		t.Fatalf("stdout = %q, want %q", result.stdout, want)
	}
}

func TestRunCommand_ProfileCompletionIsQuietWhenConfigurationCannotLoad(t *testing.T) {
	t.Run("missing configuration", func(t *testing.T) {
		result := runCommandForTest(t, []string{profileCompletionCommandName}, t.TempDir())
		if result.exitCode != 0 {
			t.Fatalf("exitCode = %d, want 0", result.exitCode)
		}
		if result.stdout != "" || result.stderr != "" {
			t.Fatalf("stdout = %q, stderr = %q, want both empty", result.stdout, result.stderr)
		}
	})

	t.Run("invalid configuration", func(t *testing.T) {
		projectRoot := t.TempDir()
		writeFile(t, projectRoot, ".switchlet.yaml", "version: 3\nprofiles: [\n")

		result := runCommandForTest(t, []string{profileCompletionCommandName}, projectRoot)
		if result.exitCode != 0 {
			t.Fatalf("exitCode = %d, want 0", result.exitCode)
		}
		if result.stdout != "" || result.stderr != "" {
			t.Fatalf("stdout = %q, stderr = %q, want both empty", result.stdout, result.stderr)
		}
	})
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
