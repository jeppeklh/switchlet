package configeditor

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

func (model Model) handleManagedValueFilesDiscovered(message managedValueFilesDiscoveredMsg) (tea.Model, tea.Cmd) {
	if model.staleManagedValueRequest(message.requestID, managedValueRequestDiscoverFiles) || model.state != editorStateManagedValueFileLoading {
		return model, nil
	}

	model.clearManagedValueRequest()
	model.managedForm.fileCandidates = append([]app.InitTargetFileCandidate(nil), message.candidates...)
	model.managedForm.fileCursor = 0
	model.state = editorStateManagedValueFileSelect
	if message.err != nil {
		model.managedForm.errorMessage = message.err.Error()
	}

	return model, nil
}

func (model Model) handleManagedValueFileInspected(message managedValueFileInspectedMsg) (tea.Model, tea.Cmd) {
	if model.staleManagedValueRequest(message.requestID, managedValueRequestInspectFile) || model.state != editorStateManagedValueSelectorLoading {
		return model, nil
	}

	model.clearManagedValueRequest()
	if message.err != nil {
		model.managedForm.errorMessage = message.err.Error()
		model.state = editorStateManagedValueFileSelect
		return model, nil
	}

	model.managedForm.target.File = message.selection.Path
	model.managedForm.target.Type = message.selection.TargetType
	model.managedForm.selectorOptions = managedValueSelectorOptions(message.selection)
	model.managedForm.selectorCursor = 0
	model.managedForm.selectorFilter = ""
	model.state = editorStateManagedValueSelectorSelect
	if len(model.managedForm.selectorOptions) == 0 {
		model.managedForm.errorMessage = "No existing manageable values were found in the selected file."
	}

	return model, nil
}

func (model Model) handleManagedValueSelectorValidated(message managedValueSelectorValidatedMsg) (tea.Model, tea.Cmd) {
	if model.staleManagedValueRequest(message.requestID, managedValueRequestValidateSelector) || model.state != editorStateManagedValueSelectorValidating {
		return model, nil
	}
	if model.managedForm.target.File != message.targetPath || model.managedForm.target.Type != message.targetType {
		return model, nil
	}

	model.clearManagedValueRequest()
	if message.err != nil {
		model.managedForm.errorMessage = message.err.Error()
		model.state = editorStateManagedValueManualSelectorInput
		return model, nil
	}

	model.setManagedValueSelector(message.selector)
	model.advanceAfterManagedValueSelector()
	return model, nil
}

func (model Model) handleManagedValueFileLoadingKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isBackKey(message):
		model.cancelManagedValueForm()
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleManagedValueFileSelectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	candidates := model.filteredManagedValueFileCandidates()
	model.clampManagedValueFileCursor(len(candidates))

	switch {
	case isMoveUpKey(message):
		if len(candidates) > 0 {
			model.managedForm.fileCursor--
			if model.managedForm.fileCursor < 0 {
				model.managedForm.fileCursor = len(candidates) - 1
			}
		}
	case isMoveDownKey(message):
		if len(candidates) > 0 {
			model.managedForm.fileCursor++
			if model.managedForm.fileCursor >= len(candidates) {
				model.managedForm.fileCursor = 0
			}
		}
	case isFirstKey(message):
		model.managedForm.fileCursor = 0
	case isLastKey(message):
		if len(candidates) > 0 {
			model.managedForm.fileCursor = len(candidates) - 1
		}
	case isOpenKey(message):
		return model, model.chooseManagedValueFile()
	case isRuneKey(message, '/'):
		model.state = editorStateManagedValueFileFilter
		model.inputValue = model.managedForm.fileFilter
		model.inputCursor = len([]rune(model.inputValue))
	case isRuneKey(message, 'm'):
		model.state = editorStateManagedValueManualFileInput
		model.inputValue = ""
		model.inputCursor = 0
		model.managedForm.errorMessage = ""
	case isBackKey(message):
		model.cancelManagedValueForm()
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleManagedValueFileFilterKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.inputValue = ""
		model.inputCursor = 0
		model.state = editorStateManagedValueFileSelect
	case tea.KeyEnter:
		model.managedForm.fileFilter = model.inputValue
		model.managedForm.fileCursor = 0
		model.inputValue = ""
		model.inputCursor = 0
		model.state = editorStateManagedValueFileSelect
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleManagedValueManualFileKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.inputValue = ""
		model.inputCursor = 0
		model.managedForm.errorMessage = ""
		model.state = editorStateManagedValueFileSelect
	case tea.KeyEnter:
		return model, model.applyManagedValueManualFile()
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleManagedValueTypeSelectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	targetTypes := managedValueTargetTypeChoices()
	model.clampManagedValueTypeCursor()
	switch {
	case isMoveUpKey(message):
		model.managedForm.typeCursor--
		if model.managedForm.typeCursor < 0 {
			model.managedForm.typeCursor = len(targetTypes) - 1
		}
	case isMoveDownKey(message):
		model.managedForm.typeCursor++
		if model.managedForm.typeCursor >= len(targetTypes) {
			model.managedForm.typeCursor = 0
		}
	case isFirstKey(message):
		model.managedForm.typeCursor = 0
	case isLastKey(message):
		model.managedForm.typeCursor = len(targetTypes) - 1
	case isOpenKey(message):
		return model, model.chooseManagedValueTargetType()
	case isBackKey(message):
		model.state = editorStateManagedValueManualFileInput
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleManagedValueSelectorLoadingKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isBackKey(message):
		model.clearManagedValueRequest()
		model.state = editorStateManagedValueFileSelect
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleManagedValueSelectorSelectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := model.filteredManagedValueSelectorOptions()
	model.clampManagedValueSelectorCursor(len(options))

	switch {
	case isMoveUpKey(message):
		if len(options) > 0 {
			model.managedForm.selectorCursor--
			if model.managedForm.selectorCursor < 0 {
				model.managedForm.selectorCursor = len(options) - 1
			}
		}
	case isMoveDownKey(message):
		if len(options) > 0 {
			model.managedForm.selectorCursor++
			if model.managedForm.selectorCursor >= len(options) {
				model.managedForm.selectorCursor = 0
			}
		}
	case isFirstKey(message):
		model.managedForm.selectorCursor = 0
	case isLastKey(message):
		if len(options) > 0 {
			model.managedForm.selectorCursor = len(options) - 1
		}
	case isOpenKey(message):
		model.chooseManagedValueSelector()
	case isRuneKey(message, '/'):
		model.state = editorStateManagedValueSelectorFilter
		model.inputValue = model.managedForm.selectorFilter
		model.inputCursor = len([]rune(model.inputValue))
	case isRuneKey(message, 'm'):
		model.state = editorStateManagedValueManualSelectorInput
		model.inputValue = managedValueTargetSelector(model.managedForm.target)
		model.inputCursor = len([]rune(model.inputValue))
		model.managedForm.errorMessage = ""
	case isBackKey(message):
		model.state = editorStateManagedValueFileSelect
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleManagedValueSelectorFilterKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.inputValue = ""
		model.inputCursor = 0
		model.state = editorStateManagedValueSelectorSelect
	case tea.KeyEnter:
		model.managedForm.selectorFilter = model.inputValue
		model.managedForm.selectorCursor = 0
		model.inputValue = ""
		model.inputCursor = 0
		model.state = editorStateManagedValueSelectorSelect
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleManagedValueManualSelectorKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.inputValue = ""
		model.inputCursor = 0
		model.managedForm.errorMessage = ""
		model.state = editorStateManagedValueSelectorSelect
	case tea.KeyEnter:
		return model, model.applyManagedValueManualSelector()
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleManagedValueSelectorValidatingKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isBackKey(message):
		model.clearManagedValueRequest()
		model.state = editorStateManagedValueManualSelectorInput
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleManagedValueNameInputKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		if model.managedForm.mode == managedValueDraftRename {
			model.cancelManagedValueForm()
		} else {
			model.state = editorStateManagedValueSelectorSelect
		}
	case tea.KeyEnter:
		model.applyManagedValueNameInput()
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleManagedValueReviewKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isOpenKey(message):
		model.completeManagedValueDraft()
	case isBackKey(message):
		if model.managedForm.mode == managedValueDraftAdd || model.managedForm.mode == managedValueDraftLocationUpdate {
			model.state = editorStateManagedValueSelectorSelect
		} else {
			model.cancelManagedValueForm()
		}
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) handleManagedValueRemoveConfirmKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case message.Type == tea.KeyEnter || isRuneKey(message, 'y'):
		model.completeManagedValueDraft()
	case message.Type == tea.KeyEsc || isRuneKey(message, 'n') || isRuneKey(message, 'h'):
		model.cancelManagedValueForm()
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}
