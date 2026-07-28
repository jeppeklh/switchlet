# Switchlet

Switchlet is a small terminal tool for safely switching named project
configuration profiles. It changes only explicitly configured JSON, YAML, TOML,
or dotenv values.

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
