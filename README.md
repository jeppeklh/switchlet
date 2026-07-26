# Switchlet

Switchlet is a terminal application for safely switching named
configuration profiles across explicitly configured targets.

It updates existing JSON or dotenv values through a small workflow:
choose the values Switchlet may manage, create profiles, apply one, and
continue working.

## Supported Scope

Switchlet currently supports:

- Version `3` configurations with named targets
- JSON targets selected by `jsonPath`
- dotenv targets selected by `key`
- one or more targets across one or more files
- profiles that update one or more targets
- partial profiles that leave omitted targets unchanged
- literal profile values and environment-backed profile values
- interactive and non-interactive profile application
- compatibility loading for Version `1` and Version `2` configurations

Switchlet does not create missing JSON values or dotenv keys, manage
YAML/TOML/XML files, create backup files, run applications, or manage
secrets.

## Installation

Supported install paths:

1. Install with Go:

   ```bash
   go install github.com/jeppeklh/switchlet/cmd/switchlet@latest
   ```

2. Download the matching release binary for your platform from the
   repository releases, place it on your `PATH`, and run `switchlet`.

   Current binary names follow this pattern:

   - `switchlet_linux_amd64`
   - `switchlet_linux_arm64`
   - `switchlet_darwin_amd64`
   - `switchlet_darwin_arm64`
   - `switchlet_windows_amd64.exe`
   - `switchlet_windows_arm64.exe`

If `GOBIN` or `$(go env GOPATH)/bin` is on your `PATH`, the Go install
command makes the executable available as `switchlet`.

Local build alternative:

```bash
go build -o switchlet ./cmd/switchlet
```

## Quick Start

1. Run `switchlet init` from your project root.
2. When stdin and stdout are interactive terminals, follow the terminal-
   native setup wizard. Otherwise, use the line-oriented fallback
   prompts.
3. Choose a supported configuration file. JSON and dotenv files are shown
   as peer options, and you can filter or enter a path manually.
4. Choose one existing value inside that file: a string-valued JSON path or
   a dotenv key that appears once.
5. Name that managed value so profiles can refer to it. Add another managed
   value only when the project needs one.
6. Add one or more profiles. For one managed value, the wizard uses the
   short path `Profile name -> Profile value`; environment-backed values
   and protected profiles are optional profile-value settings.
7. Review the managed values, profile scopes, and `.gitignore` protection,
   then press Enter to create `.switchlet.yaml`.
8. If any profile uses a literal value, keep `.switchlet.yaml`
   protection enabled in the review step or let the fallback prompt add
   it to the project `.gitignore`.
9. Run `switchlet` for the interactive workflow, or use a non-interactive
   command.

`switchlet init` validates file selection, target selector selection, and
the generated configuration before reporting success.

The interactive init wizard and the main profile picker run as a
full-screen terminal UI.

## Commands

Interactive workflow:

```bash
switchlet
```

Non-interactive workflow:

```bash
switchlet list
switchlet inspect Local
switchlet apply Local
switchlet apply Production --dry-run --allow-protected
```

Use `--json` on `list`, `inspect`, and `apply` for machine-readable
target-aware output. JSON output includes profile names, target names,
files, selectors, availability, and safe errors. It does not include
unmasked resolved secret values.

Protected profiles in the interactive TUI are confirmed in place. When a
protected profile is selected, Enter changes from `Apply` to `Continue`,
opens a confirmation screen, Enter or `y` confirms, and `n`, `Esc`, or
`q` cancels.

Protected profiles are never applied silently in non-interactive mode.
Use `--allow-protected` to opt in explicitly for non-interactive
`switchlet apply`.

Dry-run validates the full apply path without writing the target file.
Successful dry-run output ends with `No changes were written.`

Exit behavior for non-interactive commands:

- `0` success
- `1` runtime or validation failure
- `2` command-usage failure

## Key Bindings

- `↑/↓` or `j/k` move between profiles
- `Enter` applies the selected profile or continues into protected confirmation
- `i` inspects the selected profile
- `q` quits from the list or closes a secondary view
- `Ctrl+C` exits immediately from every view

## Example Configuration

```yaml
version: 3

targets:
  - name: database
    file: backend/appsettings.Development.json
    type: json
    jsonPath: ConnectionStrings.DefaultConnection

  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

profiles:
  - name: Local
    values:
      - target: database
        value: Server=localhost;Database=App;Trusted_Connection=True;
      - target: frontendApi
        value: http://localhost:5173

  - name: Staging
    protected: true
    values:
      - target: database
        valueFromEnv: STAGING_DATABASE_URL
      - target: frontendApi
        value: https://api.staging.example.com

  - name: Local Database Only
    values:
      - target: database
        value: Server=localhost;Database=App;Trusted_Connection=True;
```

`Local Database Only` is a partial profile. Applying it updates only the
`database` target and leaves `frontendApi` unchanged.

See `docs/configuration/CONFIGURATION.md` for the complete configuration
schema and validation rules.

Version `1` ASP.NET `target.connectionName` configurations and Version
`2` `target.jsonPath` configurations are still supported for existing
projects where they can be normalized safely.
