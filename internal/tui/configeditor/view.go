package configeditor

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

// View renders the current config editor state.
func (model Model) View() string {
	if model.isTerminalTooSmall() {
		return ui.RenderShell(ui.Shell{
			Title:    "Switchlet config",
			Subtitle: "Terminal too small.",
			Panels: []ui.Panel{{Title: "Resize required", Lines: []string{
				fmt.Sprintf("Minimum size: %dx%d", configEditorMinimumTerminalWidth, configEditorMinimumTerminalHeight),
				fmt.Sprintf("Current size: %dx%d", model.width, model.height),
				"Resize the terminal to continue.",
			}}},
			Actions: []ui.Action{{Key: "q", Label: "Quit"}, {Key: "Ctrl+C", Label: "Cancel immediately"}},
			Width:   model.width,
			Height:  model.height,
		})
	}

	switch model.state {
	case editorStateDirtyQuitConfirm:
		return model.dirtyQuitView()
	case editorStateSaving:
		return model.savingView()
	case editorStateSaveSuccess:
		return model.saveSuccessView()
	default:
		return model.overviewView()
	}
}

func (model Model) overviewView() string {
	overview := model.overview()
	rows := model.navigationRows(overview)
	model.clampCursor(len(rows))
	selectedRow := model.selectedRow(rows)

	return model.configEditorShell("Edit project setup", []ui.Panel{
		{Title: "Configuration", Lines: model.navigationLines(overview, rows), Focused: true},
		{Title: "Details", Lines: model.detailLines(overview, selectedRow)},
	}, model.overviewActions(overview, selectedRow))
}

func (model Model) dirtyQuitView() string {
	return model.configEditorShell("Discard pending changes?", []ui.Panel{
		{Title: "Unsaved changes", Lines: []string{
			"The draft has pending configuration changes.",
			"Quit without saving?",
			"",
			"No target files have been changed by the config editor.",
		}, Focused: true},
		{Title: "Review", Lines: reviewChangeLines(model.overview())},
	}, []ui.Action{
		{Key: "Enter/y", Label: "Discard and quit"},
		{Key: "Esc/n", Label: "Back"},
		{Key: "Ctrl+C", Label: "Cancel immediately"},
	})
}

func (model Model) savingView() string {
	return model.configEditorShell("Saving configuration", []ui.Panel{
		{Title: "Working", Lines: []string{"Validating draft and target selectors.", "Checking for stale .switchlet.yaml contents.", "Preparing safe replacement."}, Focused: true},
		{Title: "Configuration", Lines: []string{ui.RenderKeyValue("Config file", model.document.ConfigPath)}},
	}, nil)
}

func (model Model) saveSuccessView() string {
	lines := []string{
		".switchlet.yaml was updated.",
		ui.RenderKeyValue("Config file", model.savedConfigPath),
	}
	if len(model.savedChanges) > 0 {
		lines = append(lines, "", "Saved changes")
		for _, change := range model.savedChanges {
			lines = append(lines, "- "+change.Summary)
		}
	}

	return model.configEditorShell("Saved", []ui.Panel{
		{Title: "Save complete", Lines: lines, Focused: true},
	}, []ui.Action{
		{Key: "Enter/q", Label: "Exit"},
		{Key: "Ctrl+C", Label: "Exit"},
	})
}

func (model Model) configEditorShell(subtitle string, panels []ui.Panel, actions []ui.Action) string {
	if model.height > 0 {
		for index := range panels {
			panels[index].FillHeight = true
		}
	}

	overview := model.overview()
	return ui.RenderShell(ui.Shell{
		Title:    "Switchlet config",
		Subtitle: subtitle,
		Metadata: []string{
			fmt.Sprintf("%d profiles", len(overview.Profiles)),
			fmt.Sprintf("%d managed values", len(overview.ManagedValues)),
		},
		Panels:  panels,
		Actions: actions,
		Width:   model.width,
		Height:  model.height,
	})
}

