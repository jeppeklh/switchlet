# Switchlet

Switchlet is a small terminal tool for safely switching named project
configuration profiles. It changes only explicitly configured JSON, YAML, TOML,
or dotenv values, and it keeps the main workflow focused on choosing a profile,
reviewing what will change, and applying it without editing files by hand.

Switchlet is editor-independent. Run it from a terminal, a terminal pane, or a
Neovim terminal buffer inside a project that contains `.switchlet.yaml`.

## Supported Workflows

- Launch the interactive profile picker with `switchlet`.
- Create project configuration with `switchlet init`.
- Use `list`, `inspect`, `apply`, `status`, and `diff` for non-interactive work.
- Manage configured values in JSON, YAML, TOML, and dotenv files.
- Validate changes with dry-run mode before writing files.
- Use `--json` output for scripts and automation.

## Install

Install with Go:

```bash
go install github.com/jeppeklh/switchlet/cmd/switchlet@latest
```

When release binaries are available, download the archive for your platform from
GitHub Releases and place the `switchlet` binary on your `PATH`.

Local build:

```bash
go build -o switchlet ./cmd/switchlet
```

## Quick Start

Create a `.switchlet.yaml` in your project root, or run `switchlet init` for the
guided setup flow. Target files and selected values must already exist before
Switchlet writes to them.

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

Use JSON output where supported:

```bash
switchlet list --json
switchlet apply Local --dry-run --json
```

## Safety Model

- Only explicitly configured values are changed.
- Target files are validated before writes.
- Writes use safe replacement and preserve file permissions.
- Protected profiles require explicit approval before apply.
- Normal command output avoids raw replacement values.

## Documentation

- [COMMANDS.md](COMMANDS.md) is the detailed command reference.

## License

Switchlet is released under the [MIT License](LICENSE).
