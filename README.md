<br />

<h1 align="center">Switchlet</h1>

<p align="center">
  <strong>A terminal profile switcher for repository-local configuration.</strong>
  <br />
  <br />
  <a href="LICENSE"><img alt="License MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/jeppeklh/switchlet/releases/latest"><img alt="Latest release" src="https://img.shields.io/github/v/release/jeppeklh/switchlet.svg"></a>
</p>

<p align="center">
  <img alt="Switchlet demo" src="assets/switchlet.gif">
</p>

Switchlet applies named profiles to explicitly configured JSON, YAML, TOML, and
dotenv values. It validates before writing and avoids printing raw values in
normal output.

## Installation

### Go

```bash
go install github.com/jeppeklh/switchlet/cmd/switchlet@latest
```

Make sure your Go binary directory, usually `$GOBIN` or `$GOPATH/bin`, is on
your `PATH`.

### GitHub Releases

Download a prebuilt binary for Linux, macOS, or Windows from
[GitHub Releases](https://github.com/jeppeklh/switchlet/releases), then place the
`switchlet` binary somewhere on your `PATH`.

### From Source

```bash
git clone https://github.com/jeppeklh/switchlet.git
cd switchlet
make build
```

This creates `./switchlet` from `./cmd/switchlet`.

## Quick Start

Create project configuration:

```bash
switchlet init
```

Open the interactive profile picker:

```bash
switchlet
```

Common CLI workflows:

```bash
switchlet list
switchlet targets
switchlet apply Local --dry-run
switchlet apply Staging --expect-current Local
switchlet apply Local
switchlet status
switchlet doctor --strict
switchlet diff Local
```

Edit an existing `.switchlet.yaml`:

```bash
switchlet config
```

## What Switchlet Manages

- Named profiles with one or more target values.
- JSON, YAML, TOML, and dotenv configuration targets.
- Partial profiles that update only included targets.
- Literal values, environment-backed values, and protected profiles.
- Strings, booleans, integers, and exact decimal numbers through configuration Version 4.

## Safety Model

- Only explicitly configured targets are modified.
- Target files and selectors are validated before writes.
- File replacements preserve permissions and use same-directory temporary files.
- Protected profiles require confirmation, or `--allow-protected` in
  non-interactive use.
- Non-interactive apply can assert the current complete profile with
  `--expect-current` before any writes begin.
- Normal text and JSON output avoid raw current and replacement values.

## Undo

Enable private, single-level undo recording before applying a profile:

```bash
switchlet undo --enable
switchlet apply Staging
switchlet undo --dry-run
switchlet undo
```

Undo restores exact previous file contents and refuses external changes.
Snapshots can contain secrets and are stored privately without encryption.
See [undo, recovery, and storage](COMMANDS.md#undo-last-apply) for details.

## Typed Settings

Use configuration `version: 4` and declare `valueType: boolean`, `integer`, or
`number` on a target to manage values such as `false`, `8080`, and `0.75`.
Existing configurations keep their string behavior. Init detects native scalar
types in structured files; `switchlet config` validates typed profile entry and
reviews schema conversions before saving.

See [typed values](COMMANDS.md#typed-values-configuration-version-4) for the full
configuration example, numeric limits, environment parsing, and dotenv behavior.

## Reference

- [Command reference](COMMANDS.md)
- [Key bindings](KEYBINDINGS.md)

## License

Switchlet is released under the [MIT License](LICENSE).
