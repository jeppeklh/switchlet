package main

func usageText() string {
	return `Usage:
	  switchlet                                      Launch the profile switcher
	  switchlet init [--overwrite]                   Guided setup for a .switchlet.yaml in the current directory
	  switchlet config                               Edit .switchlet.yaml in a full-screen configuration editor
	  switchlet list [--json]                        List configured profiles and target counts without launching the TUI
	  switchlet inspect <profile-name> [--json]      Inspect one configured profile and its planned target changes
	  switchlet apply <profile-name> [flags]         Apply one configured profile by name
	  switchlet status [--json]                     Compare current managed values with configured profiles
	  switchlet diff <profile-name> [--json|--patch] Compare one profile with current managed values
	  switchlet version                              Show version information
	  switchlet completion <shell>                   Generate a static shell completion script
	  switchlet help [command]                       Show help text

	Interactive workflow:
	  Launches a full-screen terminal UI for selecting and applying profiles.
	  Protected profiles show Continue first, then require Enter/y to confirm and n/Esc/q to cancel.

	Non-interactive flags:
	  --json               Write machine-readable JSON for list, inspect, apply, status, or diff
	  --patch              Write read-only managed patch text for diff
	  --dry-run            Validate apply without writing target files
	  --allow-protected    Explicitly allow non-interactive use of a protected profile

	Examples:
	  switchlet
	  switchlet init
	  switchlet init --overwrite
	  switchlet config
	  switchlet list
	  switchlet inspect Local
	  switchlet apply Local --dry-run
	  switchlet status
	  switchlet diff Local
	  switchlet diff Local --patch
	  switchlet version
	  switchlet completion bash
	  switchlet help apply

	Exit codes:
	  0 success
	  1 runtime or validation failure
	  2 command-usage failure
`
}

func completionHelpText() string {
	return `Usage:
	  switchlet completion <shell>

	Generate a static shell completion script without loading .switchlet.yaml.
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
	  switchlet status [--json]

	Compare current managed target values with configured profiles without writing files.

	Flags:
	  --json   Write machine-readable JSON output

	Examples:
	  switchlet status
	  switchlet status --json
`
}

func diffHelpText() string {
	return `Usage:
	  switchlet diff <profile-name> [--json|--patch]

	Compare one configured profile with current managed target values without writing files.
	Diff is read-only and does not require --allow-protected for protected profiles.
	Patch output is read-only managed patch text for piping to tools such as delta.

	Flags:
	  --json    Write machine-readable JSON output
	  --patch   Write managed patch text; cannot be combined with --json

	Examples:
	  switchlet diff Local
	  switchlet diff Local --json
	  switchlet diff Local --patch
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
	  --json   Write machine-readable JSON output

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
	  --json   Write machine-readable JSON output

	Examples:
	  switchlet inspect Local
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
	  --dry-run            Validate the apply operation without writing target files
	  --allow-protected    Explicitly allow non-interactive use of a protected profile

	Examples:
	  switchlet apply Local --dry-run
	  switchlet apply Production --dry-run --allow-protected
	  switchlet apply Local --dry-run --json
`
}
