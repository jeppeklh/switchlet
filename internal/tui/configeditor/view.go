package configeditor

import (
	"fmt"
	"strings"

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
			Actions: []ui.Action{{Key: "q", Label: "Quit"}},
			Width:   model.width,
			Height:  model.height,
		})
	}

	switch model.state {
	case editorStateProfileNameInput:
		return model.profileNameInputView()
	case editorStateProfileIncludeValues:
		return model.profileIncludeValuesView()
	case editorStateProfileValueSource:
		return model.profileValueSourceView()
	case editorStateProfileValueInput:
		return model.profileValueInputView()
	case editorStateProfileReview:
		return model.profileReviewView()
	case editorStateProfileRemoveConfirm:
		return model.profileRemoveConfirmView()
	case editorStateManagedValueFileLoading:
		return model.managedValueLoadingView("Finding manageable configuration files", "Scanning the project for JSON, YAML, TOML, and dotenv values.")
	case editorStateManagedValueFileSelect:
		return model.managedValueFileSelectView()
	case editorStateManagedValueFileFilter:
		return model.managedValueFileSelectView()
	case editorStateManagedValueManualFileInput:
		return model.managedValueManualFileView()
	case editorStateManagedValueTypeSelect:
		return model.managedValueTypeSelectView()
	case editorStateManagedValueSelectorLoading:
		return model.managedValueLoadingView("Inspecting selected file", "Reading existing manageable values without modifying the file.")
	case editorStateManagedValueSelectorSelect:
		return model.managedValueSelectorSelectView()
	case editorStateManagedValueSelectorFilter:
		return model.managedValueSelectorSelectView()
	case editorStateManagedValueManualSelectorInput:
		return model.managedValueManualSelectorView()
	case editorStateManagedValueSelectorValidating:
		return model.managedValueLoadingView("Validating selector", "Checking that the selected value exists and is manageable.")
	case editorStateManagedValueNameInput:
		return model.managedValueNameInputView()
	case editorStateManagedValueReview:
		return model.managedValueReviewView()
	case editorStateManagedValueRemoveConfirm:
		return model.managedValueRemoveConfirmView()
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

	return model.configEditorShellWithScroll("Edit project setup", []ui.Panel{
		{Title: "Configuration", Lines: model.navigationLines(overview, rows), Focused: true},
		{Title: "Details", Lines: model.detailLines(overview, selectedRow)},
	}, model.overviewActions(overview, selectedRow), model.overviewScrollablePanelIndex(selectedRow))
}

func (model Model) dirtyQuitView() string {
	return model.configEditorShell("Discard pending changes?", []ui.Panel{
		{Title: "Unsaved changes", Lines: []string{
			"The draft has pending configuration changes.",
			"Quit without saving?",
			"",
			"No managed files have been changed by the config editor.",
		}, Focused: true},
		{Title: "Review", Lines: reviewChangeLines(model.overview())},
	}, []ui.Action{
		{Key: "Enter/y", Label: "Discard and quit"},
		{Key: "Esc/n", Label: "Back"},
	})
}

func (model Model) savingView() string {
	return model.configEditorShell("Saving configuration", []ui.Panel{
		{Title: "Working", Lines: []string{"Validating draft and managed value selectors.", "Checking for stale .switchlet.yaml contents.", "Preparing safe replacement."}, Focused: true},
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
			lines = append(lines, "- "+configEditorChangeSummary(change))
		}
	}

	return model.configEditorShell("Saved", []ui.Panel{
		{Title: "Save complete", Lines: lines, Focused: true},
	}, []ui.Action{
		{Key: "Enter/q", Label: "Exit"},
	})
}

func (model Model) configEditorShell(subtitle string, panels []ui.Panel, actions []ui.Action) string {
	return model.configEditorShellWithScroll(subtitle, panels, actions, -1)
}

func (model Model) configEditorShellWithScroll(subtitle string, panels []ui.Panel, actions []ui.Action, scrollPanelIndex int) string {
	if model.height > 0 {
		for index := range panels {
			panels[index].FillHeight = true
		}
	}
	helpActions := actions
	if scrollPanelIndex >= 0 && scrollPanelIndex < len(panels) {
		shell := ui.Shell{
			Title:      "Switchlet config",
			Subtitle:   subtitle,
			Panels:     panels,
			Actions:    actions,
			Width:      model.width,
			Height:     model.height,
			Headerless: true,
		}
		if ui.PanelScrollMetricsForShell(shell, scrollPanelIndex).CanScroll() {
			helpActions = append(helpActions, ui.PanelScrollActions()...)
		}
	}
	if model.helpOpen {
		return ui.RenderShell(ui.Shell{
			Title:      "Switchlet config",
			Subtitle:   "Help",
			Panels:     []ui.Panel{{Title: "Help", Lines: ui.HelpLines(helpActions, ui.PrimaryPanelWidth(model.width, 1)), Focused: true}},
			Actions:    ui.HelpReturnActions("Quit"),
			Width:      model.width,
			Height:     model.height,
			Headerless: true,
		})
	}
	if model.canOpenHelp() {
		actions = ui.AppendHelpAction(actions)
	}

	shell := ui.Shell{
		Title:      "Switchlet config",
		Subtitle:   subtitle,
		Panels:     panels,
		Actions:    actions,
		Width:      model.width,
		Height:     model.height,
		Headerless: true,
	}
	if scrollPanelIndex >= 0 && scrollPanelIndex < len(shell.Panels) {
		metrics := ui.PanelScrollMetricsForShell(shell, scrollPanelIndex)
		shell.Panels[scrollPanelIndex].Scroll = &ui.PanelScroll{Offset: metrics.ClampOffset(model.scrollOffset)}
		if metrics.CanScroll() {
			shell.Actions = append(shell.Actions, ui.PanelScrollActions()...)
		}
	}

	return ui.RenderShell(shell)
}

