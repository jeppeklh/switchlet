# Switchlet

Switchlet is a terminal application for safely switching named
configuration profiles in one existing JSON file.

It updates one configured existing string value through a small workflow:
configure the target once, choose a profile, apply it, and continue
working.

## Supported Scope

Switchlet currently supports:

- one existing JSON file
- one configured JSON path
- one existing string value
- literal profile values and environment-backed profile values
- interactive and non-interactive profile application

Switchlet does not create missing JSON values, manage multiple targets,
or create backup files.

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
2. Choose one of the discovered JSON files, narrow large file lists by
   name or path when needed, or enter a file path manually.
3. Browse or search for an existing string-valued JSON path, or enter a
   JSON path manually.
4. Add one or more profiles.
5. Review the generated configuration summary and press Enter to create
   `.switchlet.yaml`, or type `n` to cancel.
6. If any profile uses a literal value, let Switchlet add
   `.switchlet.yaml` to the project `.gitignore`.
7. Run `switchlet` for the interactive workflow, or use a non-interactive
   command.

`switchlet init` validates the file-selection step and the JSON-path step
separately, so correcting a bad JSON path does not force you to re-enter
the file choice.

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
output.

Protected profiles are never applied silently in non-interactive mode.
Use `--allow-protected` to opt in explicitly.

Dry-run validates the full apply path without writing the target file.
Successful dry-run output ends with `No changes were written.`

Exit behavior for non-interactive commands:

- `0` success
- `1` runtime or validation failure
- `2` command-usage failure

## Example Configuration

```yaml
version: 2

target:
  file: config/development.json
  jsonPath: database.primary.url

profiles:
  - name: Local
    value: postgres://localhost:5432/myapp

  - name: Test
    valueFromEnv: MYAPP_TEST_DATABASE_URL

  - name: Production
    valueFromEnv: MYAPP_PRODUCTION_DATABASE_URL
    protected: true
```

Version `1` ASP.NET `target.connectionName` configurations are still
supported for existing projects.

## Example Output

Short `--json` examples:

```json
{"profiles":[{"name":"Local","protected":false,"available":true,"source":"literal","environmentVariableName":"","maskedValue":"postgres://localhost:5432/myapp","unavailableReason":""}]}
{"profile":{"name":"Production","protected":true,"available":true,"source":"environment","environmentVariableName":"MYAPP_PRODUCTION_DATABASE_URL","maskedValue":"Server=prod;Password=****;","unavailableReason":""}}
{"result":{"profileName":"Production","targetPath":"database.primary.url","targetFile":"/path/to/config/development.json","protected":true,"dryRun":true}}
```

## Verification

```bash
gofmt -w .
go test ./...
go vet ./...
```
