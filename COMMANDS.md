# Switchlet Commands

This is the user-facing command reference for Switchlet.

## Interactive Commands

Start the profile picker:

```bash
switchlet
```

Create project configuration:

```bash
switchlet init
```

Replace project configuration without the interactive overwrite prompt:

```bash
switchlet init --overwrite
```

`switchlet init` launches the terminal setup wizard when stdin and stdout are
interactive terminals. Otherwise, it uses line-oriented prompts.

When `.switchlet.yaml` already exists in the current directory, interactive
init asks before replacing it. Non-interactive replacement requires
`--overwrite`. Configurations discovered only in parent directories are not
replaced from a nested directory.

The init flow supports JSON, YAML, TOML, and dotenv managed values. YAML and TOML
files are listed only when they contain manageable existing string values.

## Profile Commands

List profiles:

```bash
switchlet list
```

Inspect a profile before applying it:

```bash
switchlet inspect Local
```

Apply a profile:

```bash
switchlet apply Local
```

Validate the apply path without writing files:

```bash
switchlet apply Local --dry-run
```

Apply a protected profile non-interactively:

```bash
switchlet apply Production --allow-protected
```

Protected profiles are never applied silently. Interactive use asks for
confirmation. Non-interactive apply requires `--allow-protected`.

## Current-State Commands

Read-only commands for understanding current managed target values without
launching the TUI and without writing target files.

Report whether current managed values match configured profiles:

```bash
switchlet status
switchlet status --json
```

Compare one profile with current managed values:

```bash
switchlet diff Staging
switchlet diff Staging --json
switchlet diff Staging --patch
switchlet diff Staging --patch | delta
```

These commands are non-interactive. The interactive picker exposes the same
read-only comparison model through explicit `s` status and `d` diff actions. It
does not show automatic main-picker current-profile badges or dashboard panels.

Both commands return `0` for successful reports, `1` for runtime,
configuration, or target-read failures, and `2` for command-usage failures.

### Status Behavior

`switchlet status` reads the current values of configured JSON, YAML, TOML, and
dotenv targets and compares them with configured profiles.

A profile is reported as the current profile only when it includes every
configured target and every included resolved value equals the current target
value. Partial profiles can be reported as partial matches, but they are not
current-profile results when they omit configured targets.

When exactly one complete profile matches:

```text
$ switchlet status
Switchlet status
Current profile   Local

Matched targets
> database [json]
  file              /workspace/backend/appsettings.Development.json
  jsonPath          ConnectionStrings.DefaultConnection
> frontendApi [dotenv]
  file              /workspace/frontend/.env.local
  key               VITE_API_URL
```

When no complete profile matches:

```text
$ switchlet status
Switchlet status
State             no complete profile match

Partial matches
> Service Endpoint Only  1/1 included match; 3 omitted

Closest profiles
> Local  2/4 targets match
> Staging  1/4 targets match
```

When more than one complete profile matches, Switchlet reports the ambiguity
instead of choosing one:

```text
$ switchlet status
Switchlet status
State             multiple complete profiles match

Matches
> Local
> Local Copy
```

Profiles with missing or empty environment-backed values may be listed as
unavailable for comparison. Raw current target values and raw resolved profile
values are never printed by default text or JSON status output.

### Diff Behavior

`switchlet diff <profile>` compares only the targets included by the selected
profile. Omitted targets are unchanged by that profile and are not mismatches.

Diff is read-only and does not require `--allow-protected` for protected
profiles.

Example:

```text
$ switchlet diff Staging
Switchlet diff  Staging
Protection      protected

Would update
> database [json]
  file              /workspace/backend/appsettings.Development.json
  jsonPath          ConnectionStrings.DefaultConnection

Already matches
> frontendApi [dotenv]
  file              /workspace/frontend/.env.local
  key               VITE_API_URL

Unavailable
> workerQueue [yaml]
  file              /workspace/worker/config.yaml
  yamlPath          queue.endpoint
  environment       STAGING_WORKER_QUEUE_ENDPOINT
  reason            environment variable "STAGING_WORKER_QUEUE_ENDPOINT" is not set
```

Diff output identifies target names, files, target types, and selectors without
printing raw current target values or raw resolved profile values unless the
explicit `--patch` mode is requested.

### Managed Patch Output

Use `--patch` when you want read-only patch text for external tools:

```bash
switchlet diff Staging --patch
switchlet diff Staging --patch | delta
```

Patch output is not a full source-control diff of the target file. It is limited
to Switchlet-managed target locations. It does not invoke `delta`, require an
external pager, write target files, create target-file temporary files, or edit
`.switchlet.yaml`.

Patch output intentionally includes current and profile values for would-update
managed targets. It does not print unrelated file contents, omitted target
values, unavailable values, or unchanged already-matching values.

`--patch` cannot be combined with `--json`.

### JSON Output Contract

Use `--json` for scripts. JSON output for `status` and `diff` is the stable
automation surface and remains secret-safe.

`switchlet status --json` returns a `result` object with fields such as:

```json
{
  "result": {
    "command": "status",
    "status": "unmatched",
    "currentProfile": "",
    "targetCount": 4,
    "matches": [],
    "matchedTargets": [],
    "partialMatches": [
      {
        "profileName": "Service Endpoint Only",
        "matchedTargets": 1,
        "includedTargets": 1,
        "omittedTargets": 3
      }
    ],
    "closestProfiles": [
      {
        "profileName": "Local",
        "matchedTargets": 2,
        "targetCount": 4
      }
    ],
    "unavailableProfiles": [],
    "complete": true
  }
}
```