func (model Model) overviewScrollablePanelIndex(selectedRow navigationRow) int {
	if selectedRow.Kind == navigationRowReview {
		return 1
	}

	return -1
}

func (model Model) scrollablePanelMetrics() ui.PanelScrollMetrics {
	if model.state != editorStateOverview || model.activeTab != overviewTabReview {
		return ui.PanelScrollMetrics{}
	}

	overview := model.overview()
	rows := model.navigationRows(overview)
	model.clampCursor(len(rows))
	selectedRow := model.selectedRow(rows)
	scrollPanelIndex := model.overviewScrollablePanelIndex(selectedRow)
	if scrollPanelIndex < 0 {
		return ui.PanelScrollMetrics{}
	}

	panels := []ui.Panel{
		{Title: "Configuration", Lines: model.navigationLines(overview, rows), Focused: true},
		{Title: "Details", Lines: model.detailLines(overview, selectedRow)},
	}
	if model.height > 0 {
		for index := range panels {
			panels[index].FillHeight = true
		}
	}

	shell := ui.Shell{
		Title:      "Switchlet config",
		Subtitle:   "Edit project setup",
		Panels:     panels,
		Actions:    model.overviewActions(overview, selectedRow),
		Width:      model.width,
		Height:     model.height,
		Headerless: true,
	}

	return ui.PanelScrollMetricsForShell(shell, scrollPanelIndex)
}

func (model Model) navigationLines(overview app.ConfigEditOverview, rows []navigationRow) []string {
	lines := []string{model.overviewTabsLine(), ""}
	if model.state == editorStateFilter {
		lines = append(lines, ui.RenderInput("Filter", model.inputValue, model.inputCursor), "")
	} else if model.filter != "" {
		lines = append(lines, ui.RenderKeyValue("Active filter", model.filter), "")
	}

	if model.activeTab == overviewTabReview {
		return append(lines, "Review pending changes and save when ready.")
	}

	listRows := make([]ui.ListRow, 0, len(rows))
	for index, row := range rows {
		state := ui.RowNormal
		if index == model.cursor {
			state = ui.RowSelected
		}

		listRows = append(listRows, ui.ListRow{Label: model.navigationLabel(overview, row), State: state})
	}
	if len(listRows) == 0 {
		lines = append(lines, model.emptyTabLine())
	} else {
		lines = append(lines, ui.RenderListRows(listRows)...)
	}

	if model.filter != "" || model.state == editorStateFilter {
		lines = append(lines, "", fmt.Sprintf("Showing %d navigable row(s).", len(rows)))
	}

	return lines
}

func (model Model) overviewTabsLine() string {
	tabs := []string{
		model.overviewTabLabel(overviewTabProfiles, "Profiles"),
		model.overviewTabLabel(overviewTabTargets, "Managed values"),
		model.overviewTabLabel(overviewTabReview, "Review"),
	}

	return strings.Join(tabs, "   ")
}

func (model Model) overviewTabLabel(tab overviewTab, label string) string {
	if model.activeTab == tab {
		return ui.RenderSectionHeading(label)
	}

	return label
}

func (model Model) emptyTabLine() string {
	if model.activeTab == overviewTabTargets {
		return "No managed values configured."
	}

	return "No profiles configured."
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
		return model.managedValueDetailLines(overview.ManagedValues[row.ManagedValueIndex])
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
		"Press a to add a profile from existing managed values.",
	}
}

func profileDetailLines(profile app.ConfigEditProfileItem) []string {
	lines := []string{
		ui.RenderSectionHeading(profile.Name),
		"",
		ui.RenderSectionHeading("Managed values"),
	}
	if len(profile.Values) == 0 {
		return append(lines, "No managed values are included.")
	}

	for _, value := range profile.Values {
		lines = append(lines, "", ui.RenderSectionHeading(profileValueHeader(value.TargetName, string(value.TargetType))))
		fields := []ui.DetailField{{Label: "Source", Value: sourceLabel(value.Source)}}
		if value.Source == app.ProfileSourceEnvironment {
			fields = append(fields, ui.DetailField{Label: "Env", Value: value.EnvironmentVariableName})
		} else {
			fields = append(fields, ui.DetailField{Label: "Value", Value: "****"})
		}
		lines = append(lines, ui.RenderFieldRows(fields)...)
	}

	return lines
}

