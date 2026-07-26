# Switchlet Commands

This is the user-facing command reference for Switchlet.

## Interactive Commands

Start the profile picker:

```bash
switchlet
```

Create project configuration:

```bash
switchlet init
```

`switchlet init` launches the terminal setup wizard when stdin and stdout are
interactive terminals. Otherwise, it uses line-oriented prompts.

## Profile Commands

List profiles:

```bash
switchlet list
```

Inspect a profile before applying it:

```bash
switchlet inspect Local
```

Apply a profile:

```bash
switchlet apply Local
```

Validate the apply path without writing files:

```bash
switchlet apply Local --dry-run
```

Apply a protected profile non-interactively:

```bash
switchlet apply Production --allow-protected
```

Protected profiles are never applied silently. Interactive use asks for
confirmation. Non-interactive apply requires `--allow-protected`.

## JSON Output

Use `--json` with `list`, `inspect`, or `apply`:

```bash
switchlet list --json
switchlet inspect Local --json
switchlet apply Local --dry-run --json
```

JSON output includes profile names, target names, files, selectors,
availability, and safe errors. It does not include unmasked resolved secret
values.

## Exit Codes

- `0` means success.
- `1` means runtime or validation failure.
- `2` means command-usage failure.

## Key Bindings

The active command bar is the source of truth. In narrow terminals,
lower-priority movement or editing hints may be hidden before apply, back,
cancel, quit, or immediate-exit actions.

Main picker:

- `Up` / `Down` or `j` / `k` moves between profiles.
- `PgUp` / `PgDn` moves one visible page in long lists.
- `Home` / `End` jumps to the first or last profile.
- `Enter` applies the selected profile or opens protected confirmation.
- `i` inspects the selected profile.
- `Esc` returns from secondary views where shown.
- `q` quits from the list or closes a secondary view.
- `Ctrl+C` exits immediately.

Init wizard:

- `Enter` selects, saves, validates, or creates according to the command bar.
- `Up` / `Down` or `j` / `k` moves on choice screens.
- `f`, `/`, or `m` filters or opens manual fallback only where shown.
- `Esc` goes back, returns to the labeled step, or cancels pending work where
  shown.
- `q` cancels only on non-text-entry screens.
- Text-entry screens treat `q` as literal input and use `Ctrl+C` to cancel.

## Safety

Switchlet validates planned changes before writing. Dry runs exercise the apply
path without modifying files. Command output and final interactive summaries
avoid printing replacement values.
