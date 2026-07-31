package configeditor

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

func (model Model) profileNameInputView() string {
	lines := []string{
		ui.RenderInput("Profile name", model.inputValue, model.inputCursor),
	}
	if model.profileForm.errorMessage != "" {
		lines = append(lines, "", model.profileForm.errorMessage)
	}

	return model.configEditorShell(model.profileFormTitle(), []ui.Panel{
		{Title: model.profileFormPanelTitle("Profile name"), Lines: lines, Focused: true},
		{Title: "Guidance", Lines: []string{
			"Choose a unique profile name.",
			"q is literal text on this screen.",
			"No .switchlet.yaml changes are written until review save.",
		}},
	}, []ui.Action{
		{Key: "Enter", Label: "Continue"},
		{Key: "Left/Right", Label: "Move"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Back"},
	})
}

func (model Model) profileIncludeValuesView() string {
	return model.configEditorShell(model.profileFormTitle(), []ui.Panel{
		{Title: model.profileFormPanelTitle("Included targets"), Lines: model.profileIncludeValueLines(), Focused: true},
		{Title: "Profile", Lines: model.profileDraftSummaryLines()},
	}, []ui.Action{
		{Key: "Enter/l", Label: "Continue"},
		{Key: "j/k", Label: "Move"},
		{Key: "Space", Label: "Toggle include"},
		{Key: "e", Label: "Edit value"},
		{Key: "p", Label: "Toggle protected"},
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
	})
}

func (model Model) profileValueSourceView() string {
	value := model.activeProfileDraftValue()
	rows := []ui.ListRow{
		{Label: "literal value", State: ui.RowNormal},
		{Label: "environment variable", State: ui.RowNormal},
	}
	if model.profileForm.sourceCursor == 0 {
		rows[0].State = ui.RowSelected
	} else {
		rows[1].State = ui.RowSelected
	}

	lines := []string{
		ui.RenderKeyValue("Target", profileDraftValueLabel(value)),
		"",
	}
	lines = append(lines, ui.RenderListRows(rows)...)
	if model.profileForm.errorMessage != "" {
		lines = append(lines, "", model.profileForm.errorMessage)
	}

	return model.configEditorShell(model.profileFormTitle(), []ui.Panel{
		{Title: "Value source", Lines: lines, Focused: true},
		{Title: "Guidance", Lines: []string{
			"Literal values are stored in .switchlet.yaml.",
			"Environment-backed values store only the variable name.",
			"Resolved environment values are never shown here.",
		}},
	}, []ui.Action{
		{Key: "Enter/l", Label: "Select"},
		{Key: "j/k", Label: "Move"},
		{Key: "Space", Label: "Toggle"},
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
	})
}

func (model Model) profileValueInputView() string {
	value := model.activeProfileDraftValue()
	label := "Literal value"
	if value.Source == app.ProfileSourceEnvironment {
		label = "Environment variable"
	}
	lines := []string{
		ui.RenderKeyValue("Target", profileDraftValueLabel(value)),
		ui.RenderInput(label, model.inputValue, model.inputCursor),
	}
	if model.profileForm.errorMessage != "" {
		lines = append(lines, "", model.profileForm.errorMessage)
	}

	guidance := []string{
		"q is literal text on this screen.",
		"Values are hidden again after active entry.",
	}
	if value.Source == app.ProfileSourceEnvironment {
		guidance = append(guidance, "Enter the variable name, not its resolved value.")
	}

	return model.configEditorShell(model.profileFormTitle(), []ui.Panel{
		{Title: "Profile value", Lines: lines, Focused: true},
		{Title: "Guidance", Lines: guidance},
	}, []ui.Action{
		{Key: "Enter", Label: "Continue"},
		{Key: "Left/Right", Label: "Move"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Back"},
	})
}

func (model Model) profileReviewView() string {
	lines := model.profileDraftSummaryLines()
	lines = append(lines, "", "Review this profile draft before adding it to the pending config changes.")
	if model.profileForm.errorMessage != "" {
		lines = append(lines, "", "Could not save profile draft", model.profileForm.errorMessage)
	}

	return model.configEditorShell(model.profileFormTitle(), []ui.Panel{
		{Title: model.profileFormPanelTitle("Review profile"), Lines: lines, Focused: true},
		{Title: "Pending values", Lines: model.profileDraftValueSummaryLines()},
	}, []ui.Action{
		{Key: "Enter/l", Label: "Save draft"},
		{Key: "Space/p", Label: "Toggle protected"},
		{Key: "Esc/h", Label: "Back"},
		{Key: "q", Label: "Quit"},
	})
}

func (model Model) profileRemoveConfirmView() string {
	lines := []string{
		fmt.Sprintf("Remove profile %q?", model.profileForm.originalName),
		"",
		"This removes only the profile from the draft.",
		"Targets are not deleted.",
	}
	if model.profileForm.errorMessage != "" {
		lines = append(lines, "", model.profileForm.errorMessage)
	}

	return model.configEditorShell("Remove profile", []ui.Panel{
		{Title: "Confirm removal", Lines: lines, Focused: true},
		{Title: "Profile", Lines: model.profileDraftSummaryLines()},
	}, []ui.Action{
		{Key: "Enter/y", Label: "Remove"},
		{Key: "Esc/n", Label: "Cancel"},
		{Key: "q", Label: "Quit"},
	})
}

func (model Model) profileFormTitle() string {
	if model.profileForm.mode == profileDraftUpdate {
		return "Edit profile"
	}

	return "Add profile"
}

func (model Model) profileFormPanelTitle(title string) string {
	if model.profileForm.mode == profileDraftUpdate && model.profileForm.originalName != "" {
		return title + " - " + model.profileForm.originalName
	}

	return title
}

func (model Model) profileIncludeValueLines() []string {
	model.clampProfileIncludeCursor()
	rows := make([]ui.ListRow, 0, len(model.profileForm.draft.Values))
	for index, value := range model.profileForm.draft.Values {
		state := ui.RowNormal
		if index == model.profileForm.includeCursor {
			state = ui.RowSelected
		}
		check := "[ ] "
		if value.Included {
			check = "[x] "
		}
		rows = append(rows, ui.ListRow{Label: check + profileDraftValueLabel(value), State: state})
	}

	lines := ui.RenderListRows(rows)
	if model.profileForm.errorMessage != "" {
		lines = append(lines, "", model.profileForm.errorMessage)
	}
	if len(model.profileForm.draft.Values) > 1 {
		lines = append(lines, "", fmt.Sprintf("Included: %d of %d", model.includedProfileValueCount(), len(model.profileForm.draft.Values)))
	}

	return lines
}

func (model Model) profileDraftSummaryLines() []string {
	profileName := model.profileForm.draft.Name
	if profileName == "" {
		profileName = "(new profile)"
	}

	return []string{
		ui.RenderKeyValue("Profile", profileName),
		ui.RenderKeyValue("Protected", yesNo(model.profileForm.draft.Protected)),
		ui.RenderKeyValue("Targets", fmt.Sprintf("%d of %d", model.includedProfileValueCount(), len(model.profileForm.draft.Values))),
	}
}

func (model Model) profileDraftValueSummaryLines() []string {
	lines := make([]string, 0)
	for _, value := range model.profileForm.draft.Values {
		if !value.Included {
			continue
		}
		lines = append(lines, "- "+profileDraftValueLabel(value))
		if value.Source == app.ProfileSourceEnvironment {
			if value.EnvironmentVariableName == "" {
				lines = append(lines, "  environment: (not set)")
			} else {
				lines = append(lines, "  environment: "+value.EnvironmentVariableName)
			}
		} else {
			lines = append(lines, "  literal value: ****")
		}
	}
	if len(lines) == 0 {
		return []string{"No targets are included."}
	}

	return lines
}

func (model Model) activeProfileDraftValue() app.ConfigEditProfileValueDraft {
	if model.profileForm.valueIndex >= 0 && model.profileForm.valueIndex < len(model.profileForm.draft.Values) {
		return model.profileForm.draft.Values[model.profileForm.valueIndex]
	}

	return app.ConfigEditProfileValueDraft{}
}

func profileDraftValueLabel(value app.ConfigEditProfileValueDraft) string {
	label := value.TargetName
	if value.TargetType != "" {
		label += " [" + string(value.TargetType) + "]"
	}

	return label
}
