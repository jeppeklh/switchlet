# Switchlet Commands

Command-only reference for the `switchlet` CLI.

## Overview

| Command | Purpose |
|---|---|
| `switchlet [--config <path>]` | Launch the interactive profile picker. |
| `switchlet init [--overwrite]` | Create `.switchlet.yaml` in the current directory. |
| `switchlet config [--config <path>]` | Edit `.switchlet.yaml` in the configuration editor. |
| `switchlet list [--json] [--config <path>]` | List configured profiles. |
| `switchlet inspect <profile-name> [flags]` | Inspect one profile and its planned target changes. |
| `switchlet apply <profile-name> [flags]` | Apply one configured profile. |
| `switchlet status [flags]` | Compare current managed values with configured profiles. |
| `switchlet diff <profile-name> [flags]` | Compare one profile with current managed values. |
| `switchlet doctor [flags]` | Check project health without writing files. |
| `switchlet targets [flags]` | List configured targets without revealing managed values. |
| `switchlet version` / `switchlet --version` | Show version information. |
| `switchlet completion <shell>` | Generate a shell completion script. |
| `switchlet help [command]` | Show general or command-specific help. |

Switchlet discovers `.switchlet.yaml` by searching upward from the current
working directory by default. Use `--config <path>` with project-reading
commands to load a specific configuration file. Relative `--config` paths
resolve from the current working directory. Relative target paths still resolve
from the directory that contains the selected `.switchlet.yaml`.

## Setup Commands

### `switchlet`

Launches the full-screen profile picker after discovering and validating project
configuration.

