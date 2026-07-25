package main

func usageText() string {
	return `Usage:
	  switchlet                                      Launch the profile switcher
	  switchlet init                                 Guided setup for a new .switchlet.yaml in the current directory
	  switchlet list [--json]                        List configured profiles and target counts without launching the TUI
	  switchlet inspect <profile-name> [--json]      Inspect one configured profile and its planned target changes
	  switchlet apply <profile-name> [flags]         Apply one configured profile by name
	  switchlet help [command]                       Show help text

	Interactive workflow:
	  Launches a full-screen terminal UI for selecting and applying profiles.
	  Protected profiles show Continue first, then require Enter/y to confirm and n/Esc/q to cancel.

	Non-interactive flags:
	  --json               Write machine-readable JSON for list, inspect, or apply
	  --dry-run            Validate apply without writing target files
	  --allow-protected    Explicitly allow non-interactive use of a protected profile

	Exit codes:
	  0 success
	  1 runtime or validation failure
	  2 command-usage failure
`
}

func initHelpText() string {
	return `Usage:
	  switchlet init

	Create a new .switchlet.yaml in the current directory.

	The init flow guides you through file selection, JSON-path selection,
	profile entry, and final review. When stdin and stdout are interactive
	terminals, init launches a terminal-native wizard. It can narrow large file lists,
	browse or search existing string-valued JSON paths, keeps manual entry available
	for both selection steps, can add .switchlet.yaml to the project .gitignore when
	literal profiles are configured, and validates each step before writing the
	configuration.

	The terminal-native wizard runs as a full-screen terminal UI when standard
	terminal interaction is available.

	When standard terminal interaction is unavailable, init falls back to
	the existing line-oriented prompt flow.
`
}

func listHelpText() string {
	return `Usage:
	  switchlet list [--json]

	List configured profiles, availability, and included target counts without launching the TUI.

	Flags:
	  --json   Write machine-readable JSON output
`
}

func inspectHelpText() string {
	return `Usage:
	  switchlet inspect <profile-name> [--json]

	Inspect one configured profile by name, including planned targets and safe display values.

	Flags:
	  --json   Write machine-readable JSON output
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
`
}
