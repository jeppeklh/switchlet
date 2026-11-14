package initwizard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

type initProfileCapturedMsg struct {
	requestID int
	draft     app.ConfigEditProfileDraft
	err       error
}

func (model initWizardModel) profileDocument() app.ConfigEditDocument {
	return app.ConfigEditDocument{ProjectRoot: model.workingDirectory, Targets: model.targets, Profiles: model.profiles}
}

func (model initWizardModel) startProfileCapture() (tea.Model, tea.Cmd) {
	requestID := model.startPendingEffect(initWizardPendingEffect{
		Kind: initWizardEffectProfileCapture, StepNumber: 4, Title: "Capturing current values",
		Message:    "Reading managed values. Target files remain unchanged.",
		ReturnStep: model.step, ReturnCursor: model.cursor,
	})
	document := model.profileDocument()
	return model, func() tea.Msg {
		draft, err := app.DefaultConfigEditWorkflow().CaptureCurrentProfileDraft(document)
		return initProfileCapturedMsg{requestID, draft, err}
	}
}

func (model initWizardModel) handleProfileCaptured(message initProfileCapturedMsg) (tea.Model, tea.Cmd) {
	if model.result != nil || model.pendingEffect == nil || model.pendingEffect.Kind != initWizardEffectProfileCapture || model.pendingEffect.RequestID != message.requestID {
		return model, nil
	}
	model.restorePendingContext(*model.pendingEffect)
	if message.err != nil {
		model.setStepError("Capture failed", message.err.Error(), "Check managed values or create a fresh profile.")
		return model, nil
	}
	model.beginTemplateProfile(message.draft)
	return model, nil
}

func (model *initWizardModel) beginProfileCopy(name string) {
	draft, err := app.DefaultConfigEditWorkflow().ProfileDraft(model.profileDocument(), name)
	if err != nil {
		model.setStepError("Copy failed", err.Error(), "Choose an existing profile.")
		return
	}
	draft.Name = ""
	model.beginTemplateProfile(draft)
}

func (model *initWizardModel) beginTemplateProfile(draft app.ConfigEditProfileDraft) {
	model.templateProfile = &draft
	model.step = initWizardStepProfileName
	model.cursor = 0
	model.clearInputValue()
	model.clearError()
}

func (model initWizardModel) handleProfileTemplateKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.templateProfile == nil {
		return model, nil
	}
	draft := *model.templateProfile
	draft.Values = append([]app.ConfigEditProfileValueDraft(nil), draft.Values...)
	model.templateProfile = &draft
	model.clampCursor(len(draft.Values))
	switch {
	case isMoveUpKey(key):
		model.cursor = max(0, model.cursor-1)
	case isMoveDownKey(key):
		model.cursor = min(len(draft.Values)-1, model.cursor+1)
	case key.Type == tea.KeyEsc:
		model.step = initWizardStepProfileName
		model.setInputValue(draft.Name)
	case isRuneKey(key, 'p'):
		draft.Protected = !draft.Protected
	case key.Type == tea.KeySpace:
		draft.Values[model.cursor].Included = !draft.Values[model.cursor].Included
	case isRuneKey(key, 'e'):
		value := draft.Values[model.cursor]
		model.draftProfile = initWizardProfileDraft{Name: draft.Name, TargetIndex: model.cursor, Protected: draft.Protected, UseEnvironment: value.Source == app.ProfileSourceEnvironment, Value: value.LiteralValue}
		if model.draftProfile.UseEnvironment {
			model.draftProfile.Value = value.EnvironmentVariableName
		}
		model.beginProfileValueSource()
	case key.Type == tea.KeyEnter:
		document, err := app.DefaultConfigEditWorkflow().AddProfileDraft(model.profileDocument(), draft)
		if err != nil {
			model.setStepError("Profile needs attention", err.Error(), "Edit or include the required values.")
			return model, nil
		}
		model.profiles = document.Profiles
		model.templateProfile = nil
		model.syncIgnorePreference()
		model.beginProfileAdded()
	}
	return model, nil
}

func (model *initWizardModel) saveTemplateValue() {
	draft := *model.templateProfile
	draft.Values = append([]app.ConfigEditProfileValueDraft(nil), draft.Values...)
	value := &draft.Values[model.draftProfile.TargetIndex]
	value.Included = true
	value.Source = app.ProfileSourceLiteral
	value.LiteralValue = model.draftProfile.Value
	value.EnvironmentVariableName = ""
	if model.draftProfile.UseEnvironment {
		value.Source = app.ProfileSourceEnvironment
		value.EnvironmentVariableName = model.draftProfile.Value
		value.LiteralValue = ""
	}
	draft.Protected = model.draftProfile.Protected
	model.templateProfile = &draft
	model.step = initWizardStepProfileTemplate
	model.cursor = model.draftProfile.TargetIndex
	model.clearInputValue()
	model.clearError()
}

func (model initWizardModel) profileTemplateView() string {
	draft := model.templateProfile
	choices := make([]string, len(draft.Values))
	for index, value := range draft.Values {
		mark := "[ ]"
		if value.Included {
			mark = "[x]"
		}
		source := "literal (hidden)"
		if value.Source == app.ProfileSourceEnvironment {
			source = "env " + value.EnvironmentVariableName
		}
		choices[index] = fmt.Sprintf("%s %s · %s", mark, value.TargetName, source)
	}
	lines := model.choiceLines(choices, model.cursor, 6)
	lines = append(lines, ui.RenderKeyValue("Protection", profileProtectionSummary(draft.Protected)))
	return model.initWizardShell(4, "Review "+draft.Name, []ui.Panel{{Title: "Profile values", Lines: model.withErrorLines(lines), Focused: true}}, []ui.Action{
		{Key: "Enter", Label: "Add profile"}, {Key: "↑/↓", Label: "Move"}, {Key: "e", Label: "Edit"}, {Key: "Space", Label: "Include"}, {Key: "p", Label: "Protected"}, {Key: "Esc", Label: "Name"}, {Key: "q", Label: "Cancel"},
	})
}
