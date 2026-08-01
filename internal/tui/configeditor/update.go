package configeditor

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

// Update handles Bubble Tea messages for the config editor.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case saveCompletedMsg:
		return model.handleSaveCompleted(message)
	case managedValueFilesDiscoveredMsg:
		return model.handleManagedValueFilesDiscovered(message)
	case managedValueFileInspectedMsg:
		return model.handleManagedValueFileInspected(message)
	case managedValueSelectorValidatedMsg:
		return model.handleManagedValueSelectorValidated(message)
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case tea.KeyMsg:
		if message.Type == tea.KeyCtrlC {
			if model.state == editorStateSaveSuccess {
				model.completeSaved()
				return model, tea.Quit
			}

			model.cancel()
			return model, tea.Quit
		}
		if model.isTerminalTooSmall() {
			if isQuitKey(message) {
				model.cancel()
				return model, tea.Quit
			}
			return model, nil
		}

		switch model.state {
		case editorStateFilter:
			return model.handleFilterKey(message)
		case editorStateProfileNameInput:
			return model.handleProfileNameInputKey(message)
		case editorStateProfileIncludeValues:
			return model.handleProfileIncludeValuesKey(message)
		case editorStateProfileValueSource:
			return model.handleProfileValueSourceKey(message)
		case editorStateProfileValueInput:
			return model.handleProfileValueInputKey(message)
		case editorStateProfileReview:
			return model.handleProfileReviewKey(message)
		case editorStateProfileRemoveConfirm:
			return model.handleProfileRemoveConfirmKey(message)
		case editorStateManagedValueFileLoading:
			return model.handleManagedValueFileLoadingKey(message)
		case editorStateManagedValueFileSelect:
			return model.handleManagedValueFileSelectKey(message)
		case editorStateManagedValueFileFilter:
			return model.handleManagedValueFileFilterKey(message)
		case editorStateManagedValueManualFileInput:
			return model.handleManagedValueManualFileKey(message)
		case editorStateManagedValueTypeSelect:
			return model.handleManagedValueTypeSelectKey(message)
		case editorStateManagedValueSelectorLoading:
			return model.handleManagedValueSelectorLoadingKey(message)
		case editorStateManagedValueSelectorSelect:
			return model.handleManagedValueSelectorSelectKey(message)
		case editorStateManagedValueSelectorFilter:
			return model.handleManagedValueSelectorFilterKey(message)
		case editorStateManagedValueManualSelectorInput:
			return model.handleManagedValueManualSelectorKey(message)
		case editorStateManagedValueSelectorValidating:
			return model.handleManagedValueSelectorValidatingKey(message)
		case editorStateManagedValueNameInput:
			return model.handleManagedValueNameInputKey(message)
		case editorStateManagedValueReview:
			return model.handleManagedValueReviewKey(message)
		case editorStateManagedValueRemoveConfirm:
			return model.handleManagedValueRemoveConfirmKey(message)
		case editorStateDirtyQuitConfirm:
			return model.handleDirtyQuitKey(message)
		case editorStateSaving:
			return model, nil
		case editorStateSaveSuccess:
			return model.handleSaveSuccessKey(message)
		default:
			return model.handleOverviewKey(message)
		}
	default:
		return model, nil
	}
}

