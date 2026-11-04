package main

func usageText() string {
	return `Usage:
	  switchlet [--config <path>]                    Launch the profile switcher
	  switchlet init [--overwrite]                   Guided setup for a .switchlet.yaml in the current directory
	  switchlet config [--config <path>]             Edit .switchlet.yaml in a full-screen configuration editor
	  switchlet list [--json] [--config <path>]      List configured profiles and target counts without launching the TUI
	  switchlet inspect <profile-name> [flags]       Inspect one configured profile and its planned target changes
	  switchlet apply <profile-name> [flags]         Apply one configured profile by name
	  switchlet undo [flags]                         Undo the last recorded apply or configure recording
	  switchlet status [flags]                       Compare current managed values with configured profiles
	  switchlet diff <profile-name> [flags]          Compare one profile with current managed values
	  switchlet doctor [flags]                       Check project health without writing files
	  switchlet targets [flags]                      List configured targets without revealing managed values
	  switchlet version                              Show version information
	  switchlet completion <shell>                   Generate a shell completion script
	  switchlet help [command]                       Show help text

	Interactive workflow:
	  Launches a full-screen terminal UI for selecting and applying profiles.
	  Protected profiles show Continue first, then require Enter/y to confirm and n/Esc/q to cancel.

	Non-interactive flags:
	  --json               Write machine-readable JSON for list, inspect, apply, undo, status, diff, or doctor
	  --short              Write concise text output for status
	  --name               Write only the current complete profile name for status
	  --patch              Write read-only managed patch text for diff
	  --expect             Assert that status matches one expected profile
	  --expect-current     Assert the current complete profile before apply or dry run
	  --config             Load a specific .switchlet.yaml for project-reading commands
	  --exit-code          Return non-zero from diff when the selected profile would change files
	  --dry-run            Preview apply or undo impact without writing target files
	  --allow-protected    Explicitly allow non-interactive use of a protected profile
	  --strict             Return non-zero from doctor when warnings are present
	  --no-color           Disable styled command output; also honored through NO_COLOR

	Examples:
	  switchlet
	  switchlet --config ../other/.switchlet.yaml
	  switchlet init
	  switchlet init --overwrite
	  switchlet config
	  switchlet list
	  switchlet inspect Local
	  switchlet apply Local --dry-run
	  switchlet apply --expect-current Local Staging
	  switchlet apply Staging --expect-current Local
	  switchlet status
	  switchlet status --short
	  switchlet status --name
	  switchlet status --expect Local
	  switchlet diff Local
	  switchlet diff Local --exit-code
	  switchlet diff Local --patch
	  switchlet doctor
	  switchlet doctor --strict
	  switchlet targets
	  switchlet version
	  switchlet completion bash
	  switchlet completion powershell
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
	Leading global options such as --config and --no-color may appear before the
	command name. When --config is present, dynamic profile completion uses that
	explicit configuration path.
	Script generation itself does not load .switchlet.yaml.
	Supported shells: bash, zsh, fish, powershell.

	Examples:
	  switchlet completion bash
	  switchlet completion zsh
	  switchlet completion fish
	  switchlet completion powershell
`
}

func configHelpText() string {
	return `Usage:
	  switchlet config [--config <path>]

	Open the interactive configuration editor for the discovered .switchlet.yaml.
	The editor is a full-screen terminal workflow for editing profiles and managed
	values in an in-memory draft. Nothing is written until you review and save the
	changes.

	This command requires stdin and stdout to be interactive terminals.

	Examples:
	  switchlet config
	  switchlet config --config ../other/.switchlet.yaml
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
	  switchlet status [--json] [--short] [--expect <profile-name>] [--name] [--config <path>]

	Compare current managed target values with configured profiles without writing files.
	Use --expect to return success only when exactly that profile is the current
	complete profile.

	Flags:
	  --json       Write machine-readable JSON output
	  --short      Write a concise current-profile summary; cannot be combined with --json or --expect
	  --name       Write only the current complete profile name; cannot be combined with --json, --short, or --expect
	  --expect     Assert that the current complete profile matches the named profile
	  --config     Load a specific .switchlet.yaml instead of discovering upward
	  --no-color   Disable styled command output

	Examples:
	  switchlet status
	  switchlet status --short
	  switchlet status --name
	  switchlet status --expect Local
	  switchlet status --expect=-Local
	  switchlet status --json
	  switchlet status --config ../other/.switchlet.yaml
`
}

