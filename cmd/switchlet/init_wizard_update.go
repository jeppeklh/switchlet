package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/config"
)

// Update handles Bubble Tea messages for the init wizard.
func (model initWizardModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case tea.KeyMsg:
		if message.Type == tea.KeyCtrlC {
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
		if !isTextEntryStep(model.step) && isQuitKey(message) {
			model.cancel()
			return model, tea.Quit
		}

		switch model.step {
		case initWizardStepFileSelect:
			return model.handleFileSelectKey(message)
		case initWizardStepFileFilter:
			return model.handleFileFilterKey(message)
		case initWizardStepManualFile:
			return model.handleManualFileKey(message)
		case initWizardStepTypeSelect:
			return model.handleTypeSelectKey(message)
		case initWizardStepPathBrowse:
			return model.handlePathBrowseKey(message)
		case initWizardStepPathSearch:
			return model.handlePathSearchKey(message)
		case initWizardStepManualPath:
			return model.handleManualPathKey(message)
		case initWizardStepDotenvKeySelect:
			return model.handleDotenvKeySelectKey(message)
		case initWizardStepManualDotenvKey:
			return model.handleManualDotenvKeyKey(message)
		case initWizardStepTargetName:
			return model.handleTargetNameKey(message)
		case initWizardStepTargetSummary:
			return model.handleTargetSummaryKey(message)
		case initWizardStepProfileName:
			return model.handleProfileNameKey(message)
		case initWizardStepProfileTargetInclude:
			return model.handleProfileTargetIncludeKey(message)
		case initWizardStepProfileSource:
			return model.handleProfileSourceKey(message)
		case initWizardStepProfileValue:
			return model.handleProfileValueKey(message)
		case initWizardStepProfileProtected:
			return model.handleProfileProtectedKey(message)
		case initWizardStepProfileSummary:
			return model.handleProfileSummaryKey(message)
		case initWizardStepReview:
			return model.handleReviewKey(message)
		default:
			return model, nil
		}
	default:
		return model, nil
	}
}

func (model initWizardModel) handleFileSelectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	matchingCandidates := model.filteredFileCandidates(model.fileFilter)
	model.clampCursor(len(matchingCandidates))

	switch {
	case isMoveUpKey(message):
		if len(matchingCandidates) > 0 {
			model.cursor--
			if model.cursor < 0 {
				model.cursor = len(matchingCandidates) - 1
			}
		}
	case isMoveDownKey(message):
		if len(matchingCandidates) > 0 {
			model.cursor++
			if model.cursor >= len(matchingCandidates) {
				model.cursor = 0
			}
		}
	case isRuneKey(message, 'f'), isRuneKey(message, '/'):
		model.step = initWizardStepFileFilter
		model.cursor = 0
		model.errorMessage = ""
		model.setInputValue(model.fileFilter)
	case isRuneKey(message, 'm'):
		model.step = initWizardStepManualFile
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
	case message.Type == tea.KeyEnter:
		if len(matchingCandidates) == 0 {
			return model, nil
		}

		selectedCandidate := matchingCandidates[model.cursor]
		selectedFile, err := inspectTargetFileCandidate(selectedCandidate, model.dependencies)
		if err != nil {
			model.errorMessage = err.Error()
			return model, nil
		}

		model.selectedFile = selectedFile
		model.beginSelectorForSelectedFile()
	}

	return model, nil
}

