# Switchlet

Switchlet is a small terminal application for safely switching named
configuration profiles.

Version 0.1 supports one concrete workflow:

- discover `.switchlet.yaml`
- inspect configured profiles
- apply one selected profile to an ASP.NET `appsettings.Development.json`
  connection string
- exit immediately so development can continue

Authoritative project documentation lives under `docs/`.

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

1. Create `.switchlet.yaml` in your project.
2. Point `target.file` at the existing
   `appsettings.Development.json` file.
3. Configure one or more profiles.
4. Run `switchlet` from the project root or any nested directory.
   If you used the local build instead of `go install`, run `./switchlet`
   from the directory where you built it.
5. Inspect a profile if needed, then apply it.

## Configuration Example

```yaml
version: 1

target:
  file: src/MyApplication/appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;Trusted_Connection=True;"

  - name: Test
    valueFromEnv: MYAPPLICATION_TEST_CONNECTION_STRING

  - name: Production
    valueFromEnv: MYAPPLICATION_PRODUCTION_CONNECTION_STRING
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
switchlet
```

Switchlet does not require a dedicated Neovim plugin.

## Security Guidance

- Prefer environment variables for sensitive connection strings.
- Inspection masks `Password` and `Pwd` values case-insensitively.
- Masking is best-effort and intended for display safety, not provider-
  specific parsing.
- Switchlet never intentionally writes secrets to errors.

## JSON Formatting Behavior

- Switchlet updates only `ConnectionStrings.<connectionName>`.
- Unrelated JSON values remain semantically unchanged.
- Version 0.1 may normalize indentation and whitespace when writing.
- Target files are written through a same-directory temporary file and a
  safe replacement step.

## Current Limitations

Version 0.1 supports only:

- one target file
- one configured connection string
- ASP.NET `appsettings.Development.json`
- literal and environment-backed values

Version 0.1 does not support:

- profile editing
- profile creation
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

Before tagging Version 0.1:

1. Run all development commands successfully.
2. Verify the terminal workflow manually: list, inspection, protected
   confirmation, success exit, and recoverable errors.
3. Verify the Neovim terminal workflow.
4. Confirm documentation still matches the implementation.
5. Perform platform verification for Linux, macOS, and Windows.

## Roadmap

Planned future work is documented in `docs/product/ROADMAP.md`.
