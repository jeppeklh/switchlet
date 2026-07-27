# Switchlet

Switchlet is a small terminal tool for switching project configuration
profiles safely.

It changes only the JSON, YAML, or dotenv values you explicitly choose, then
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
or dotenv values Switchlet may manage and which profiles should be available.

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