```bash
switchlet
switchlet --config ../other/.switchlet.yaml
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

Opens the interactive configuration editor for the discovered or selected
`.switchlet.yaml`.

```bash
switchlet config
switchlet config --config ../other/.switchlet.yaml
```

The editor works on an in-memory draft and writes only from the save review.
Saving preserves comments in unchanged sections where safe, but formatting may
still normalize and some comments may move or be dropped when exact
preservation is unsafe.

## Profile Commands

### `switchlet list`

Lists configured profiles, availability, protection, and target counts.

```bash
switchlet list
switchlet list --json
switchlet list --config ../other/.switchlet.yaml
switchlet list --no-color
```

### `switchlet inspect`

Shows one profile's planned target changes without writing files.

```bash
switchlet inspect Local
switchlet inspect -- -Local
switchlet inspect Local --json
switchlet inspect Local --config ../other/.switchlet.yaml
switchlet inspect Local --no-color
```

### `switchlet apply`

Applies one configured profile by name.

```bash
switchlet apply Local
switchlet apply Local --dry-run
switchlet apply --expect-current Local Staging
switchlet apply Staging --expect-current Local
switchlet apply --dry-run -- -Local
switchlet apply Production --allow-protected
switchlet apply Local --dry-run --json
switchlet apply Local --config ../other/.switchlet.yaml
```

| Flag | Description |
|---|---|
| `--dry-run` | Preview would-update, already-matching, unavailable, and omitted targets without writing target files. |
| `--allow-protected` | Explicitly allow non-interactive apply for protected profiles. |
| `--expect-current <profile>` | Refuse apply or dry run unless the current complete profile matches the expected profile. |
| `--json` | Write machine-readable JSON output. |
| `--config <path>` | Load a specific `.switchlet.yaml` instead of discovering upward. |
| `--no-color` | Disable styled command output. |

Protected profiles are never applied silently. Non-interactive apply requires
`--allow-protected` for protected profiles.

`--expect-current` uses the same complete-profile semantics as
`switchlet status --expect`. It exits non-zero without writing files when the
expected profile is missing, no complete profile matches, a different complete
profile matches, multiple complete profiles match, or current target values
cannot be read safely.

`--expect-current <profile>` may appear before or after the selected profile
name. If the selected profile name starts with `-`, use `--` before that
selected profile name and place all flags, including `--expect-current`, before
`--`.

For profiles that update multiple target files, Switchlet prepares every target
file before replacing any file. Files whose managed values already match are
validated but are not preflighted or replaced; their bytes and modification times
remain unchanged. Other files receive a write preflight before replacement.
If a write fails, Switchlet attempts best-effort restoration of every file
already replaced from its original same-apply contents. This includes the failing
file when replacement succeeded but directory syncing failed. Switchlet reports
whether restoration succeeded or files may still be partially updated. Undo recording is optional; see [Undo Last Apply](#undo-last-apply).

YAML applies preserve surrounding formatting for ordinary single-line plain or
quoted values, including comments and line endings. Complex scalar syntax uses
the existing serializer fallback and may normalize formatting. Candidate source
edits are re-parsed and checked for semantic correctness before writing.

Use `--` before a positional profile name that starts with `-`. For `apply`, put
flags before `--`, for example `switchlet apply --dry-run -- -Local`.

## Undo Last Apply

Undo recording is **off by default** and enabled per configuration path:

```bash
switchlet undo --enable
switchlet apply Staging
switchlet undo --dry-run
switchlet undo
```

### Interactive Review

In the picker, press `u` to open **Undo and recovery**. This shows recording
status, the recorded apply, and affected files. `Enter` prepares an available
undo or interrupted-operation recovery; `Enter`/`y` on the resulting confirmation
executes it. Opening or cancelling the screen does not enable recording or write
target files. Ordinary apply keeps its existing Enter/Space behavior.

When recording is disabled, `e` opens an opt-in confirmation explaining that
whole-file snapshots can contain secrets and are private but unencrypted.
`x` separately reviews discarding a saved record. Discard confirmation explicitly
states that it removes the record without restoring or changing target files.
`n`/`Esc` cancels a confirmation; `Esc` from the overview or result returns to the
picker. `r` refreshes status, and `PgUp`/`PgDn` or `Home`/`End` scroll long reviews.

Confirmation checks the reviewed record and recording settings under the project
lock, then repeats all file guards. A replaced record or external edit invalidates
the review and requires a fresh review. A completed restoration refreshes the
picker's current-profile comparison while preserving its selection and filter.

Unavailable optional storage does not prevent browsing the picker. Unreadable or
corrupt records cannot be approved in the UI: inspect `switchlet undo --status`,
then use the existing CLI discard action only after inspecting affected files.
CLI recovery remains available when configuration or target validation prevents
the picker from starting; use `--config` to select that configuration explicitly.

### Restoration Guarantees

When enabled, changed CLI and TUI applies save one snapshot of the affected files.
Undo restores their exact previous bytes and file modes, including comments,
formatting, unmanaged contents, and values that did not match a named profile.
It works with all supported formats and scalar types. The explicit undo operation
uses captured files; it does not resolve profiles or environment values again.
Existing protected-profile approval checks still apply to profile application.

Every affected file must still match the recorded post-apply bytes and modes.
A changed or missing file, symlink, or permission change refuses restoration
before any target is written. There is no force flag. Snapshot restore uses the
same preparation, freshness checks, replacement, verification, and best-effort
recovery machinery as apply; multiple files are not a filesystem transaction.

Successful undo consumes the snapshot. A second undo cannot toggle or step back
again. A new successful changed apply replaces the prior snapshot. Dry runs,
unchanged applies, and failed applies that safely roll back retain the prior
successful snapshot.

Other actions:

```bash
switchlet undo --status           # Show opt-in, snapshot state, and affected files
switchlet undo --discard          # Delete the snapshot; retain opt-in
switchlet undo --disable          # Delete the snapshot and disable recording
switchlet undo --dry-run --json   # Read-only validation and value-safe metadata
switchlet undo --config ../project/.switchlet.yaml
```

`--enable`, `--disable`, `--discard`, `--status`, and `--recover` are mutually
exclusive actions. `--dry-run` applies only to ordinary undo or recovery.
`--config` and `--json` are supported for all actions. Undo can use an explicit
configuration path even if the configuration contents can no longer parse.

### Interrupted Operations

Snapshots are published as pending before file replacement and marked ready only
after verification. An interrupted apply/undo or failed post-write verification
blocks subsequent applies, preserving recovery information:

```bash
switchlet undo --status
switchlet undo --recover --dry-run
switchlet undo --recover
```

Recovery accepts only each file's exact recorded before/after bytes and modes.
It restores the original pre-apply state and consumes the record. Unexpected edits
still cause refusal before writes. `--discard` explicitly abandons the recovery
record without changing target files. Discarding corrupt metadata resets recording
to disabled, so it must be explicitly enabled again.

### Storage And Privacy

Whole-file snapshots can contain credentials and other values Switchlet does not
manage. They are **unencrypted** private local state outside the repository by
default, with one snapshot per selected configuration path. Unix storage requires
current-user ownership and private modes; Windows storage uses a current-user and
SYSTEM ACL. Normal output and JSON expose metadata only, never stored contents.

State lives under `switchlet/undo/<project-key>` in the OS user-state location:
`XDG_STATE_HOME` or `~/.local/state` on Linux, Application Support on macOS, and
Local AppData on Windows. `SWITCHLET_STATE_HOME` can select an absolute custom root;
keep it private and outside version control. `undo --status` reports the location.
Applying with recording disabled creates only private coordination metadata, not
file snapshots. Read-only status does not create storage. Process locks serialize
Switchlet applies and undo/preferences operations for the same configuration.

## Typed Values (Configuration Version 4)

Version 4 adds `valueType` to each target. The file `type` still selects JSON,
YAML, TOML, or dotenv. `valueType` describes the managed setting and defaults to
`string` when omitted. The setting must already exist with a compatible native
type; Switchlet does not change a string setting into a boolean or number.

```yaml
version: 4
targets:
  - name: featureEnabled
    file: settings.json
    jsonPath: Features.Enabled
    valueType: boolean
  - name: port
    file: settings.json
    jsonPath: Server.Port
    valueType: integer
  - name: timeout
    file: settings.json
    jsonPath: Server.Timeout
    valueType: number
