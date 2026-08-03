package main

func usageText() string {
	return `Usage:
	  switchlet                                      Launch the profile switcher
	  switchlet init [--overwrite]                   Guided setup for a .switchlet.yaml in the current directory
	  switchlet config                               Edit .switchlet.yaml in a full-screen configuration editor
	  switchlet list [--json]                        List configured profiles and target counts without launching the TUI
	  switchlet inspect <profile-name> [--json]      Inspect one configured profile and its planned target changes
	  switchlet apply <profile-name> [flags]         Apply one configured profile by name
	  switchlet status [flags]                       Compare current managed values with configured profiles
	  switchlet diff <profile-name> [flags]          Compare one profile with current managed values
	  switchlet doctor [--json]                      Check project health without writing files
	  switchlet version                              Show version information
	  switchlet completion <shell>                   Generate a shell completion script
	  switchlet help [command]                       Show help text

	Interactive workflow:
	  Launches a full-screen terminal UI for selecting and applying profiles.
	  Protected profiles show Continue first, then require Enter/y to confirm and n/Esc/q to cancel.

	Non-interactive flags:
	  --json               Write machine-readable JSON for list, inspect, apply, status, diff, or doctor
	  --short              Write concise text output for status
	  --patch              Write read-only managed patch text for diff
	  --expect             Assert that status matches one expected profile
	  --exit-code          Return non-zero from diff when the selected profile would change files
	  --dry-run            Preview apply impact without writing target files
	  --allow-protected    Explicitly allow non-interactive use of a protected profile
	  --no-color           Disable styled command output; also honored through NO_COLOR

	Examples:
	  switchlet
	  switchlet init
	  switchlet init --overwrite
	  switchlet config
	  switchlet list
	  switchlet inspect Local
	  switchlet apply Local --dry-run
	  switchlet status
	  switchlet status --short
	  switchlet status --expect Local
	  switchlet diff Local
	  switchlet diff Local --exit-code
	  switchlet diff Local --patch
	  switchlet doctor
	  switchlet version
	  switchlet completion bash
	  switchlet help apply

	Exit codes:
	  0 success
	  1 runtime failure, failed expectation, or detected diff
	  2 command-usage failure
`
}

func completionHelpText() string {
	return `Usage:
	  switchlet completion <shell>

	Generate a shell completion script. The generated script completes commands and
	flags statically, and completes profile names dynamically for inspect, apply,
	diff, and status --expect when a .switchlet.yaml can be discovered.

	Dynamic profile completion loads configuration schema only. It does not read
	target files, resolve environment variables, or inspect current managed values.
	Script generation itself does not load .switchlet.yaml.
	Supported shells: bash, zsh, fish.

	Examples:
	  switchlet completion bash
	  switchlet completion zsh
	  switchlet completion fish
`
}

func configHelpText() string {
	return `Usage:
	  switchlet config

	Open the interactive configuration editor for the discovered .switchlet.yaml.
	The editor is a full-screen terminal workflow for editing profiles and managed
	values in an in-memory draft. Nothing is written until you review and save the
	changes.

	This command requires stdin and stdout to be interactive terminals.

	Examples:
	  switchlet config
`
}

func versionHelpText() string {
	return `Usage:
	  switchlet version
	  switchlet --version

	Show the Switchlet version and exit without loading .switchlet.yaml.

	Examples:
	  switchlet version
	  switchlet --version
`
}

func statusHelpText() string {
	return `Usage:
	  switchlet status [--json] [--short] [--expect <profile-name>]

	Compare current managed target values with configured profiles without writing files.
	Use --expect to return success only when exactly that profile is the current
	complete profile.

	Flags:
	  --json       Write machine-readable JSON output
	  --short      Write a concise current-profile summary; cannot be combined with --json or --expect
	  --expect     Assert that the current complete profile matches the named profile
	  --no-color   Disable styled command output

	Examples:
	  switchlet status
	  switchlet status --short
	  switchlet status --expect Local
	  switchlet status --expect=-Local
	  switchlet status --json
`
}

