# Switchlet

Switchlet is a small terminal tool for switching project configuration
profiles safely.

It changes only the JSON, YAML, TOML, or dotenv values you explicitly choose, then
exits so you can continue your normal development workflow.

## Install

Install with Go:

```bash
go install github.com/jeppeklh/switchlet/cmd/switchlet@latest
```

Or download a release binary for your platform and place it on your `PATH`.

Local build:

```bash
go build -o switchlet ./cmd/switchlet
```

## First Run

From your project root:

```bash
switchlet init
```

The setup wizard creates `.switchlet.yaml` by asking which existing JSON, YAML,
TOML, or dotenv values Switchlet may manage and which profiles should be
available.

If `.switchlet.yaml` already exists in the project root, interactive init asks
before replacing it. Use `switchlet init --overwrite` to replace it without that
initial prompt.

Then run:

```bash
switchlet
```

Choose a profile, apply it, and continue working.

## Commands

See [COMMANDS.md](COMMANDS.md) for command examples, JSON output, exit codes,
and key bindings.

You can also run:

```bash
switchlet help
switchlet help apply
```
