package main

import (
	"path/filepath"
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
				"__switchlet_completion_context",
				"inspect|diff)",
				`"${COMP_WORDS[COMP_CWORD-1]}" == "--expect"`,
				`"${COMP_WORDS[COMP_CWORD-1]}" == "--expect-current"`,
				"while IFS= read -r candidate",
				"COMPREPLY+=(\"$candidate\")",
				"compopt -o filenames",
				`switchlet "${completion_load_args[@]}" __complete-profile-names`,
				"complete -F _switchlet_completion switchlet",
				"--dry-run",
				"--expect-current",
				"--config",
				"--short",
				"--name",
				"--expect",
				"--patch",
				"--exit-code",
				"--strict",
				"--no-color",
				"doctor",
				"targets",
				"bash zsh fish powershell",
			},
		},
		{
			name:  "zsh",
			shell: "zsh",
			expected: []string{
				"#compdef switchlet",
				"switchlet_completion_load_args",
				"_switchlet_profile_names",
				"inspect|apply|diff",
				`"$switchlet_completion_command_name" == status`,
				`"${words[CURRENT-1]}" == "--expect"`,
				`"${words[CURRENT-1]}" == "--expect-current"`,
				"profiles=(\"${(@f)output}\")",
				"compadd -a profiles",
				`completion_command+=("__complete-profile-names")`,
				"apply:Apply one configured profile",
				"doctor:Check project health",
				"targets:List configured targets",
				"--allow-protected[Allow non-interactive apply for a protected profile]",
				"--expect-current[Require the current complete profile before apply or dry run]",
				"--short[Write a concise current-profile summary]",
				"--name[Write only the current complete profile name]",
				"--expect[Assert the current complete profile]",
				"--strict[Return non-zero when warnings are present]",
				"--config[Load a specific .switchlet.yaml]",
				"--exit-code[Return non-zero when the profile would change files]",
				"--no-color[Disable styled command output]",
				"_values 'shell' bash zsh fish powershell",
			},
		},
		{
			name:  "fish",
			shell: "fish",
			expected: []string{
				"# fish completion for switchlet",
				"__switchlet_completion_command_context",
				"__switchlet_completion_load_args",
				"__switchlet_complete_profiles",
				"case inspect apply diff",
				`if test "$__switchlet_command_name" = status`,
				`test "$previous" = "--expect"`,
				`test "$previous" = "--expect-current"`,
				"-a '(__switchlet_complete_profiles)'",
				"switchlet $__switchlet_completion_load_args __complete-profile-names",
				"__switchlet_profile_completion_needed",
				"complete -c switchlet -n '__fish_use_subcommand' -a \"completion\"",
				"complete -c switchlet -n '__fish_seen_subcommand_from init' -l overwrite",
				"complete -c switchlet -n '__fish_seen_subcommand_from list' -l config",
				"complete -c switchlet -n '__fish_seen_subcommand_from apply' -l expect-current",
				"complete -c switchlet -n '__fish_seen_subcommand_from status' -l short",
				"complete -c switchlet -n '__fish_seen_subcommand_from status' -l name",
				"complete -c switchlet -n '__fish_seen_subcommand_from status' -l expect",
				"complete -c switchlet -n '__fish_seen_subcommand_from diff' -l patch",
				"complete -c switchlet -n '__fish_seen_subcommand_from diff' -l exit-code",
				"complete -c switchlet -n '__fish_seen_subcommand_from doctor' -l strict",
				"complete -c switchlet -n '__fish_seen_subcommand_from doctor' -l json",
				"complete -c switchlet -n '__fish_seen_subcommand_from targets' -l json",
				"complete -c switchlet -l no-color -d 'Disable styled command output'",
			},
		},
		{
			name:  "powershell",
			shell: "powershell",
			expected: []string{
				"# PowerShell completion for switchlet",
				"Register-ArgumentCompleter -Native -CommandName switchlet",
				"$__switchletSupportedShells = @('bash', 'zsh', 'fish', 'powershell')",
				"function __switchlet_ParseCompletionContext",
				"function __switchlet_ExpectedProfileCompletionNeeded",
				"function __switchlet_SelectedProfileCompletionState",
				"switchlet @($Context.LoadArgs) '__complete-profile-names' 2>$null",
				"$Context.Command -eq 'status' -and $previousToken -eq '--expect'",
				"$Context.Command -eq 'apply' -and $previousToken -eq '--expect-current'",
				"$__switchletCommandFlags.ContainsKey($Context.Command)",
				"Apply one configured profile",
				"--allow-protected",
				"--expect-current",
				"--strict",
				"powershell",
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

func TestRunCommand_ProfileCompletionUsesExplicitConfigWhenProvided(t *testing.T) {
	workingDirectory, explicitConfigPath := writeConfigSelectionProjects(t)
	relativeConfigPath, err := filepath.Rel(workingDirectory, explicitConfigPath)
	if err != nil {
		t.Fatalf("make relative config path: %v", err)
	}

	result := runCommandForTest(t, []string{"--config", relativeConfigPath, profileCompletionCommandName}, workingDirectory)
	if result.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout: %q, stderr: %q)", result.exitCode, result.stdout, result.stderr)
	}
	if result.stdout != "Explicit\n" {
		t.Fatalf("stdout = %q, want explicit-config profile list", result.stdout)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
}

func TestRunCommand_CompletionRejectsUnsupportedShell(t *testing.T) {
	result := runCommandForTest(t, []string{"completion", "tcsh"}, t.TempDir())
	if result.exitCode != usageExitCode {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, usageExitCode)
	}
	for _, expected := range []string{`unsupported completion shell "tcsh"`, "Supported shells: bash, zsh, fish, powershell", "switchlet completion <shell>"} {
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
	if !strings.Contains(result.stdout, "Supported shells: bash, zsh, fish, powershell") {
		t.Fatalf("stdout %q does not include supported shells", result.stdout)
	}
}