func (model initWizardModel) handleFileFilterKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	matchingCandidates := model.filteredFileCandidates(model.inputValue)
	model.clampCursor(len(matchingCandidates))

	switch message.Type {
	case tea.KeyEsc:
		model.fileFilter = strings.TrimSpace(model.inputValue)
		model.step = initWizardStepFileSelect
		model.errorMessage = ""
		model.clearInputValue()
		model.clampCursor(len(model.filteredFileCandidates(model.fileFilter)))
		return model, nil
	case tea.KeyEnter:
		if len(matchingCandidates) == 0 {
			model.errorMessage = fmt.Sprintf("No discovered target files match %q.", model.inputValue)
			return model, nil
		}

		selectedCandidate := matchingCandidates[model.cursor]
		selectedFile, err := inspectTargetFileCandidate(selectedCandidate, model.dependencies)
		if err != nil {
			model.errorMessage = err.Error()
			return model, nil
		}

		model.fileFilter = strings.TrimSpace(model.inputValue)
		model.selectedFile = selectedFile
		model.beginSelectorForSelectedFile()
		return model, nil
	case tea.KeyUp:
		if len(matchingCandidates) > 0 {
			model.cursor--
			if model.cursor < 0 {
				model.cursor = len(matchingCandidates) - 1
			}
		}
		return model, nil
	case tea.KeyDown:
		if len(matchingCandidates) > 0 {
			model.cursor++
			if model.cursor >= len(matchingCandidates) {
				model.cursor = 0
			}
		}
		return model, nil
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.errorMessage = ""
		model.clampCursor(len(model.filteredFileCandidates(model.inputValue)))
		return model, nil
	}
}

func (model initWizardModel) handleManualFileKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	return model.handleTextInputKey(message, initWizardStepFileSelect, func(value string) (initWizardModel, error) {
		resolvedTargetPath := resolveTargetPath(model.workingDirectory, value)
		targetType, ok := config.InferTargetType(resolvedTargetPath)
		if !ok {
			model.selectedFile = targetFileSelection{
				path:        resolvedTargetPath,
				displayPath: displayTargetPath(model.workingDirectory, resolvedTargetPath),
			}
			model.step = initWizardStepTypeSelect
			model.cursor = 0
			model.errorMessage = ""
			model.clearInputValue()
			return model, nil
		}

		selectedFile, err := inspectTargetFile(resolvedTargetPath, displayTargetPath(model.workingDirectory, resolvedTargetPath), targetType, model.dependencies)
		if err != nil {
			return model, err
		}

		model.selectedFile = selectedFile
		model.beginSelectorForSelectedFile()
		return model, nil
	})
}

func (model *initWizardModel) beginSelectorForSelectedFile() {
	if model.selectedFile.targetType == config.TargetTypeDotenv {
		model.beginDotenvKeySelect()
		return
	}

	model.beginPathBrowse()
}

func (model initWizardModel) handleTypeSelectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isMoveUpKey(message), isMoveDownKey(message):
		model.cursor = 1 - model.cursor
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepManualFile
		model.cursor = 0
		model.errorMessage = ""
		model.setInputValue(model.selectedFile.displayPath)
	case message.Type == tea.KeyEnter:
		targetType := config.TargetTypeJSON
		if model.cursor == 1 {
			targetType = config.TargetTypeDotenv
		}

		selectedFile, err := inspectTargetFile(model.selectedFile.path, model.selectedFile.displayPath, targetType, model.dependencies)
		if err != nil {
			model.errorMessage = err.Error()
			return model, nil
		}

		model.selectedFile = selectedFile
		model.beginSelectorForSelectedFile()
	}

	return model, nil
}

func (model initWizardModel) handleDotenvKeySelectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	keys := model.selectedFile.dotenvKeys
	choiceCount := len(keys) + 1
	model.clampCursor(choiceCount)

	switch {
	case isMoveUpKey(message):
		model.cursor--
		if model.cursor < 0 {
			model.cursor = choiceCount - 1
		}
	case isMoveDownKey(message):
		model.cursor++
		if model.cursor >= choiceCount {
			model.cursor = 0
		}
	case isRuneKey(message, 'm'):
		model.step = initWizardStepManualDotenvKey
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepFileSelect
		model.cursor = 0
		model.errorMessage = ""
	case message.Type == tea.KeyEnter:
		if model.cursor < len(keys) {
			model.selectedDotenvKey = keys[model.cursor]
			model.beginTargetName()
			return model, nil
		}

		model.step = initWizardStepManualDotenvKey
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
	}

	return model, nil
}

