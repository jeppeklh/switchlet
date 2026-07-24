# Switchlet

Switchlet is a terminal application for safely switching named
configuration profiles in an existing JSON file.

It updates one configured existing string value through a small terminal
workflow: select a profile, apply it, and continue working.

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

## Setup

1. Run `switchlet init` from your project root.
2. Enter the path to the existing JSON file you want to update.
3. Enter the JSON path to the existing string value you want Switchlet to
   manage.
4. Add one or more profiles.
5. Run `switchlet`.
6. Select a profile and apply it.

Example configuration:

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
