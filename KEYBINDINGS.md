# Switchlet Key Bindings

Interactive key reference for Switchlet's terminal UI.

The active command bar is the source of truth. In narrow terminals, lower
priority hints may be hidden before primary, back, cancel, or quit actions.

## Main Picker

### Profile List

| Key | Action |
|---|---|
| `Up` / `Down` | Move between profiles. |
| `j` / `k` | Move between profiles. |
| `PgUp` / `PgDn` | Move one visible page in long profile lists. |
| `Home` / `End` | Jump to the first or last profile. |
| `/` | Search and filter profiles by name. |
| `n` / `N` | Move to the next or previous match when a filter is active. |
| `Esc` | Clear an active profile filter. |
| `Enter` | Apply the selected profile and exit, or open protected confirmation. |
| `Space` | Apply the selected profile and return to the list, or open protected confirmation. |
| `i` | Inspect the selected profile. |
| `c` | Open the configuration editor. |
| `s` | Open current status for managed configuration. |
| `d` | Open read-only diff for the selected profile. |
| `v` | Toggle managed value visibility where reveal is supported. |
| `q` | Quit without applying changes. |
| `Ctrl+C` | Exit immediately. |

### Profile Search

| Key | Action |
|---|---|
| `Enter` | Apply the typed filter and return to the profile list. |
| `Esc` | Cancel search input and keep the previous active filter. |
| `Left` / `Right` | Move within the search field. |
| `Home` / `End` | Jump within the search field. |
| `Backspace` / `Delete` | Edit the search field. |

While search input is focused, regular characters are inserted into the search
field. `q` is literal input instead of quit.

### Secondary Views

| View | Key | Action |
|---|---|---|
| Inspection | `Esc` / `i` | Return to the profile list. |
| Inspection | `Enter` | Apply the selected profile and exit. |
| Inspection | `Space` | Apply the selected profile and return to the list. |
| Inspection | `v` | Toggle managed value visibility. |
| Status | `Esc` / `s` | Return to the profile list. |
| Status | `r` | Refresh status. |
| Diff | `Esc` / `d` | Return to the profile list. |
| Diff | `r` | Refresh diff. |
| Diff | `v` | Toggle managed value visibility. |
| Comparison error | `Esc` | Return to the profile list. |
| Comparison error | `r` | Retry the failed status or diff request. |
| Recoverable error | `Any non-quit key` | Return to the profile list. |
| Protected confirmation | `Enter` / `y` | Confirm apply. |
| Protected confirmation | `Esc` / `n` | Cancel and return to the profile list. |
| Protected confirmation | `q` | Quit without applying changes. |
| Any main view | `q` | Quit. |
| Any main view | `Ctrl+C` | Exit immediately. |

Values are hidden by default on every launch. The reveal toggle is session-local
and is not accepted on protected confirmation, status, recoverable errors,
comparison errors, success screens, or final terminal summaries.

Status and diff screens are read-only. They do not apply profiles, write target
files, create target-file temporary files, or edit `.switchlet.yaml`.

## Init Wizard

The init wizard changes its command bar by step.

| Key | Action |
|---|---|
| `Enter` | Select, continue, validate, or create according to the active command bar. |
| `Up` / `Down` | Move on choice screens. |
| `j` / `k` | Move on choice screens. |
| `f` / `/` | Filter file lists where shown. |
| `s` / `/` | Search structured JSON, YAML, or TOML selectors where shown. |
| `m` | Open manual file, selector, or key entry where shown. |
| `Left` / `Right` | Move the cursor on text-entry screens. |
| `Home` / `End` | Jump within text-entry fields or choice lists where shown. |
| `Backspace` / `Delete` | Edit text-entry fields. |
| `Esc` | Go back, return to the labeled step, or cancel pending work where shown. |
| `q` | Cancel on non-text-entry screens. |
| `Ctrl+C` | Cancel immediately. |

On text-entry screens, `q` is literal input rather than cancel. Use `Esc` or
`Ctrl+C` according to the active command bar.

When init asks whether to replace an existing current-directory
`.switchlet.yaml`, `y` chooses replacement and `n` keeps the existing file.

## Configuration Editor

### Overview

| Key | Action |
|---|---|
| `h` / `l` | Move between `Profiles`, `Targets`, and `Review` tabs. |
| `Left` / `Right` | Move between tabs. |
| `j` / `k` | Move through rows. |
| `Up` / `Down` | Move through rows. |
| `g` / `G` | Jump to the first or last row. |
| `Home` / `End` | Jump to the first or last row. |
| `/` | Filter profiles or targets. |
| `n` / `N` | Move between filtered matches when a filter is active. |
| `Esc` / `h` | Clear an active filter or go back where shown. |
| `a` | Add a profile or target for the selected section. |
| `e` | Edit the selected profile or target location. |
| `r` | Rename the selected profile or target. |
| `d` | Delete the selected profile or target. |
| `Space` | Toggle protection for the selected profile. |
| `s` | Save from the `Review` tab when the draft is saveable. |
| `c` | Return to the profile picker when the editor was opened from the picker. |
| `q` | Quit, with dirty-draft confirmation when needed. |
| `Ctrl+C` | Exit immediately. |

### Text Entry

| Key | Action |
|---|---|
| `Left` / `Ctrl+B` | Move cursor left. |
| `Right` / `Ctrl+F` | Move cursor right. |
| `Home` / `Ctrl+A` | Move to the start of the field. |
| `End` / `Ctrl+E` | Move to the end of the field. |
| `Backspace` | Delete before the cursor. |
| `Delete` | Delete at the cursor. |
| `Ctrl+U` | Clear the field. |
| `Ctrl+K` | Delete from the cursor to the end of the field. |
| `Esc` | Leave the current text-entry screen where shown. |

On text-entry screens, regular characters are inserted into the field.

### Save And Quit Confirmation

| Key | Action |
|---|---|
| `Enter` / `y` | Confirm discard when quitting with unsaved changes. |
| `Esc` / `n` / `h` | Return to the editor from discard confirmation. |
| `Enter` / `q` | Exit after a successful save. |