func (model Model) navigationLines(overview app.ConfigEditOverview, rows []navigationRow) []string {
	lines := make([]string, 0)
	if model.state == editorStateFilter {
		lines = append(lines, ui.RenderInput("Filter", model.inputValue, model.inputCursor), "")
	} else if model.filter != "" {
		lines = append(lines, ui.RenderKeyValue("Active filter", model.filter), "")
	}

	listRows := make([]ui.ListRow, 0, len(rows))
	for index, row := range rows {
		state := ui.RowNormal
		if index == model.cursor {
			state = ui.RowSelected
		}

		listRows = append(listRows, ui.ListRow{Label: model.navigationLabel(overview, row), State: state, Badges: navigationBadges(overview, row)})
	}

	lines = append(lines, ui.RenderListRows(listRows)...)
	if model.filter != "" || model.state == editorStateFilter {
		lines = append(lines, "", fmt.Sprintf("Showing %d navigable row(s).", len(rows)))
	}

	return lines
}

func (model Model) navigationLabel(overview app.ConfigEditOverview, row navigationRow) string {
	switch row.Kind {
	case navigationRowProfile:
		return "  " + overview.Profiles[row.ProfileIndex].Name
	case navigationRowManagedValue:
		return "  " + overview.ManagedValues[row.ManagedValueIndex].TargetName
	default:
		return row.Label
	}
}

func navigationBadges(overview app.ConfigEditOverview, row navigationRow) []ui.Badge {
	switch row.Kind {
	case navigationRowProfilesSection:
		return []ui.Badge{{Label: fmt.Sprintf("%d", len(overview.Profiles))}}
	case navigationRowProfile:
		profile := overview.Profiles[row.ProfileIndex]
		badges := []ui.Badge{{Label: fmt.Sprintf("%d/%d", profile.ValueCount, profile.TotalManagedValues)}}
		if profile.Protected {
			badges = append(badges, ui.Badge{Label: "protected"})
		}
		if profile.Partial {
			badges = append(badges, ui.Badge{Label: "partial"})
		}
		return badges
	case navigationRowManagedValuesSection:
		return []ui.Badge{{Label: fmt.Sprintf("%d", len(overview.ManagedValues))}}
	case navigationRowManagedValue:
		return []ui.Badge{{Label: string(overview.ManagedValues[row.ManagedValueIndex].TargetType)}}
	case navigationRowReview:
		if overview.Saveable {
			return []ui.Badge{{Label: "saveable"}}
		}
		if overview.Dirty {
			return []ui.Badge{{Label: "pending"}}
		}
	}

	return nil
}

func (model Model) detailLines(overview app.ConfigEditOverview, row navigationRow) []string {
	if model.state == editorStateFilter {
		return []string{
			"Filter profiles and managed values by name, file, type, or selector.",
			"",
			"Enter applies the filter.",
			"Esc returns without changing the active filter.",
			"q is literal text on this screen.",
		}
	}

	switch row.Kind {
	case navigationRowProfilesSection:
		return profileSectionLines(overview)
	case navigationRowProfile:
		return profileDetailLines(overview.Profiles[row.ProfileIndex])
	case navigationRowManagedValuesSection:
		return managedValueSectionLines(overview)
	case navigationRowManagedValue:
		return managedValueDetailLines(overview.ManagedValues[row.ManagedValueIndex])
	case navigationRowReview:
		return model.reviewLines(overview)
	default:
		return nil
	}
}

func profileSectionLines(overview app.ConfigEditOverview) []string {
	return []string{
		ui.RenderKeyValue("Profiles", fmt.Sprintf("%d", len(overview.Profiles))),
		ui.RenderKeyValue("Managed values", fmt.Sprintf("%d", len(overview.ManagedValues))),
		"",
		"Select a profile to inspect its included managed values.",
		"Profile add, edit, rename, and delete flows arrive in later phases.",
	}
}

