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
| `/` | Search and filter profiles by name or safe target metadata. |
| `n` / `N` | Move to the next or previous match when a filter is active. |
| `Esc` | Clear an active profile filter. |
| `Enter` | Apply the selected profile and exit, or open protected confirmation. |
| `Space` | Apply the selected profile and return to the list, or open protected confirmation. |
| `i` | Inspect the selected profile. |
| `c` | Open the configuration editor. |
| `s` | Open current status for managed configuration. |
| `d` | Open managed patch preview for the selected profile. |
| `r` | Refresh profile availability and current managed state. |
| `v` | Toggle managed value visibility where reveal is supported. |
| `?` | Open contextual help for the active main-picker screen. |
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
| Inspection | `PgUp` / `PgDn` | Scroll long profile details. |
| Inspection | `Home` / `End` | Jump to the top or bottom of long profile details. |
| Status | `Esc` / `s` | Return to the profile list. |
| Status | `r` | Refresh status. |
| Status | `PgUp` / `PgDn` | Scroll long status results. |
| Status | `Home` / `End` | Jump to the top or bottom of long status results. |
| Diff | `Esc` / `d` | Return to the profile list. |
| Diff | `Enter` | Apply the diffed profile and exit, or open protected confirmation. |
| Diff | `Space` | Apply the diffed profile and return to the list, or open protected confirmation. |
| Diff | `r` | Refresh diff. |
| Diff | `v` | Toggle managed value visibility. |
| Diff | `PgUp` / `PgDn` | Scroll long diff previews. |
| Diff | `Home` / `End` | Jump to the top or bottom of long diff previews. |
| Comparison error | `Esc` | Return to the profile list. |
| Comparison error | `r` | Retry the failed status or diff request. |
| Comparison error | `PgUp` / `PgDn` | Scroll long error details. |
| Comparison error | `Home` / `End` | Jump to the top or bottom of long error details. |
| Recoverable error | Other non-quit key | Return to the profile list. |
| Recoverable error | `PgUp` / `PgDn` | Scroll long error details when available. |
| Recoverable error | `Home` / `End` | Jump to the top or bottom of long error details when available. |
| Protected confirmation | `Enter` / `y` | Confirm apply. |
| Protected confirmation | `Esc` / `n` | Cancel and return to the profile list. |
| Protected confirmation | `q` | Quit without applying changes. |
| Any main view | `q` | Quit. |
| Any main view | `Ctrl+C` | Exit immediately. |

Values are hidden by default on every launch. The reveal toggle is session-local
and is not accepted on protected confirmation, status, recoverable errors,
comparison errors, success screens, or final terminal summaries.

Status screens are read-only. The diff screen remains read-only until `Enter` or
`Space` is pressed. Opening status or diff never applies a profile, writes target
files, creates target-file temporary files, or edits `.switchlet.yaml`.

## Init Wizard

The init wizard changes its command bar by step.

| Key | Action |
|---|---|
| `Enter` | Select, continue, validate, or create according to the active command bar. |
| `Up` / `Down` | Move on choice screens. |
| `j` / `k` | Move on choice screens. |
| `f` / `/` | Filter file lists where shown. |
| `s` / `/` | Search structured JSON, YAML, or TOML selectors where shown. |
| `/` | Search dotenv keys from the value list. |
| `Space` | Mark or unmark a setting in a value list. |
| `Tab` | Mark or unmark the highlighted setting while typing a value search; spaces remain search text. |
| `Ctrl+R` | Review all marked settings, including those hidden by the current search. |
| `m` | Open manual file, selector, or key entry where shown. |
| `Left` / `Right` | Move the cursor on text-entry screens. |
| `Home` / `End` | Jump within text-entry fields or choice lists where shown. |
| `Backspace` / `Delete` | Edit text-entry fields. |
| `Esc` | Go back, return to the labeled step, or cancel pending work where shown. |
| `q` | Cancel on non-text-entry screens. |
| `?` | Open contextual help on non-text-entry screens. |
| `Ctrl+C` | Cancel immediately. |

On text-entry screens, `q` is literal input rather than cancel. Use `Esc` or
`Ctrl+C` according to the active command bar.

When init asks whether to replace an existing current-directory
`.switchlet.yaml`, `y` chooses replacement and `n` keeps the existing file.

The managed-values checkpoint offers **Capture current values** to start a
literal profile (`C Capture`). The profile checkpoint offers **Copy <name>** for each existing
profile, alongside fresh profile creation; `D Copy last` copies the most recently
added profile. Capture/copy review uses `e` to edit
the selected value, `Space` to include/omit it, `p` to toggle protection, and
`Enter` to add the profile draft. `Esc` returns to its name without discarding
the captured/copied values. Copied environment references remain references.

### Batch Managed-Value Review (Init And Config Editor)

When adding managed values, mark settings from one file, then press `Ctrl+R`.
Marks survive filtering and navigation within that file. Changing files or
formats starts a new selection. Single-setting `Enter` and manual entry remain
available.

| Key | Action |
|---|---|
| `Up` / `Down`, `j` / `k` | Select a review row. |
| `Home` / `End`, `g` / `G` | Select the first/last row. |
| `r` | Rename the selected managed value; selector paths are initial name suggestions. |
| `t` | Cycle the selected dotenv value's type. Structured-file types are detected. |
| `Enter` | Validate and add the whole batch to the draft. |
| `Esc` | Return to selections, retaining marks, names, types, and filter context. |

A rejected batch changes no draft targets. Correct names/types in review or
return to selection to unmark settings. Saving configuration remains explicit.
In config-editor selector lists, `Space` marks settings and `/` opens filtering;
finish filtering with `Enter` before marking more settings.

## Configuration Editor

### Overview

| Key | Action |
|---|---|
| `h` / `l` | Move between `Profiles`, `Managed values`, and `Review` tabs. |
| `Left` / `Right` | Move between tabs. |
| `j` / `k` | Move through rows. |
| `Up` / `Down` | Move through rows. |
| `g` / `G` | Jump to the first or last row. |
| `Home` / `End` | Jump to the first or last row. |
| `/` | Filter profiles or managed values by name or safe managed-value metadata. |
| `n` / `N` | Move between filtered matches when a filter is active. |
| `Esc` / `h` | Clear an active filter or go back where shown. |
| `a` | Add a profile or managed value for the selected section. |
| `C` | Capture current managed values into a new profile from the `Profiles` tab. |
| `e` | Edit the selected profile or managed-value location. |
| `D` | Duplicate the selected profile. |
| `r` | Rename the selected profile or managed value. |
| `d` | Delete the selected profile or managed value. |
| `Space` | Toggle protection for the selected profile. |
| `s` | Save from the `Review` tab when the draft is saveable. |
| `PgUp` / `PgDn` | Scroll long pending-change details on the `Review` tab. |
| `Home` / `End` | Jump to the top or bottom of long pending-change details on the `Review` tab. |
| `c` | Return to the profile picker when the editor was opened from the picker. |
| `?` | Open contextual help on non-text-entry editor screens. |
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
