package configeditor

import (
	"fmt"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

func (model Model) managedValueLoadingView(title string, message string) string {
	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: title, Lines: append([]string{message}, model.managedValueErrorLines()...), Focused: true},
		{Title: "Managed value", Lines: model.managedValueTargetSummaryLines()},
	}, []ui.Action{
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
		{Key: "Ctrl+C", Label: "Cancel"},
	})
}

func (model Model) managedValueFileSelectView() string {
	lines := make([]string, 0)
	if model.state == editorStateManagedValueFileFilter {
		lines = append(lines, ui.RenderInput("Filter files", model.inputValue, model.inputCursor), "")
	} else if strings.TrimSpace(model.managedForm.fileFilter) != "" {
		lines = append(lines, ui.RenderKeyValue("Active filter", model.managedForm.fileFilter), "")
	}

	candidates := model.filteredManagedValueFileCandidates()
	model.clampManagedValueFileCursor(len(candidates))
	if len(candidates) == 0 {
		lines = append(lines, "No discovered manageable files match.", "Press m to enter a file path manually.")
	} else {
		rows := make([]ui.ListRow, 0, len(candidates))
		for index, candidate := range candidates {
			state := ui.RowNormal
			if index == model.managedForm.fileCursor {
				state = ui.RowSelected
			}
			rows = append(rows, ui.ListRow{
				Label:  candidate.RelativePath,
				State:  state,
				Badges: []ui.Badge{{Label: string(candidate.Type)}},
			})
		}
		lines = append(lines, ui.RenderListRows(rows)...)
	}
	lines = append(lines, model.managedValueErrorLines()...)

	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: "Choose file", Lines: lines, Focused: true},
		{Title: "Selected value", Lines: []string{
			"Choose a file before choosing the managed value inside it.",
			"Only files with existing manageable values are listed.",
			"Switchlet does not create missing target-file values.",
		}},
	}, model.managedValueFileActions())
}

func (model Model) managedValueManualFileView() string {
	lines := []string{ui.RenderInput("File path", model.inputValue, model.inputCursor)}
	lines = append(lines, model.managedValueErrorLines()...)

	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: "Manual file", Lines: lines, Focused: true},
		{Title: "Supported types", Lines: []string{
			"JSON files use jsonPath.",
			"YAML files use yamlPath.",
			"TOML files use tomlPath.",
			"dotenv files use key.",
			"If the type cannot be inferred, choose it explicitly next.",
		}},
	}, []ui.Action{
		{Key: "Enter", Label: "Continue"},
		{Key: "Left/Right", Label: "Move"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Back"},
		{Key: "Ctrl+C", Label: "Cancel"},
	})
}

func (model Model) managedValueTypeSelectView() string {
	targetTypes := managedValueTargetTypeChoices()
	model.clampManagedValueTypeCursor()
	rows := make([]ui.ListRow, 0, len(targetTypes))
	for index, targetType := range targetTypes {
		state := ui.RowNormal
		if index == model.managedForm.typeCursor {
			state = ui.RowSelected
		}
		rows = append(rows, ui.ListRow{Label: managedValueTypeDisplayName(targetType), State: state})
	}

	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: "Choose type", Lines: ui.RenderListRows(rows), Focused: true},
		{Title: "File", Lines: []string{ui.RenderKeyValue("File", model.managedValueDisplayPath(model.managedForm.target.File))}},
	}, []ui.Action{
		{Key: "Enter/l", Label: "Select"},
		{Key: "j/k", Label: "Move"},
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
		{Key: "Ctrl+C", Label: "Cancel"},
	})
}

