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

`switchlet init` launches the terminal setup wizard when stdin and stdout are
interactive terminals. Otherwise, it uses line-oriented prompts.

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

## JSON Output

Use `--json` with `list`, `inspect`, or `apply`:

```bash
switchlet list --json
switchlet inspect Local --json
switchlet apply Local --dry-run --json
```

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
- `Esc` returns from secondary views where shown.
- `q` quits from the list or closes a secondary view.
- `Ctrl+C` exits immediately.

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