func (model initWizardModel) handleManualDotenvKeyKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	return model.handleTextInputKey(message, initWizardStepDotenvKeySelect, func(value string) (initWizardModel, error) {
		if err := model.dependencies.validateDotenvTarget(model.selectedFile.path, value); err != nil {
			return model, err
		}

		model.selectedDotenvKey = value
		model.beginTargetName()
		return model, nil
	})
}

func (model initWizardModel) handleTargetNameKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	previousStep := initWizardStepPathBrowse
	if model.selectedFile.targetType == config.TargetTypeDotenv {
		previousStep = initWizardStepDotenvKeySelect
	}

	return model.handleTextInputKey(message, previousStep, func(value string) (initWizardModel, error) {
		if model.targetNameExists(value) {
			return model, fmt.Errorf("target name %q is already configured", value)
		}

		model.appendTarget(value)
		return model, nil
	})
}

func (model initWizardModel) handleTargetSummaryKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	choiceCount := 3
	model.clampCursor(choiceCount)

	switch {
	case isMoveUpKey(message):
		model.cursor--
		if model.cursor < 0 {
			model.cursor = choiceCount - 1
		}
	case isMoveDownKey(message):
		model.cursor++
		if model.cursor >= choiceCount {
			model.cursor = 0
		}
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepFileSelect
		model.cursor = 0
		model.errorMessage = ""
	case message.Type == tea.KeyEnter:
		switch model.cursor {
		case 0:
			model.beginProfileEntry()
		case 1:
			model.step = initWizardStepFileSelect
			model.cursor = 0
			model.errorMessage = ""
		case 2:
			model.removeLastTarget()
		}
	}

	return model, nil
}

func (model initWizardModel) handlePathBrowseKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	choices := len(model.browseNodes)
	if len(model.browseAncestors) > 0 {
		choices++
	}
	choices += 2
	model.clampCursor(choices)

	switch {
	case isMoveUpKey(message):
		model.cursor--
		if model.cursor < 0 {
			model.cursor = choices - 1
		}
	case isMoveDownKey(message):
		model.cursor++
		if model.cursor >= choices {
			model.cursor = 0
		}
	case isRuneKey(message, 's'), isRuneKey(message, '/'):
		model.step = initWizardStepPathSearch
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
	case isRuneKey(message, 'm'):
		model.step = initWizardStepManualPath
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
	case message.Type == tea.KeyEsc:
		if len(model.browseAncestors) > 0 {
			previousLevel := model.browseAncestors[len(model.browseAncestors)-1]
			model.browseAncestors = model.browseAncestors[:len(model.browseAncestors)-1]
			model.browseNodes = previousLevel.nodes
			model.cursor = 0
			model.errorMessage = ""
		} else {
			model.step = initWizardStepFileSelect
			model.cursor = 0
			model.errorMessage = ""
		}
	case message.Type == tea.KeyEnter:
		if model.cursor < len(model.browseNodes) {
			selectedNode := model.browseNodes[model.cursor]
			if selectedNode.Selectable {
				model.selectedJSONPath = selectedNode.JSONPath
				model.beginTargetName()
				return model, nil
			}

			model.browseAncestors = append(model.browseAncestors, targetBrowseLevel{
				path:  selectedNode.JSONPath,
				nodes: model.browseNodes,
			})
			model.browseNodes = selectedNode.Children
			model.cursor = 0
			model.errorMessage = ""
			return model, nil
		}

		actionIndex := len(model.browseNodes)
		if len(model.browseAncestors) > 0 {
			if model.cursor == actionIndex {
				previousLevel := model.browseAncestors[len(model.browseAncestors)-1]
				model.browseAncestors = model.browseAncestors[:len(model.browseAncestors)-1]
				model.browseNodes = previousLevel.nodes
				model.cursor = 0
				model.errorMessage = ""
				return model, nil
			}
			actionIndex++
		}

		if model.cursor == actionIndex {
			model.step = initWizardStepPathSearch
			model.cursor = 0
			model.errorMessage = ""
			model.inputValue = ""
			return model, nil
		}

		model.step = initWizardStepManualPath
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = ""
	}

	return model, nil
}