func diffHelpText() string {
	return `Usage:
	  switchlet diff <profile-name> [--json] [--patch] [--exit-code] [--config <path>]

	Compare one configured profile with current managed target values without writing files.
	Diff is read-only and does not require --allow-protected for protected profiles.
	Patch output is read-only managed patch text for piping to tools such as delta.
	Use --exit-code to return non-zero when included targets would update or any
	included profile value is unavailable.

	Flags:
	  --json       Write machine-readable JSON output
	  --patch      Write managed patch text; cannot be combined with --json
	  --exit-code  Return non-zero when the selected profile differs from current managed values
	  --config     Load a specific .switchlet.yaml instead of discovering upward
	  --no-color   Disable styled command output

	Examples:
	  switchlet diff Local
	  switchlet diff -- -Local
	  switchlet diff Local --json
	  switchlet diff Local --exit-code
	  switchlet diff Local --patch
	  switchlet diff Local --config ../other/.switchlet.yaml
`
}

func doctorHelpText() string {
	return `Usage:
	  switchlet doctor [--json] [--strict] [--config <path>]

	Run read-only project health checks without writing target files, temporary
	target files, or .switchlet.yaml.

	Doctor checks configuration discovery, configuration loading, startup target
	validation, profile availability, and current-state comparison availability.
	Missing or invalid configuration and target-read failures return a non-zero exit
	code. Unavailable environment-backed profile values are reported as warnings.

	Flags:
	  --json       Write machine-readable JSON output
	  --strict     Return non-zero when warnings are present
	  --config     Load a specific .switchlet.yaml instead of discovering upward
	  --no-color   Disable styled command output

	Examples:
	  switchlet doctor
	  switchlet doctor --json
	  switchlet doctor --strict
	  switchlet doctor --config ../other/.switchlet.yaml
`
}

func targetsHelpText() string {
	return `Usage:
	  switchlet targets [--json] [--config <path>]

	List configured target names, files, target types, and selectors without
	revealing managed values.

	Flags:
	  --json       Write machine-readable JSON output
	  --config     Load a specific .switchlet.yaml instead of discovering upward
	  --no-color   Disable styled command output

	Examples:
	  switchlet targets
	  switchlet targets --json
	  switchlet targets --config ../other/.switchlet.yaml
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
	  switchlet list [--json] [--config <path>]

	List configured profiles, availability, and included target counts without launching the TUI.

	Flags:
	  --json       Write machine-readable JSON output
	  --config     Load a specific .switchlet.yaml instead of discovering upward
	  --no-color   Disable styled command output

	Examples:
	  switchlet list
	  switchlet list --json
	  switchlet list --config ../other/.switchlet.yaml
`
}

func inspectHelpText() string {
	return `Usage:
	  switchlet inspect <profile-name> [--json] [--config <path>]

	Inspect one configured profile by name, including planned targets and safe display values.

	Flags:
	  --json       Write machine-readable JSON output
	  --config     Load a specific .switchlet.yaml instead of discovering upward
	  --no-color   Disable styled command output

	Examples:
	  switchlet inspect Local
	  switchlet inspect -- -Local
	  switchlet inspect Local --json
	  switchlet inspect Local --config ../other/.switchlet.yaml
`
}

func applyHelpText() string {
	return `Usage:
	  switchlet apply <profile-name> [--json] [--dry-run] [--allow-protected] [--expect-current <profile-name>] [--config <path>]

	Apply one configured profile by name. A profile may update one or more configured targets.
	Use --expect-current to refuse apply or dry run unless the current complete
	profile matches the expected profile name. --expect-current may appear before
	or after the selected profile name.

	Protected profiles in the interactive TUI already prompt for confirmation.
	Use --allow-protected only for non-interactive switchlet apply.

	Flags:
	  --json               Write machine-readable JSON output
	  --dry-run            Preview would-update, already-matching, unavailable, and omitted targets without writing files
	  --allow-protected    Explicitly allow non-interactive use of a protected profile
	  --expect-current     Require the current complete profile to match before apply or dry run
	  --config             Load a specific .switchlet.yaml instead of discovering upward
	  --no-color           Disable styled command output

	Examples:
	  switchlet apply Local --dry-run
	  switchlet apply --expect-current Local Staging
	  switchlet apply Staging --expect-current Local
	  switchlet apply --dry-run --expect-current Local -- -Production
	  switchlet apply Production --dry-run --allow-protected
	  switchlet apply Local --dry-run --json
	  switchlet apply Local --config ../other/.switchlet.yaml
`
}
