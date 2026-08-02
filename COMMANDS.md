# Switchlet Commands

Command-only reference for the `switchlet` CLI.

## Overview

| Command | Purpose |
|---|---|
| `switchlet` | Launch the interactive profile picker. |
| `switchlet init [--overwrite]` | Create `.switchlet.yaml` in the current directory. |
| `switchlet config` | Edit the discovered `.switchlet.yaml` in the configuration editor. |
| `switchlet list [--json]` | List configured profiles. |
| `switchlet inspect <profile-name> [--json]` | Inspect one profile and its planned target changes. |
| `switchlet apply <profile-name> [flags]` | Apply one configured profile. |
| `switchlet status [--json\|--short]` | Compare current managed values with configured profiles. |
| `switchlet diff <profile-name> [--json\|--patch]` | Compare one profile with current managed values. |
| `switchlet version` / `switchlet --version` | Show version information. |
| `switchlet completion <shell>` | Generate a shell completion script. |
| `switchlet help [command]` | Show general or command-specific help. |

Switchlet discovers `.switchlet.yaml` by searching upward from the current
working directory. Relative target paths are resolved from the directory that
contains `.switchlet.yaml`.

## Setup Commands

### `switchlet`

Launches the full-screen profile picker after discovering and validating project
configuration.

```bash
switchlet
```

This command requires stdin and stdout to be interactive terminals. In scripts,
CI jobs, or redirected shells, use non-interactive commands such as
`switchlet list`, `switchlet status`, or `switchlet apply <profile-name>`.

### `switchlet init`

Creates a new `.switchlet.yaml` in the current directory.

```bash
switchlet init
switchlet init --overwrite
```

| Flag | Description |
|---|---|
| `--overwrite` | Replace an existing `.switchlet.yaml` in the current directory without the initial overwrite prompt. |

`switchlet init` refuses to replace a `.switchlet.yaml` discovered only in a
parent directory. The setup flow supports JSON, YAML, TOML, and dotenv targets.

### `switchlet config`

Opens the interactive configuration editor for the discovered `.switchlet.yaml`.

```bash
switchlet config
```

The editor works on an in-memory draft and writes only from the save review.
Saving may normalize `.switchlet.yaml` formatting.

## Profile Commands

### `switchlet list`

Lists configured profiles, availability, protection, and target counts.

```bash
switchlet list
switchlet list --json
switchlet list --no-color
```

### `switchlet inspect`

Shows one profile's planned target changes without writing files.

```bash
switchlet inspect Local
switchlet inspect Local --json
switchlet inspect Local --no-color
```

### `switchlet apply`

Applies one configured profile by name.

```bash
switchlet apply Local
switchlet apply Local --dry-run
switchlet apply Production --allow-protected
switchlet apply Local --dry-run --json
```

| Flag | Description |
|---|---|
| `--dry-run` | Validate the apply operation without writing target files. |
| `--allow-protected` | Explicitly allow non-interactive apply for protected profiles. |
| `--json` | Write machine-readable JSON output. |
| `--no-color` | Disable styled command output. |

Protected profiles are never applied silently. Non-interactive apply requires
`--allow-protected` for protected profiles.

## Current-State Commands

### `switchlet status`

Compares current managed target values with configured profiles without writing
files.

```bash
switchlet status
switchlet status --short
switchlet status --json
switchlet status --no-color
```

A profile is reported as current only when it includes every configured target
and every included resolved value matches the current target value. Partial
profiles can be reported as partial matches, but they are not current-profile
results when they omit configured targets.

Use `--short` for a one-line human-readable current-state summary. It is
value-safe and cannot be combined with `--json`.

### `switchlet diff`

Compares one configured profile with current managed target values without
writing files.

```bash
switchlet diff Staging
switchlet diff Staging --json
switchlet diff Staging --patch
switchlet diff Staging --no-color
switchlet diff Staging --patch | delta
```

Diff compares only targets included by the selected profile. Omitted targets are
unchanged by that profile. Protected profiles can be diffed without
`--allow-protected` because the command is read-only.

## Output Modes

### Color Control

Use `--no-color` to disable ANSI styling in human-readable command output and
command errors:

```bash
switchlet --no-color status
switchlet diff Local --no-color
```

Switchlet also honors `NO_COLOR` when the environment variable is present and
not empty. JSON output and managed patch output remain plain and unstyled.

### JSON

Use `--json` with commands that support structured output:

```bash
switchlet list --json
switchlet inspect Local --json
switchlet apply Local --dry-run --json
switchlet status --json
switchlet diff Local --json
```

JSON output includes profile names, target names, files, selectors,
availability, and safe errors. It does not include raw current target values or
raw resolved profile values.

### Short Status

Use `--short` with `status` for a concise text summary of the current profile:

```bash
switchlet status --short
```

Short status output is value-safe and cannot be combined with `--json`.

### Managed Patch

Use `--patch` with `diff` when you want read-only managed patch text:

```bash
switchlet diff Staging --patch
switchlet diff Staging --patch | delta
```

Patch output is limited to Switchlet-managed target locations. It is not a full
source-control diff, does not invoke an external pager, and cannot be combined
with `--json`.

Patch output redacts current and profile values by default. It shows which
Switchlet-managed targets would change, but it does not print raw managed values,
unrelated file contents, omitted target values, unavailable values, or unchanged
already-matching values.

## Utility Commands

### `switchlet version`

Shows the Switchlet version without loading project configuration.

```bash
switchlet version
switchlet --version
```

### `switchlet completion`

Generates shell completion scripts. Script generation itself does not load
project configuration.

```bash
switchlet completion bash
switchlet completion zsh
switchlet completion fish
```

Completion includes static commands and flags. For `inspect`, `apply`, and
`diff`, the generated scripts dynamically complete profile names from the
discovered `.switchlet.yaml` when one is available. Dynamic profile completion
loads configuration schema only; it does not read target files, resolve
environment variables, or inspect current managed values.

## Examples

```bash
switchlet init
switchlet list
switchlet inspect Local
switchlet apply Local --dry-run
switchlet apply Local
switchlet status
switchlet diff Local
switchlet version
switchlet completion bash
switchlet help apply
```

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Runtime, configuration, validation, or target-read failure. |
| `2` | Command-usage failure. |
