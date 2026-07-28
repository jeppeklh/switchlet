# Switchlet

[![CI](https://github.com/jeppeklh/switchlet/actions/workflows/ci.yml/badge.svg)](https://github.com/jeppeklh/switchlet/actions/workflows/ci.yml) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Switchlet is a small terminal tool for safely switching named project
configuration profiles. It updates only explicitly configured JSON, YAML, TOML,
or dotenv values, so local development configuration can be changed repeatably
without opening files by hand.

Run it directly in your terminal, terminal pane, or Neovim terminal buffer from
anywhere inside a configured project.

## What It Does

- Opens an interactive profile picker with `switchlet`.
- Creates project configuration with `switchlet init`.
- Lists, inspects, applies, checks status, and compares profiles from the CLI.
- Supports JSON, YAML, TOML, and dotenv managed values.
- Provides dry-run mode and JSON output for automation.
- Keeps normal output free of raw replacement values.

## Install

Install with Go:

```bash
go install github.com/jeppeklh/switchlet/cmd/switchlet@latest
```

Tagged releases publish binaries for Linux, macOS, and Windows. Download the
binary for your platform from [GitHub Releases](https://github.com/jeppeklh/switchlet/releases)
and place the `switchlet` binary on your `PATH`.

Local build:

```bash
go build -o switchlet ./cmd/switchlet
```

## Quick Start

Start with the guided setup flow:

```bash
switchlet init
```

The wizard creates `.switchlet.yaml` after you select existing target files,
selectors, and profiles. Target files and selected values must already exist
before Switchlet writes to them.

List and inspect profiles:

```bash
switchlet list
switchlet inspect Local
```

Validate and apply a profile:

```bash
switchlet apply Local --dry-run
switchlet apply Local
```

Check current managed state and compare a profile without writing files:

```bash
switchlet status
switchlet diff Local
```

Use JSON output for automation:

```bash
switchlet list --json
switchlet apply Local --dry-run --json
```

## Example Configuration

`switchlet init` automatically creates `.switchlet.yaml`. This example shows the
Version `3` configuration shape.

```yaml
version: 3

targets:
  - name: database
    file: appsettings.Development.json
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
```

## Safety Model

- Only explicitly configured values are changed.
- Target files are validated before writes.
- Writes use safe replacement and preserve file permissions.
- Protected profiles require explicit approval before apply.
- Normal command output avoids raw replacement values.

## Commands

See [COMMANDS.md](COMMANDS.md) for the full command reference, including
interactive keys, JSON output, status, diff, and managed patch output.

## License

Switchlet is released under the [MIT License](LICENSE).