func (model Model) handleOverviewKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	overview := model.overview()
	rows := model.navigationRows(overview)
	model.clampCursor(len(rows))

	switch {
	case isPreviousTabKey(message):
		model.selectPreviousOverviewTab()
		model.filter = ""
		rows = model.navigationRows(overview)
		model.clampCursor(len(rows))
	case isNextTabKey(message):
		model.selectNextOverviewTab()
		model.filter = ""
		rows = model.navigationRows(overview)
		model.clampCursor(len(rows))
	case isMoveUpKey(message):
		if len(rows) > 0 {
			model.cursor--
			if model.cursor < 0 {
				model.cursor = len(rows) - 1
			}
		}
	case isMoveDownKey(message):
		if len(rows) > 0 {
			model.cursor++
			if model.cursor >= len(rows) {
				model.cursor = 0
			}
		}
	case isFirstKey(message):
		model.cursor = 0
	case isLastKey(message):
		if len(rows) > 0 {
			model.cursor = len(rows) - 1
		}
	case message.Type == tea.KeyEsc:
		if model.filter != "" {
			model.filter = ""
			model.cursor = 0
		}
	case model.embedded && isRuneKey(message, 'c'):
		return model.beginReturnToPicker()
	case isRuneKey(message, '/'):
		if model.activeTab != overviewTabReview {
			model.beginFilter()
		}
	case model.filter != "" && isRuneKey(message, 'n'):
		model.moveToNextFilteredRow(rows)
	case model.filter != "" && isRuneKey(message, 'N'):
		model.moveToPreviousFilteredRow(rows)
	case isRuneKey(message, 'a'):
		selectedRow := model.selectedRow(rows)
		if selectedRow.Kind == navigationRowProfilesSection || selectedRow.Kind == navigationRowProfile {
			model.beginAddProfile()
		} else if selectedRow.Kind == navigationRowManagedValuesSection || selectedRow.Kind == navigationRowManagedValue {
			return model, model.beginAddManagedValue()
		}
	case isRuneKey(message, 'e'):
		selectedRow := model.selectedRow(rows)
		if selectedRow.Kind == navigationRowProfile {
			model.beginEditProfile(selectedRow.Label)
		} else if selectedRow.Kind == navigationRowManagedValue {
			return model, model.beginEditManagedValueLocation(selectedRow.Label)
		}
	case isRuneKey(message, 'r'):
		selectedRow := model.selectedRow(rows)
		if selectedRow.Kind == navigationRowProfile {
			model.beginRenameProfile(selectedRow.Label)
		} else if selectedRow.Kind == navigationRowManagedValue {
			model.beginRenameManagedValue(selectedRow.Label)
		}
	case isRuneKey(message, 'd'):
		selectedRow := model.selectedRow(rows)
		if selectedRow.Kind == navigationRowProfile {
			model.beginRemoveProfile(selectedRow.Label)
		} else if selectedRow.Kind == navigationRowManagedValue {
			model.beginRemoveManagedValue(selectedRow.Label)
		}
	case message.Type == tea.KeySpace:
		if selectedRow := model.selectedRow(rows); selectedRow.Kind == navigationRowProfile {
			model.beginToggleProfileProtected(selectedRow.Label)
		}
	case isRuneKey(message, 's'):
		if model.selectedRow(rows).Kind == navigationRowReview && overview.Saveable {
			model.state = editorStateSaving
			model.saveError = ""
			return model, saveDocument(model.workflow, model.document)
		}
	case isQuitKey(message):
		return model.beginQuit()
	}

	return model, nil
}

func (model Model) beginQuit() (tea.Model, tea.Cmd) {
	if model.overview().Dirty {
		model.returnToPicker = false
		model.state = editorStateDirtyQuitConfirm
		return model, nil
	}

	model.quitWithoutSaving()
	return model, tea.Quit
}

func (model Model) beginReturnToPicker() (tea.Model, tea.Cmd) {
	if model.overview().Dirty {
		model.returnToPicker = true
		model.state = editorStateDirtyQuitConfirm
		return model, nil
	}

	model.closeWithoutSaving()
	return model, nil
}

func (model *Model) openSelectedRow(rows []navigationRow) {
	selectedRow := model.selectedRow(rows)
	switch selectedRow.Kind {
	case navigationRowProfilesSection:
		for index, row := range rows {
			if row.Kind == navigationRowProfile {
				model.cursor = index
				return
			}
		}
	case navigationRowManagedValuesSection:
		for index, row := range rows {
			if row.Kind == navigationRowManagedValue {
				model.cursor = index
				return
			}
		}
	}
}

func (model Model) selectedRow(rows []navigationRow) navigationRow {
	if len(rows) == 0 {
		switch model.activeTab {
		case overviewTabTargets:
			return navigationRow{Kind: navigationRowManagedValuesSection, Label: "Managed values"}
		case overviewTabReview:
			return navigationRow{Kind: navigationRowReview, Label: "Review changes"}
		default:
			return navigationRow{Kind: navigationRowProfilesSection, Label: "Profiles"}
		}
	}
	if model.cursor < 0 {
		return rows[0]
	}
	if model.cursor >= len(rows) {
		return rows[len(rows)-1]
	}

	return rows[model.cursor]
}

func (model *Model) moveToNextFilteredRow(rows []navigationRow) {
	if len(rows) == 0 {
		return
	}

	for offset := 1; offset <= len(rows); offset++ {
		candidate := (model.cursor + offset) % len(rows)
		if rows[candidate].Kind == navigationRowProfile || rows[candidate].Kind == navigationRowManagedValue {
			model.cursor = candidate
			return
		}
	}
}

func (model *Model) moveToPreviousFilteredRow(rows []navigationRow) {
	if len(rows) == 0 {
		return
	}

	for offset := 1; offset <= len(rows); offset++ {
		candidate := model.cursor - offset
		if candidate < 0 {
			candidate += len(rows)
		}
		if rows[candidate].Kind == navigationRowProfile || rows[candidate].Kind == navigationRowManagedValue {
			model.cursor = candidate
			return
		}
	}
}

func (model Model) handleFilterKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.cancelFilter()
	case tea.KeyEnter:
		model.applyFilter()
	default:
		model.handleInputEditKey(message)
	}

	return model, nil
}

