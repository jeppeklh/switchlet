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
2. When stdin and stdout are interactive terminals, follow the terminal-
   native setup wizard. Otherwise, use the line-oriented fallback
   prompts.
3. Choose one of the discovered JSON files, narrow large file lists by
   name or path when needed, or enter a file path manually.
4. Browse or search for an existing string-valued JSON path, or enter a
   JSON path manually.
5. Add one or more profiles.
6. Review the generated configuration summary and press Enter to create
   `.switchlet.yaml`.
7. If any profile uses a literal value, keep `.switchlet.yaml`
   protection enabled in the review step or let the fallback prompt add
   it to the project `.gitignore`.
8. Run `switchlet` for the interactive workflow, or use a non-interactive
   command.

`switchlet init` validates the file-selection step and the JSON-path step
separately, so correcting a bad JSON path does not force you to re-enter
the file choice.

The interactive init wizard and the main profile picker are designed to
work the same in a normal terminal and a Neovim terminal buffer.

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

## Neovim Terminal

Switchlet stays editor-independent. You can launch the same terminal UI
from a Neovim terminal buffer without a separate plugin:

```vim
:terminal switchlet
```

The same inspection, protected confirmation, and key bindings apply in a
shell terminal and a Neovim terminal buffer.

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
