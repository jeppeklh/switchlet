# Switchlet

Switchlet is a small terminal application for safely switching named
configuration profiles.

The current implementation supports one concrete target workflow:

- create `.switchlet.yaml` with `switchlet init`
- discover `.switchlet.yaml`
- inspect configured profiles
- apply one selected profile to one configured existing JSON string value
- exit immediately so development can continue

Authoritative project documentation lives under `docs/`.

## Status

- Version `v0.1.0` remains supported through backward-compatible
  configuration loading.
- Version `0.2` development is in progress.
- Version `0.2` focuses on generic JSON target support while preserving
  the existing launch, select, apply, and exit workflow.

## Installation

Install the binary into your Go bin directory:

```bash
go install ./cmd/switchlet
```

If `GOBIN` or `$(go env GOPATH)/bin` is on your `PATH`, the executable is
available as `switchlet`.

Local build alternative:

```bash
go build -o switchlet ./cmd/switchlet
```

This creates a local `./switchlet` binary in the repository root.

## Quick Start

1. Run `switchlet init` from your project root.
2. Enter the existing JSON file path.
3. Enter the JSON path to the existing string value you want Switchlet to
   update.
4. Enter one or more profiles.
5. Run `switchlet` from the project root or any nested directory.
   If you used the local build instead of `go install`, run `./switchlet`
   from the directory where you built it.
6. Inspect a profile if needed, then apply it.

Manual `.switchlet.yaml` authoring is still supported when you prefer to
manage the file yourself.

## Configuration Example

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

Configuration rules and validation details:

- `docs/configuration/CONFIGURATION.md`
- `docs/configuration/EXAMPLES.md`

## Literal Values And Environment Variables

- Use `value` for non-sensitive literal connection strings.
- Use `valueFromEnv` for secrets or environment-specific values.
- Missing or empty environment variables make only that profile
  unavailable.
- Unavailable profiles remain visible in the UI and explain why they
  cannot be applied.

## Protected Profiles

- `protected: true` requires explicit confirmation before applying the
  profile.
- Confirmation never shows the full connection string.
- Protection is opt-in and based only on configuration, not profile
  names.

## Key Bindings

From the profile list:

- `↑` / `k`: move up
- `↓` / `j`: move down
- `Enter`: apply selected profile
- `i`: inspect selected profile
- `q`: quit
- `Ctrl+C`: immediate quit

From inspection:

- `Enter`: apply selected profile
- `i`, `Esc`, `q`: return to the list
- `Ctrl+C`: immediate quit

From protected confirmation:

- `y`: confirm
- `n`, `Esc`, `q`: cancel and return to the list
- `Ctrl+C`: immediate quit

## Neovim Launch Example

Open a terminal inside Neovim and run:

```bash
switchlet init
switchlet
```

Switchlet does not require a dedicated Neovim plugin.

## Security Guidance

- Prefer environment variables for sensitive connection strings.
- Inspection masks `Password` and `Pwd` values case-insensitively inside
  connection-string-like values.
- Masking is best-effort and intended for display safety, not provider-
  specific parsing.
- Switchlet never intentionally writes secrets to errors.

## JSON Formatting Behavior

- Switchlet updates only the configured JSON path.
- Unrelated JSON values remain semantically unchanged.
- Version 0.2 may normalize indentation and whitespace when writing.
- Target files are written through a same-directory temporary file and a
  safe replacement step.

## Current Limitations

Version 0.2 currently supports only:

- one existing JSON file
- one configured JSON path
- one existing string value
- one target file
- literal and environment-backed values
- backward-compatible loading of Version `1` `connectionName`
  configurations when they can be mapped to a JSON path

Version 0.2 does not support:

- profile editing
- multiple simultaneous target updates
- additional configuration formats
- secret-manager integrations

## Development Commands

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go mod tidy
go mod verify
```

## Release Checklist

Before tagging `v0.1.0`:

1. Run all development commands successfully.
2. Verify the terminal workflow manually: list, inspection, protected
   confirmation, success exit, and recoverable errors.
3. Verify the Neovim terminal workflow.
4. Confirm documentation still matches the implementation.
5. Perform platform verification for Linux, macOS, and Windows.

## Roadmap

Planned future work is documented in `docs/product/ROADMAP.md`.
