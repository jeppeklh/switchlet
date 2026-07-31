package configeditor

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

func (model Model) handleProfileNameInputKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.cancelProfileForm()
	case tea.KeyEnter:
		model.applyProfileNameInput()
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleProfileIncludeValuesKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	model.clampProfileIncludeCursor()
	switch {
	case isMoveUpKey(message):
		model.profileForm.includeCursor--
		if model.profileForm.includeCursor < 0 {
			model.profileForm.includeCursor = len(model.profileForm.draft.Values) - 1
		}
	case isMoveDownKey(message):
		model.profileForm.includeCursor++
		if model.profileForm.includeCursor >= len(model.profileForm.draft.Values) {
			model.profileForm.includeCursor = 0
		}
	case isFirstKey(message):
		model.profileForm.includeCursor = 0
	case isLastKey(message):
		model.profileForm.includeCursor = len(model.profileForm.draft.Values) - 1
	case message.Type == tea.KeySpace:
		if len(model.profileForm.draft.Values) > 0 {
			value := &model.profileForm.draft.Values[model.profileForm.includeCursor]
			value.Included = !value.Included
			if value.Included && value.Source == "" {
				value.Source = app.ProfileSourceLiteral
			}
		}
	case isRuneKey(message, 'p'):
		model.profileForm.draft.Protected = !model.profileForm.draft.Protected
	case isRuneKey(message, 'e'):
		if len(model.profileForm.draft.Values) > 0 {
			model.profileForm.draft.Values[model.profileForm.includeCursor].Included = true
			model.beginProfileValueSource(model.profileForm.includeCursor)
		}
	case isOpenKey(message):
		if model.includedProfileValueCount() == 0 {
			model.profileForm.errorMessage = "Include at least one target."
			return model, nil
		}
		if nextIndex, ok := model.nextIncompleteIncludedProfileValue(0); ok {
			model.beginProfileValueSource(nextIndex)
			return model, nil
		}
		model.profileForm.errorMessage = ""
		model.state = editorStateProfileReview
	case isBackKey(message):
		model.cancelProfileForm()
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleProfileValueSourceKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isMoveUpKey(message) || isMoveDownKey(message) || message.Type == tea.KeySpace:
		if model.profileForm.sourceCursor == 0 {
			model.profileForm.sourceCursor = 1
		} else {
			model.profileForm.sourceCursor = 0
		}
	case isFirstKey(message):
		model.profileForm.sourceCursor = 0
	case isLastKey(message):
		model.profileForm.sourceCursor = 1
	case isOpenKey(message):
		if model.profileForm.valueIndex >= 0 && model.profileForm.valueIndex < len(model.profileForm.draft.Values) {
			if model.profileForm.sourceCursor == 1 {
				model.profileForm.draft.Values[model.profileForm.valueIndex].Source = app.ProfileSourceEnvironment
			} else {
				model.profileForm.draft.Values[model.profileForm.valueIndex].Source = app.ProfileSourceLiteral
			}
			model.beginProfileValueInput(model.profileForm.valueIndex)
		}
	case isBackKey(message):
		model.state = editorStateProfileIncludeValues
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleProfileValueInputKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.beginProfileValueSource(model.profileForm.valueIndex)
	case tea.KeyEnter:
		model.applyProfileValueInput()
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleProfileReviewKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case message.Type == tea.KeySpace || isRuneKey(message, 'p'):
		model.profileForm.draft.Protected = !model.profileForm.draft.Protected
	case isOpenKey(message):
		model.completeProfileDraft()
	case isBackKey(message):
		model.state = editorStateProfileIncludeValues
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleProfileRemoveConfirmKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case message.Type == tea.KeyEnter || isRuneKey(message, 'y'):
		model.removeProfileDraft()
	case message.Type == tea.KeyEsc || isRuneKey(message, 'n') || isRuneKey(message, 'h'):
		model.cancelProfileForm()
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}
