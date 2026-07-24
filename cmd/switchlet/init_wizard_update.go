package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
		case initWizardStepPathBrowse:
			return model.handlePathBrowseKey(message)
		case initWizardStepPathSearch:
			return model.handlePathSearchKey(message)
		case initWizardStepManualPath:
			return model.handleManualPathKey(message)
		case initWizardStepProfileName:
			return model.handleProfileNameKey(message)
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
		model.inputValue = model.fileFilter
	case isRuneKey(message, 'm'):
		model.step = initWizardStepManualFile
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = ""
	case message.Type == tea.KeyEnter:
		if len(matchingCandidates) == 0 {
			return model, nil
		}

		selectedCandidate := matchingCandidates[model.cursor]
		nodes, err := model.dependencies.inspectStringTargets(selectedCandidate.Path)
		if err != nil {
			model.errorMessage = err.Error()
			return model, nil
		}

		model.selectedFile = targetFileSelection{
			path:        selectedCandidate.Path,
			displayPath: selectedCandidate.RelativePath,
			nodes:       nodes,
		}
		model.beginPathBrowse()
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
		model.inputValue = ""
		model.clampCursor(len(model.filteredFileCandidates(model.fileFilter)))
		return model, nil
	case tea.KeyEnter:
		if len(matchingCandidates) == 0 {
			model.errorMessage = fmt.Sprintf("No discovered target JSON files match %q.", model.inputValue)
			return model, nil
		}

		selectedCandidate := matchingCandidates[model.cursor]
		nodes, err := model.dependencies.inspectStringTargets(selectedCandidate.Path)
		if err != nil {
			model.errorMessage = err.Error()
			return model, nil
		}

		model.fileFilter = strings.TrimSpace(model.inputValue)
		model.selectedFile = targetFileSelection{
			path:        selectedCandidate.Path,
			displayPath: selectedCandidate.RelativePath,
			nodes:       nodes,
		}
		model.beginPathBrowse()
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
	case tea.KeyBackspace, tea.KeyDelete:
		model.inputValue = trimLastRune(model.inputValue)
		model.errorMessage = ""
		model.clampCursor(len(model.filteredFileCandidates(model.inputValue)))
		return model, nil
	case tea.KeySpace, tea.KeyRunes:
		model.inputValue += message.String()
		model.errorMessage = ""
		model.clampCursor(len(model.filteredFileCandidates(model.inputValue)))
		return model, nil
	default:
		return model, nil
	}
}

func (model initWizardModel) handleManualFileKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	return model.handleTextInputKey(message, initWizardStepFileSelect, func(value string) (initWizardModel, error) {
		resolvedTargetPath := resolveTargetPath(model.workingDirectory, value)
		nodes, err := model.dependencies.inspectStringTargets(resolvedTargetPath)
		if err != nil {
			return model, err
		}

		model.selectedFile = targetFileSelection{
			path:        resolvedTargetPath,
			displayPath: displayTargetPath(model.workingDirectory, resolvedTargetPath),
			nodes:       nodes,
		}
		model.beginPathBrowse()
		return model, nil
	})
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
		model.inputValue = ""
	case isRuneKey(message, 'm'):
		model.step = initWizardStepManualPath
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = ""
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
				model.beginProfileEntry()
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
		model.inputValue = ""
		return model, nil
	case tea.KeyEnter:
		if len(matchingPaths) == 0 {
			model.errorMessage = fmt.Sprintf("No selectable JSON paths in %s match %q.", model.selectedFile.displayPath, model.inputValue)
			return model, nil
		}

		model.selectedJSONPath = matchingPaths[model.cursor]
		model.beginProfileEntry()
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
	case tea.KeyBackspace, tea.KeyDelete:
		model.inputValue = trimLastRune(model.inputValue)
		model.errorMessage = ""
		model.clampCursor(len(model.filteredSelectableJSONPaths(model.inputValue)))
		return model, nil
	case tea.KeySpace, tea.KeyRunes:
		model.inputValue += message.String()
		model.errorMessage = ""
		model.clampCursor(len(model.filteredSelectableJSONPaths(model.inputValue)))
		return model, nil
	default:
		return model, nil
	}
}

func (model initWizardModel) handleManualPathKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	return model.handleTextInputKey(message, initWizardStepPathBrowse, func(value string) (initWizardModel, error) {
		if err := model.dependencies.validateStringTarget(model.selectedFile.path, value); err != nil {
			return model, err
		}

		model.selectedJSONPath = value
		model.beginProfileEntry()
		return model, nil
	})
}

func (model initWizardModel) handleProfileNameKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	previousStep := initWizardStepPathBrowse
	if len(model.profiles) > 0 {
		previousStep = initWizardStepProfileSummary
	}

	return model.handleTextInputKey(message, previousStep, func(value string) (initWizardModel, error) {
		if model.profileNameExists(value) {
			return model, fmt.Errorf("profile name %q is already configured", value)
		}

		model.draftProfile = initWizardProfileDraft{Name: value}
		model.step = initWizardStepProfileSource
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = ""
		return model, nil
	})
}

func (model initWizardModel) handleProfileSourceKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isMoveUpKey(message), isMoveDownKey(message):
		model.cursor = 1 - model.cursor
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepProfileName
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = model.draftProfile.Name
	case message.Type == tea.KeyEnter:
		model.draftProfile.UseEnvironment = model.cursor == 1
		model.step = initWizardStepProfileValue
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = ""
	}

	return model, nil
}

func (model initWizardModel) handleProfileValueKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	return model.handleTextInputKey(message, initWizardStepProfileSource, func(value string) (initWizardModel, error) {
		model.draftProfile.Value = value
		model.step = initWizardStepProfileProtected
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = ""
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
		model.inputValue = model.draftProfile.Value
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
		model.step = initWizardStepPathBrowse
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
			model.step = initWizardStepPathBrowse
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

func (model initWizardModel) handleTextInputKey(message tea.KeyMsg, previousStep initWizardStep, submit func(string) (initWizardModel, error)) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.step = previousStep
		model.cursor = 0
		model.errorMessage = ""
		model.inputValue = ""
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
	case tea.KeyBackspace, tea.KeyDelete:
		model.inputValue = trimLastRune(model.inputValue)
		model.errorMessage = ""
		return model, nil
	case tea.KeySpace, tea.KeyRunes:
		model.inputValue += message.String()
		model.errorMessage = ""
		return model, nil
	default:
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

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}

	return string(runes[:len(runes)-1])
}