func profileDetailLines(profile app.ConfigEditProfileItem) []string {
	lines := []string{
		ui.RenderKeyValue("Profile", profile.Name),
		ui.RenderKeyValue("Protected", yesNo(profile.Protected)),
		ui.RenderKeyValue("Managed values", fmt.Sprintf("%d of %d", profile.ValueCount, profile.TotalManagedValues)),
		ui.RenderKeyValue("Partial", yesNo(profile.Partial)),
		"",
		"Included managed values",
	}
	if len(profile.Values) == 0 {
		return append(lines, "No managed values are included.")
	}

	for _, value := range profile.Values {
		label := value.TargetName
		if value.TargetType != "" {
			label += " [" + string(value.TargetType) + "]"
		}
		lines = append(lines, "- "+label)
		if value.Source == app.ProfileSourceEnvironment {
			lines = append(lines, "  environment: "+value.EnvironmentVariableName)
		} else {
			lines = append(lines, "  literal value: ****")
		}
	}

	return lines
}

func managedValueSectionLines(overview app.ConfigEditOverview) []string {
	return []string{
		ui.RenderKeyValue("Managed values", fmt.Sprintf("%d", len(overview.ManagedValues))),
		"",
		"Select a managed value to inspect its file, type, selector, and profile usage.",
		"Managed-value add, edit, rename, and delete flows arrive in later phases.",
	}
}

func managedValueDetailLines(managedValue app.ConfigEditManagedValueItem) []string {
	return []string{
		ui.RenderKeyValue("Managed value", managedValue.TargetName),
		ui.RenderKeyValue("File", managedValue.TargetFile),
		ui.RenderKeyValue("Type", string(managedValue.TargetType)),
		ui.RenderKeyValue(managedValue.SelectorName, managedValue.Selector),
		ui.RenderKeyValue("Profiles", fmt.Sprintf("%d include this", managedValue.IncludedProfileCount)),
	}
}

func (model Model) reviewLines(overview app.ConfigEditOverview) []string {
	lines := reviewChangeLines(overview)
	if model.saveError != "" {
		lines = append(lines, "", "Save failed", model.saveError)
	}
	if overview.ValidationError != "" {
		lines = append(lines, "", "Save unavailable", overview.ValidationError)
	} else if overview.Saveable {
		lines = append(lines, "", "Press s to save .switchlet.yaml.")
	} else {
		lines = append(lines, "", "Save is unavailable until the draft changes.")
	}

	return lines
}

func reviewChangeLines(overview app.ConfigEditOverview) []string {
	if len(overview.Changes) == 0 {
		return []string{"No pending changes."}
	}

	lines := make([]string, 0, len(overview.Changes)*2)
	for _, change := range overview.Changes {
		prefix := "- "
		if change.Warning {
			prefix = "! "
		}
		lines = append(lines, prefix+change.Summary)
		for _, detail := range change.Detail {
			lines = append(lines, "  "+detail)
		}
	}

	return lines
}

func (model Model) overviewActions(overview app.ConfigEditOverview, selectedRow navigationRow) []ui.Action {
	if model.state == editorStateFilter {
		return []ui.Action{
			{Key: "Enter", Label: "Apply filter"},
			{Key: "Left/Right", Label: "Move"},
			{Key: "Home/End", Label: "Jump"},
			{Key: "Bksp/Del", Label: "Edit"},
			{Key: "Esc", Label: "Back"},
			{Key: "Ctrl+C", Label: "Cancel"},
		}
	}

	actions := []ui.Action{
		{Key: "Enter/l", Label: "Open"},
		{Key: "j/k", Label: "Move"},
		{Key: "g/G", Label: "First/Last"},
		{Key: "/", Label: "Filter"},
	}
	if model.filter != "" {
		actions = append(actions,
			ui.Action{Key: "n/N", Label: "Matches"},
			ui.Action{Key: "Esc/h", Label: "Clear filter"},
		)
	}
	if selectedRow.Kind == navigationRowReview && overview.Saveable {
		actions = append(actions, ui.Action{Key: "s", Label: "Save"})
	}
	actions = append(actions,
		ui.Action{Key: "q", Label: "Quit"},
		ui.Action{Key: "Ctrl+C", Label: "Cancel"},
	)

	return actions
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}