func diffHelpText() string {
	return `Usage:
	  switchlet diff <profile-name> [--json] [--patch] [--exit-code]

	Compare one configured profile with current managed target values without writing files.
	Diff is read-only and does not require --allow-protected for protected profiles.
	Patch output is read-only managed patch text for piping to tools such as delta.
	Use --exit-code to return non-zero when included targets would update or any
	included profile value is unavailable.

	Flags:
	  --json       Write machine-readable JSON output
	  --patch      Write managed patch text; cannot be combined with --json
	  --exit-code  Return non-zero when the selected profile differs from current managed values
	  --no-color   Disable styled command output

	Examples:
	  switchlet diff Local
	  switchlet diff -- -Local
	  switchlet diff Local --json
	  switchlet diff Local --exit-code
	  switchlet diff Local --patch
`
}

func doctorHelpText() string {
	return `Usage:
	  switchlet doctor [--json]

	Run read-only project health checks without writing target files, temporary
	target files, or .switchlet.yaml.

	Doctor checks configuration discovery, configuration loading, startup target
	validation, profile availability, and current-state comparison availability.
	Missing or invalid configuration and target-read failures return a non-zero exit
	code. Unavailable environment-backed profile values are reported as warnings.

	Flags:
	  --json       Write machine-readable JSON output
	  --no-color   Disable styled command output

	Examples:
	  switchlet doctor
	  switchlet doctor --json
`
}

func initHelpText() string {
	return `Usage:
	  switchlet init [--overwrite]

	Create a new .switchlet.yaml in the current directory.
	When .switchlet.yaml already exists in the current directory, interactive init
	asks before replacing it. Use --overwrite to replace it without that prompt.
	Configurations discovered only in parent directories are not replaced from a
	nested directory.

	The init flow guides you through target-file selection, selector selection,
	named target entry, profile entry, and final review. When stdin and stdout are
	interactive terminals, init launches a terminal-native wizard. It can narrow
	large file lists, browse or search existing string-valued JSON/YAML paths,
	select dotenv keys, keeps manual entry available, can add .switchlet.yaml to the
	project .gitignore when literal profiles are configured, and writes a
	validated Version 3 target/profile configuration.

	The terminal-native wizard runs as a full-screen terminal UI when standard
	terminal interaction is available.

	When standard terminal interaction is unavailable, init falls back to
	the existing line-oriented prompt flow.

	Flags:
	  --overwrite   Replace an existing .switchlet.yaml in the current directory without prompting

	Examples:
	  switchlet init
	  switchlet init --overwrite
`
}

func listHelpText() string {
	return `Usage:
	  switchlet list [--json]

	List configured profiles, availability, and included target counts without launching the TUI.

	Flags:
	  --json       Write machine-readable JSON output
	  --no-color   Disable styled command output

	Examples:
	  switchlet list
	  switchlet list --json
`
}

func inspectHelpText() string {
	return `Usage:
	  switchlet inspect <profile-name> [--json]

	Inspect one configured profile by name, including planned targets and safe display values.

	Flags:
	  --json       Write machine-readable JSON output
	  --no-color   Disable styled command output

	Examples:
	  switchlet inspect Local
	  switchlet inspect -- -Local
	  switchlet inspect Local --json
`
}

func applyHelpText() string {
	return `Usage:
	  switchlet apply <profile-name> [--json] [--dry-run] [--allow-protected]

	Apply one configured profile by name. A profile may update one or more configured targets.

	Protected profiles in the interactive TUI already prompt for confirmation.
	Use --allow-protected only for non-interactive switchlet apply.

	Flags:
	  --json               Write machine-readable JSON output
	  --dry-run            Preview would-update, already-matching, unavailable, and omitted targets without writing files
	  --allow-protected    Explicitly allow non-interactive use of a protected profile
	  --no-color           Disable styled command output

	Examples:
	  switchlet apply Local --dry-run
	  switchlet apply --dry-run -- -Local
	  switchlet apply Production --dry-run --allow-protected
	  switchlet apply Local --dry-run --json
`
}