profiles:
  - name: Local
    values:
      - target: featureEnabled
        value: false
      - target: port
        value: 8080
      - target: timeout
        value: 0.75
```

Supported value types:

| `valueType` | Literal examples | Rules |
|---|---|---|
| `string` | `"8080"`, `"false"` | Text; existing non-empty profile rules apply. |
| `boolean` | `true`, `false` | Both values are valid, including `false`. |
| `integer` | `0`, `8080`, `-30` | Decimal integers from -9223372036854775808 through 9223372036854775807. |
| `number` | `0`, `0.75`, `1.25e2` | Finite decimal numbers; exact decimal digits are retained. |

In Version 4, literal YAML types must match: `value: "false"` is a string and is
rejected for a boolean target. Numeric literals and entered/environment values
use decimal syntax compatible with JSON numbers. Hexadecimal, octal, and other
format-specific literal syntax are not accepted in profile values. Existing
source files may retain supported native spellings such as TOML `1_000` or `0x10`.
A `number` target accepts integer or fractional source values. Written
integer-valued numbers use a decimal point (for example `30.0`), preserving a
numeric rather than string representation across formats.

Number input and canonical decimal text are limited to 512 characters; written
exponents must be between -1024 and 1024 before magnitude validation.
Overflow, non-finite values (`NaN`/infinity), and underflow to zero are rejected.
Floating-point parsing checks magnitude only; exact decimal arithmetic produces
the stored/comparison text, so high-precision fractional values are not rounded
through a Go `float64`. Consuming applications still apply their own numeric rules.

`valueFromEnv` keeps its existing syntax. The environment text is parsed using
the target's declared type; booleans require lowercase `true` or `false`, and
invalid/empty values make the profile unavailable without printing their content.
All included values are resolved and validated before any target file is replaced.

Dotenv has no native scalar types. For typed dotenv targets, Switchlet validates
the decoded text using `valueType` and writes canonical text such as `false` or
`8080`. In `switchlet config`, press `t` on the dotenv managed-value review screen
to choose string, boolean, integer, or number. Init defaults dotenv settings to
strings; structured-file init detects supported native scalar types automatically.
Typed literal entry validates before proceeding and keeps zero/false intact.

Status, diff, previews, dry runs, and post-apply verification use the same typed
normalization. Type mismatches fail before comparison. Equivalent number spellings
(such as `1.25e2` and `125.0`) already match and do not trigger file replacement.
Normal output keeps values hidden; optional `valueType` metadata is included in
JSON target descriptors and profile-value descriptors.

Versions 1, 2, and 3 retain their existing string semantics. `valueType` requires
Version 4. String-only init continues to generate Version 3; adding an explicitly
typed target generates Version 4. The config editor reviews a conversion to
Version 4 before save and preserves an existing Version 4 document on subsequent
saves. Changing a type alone counts as a configuration change.

## Current-State Commands

### `switchlet status`

Compares current managed target values with configured profiles without writing
files.

```bash
switchlet status
switchlet status --short
switchlet status --name
switchlet status --json
switchlet status --expect Local
switchlet status --expect=Local
switchlet status --expect=-Local
switchlet status --config ../other/.switchlet.yaml
switchlet status --no-color
```

A profile is reported as current only when it includes every configured target
and every included resolved value matches the current target value. Partial
profiles can be reported as partial matches, but they are not current-profile
results when they omit configured targets.

Use `--short` for a one-line human-readable current-state summary. It is
value-safe and cannot be combined with `--json`.

Use `--name` for script-friendly current-profile output. It prints exactly the
current complete profile name and a trailing newline when one complete profile
matches. It exits non-zero for no complete match, ambiguous complete matches,
configuration failures, and target-read failures. It cannot be combined with
`--json`, `--short`, or `--expect`.

Use `--expect <profile-name>` or `--expect=<profile-name>` for scripts that
need to assert the current complete profile. It exits `0` only when exactly the
expected complete profile matches current managed values. It exits non-zero for
no match, a different match, ambiguous matches, a missing expected profile, or
target-read failures. `--expect` cannot be combined with `--short`.

### `switchlet diff`

Compares one configured profile with current managed target values without
writing files.

```bash
switchlet diff Staging
switchlet diff -- -Staging
switchlet diff Staging --json
switchlet diff Staging --exit-code
switchlet diff Staging --patch
switchlet diff Staging --config ../other/.switchlet.yaml
switchlet diff Staging --no-color
switchlet diff Staging --patch | delta
```

Diff compares only targets included by the selected profile. Omitted targets are
unchanged by that profile. Protected profiles can be diffed without
`--allow-protected` because the command is read-only.

Use `--exit-code` when scripts need change detection. The command exits `0` when
every included target already matches and no included value is unavailable. It
exits non-zero when any included target would update or any included value is
unavailable. Omitted targets in partial profiles do not by themselves make the
command non-zero.

### `switchlet doctor`

Runs read-only project health checks without writing target files, temporary
target files, or `.switchlet.yaml`.

```bash
switchlet doctor
switchlet doctor --json
switchlet doctor --strict
switchlet doctor --config ../other/.switchlet.yaml
```

Doctor checks configuration discovery, configuration loading and schema
validation, startup target validation, profile availability, and current-state
comparison availability. Missing or invalid configuration and target-read
failures return non-zero. Unavailable environment-backed profile values are
reported as warnings. Failed and warning checks include a value-safe recovery
hint when Switchlet can identify a clear next step.

Use `--strict` when warnings should fail CI or shell automation. It preserves
the same doctor report data and statuses, but changes the exit code to non-zero
when warnings are present.

### `switchlet targets`

Lists configured target names, files, target types, and selectors without
revealing managed values.

```bash
switchlet targets
switchlet targets --json
switchlet targets --config ../other/.switchlet.yaml
```

This command is inventory-only. It does not resolve profile values or expose raw
current or replacement values.

## Output Modes

### Color Control

Use `--no-color` to disable ANSI styling in human-readable command output and
command errors:

```bash
switchlet --no-color status
switchlet diff Local --no-color
```

Switchlet also honors `NO_COLOR` when the environment variable is present and
not empty. Command output is plain by default when stdout is not an interactive
terminal. JSON output and managed patch output remain plain and unstyled.

### JSON

Use `--json` with commands that support structured output:

```bash
switchlet list --json
switchlet inspect Local --json
switchlet apply Local --dry-run --json
switchlet status --json
switchlet diff Local --json
switchlet doctor --json
switchlet targets --json
```

JSON output includes top-level `schemaVersion: 2`, profile names, target names,
files, selectors, availability, and safe errors. The schema version describes
Switchlet command JSON output, not the `.switchlet.yaml` configuration schema.
JSON output does not include raw current target values or raw resolved profile
values. Masked profile values use safe labels such as `hidden literal value` or
`hidden environment value`; doctor checks may include an additive `hint` field.
Schema version `2` standardizes unavailable-value objects on
`environmentVariableName`; the older `environmentVariable` field is not emitted
in this schema version.

`apply --dry-run --json` also includes an additive `preview` object with
would-update, already-matching, unavailable, and omitted target categories.

### Short Status

Use `--short` with `status` for a concise text summary of the current profile:

```bash
switchlet status --short
```

Short status output is value-safe and cannot be combined with `--json`.

### Name Status

Use `--name` with `status` when a script needs only the current complete profile
name:

```bash
switchlet status --name
```

Name status output is value-safe and cannot be combined with `--json`,
`--short`, or `--expect`.

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
switchlet completion powershell
```

Completion includes static commands and flags. For `inspect`, `apply`, `diff`,
and `status --expect`, the generated scripts dynamically complete profile names
from the discovered `.switchlet.yaml` when one is available. Dynamic profile
completion loads configuration schema only; it does not read target files,
resolve environment variables, or inspect current managed values. Leading global
options such as `--config <path>` and `--no-color` may appear before the command
name. When `--config` is present, dynamic profile completion uses that explicit
configuration path. Supported shells are `bash`, `zsh`, `fish`, and
`powershell`.

## Examples

```bash
switchlet init
switchlet list
switchlet inspect Local
switchlet apply Local --dry-run
switchlet apply --expect-current Local Staging
switchlet apply Staging --expect-current Local
switchlet apply Local
switchlet status
switchlet doctor
switchlet doctor --strict
switchlet diff Local
switchlet targets
switchlet version
switchlet completion bash
switchlet completion powershell
switchlet help apply
```

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Runtime, configuration, validation, or target-read failure; failed `status --expect`; failed `apply --expect-current`; `doctor --strict` warnings; or `diff --exit-code` detected changes or unavailable values. |
| `2` | Command-usage failure. |