func (model Model) handleDirtyQuitKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case message.Type == tea.KeyEnter || isRuneKey(message, 'y'):
		if model.returnToPicker {
			model.returnToPicker = false
			model.closeWithoutSaving()
			return model, nil
		}

		model.cancel()
		return model, tea.Quit
	case message.Type == tea.KeyEsc || isRuneKey(message, 'n') || isRuneKey(message, 'h'):
		model.returnToPicker = false
		model.state = editorStateOverview
	}

	return model, nil
}

func (model Model) handleSaveSuccessKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEnter || isQuitKey(message) {
		model.completeSaved()
		return model, tea.Quit
	}

	return model, nil
}

func (model Model) handleSaveCompleted(message saveCompletedMsg) (tea.Model, tea.Cmd) {
	if message.err != nil {
		model.state = editorStateOverview
		model.saveError = message.err.Error()
		model.selectReviewOverview()
		return model, nil
	}

	model.state = editorStateSaveSuccess
	model.savedConfigPath = message.configPath
	model.savedChanges = append([]app.ConfigEditChange(nil), message.changes...)
	return model, nil
}

func (model *Model) selectReviewOverview() {
	model.selectOverviewTab(overviewTabReview)
}

func (model *Model) handleInputEditKey(message tea.KeyMsg) bool {
	switch message.Type {
	case tea.KeyLeft, tea.KeyCtrlB:
		model.moveInputCursorLeft()
	case tea.KeyRight, tea.KeyCtrlF:
		model.moveInputCursorRight()
	case tea.KeyHome, tea.KeyCtrlA:
		model.moveInputCursorToStart()
	case tea.KeyEnd, tea.KeyCtrlE:
		model.moveInputCursorToEnd()
	case tea.KeyBackspace:
		model.deleteInputRuneBeforeCursor()
	case tea.KeyDelete:
		model.deleteInputRuneAtCursor()
	case tea.KeyCtrlU:
		model.clearInputValue()
	case tea.KeyCtrlK:
		model.clampInputCursor()
		model.inputValue = string([]rune(model.inputValue)[:model.inputCursor])
	case tea.KeySpace:
		model.insertInputValue(" ")
	case tea.KeyRunes:
		model.insertInputValue(string(message.Runes))
	default:
		return false
	}

	return true
}

func (model *Model) clampInputCursor() {
	inputLength := len([]rune(model.inputValue))
	if model.inputCursor < 0 {
		model.inputCursor = 0
	}
	if model.inputCursor > inputLength {
		model.inputCursor = inputLength
	}
}

func (model *Model) moveInputCursorLeft() {
	model.clampInputCursor()
	if model.inputCursor > 0 {
		model.inputCursor--
	}
}

func (model *Model) moveInputCursorRight() {
	model.clampInputCursor()
	if model.inputCursor < len([]rune(model.inputValue)) {
		model.inputCursor++
	}
}

func (model *Model) moveInputCursorToStart() {
	model.inputCursor = 0
}

func (model *Model) moveInputCursorToEnd() {
	model.inputCursor = len([]rune(model.inputValue))
}

func (model *Model) insertInputValue(value string) {
	model.clampInputCursor()
	currentRunes := []rune(model.inputValue)
	insertedRunes := []rune(value)

	updatedRunes := make([]rune, 0, len(currentRunes)+len(insertedRunes))
	updatedRunes = append(updatedRunes, currentRunes[:model.inputCursor]...)
	updatedRunes = append(updatedRunes, insertedRunes...)
	updatedRunes = append(updatedRunes, currentRunes[model.inputCursor:]...)

	model.inputValue = string(updatedRunes)
	model.inputCursor += len(insertedRunes)
}

func (model *Model) deleteInputRuneBeforeCursor() {
	model.clampInputCursor()
	if model.inputCursor == 0 {
		return
	}

	currentRunes := []rune(model.inputValue)
	updatedRunes := make([]rune, 0, len(currentRunes)-1)
	updatedRunes = append(updatedRunes, currentRunes[:model.inputCursor-1]...)
	updatedRunes = append(updatedRunes, currentRunes[model.inputCursor:]...)

	model.inputValue = string(updatedRunes)
	model.inputCursor--
}

func (model *Model) deleteInputRuneAtCursor() {
	model.clampInputCursor()
	currentRunes := []rune(model.inputValue)
	if model.inputCursor >= len(currentRunes) {
		return
	}

	updatedRunes := make([]rune, 0, len(currentRunes)-1)
	updatedRunes = append(updatedRunes, currentRunes[:model.inputCursor]...)
	updatedRunes = append(updatedRunes, currentRunes[model.inputCursor+1:]...)

	model.inputValue = string(updatedRunes)
}

func (model *Model) clearInputValue() {
	model.inputValue = ""
	model.inputCursor = 0
}