func (model Model) managedValueSelectorSelectView() string {
	lines := make([]string, 0)
	if model.state == editorStateManagedValueSelectorFilter {
		lines = append(lines, ui.RenderInput("Filter values", model.inputValue, model.inputCursor), "")
	} else if strings.TrimSpace(model.managedForm.selectorFilter) != "" {
		lines = append(lines, ui.RenderKeyValue("Active filter", model.managedForm.selectorFilter), "")
	}

	options := model.filteredManagedValueSelectorOptions()
	model.clampManagedValueSelectorCursor(len(options))
	if len(options) == 0 {
		lines = append(lines, "No existing manageable values match.", "Press m to enter a selector manually.")
	} else {
		rows := make([]ui.ListRow, 0, len(options))
		for index, option := range options {
			state := ui.RowNormal
			if index == model.managedForm.selectorCursor {
				state = ui.RowSelected
			}
			rows = append(rows, ui.ListRow{Label: option.Label, State: state})
		}
		lines = append(lines, ui.RenderListRows(rows)...)
	}
	lines = append(lines, model.managedValueErrorLines()...)

	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: "Choose value", Lines: lines, Focused: true},
		{Title: "Target", Lines: append(model.managedValueTargetSummaryLines(), "", "Existing string values only.", "Missing values are not created.")},
	}, model.managedValueSelectorActions())
}

func (model Model) managedValueManualSelectorView() string {
	label := managedValueSelectorInputLabel(model.managedForm.target.Type)
	lines := []string{ui.RenderInput(label, model.inputValue, model.inputCursor)}
	lines = append(lines, model.managedValueErrorLines()...)

	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: "Manual selector", Lines: lines, Focused: true},
		{Title: "Target", Lines: model.managedValueTargetSummaryLines()},
	}, []ui.Action{
		{Key: "Enter", Label: "Validate"},
		{Key: "Left/Right", Label: "Move"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Back"},
		{Key: "Ctrl+C", Label: "Cancel"},
	})
}

func (model Model) managedValueNameInputView() string {
	lines := []string{ui.RenderInput("Managed value name", model.inputValue, model.inputCursor)}
	lines = append(lines, model.managedValueErrorLines()...)

	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: "Name managed value", Lines: lines, Focused: true},
		{Title: "Selected value", Lines: model.managedValueTargetSummaryLines()},
	}, []ui.Action{
		{Key: "Enter", Label: "Review"},
		{Key: "Left/Right", Label: "Move"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Back"},
		{Key: "Ctrl+C", Label: "Cancel"},
	})
}

func (model Model) managedValueReviewView() string {
	lines := model.managedValueReviewLines()
	lines = append(lines, model.managedValueErrorLines()...)

	return model.configEditorShell(model.managedValueFormTitle(), []ui.Panel{
		{Title: "Review managed value", Lines: lines, Focused: true},
		{Title: "Pending config", Lines: reviewChangeLines(model.overview())},
	}, []ui.Action{
		{Key: "Enter/l", Label: "Save draft"},
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
		{Key: "Ctrl+C", Label: "Cancel"},
	})
}

func (model Model) managedValueRemoveConfirmView() string {
	lines := []string{
		fmt.Sprintf("Remove managed value %q?", model.managedForm.originalName),
		"Profile values for this managed value will be removed from the draft.",
	}
	if len(model.managedForm.removeResult.AffectedProfiles) > 0 {
		lines = append(lines, "", "Affected profiles")
		for _, profileName := range model.managedForm.removeResult.AffectedProfiles {
			lines = append(lines, "- "+profileName)
		}
	}
	if len(model.managedForm.removeResult.InvalidProfiles) > 0 {
		lines = append(lines, "", "Profiles needing changes before save")
		for _, profileName := range model.managedForm.removeResult.InvalidProfiles {
			lines = append(lines, "- "+profileName)
		}
	}
	lines = append(lines, model.managedValueErrorLines()...)

	return model.configEditorShell("Remove managed value", []ui.Panel{
		{Title: "Confirm removal", Lines: lines, Focused: true},
		{Title: "Managed value", Lines: model.managedValueTargetSummaryLines()},
	}, []ui.Action{
		{Key: "Enter/y", Label: "Remove"},
		{Key: "Esc/n", Label: "Cancel"},
		{Key: "q", Label: "Quit"},
		{Key: "Ctrl+C", Label: "Cancel"},
	})
}