func (model initWizardModel) handlePathSearchKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	matchingPaths := model.filteredSelectableJSONPaths(model.inputValue)
	model.clampCursor(len(matchingPaths))

	switch message.Type {
	case tea.KeyEsc:
		model.step = initWizardStepPathBrowse
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
		return model, nil
	case tea.KeyEnter:
		if len(matchingPaths) == 0 {
			model.errorMessage = fmt.Sprintf("No selectable JSON paths in %s match %q.", model.selectedFile.displayPath, model.inputValue)
			return model, nil
		}

		model.selectedJSONPath = matchingPaths[model.cursor]
		model.beginTargetName()
		return model, nil
	case tea.KeyUp:
		if len(matchingPaths) > 0 {
			model.cursor--
			if model.cursor < 0 {
				model.cursor = len(matchingPaths) - 1
			}
		}
		return model, nil
	case tea.KeyDown:
		if len(matchingPaths) > 0 {
			model.cursor++
			if model.cursor >= len(matchingPaths) {
				model.cursor = 0
			}
		}
		return model, nil
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.errorMessage = ""
		model.clampCursor(len(model.filteredSelectableJSONPaths(model.inputValue)))
		return model, nil
	}
}

func (model initWizardModel) handleManualPathKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	return model.handleTextInputKey(message, initWizardStepPathBrowse, func(value string) (initWizardModel, error) {
		if err := model.dependencies.validateStringTarget(model.selectedFile.path, value); err != nil {
			return model, err
		}

		model.selectedJSONPath = value
		model.beginTargetName()
		return model, nil
	})
}

func (model initWizardModel) handleProfileNameKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	previousStep := initWizardStepTargetSummary
	if len(model.profiles) > 0 {
		previousStep = initWizardStepProfileSummary
	}

	return model.handleTextInputKey(message, previousStep, func(value string) (initWizardModel, error) {
		if model.profileNameExists(value) {
			return model, fmt.Errorf("profile name %q is already configured", value)
		}

		model.draftProfile = initWizardProfileDraft{Name: value, Values: make([]config.ProfileValue, 0, len(model.targets))}
		if len(model.targets) == 1 {
			model.beginProfileSource()
		} else {
			model.beginProfileTargetInclude()
		}
		return model, nil
	})
}

func (model initWizardModel) handleProfileTargetIncludeKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isMoveUpKey(message), isMoveDownKey(message):
		model.cursor = 1 - model.cursor
	case message.Type == tea.KeyEsc:
		if model.draftProfile.TargetIndex == 0 {
			model.step = initWizardStepProfileName
			model.cursor = 0
			model.errorMessage = ""
			model.setInputValue(model.draftProfile.Name)
			return model, nil
		}

		model.draftProfile.TargetIndex--
		model.beginProfileTargetInclude()
	case message.Type == tea.KeyEnter:
		if model.cursor == 0 {
			model.beginProfileSource()
			return model, nil
		}

		model.advanceDraftProfileTarget()
	}

	return model, nil
}

func (model initWizardModel) handleProfileSourceKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isMoveUpKey(message), isMoveDownKey(message):
		model.cursor = 1 - model.cursor
	case message.Type == tea.KeyEsc:
		if len(model.targets) == 1 {
			model.step = initWizardStepProfileName
			model.cursor = 0
			model.errorMessage = ""
			model.setInputValue(model.draftProfile.Name)
		} else {
			model.beginProfileTargetInclude()
		}
	case message.Type == tea.KeyEnter:
		model.draftProfile.UseEnvironment = model.cursor == 1
		model.step = initWizardStepProfileValue
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
	}

	return model, nil
}