`switchlet diff <profile> --json` returns a `result` object with fields such as:

```json
{
  "result": {
    "command": "diff",
    "profileName": "Staging",
    "protected": true,
    "complete": false,
    "wouldUpdate": [
      {
        "targetName": "database",
        "targetType": "json",
        "targetFile": "/workspace/backend/appsettings.Development.json",
        "selectorName": "jsonPath",
        "selector": "ConnectionStrings.DefaultConnection"
      }
    ],
    "alreadyMatches": [
      {
        "targetName": "frontendApi",
        "targetType": "dotenv",
        "targetFile": "/workspace/frontend/.env.local",
        "selectorName": "key",
        "selector": "VITE_API_URL"
      }
    ],
    "unavailable": [
      {
        "targetName": "workerQueue",
        "targetType": "yaml",
        "targetFile": "/workspace/worker/config.yaml",
        "selectorName": "yamlPath",
        "selector": "queue.endpoint",
        "environmentVariable": "STAGING_WORKER_QUEUE_ENDPOINT",
        "reason": "environment variable \"STAGING_WORKER_QUEUE_ENDPOINT\" is not set"
      }
    ],
    "omittedTargets": []
  }
}
```

These JSON objects may grow through additive fields after release, but they must
not include raw current target values or raw resolved profile values.

## JSON Output

Use `--json` with `list`, `inspect`, `apply`, `status`, or `diff`:

```bash
switchlet list --json
switchlet inspect Local --json
switchlet apply Local --dry-run --json
switchlet status --json
switchlet diff Local --json
```

Use `--patch` with `diff` when you want managed patch text instead of JSON.

JSON output includes profile names, target names, files, selectors,
availability, and safe errors. It does not include unmasked resolved secret
values.

## Examples

List profiles from a mixed JSON, YAML, TOML, and dotenv project:

```text
$ switchlet list
Local [4 targets]
Staging [4 targets, protected]
Service Endpoint Only [1 target, partial]
```

Inspect a profile with mixed target types:

```text
$ switchlet inspect Staging
Profile: Staging
Availability: Available
Source: Mixed
Protection: Protected

Changes: 4 targets

Planned targets:
- database [json]
  file: /workspace/backend/appsettings.Development.json
  jsonPath: ConnectionStrings.DefaultConnection
  status: available
  source: Environment variable
  environment variable: STAGING_DATABASE_URL
  masked value: Server=staging;Database=App;Password=****;
- workerQueue [yaml]
  file: /workspace/worker/config.yaml
  yamlPath: queue.endpoint
  status: available
  source: Environment variable
  environment variable: STAGING_WORKER_QUEUE_ENDPOINT
  masked value: https://queue.staging.example.com
- serviceEndpoint [toml]
  file: /workspace/services/development.toml
  tomlPath: services.api.endpoint
  status: available
  source: Environment variable
  environment variable: STAGING_SERVICE_ENDPOINT
  masked value: https://services.staging.example.com
- frontendApi [dotenv]
  file: /workspace/frontend/.env.local
  key: VITE_API_URL
  status: available
  source: Literal
  masked value: https://api.staging.example.com
```

Dry-run output identifies TOML context without printing replacement values:

```text
$ switchlet apply "Service Endpoint Only" --dry-run
Dry run successful for profile "Service Endpoint Only"

Planned target:
would update /workspace/services/development.toml
  serviceEndpoint [toml]
  services.api.endpoint

No changes were written.
```

## Exit Codes

- `0` means success.
- `1` means runtime or validation failure.
- `2` means command-usage failure.

## Key Bindings

The active command bar is the source of truth. In narrow terminals,
lower-priority movement or editing hints may be hidden before apply, back,
cancel, quit, or immediate-exit actions.

Main picker:

- `Up` / `Down` or `j` / `k` moves between profiles.
- `PgUp` / `PgDn` moves one visible page in long lists.
- `Home` / `End` jumps to the first or last profile.
- `Enter` applies the selected profile or opens protected confirmation.
- `i` inspects the selected profile.
- `s` opens current status for the managed configuration.
- `d` opens a read-only diff for the selected profile, and returns from diff
  loading or diff ready back to the profile list.
- `v` toggles managed value visibility in the profile list, inspection, and
  selected-profile diff.
- `r` refreshes status, diff, loading, or comparison-error screens when shown.
- `Esc` returns from secondary views where shown.
- `q` quits from the list or closes a secondary view.
- `Ctrl+C` exits immediately.

Values are hidden by default on every launch. The reveal toggle is local to the
active TUI session and is not accepted on protected confirmation, status,
recoverable errors, comparison errors, success screens, or final terminal
summaries.

Status and diff screens are read-only. They do not apply profiles, ask for
protected-profile confirmation, write target files, create target-file temporary
files, or edit `.switchlet.yaml`.

Init wizard:

- `Enter` selects, saves, validates, or creates according to the command bar.
- `Up` / `Down` or `j` / `k` moves on choice screens.
- `f`, `/`, or `m` filters or opens manual fallback only where shown.
- `Esc` goes back, returns to the labeled step, or cancels pending work where
  shown.
- `q` cancels only on non-text-entry screens.
- Text-entry screens treat `q` as literal input and use `Ctrl+C` to cancel.

## Safety

Switchlet validates planned changes before writing. Dry runs exercise the apply
path without modifying files. Command output and final interactive summaries
identify target names, files, types, and selectors without printing replacement
values.
