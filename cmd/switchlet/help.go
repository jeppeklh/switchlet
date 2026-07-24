package main

func usageText() string {
	return `Usage:
	  switchlet                                      Launch the profile switcher
	  switchlet init                                 Guided setup for a new .switchlet.yaml in the current directory
	  switchlet list [--json]                        List configured profiles without launching the TUI
	  switchlet inspect <profile-name> [--json]      Inspect one configured profile by name
	  switchlet apply <profile-name> [flags]         Apply one configured profile by name
	  switchlet help [command]                       Show help text

	Non-interactive flags:
	  --json               Write machine-readable JSON for list, inspect, or apply
	  --dry-run            Validate apply without writing the target file
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

	The init flow discovers candidate JSON files, lets you narrow large file
	lists by name or path, lets you browse existing string-valued JSON
	paths inside the selected file, keeps manual entry available for both
	steps, and validates each step before writing the configuration.
`
}

func listHelpText() string {
	return `Usage:
	  switchlet list [--json]

	List configured profiles without launching the TUI.

	Flags:
	  --json   Write machine-readable JSON output
`
}

func inspectHelpText() string {
	return `Usage:
	  switchlet inspect <profile-name> [--json]

	Inspect one configured profile by name.

	Flags:
	  --json   Write machine-readable JSON output
`
}

func applyHelpText() string {
	return `Usage:
	  switchlet apply <profile-name> [--json] [--dry-run] [--allow-protected]

	Apply one configured profile by name.

	Flags:
	  --json               Write machine-readable JSON output
	  --dry-run            Validate the apply operation without writing the target file
	  --allow-protected    Explicitly allow non-interactive use of a protected profile
`
}