func (model initWizardModel) handleProfileValueKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	return model.handleTextInputKey(message, initWizardStepProfileSource, func(value string) (initWizardModel, error) {
		model.draftProfile.Value = value
		model.appendDraftProfileValue()
		return model, nil
	})
}

func (model initWizardModel) handleProfileProtectedKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isMoveUpKey(message), isMoveDownKey(message):
		model.cursor = 1 - model.cursor
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepProfileValue
		model.cursor = 0
		model.errorMessage = ""
		model.setInputValue(model.draftProfile.Value)
	case message.Type == tea.KeyEnter:
		model.draftProfile.Protected = model.cursor == 1
		model.appendDraftProfile()
	}

	return model, nil
}

func (model initWizardModel) handleProfileSummaryKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	choiceCount := 3
	if len(model.profiles) > 0 {
		choiceCount++
	}
	model.clampCursor(choiceCount)

	switch {
	case isMoveUpKey(message):
		model.cursor--
		if model.cursor < 0 {
			model.cursor = choiceCount - 1
		}
	case isMoveDownKey(message):
		model.cursor++
		if model.cursor >= choiceCount {
			model.cursor = 0
		}
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepTargetSummary
		model.cursor = 0
		model.errorMessage = ""
	case message.Type == tea.KeyEnter:
		switch model.cursor {
		case 0:
			model.beginReview()
		case 1:
			model.beginProfileEntry()
		case 2:
			model.removeLastProfile()
		default:
			model.step = initWizardStepTargetSummary
			model.cursor = 0
			model.errorMessage = ""
		}
	}

	return model, nil
}

func (model initWizardModel) handleReviewKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	choiceCount := 2
	if hasLiteralProfiles(model.profiles) {
		choiceCount++
	}
	model.clampCursor(choiceCount)

	switch {
	case isMoveUpKey(message):
		model.cursor--
		if model.cursor < 0 {
			model.cursor = choiceCount - 1
		}
	case isMoveDownKey(message):
		model.cursor++
		if model.cursor >= choiceCount {
			model.cursor = 0
		}
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepProfileSummary
		model.cursor = 0
		model.errorMessage = ""
	case message.Type == tea.KeyEnter:
		if model.cursor == 0 {
			model.complete()
			return model, tea.Quit
		}

		if hasLiteralProfiles(model.profiles) && model.cursor == 1 {
			model.shouldIgnoreConfig = !model.shouldIgnoreConfig
			model.shouldIgnoreConfigSet = true
			return model, nil
		}

		model.step = initWizardStepProfileSummary
		model.cursor = 0
		model.errorMessage = ""
	}

	return model, nil
}

func (model *initWizardModel) handleInputEditKey(message tea.KeyMsg) bool {
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

func (model initWizardModel) handleTextInputKey(message tea.KeyMsg, previousStep initWizardStep, submit func(string) (initWizardModel, error)) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.step = previousStep
		model.cursor = 0
		model.errorMessage = ""
		model.clearInputValue()
		return model, nil
	case tea.KeyEnter:
		enteredValue := strings.TrimSpace(model.inputValue)
		if enteredValue == "" {
			model.errorMessage = "value must not be empty"
			return model, nil
		}

		nextModel, err := submit(enteredValue)
		if err != nil {
			model.errorMessage = err.Error()
			return model, nil
		}

		return nextModel, nil
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.errorMessage = ""
		return model, nil
	}
}

func isMoveUpKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyUp || isRuneKey(message, 'k')
}

func isMoveDownKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyDown || isRuneKey(message, 'j')
}

func isQuitKey(message tea.KeyMsg) bool {
	return isRuneKey(message, 'q')
}

func isRuneKey(message tea.KeyMsg, key rune) bool {
	return message.Type == tea.KeyRunes && message.String() == string(key)
}