func (model Model) managedValueFileActions() []ui.Action {
	if model.state == editorStateManagedValueFileFilter {
		return []ui.Action{
			{Key: "Enter", Label: "Apply filter"},
			{Key: "Left/Right", Label: "Move"},
			{Key: "Bksp/Del", Label: "Edit"},
			{Key: "Esc", Label: "Back"},
			{Key: "Ctrl+C", Label: "Cancel"},
		}
	}

	return []ui.Action{
		{Key: "Enter/l", Label: "Select"},
		{Key: "j/k", Label: "Move"},
		{Key: "/", Label: "Filter"},
		{Key: "m", Label: "Manual path"},
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
		{Key: "Ctrl+C", Label: "Cancel"},
	}
}

func (model Model) managedValueSelectorActions() []ui.Action {
	if model.state == editorStateManagedValueSelectorFilter {
		return []ui.Action{
			{Key: "Enter", Label: "Apply filter"},
			{Key: "Left/Right", Label: "Move"},
			{Key: "Bksp/Del", Label: "Edit"},
			{Key: "Esc", Label: "Back"},
			{Key: "Ctrl+C", Label: "Cancel"},
		}
	}

	return []ui.Action{
		{Key: "Enter/l", Label: "Select"},
		{Key: "j/k", Label: "Move"},
		{Key: "/", Label: "Filter"},
		{Key: "m", Label: "Manual selector"},
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
		{Key: "Ctrl+C", Label: "Cancel"},
	}
}

func (model Model) managedValueReviewLines() []string {
	lines := make([]string, 0)
	switch model.managedForm.mode {
	case managedValueDraftRename:
		lines = append(lines,
			ui.RenderKeyValue("Rename", fmt.Sprintf("%s -> %s", model.managedForm.originalName, model.managedForm.target.Name)),
		)
		if len(model.managedForm.renameResult.UpdatedProfiles) > 0 {
			lines = append(lines, "", "Profile references updated")
			for _, profileName := range model.managedForm.renameResult.UpdatedProfiles {
				lines = append(lines, "- "+profileName)
			}
		}
	case managedValueDraftLocationUpdate:
		lines = append(lines, ui.RenderKeyValue("Managed value", model.managedForm.originalName), "")
		lines = append(lines, model.managedValueTargetSummaryLines()...)
	case managedValueDraftRemove:
		lines = append(lines, fmt.Sprintf("Remove managed value %q.", model.managedForm.originalName))
	default:
		lines = append(lines, model.managedValueTargetSummaryLines()...)
		lines = append(lines, "", "Existing profiles remain partial until values are added by editing profiles.")
	}

	return lines
}

func (model Model) managedValueTargetSummaryLines() []string {
	targetName := model.managedForm.target.Name
	if targetName == "" {
		targetName = "(new managed value)"
	}

	targetType := managedValueTypeDisplayName(model.managedForm.target.Type)
	if model.managedForm.target.Type == "" {
		targetType = "(not selected)"
	}

	selectorName := managedValueSelectorFieldName(model.managedForm.target.Type)
	selector := managedValueTargetSelector(model.managedForm.target)
	if selector == "" {
		selector = "(not selected)"
	}

	return []string{
		ui.RenderKeyValue("Managed value", targetName),
		ui.RenderKeyValue("File", model.managedValueDisplayPath(model.managedForm.target.File)),
		ui.RenderKeyValue("Type", targetType),
		ui.RenderKeyValue(selectorName, selector),
	}
}

func (model Model) managedValueErrorLines() []string {
	if strings.TrimSpace(model.managedForm.errorMessage) == "" {
		return nil
	}

	return []string{"", "Could not continue", model.managedForm.errorMessage}
}

func (model Model) managedValueDisplayPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "(not selected)"
	}

	return app.DisplayInitTargetPath(model.document.ProjectRoot, path)
}

func (model Model) managedValueFormTitle() string {
	switch model.managedForm.mode {
	case managedValueDraftRename:
		return "Rename managed value"
	case managedValueDraftLocationUpdate:
		return "Edit managed value"
	case managedValueDraftRemove:
		return "Remove managed value"
	default:
		return "Add managed value"
	}
}