func managedValueSectionLines(overview app.ConfigEditOverview) []string {
	return []string{
		ui.RenderKeyValue("Managed values", fmt.Sprintf("%d", len(overview.ManagedValues))),
		"",
		"Select a managed value to inspect its file, type, selector, and profile usage.",
		"Press a to add one through files-first selection.",
	}
}

func (model Model) managedValueDetailLines(managedValue app.ConfigEditManagedValueItem) []string {
	fields := []ui.DetailField{
		{Label: "File", Value: app.DisplayInitTargetPath(model.document.ProjectRoot, managedValue.TargetFile)},
		{Label: "Type", Value: string(managedValue.TargetType)},
		{Label: selectorDetailLabel(managedValue.SelectorName), Value: managedValue.Selector},
		{Label: "Profiles", Value: fmt.Sprintf("%d", managedValue.IncludedProfileCount)},
	}

	return append([]string{ui.RenderSectionHeading(managedValue.TargetName), ""}, ui.RenderFieldRows(fields)...)
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
		lines = append(lines, prefix+configEditorChangeSummary(change))
		for _, detail := range change.Detail {
			lines = append(lines, "  "+configEditorChangeDetail(detail))
		}
	}

	return lines
}

func configEditorChangeSummary(change app.ConfigEditChange) string {
	summary := change.Summary
	switch change.Kind {
	case app.ConfigEditChangeManagedValueAdded, app.ConfigEditChangeManagedValueUpdated, app.ConfigEditChangeManagedValueRenamed, app.ConfigEditChangeManagedValueRemoved:
		summary = strings.ReplaceAll(summary, "target", "managed value")
		summary = strings.ReplaceAll(summary, "Target", "Managed value")
	}

	return summary
}

func configEditorChangeDetail(detail string) string {
	switch detail {
	case "Targets or value sources changed.":
		return "Managed values or value sources changed."
	}
	if strings.HasPrefix(detail, "Targets:") {
		return strings.Replace(detail, "Targets:", "Managed values:", 1)
	}

	return detail
}

func (model Model) overviewActions(overview app.ConfigEditOverview, selectedRow navigationRow) []ui.Action {
	if model.state == editorStateFilter {
		return []ui.Action{
			{Key: "Enter", Label: "Apply filter"},
			{Key: "Left/Right", Label: "Move"},
			{Key: "Home/End", Label: "Jump"},
			{Key: "Bksp/Del", Label: "Edit"},
			{Key: "Esc", Label: "Back"},
		}
	}

	actions := []ui.Action{
		{Key: "h/l", Label: "Tabs"},
		{Key: "j/k", Label: "Move"},
		{Key: "g/G", Label: "First/Last"},
	}
	if model.activeTab != overviewTabReview {
		actions = append(actions, ui.Action{Key: "/", Label: "Filter"})
	}
	if model.filter != "" {
		actions = append(actions,
			ui.Action{Key: "n/N", Label: "Matches"},
			ui.Action{Key: "Esc/h", Label: "Clear filter"},
		)
	}
	if selectedRow.Kind == navigationRowProfilesSection || selectedRow.Kind == navigationRowProfile {
		actions = append(actions, ui.Action{Key: "a", Label: "Add profile"})
	}
	if selectedRow.Kind == navigationRowProfile {
		actions = append(actions,
			ui.Action{Key: "e", Label: "Edit"},
			ui.Action{Key: "D", Label: "Duplicate"},
			ui.Action{Key: "r", Label: "Rename"},
			ui.Action{Key: "d", Label: "Delete"},
		)
	}
	if selectedRow.Kind == navigationRowManagedValuesSection || selectedRow.Kind == navigationRowManagedValue {
		actions = append(actions, ui.Action{Key: "a", Label: "Add value"})
	}
	if selectedRow.Kind == navigationRowManagedValue {
		actions = append(actions,
			ui.Action{Key: "e", Label: "Edit location"},
			ui.Action{Key: "r", Label: "Rename"},
			ui.Action{Key: "d", Label: "Delete"},
		)
	}
	if selectedRow.Kind == navigationRowReview && overview.Saveable {
		actions = append(actions, ui.Action{Key: "s", Label: "Save"})
	}
	if model.embedded {
		actions = append(actions, ui.Action{Key: "c", Label: "Picker"})
	}
	actions = append(actions,
		ui.Action{Key: "q", Label: "Quit"},
	)

	return actions
}

func profileValueHeader(targetName string, targetType string) string {
	if targetType == "" {
		return targetName
	}

	return targetName + " [" + targetType + "]"
}

func sourceLabel(source app.ProfileSource) string {
	if source == app.ProfileSourceEnvironment {
		return "environment"
	}

	return "literal"
}

func selectorDetailLabel(selectorName string) string {
	switch selectorName {
	case "jsonPath":
		return "jsonPath"
	case "yamlPath":
		return "yamlPath"
	case "tomlPath":
		return "tomlPath"
	case "key":
		return "key"
	default:
		return selectorName
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}
