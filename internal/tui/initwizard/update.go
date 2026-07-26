package initwizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

// Update handles Bubble Tea messages for the init wizard.
func (model initWizardModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case fileInspectedMsg:
		return model.handleFileInspected(message)
	case jsonSelectorValidatedMsg:
		return model.handleJSONSelectorValidated(message)
	case dotenvKeyValidatedMsg:
		return model.handleDotenvKeyValidated(message)
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
		if model.isPending() {
			return model.handlePendingKey(message)
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
		case initWizardStepManagedValueName:
			return model.handleManagedValueNameKey(message)
		case initWizardStepManagedValueCheckpoint:
			return model.handleManagedValueCheckpointKey(message)
		case initWizardStepProfileName:
			return model.handleProfileNameKey(message)
		case initWizardStepProfileTargetInclude:
			return model.handleProfileTargetIncludeKey(message)
		case initWizardStepProfileValueSource:
			return model.handleProfileValueSourceKey(message)
		case initWizardStepProfileValue:
			return model.handleProfileValueKey(message)
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

func (model initWizardModel) handlePendingKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isQuitKey(message):
		model.cancel()
		return model, tea.Quit
	case message.Type == tea.KeyEsc:
		model.cancelPendingEffect()
		return model, nil
	default:
		return model, nil
	}
}

func (model initWizardModel) handleFileInspected(message fileInspectedMsg) (tea.Model, tea.Cmd) {
	if model.staleFileInspected(message) {
		return model, nil
	}

	pendingEffect := *model.pendingEffect
	model.pendingEffect = nil
	if message.err != nil {
		model.restorePendingContext(pendingEffect)
		model.errorDetail = fileInspectionError(pendingEffect, message.err)
		return model, nil
	}

	if pendingEffect.ReturnStep == initWizardStepFileFilter {
		model.fileFilter = pendingEffect.FileFilter
	}
	model.selectedFile = message.selection
	model.beginSelectorForSelectedFile()
	return model, nil
}

func (model initWizardModel) handleJSONSelectorValidated(message jsonSelectorValidatedMsg) (tea.Model, tea.Cmd) {
	if model.staleJSONSelectorValidated(message) {
		return model, nil
	}

	pendingEffect := *model.pendingEffect
	model.pendingEffect = nil
	if message.err != nil {
		model.restorePendingContext(pendingEffect)
		model.errorDetail = jsonSelectorValidationError(pendingEffect, message.err)
		return model, nil
	}

	model.selectedJSONPath = message.jsonPath
	model.beginManagedValueName()
	return model, nil
}

func (model initWizardModel) handleDotenvKeyValidated(message dotenvKeyValidatedMsg) (tea.Model, tea.Cmd) {
	if model.staleDotenvKeyValidated(message) {
		return model, nil
	}

	pendingEffect := *model.pendingEffect
	model.pendingEffect = nil
	if message.err != nil {
		model.restorePendingContext(pendingEffect)
		model.errorDetail = dotenvKeyValidationError(pendingEffect, message.err)
		return model, nil
	}

	model.selectedDotenvKey = message.key
	model.beginManagedValueName()
	return model, nil
}

func (model initWizardModel) startFileCandidateInspection(candidate app.InitTargetFileCandidate, returnStep initWizardStep, fileFilter string) (tea.Model, tea.Cmd) {
	requestID := model.startPendingEffect(initWizardPendingEffect{
		Kind:              initWizardEffectFileInspection,
		StepNumber:        1,
		Title:             "Inspecting configuration file",
		Message:           "Inspecting " + candidate.RelativePath + ".",
		ReturnStep:        returnStep,
		ReturnCursor:      model.cursor,
		ReturnInputValue:  model.inputValue,
		ReturnInputCursor: model.inputCursor,
		TargetPath:        candidate.Path,
		DisplayPath:       candidate.RelativePath,
		TargetType:        candidate.Type,
		FileFilter:        fileFilter,
	})

	return model, inspectTargetFileCandidate(model.workflow, requestID, candidate)
}

func (model initWizardModel) startFileInspection(targetPath string, displayPath string, targetType app.InitTargetType, returnStep initWizardStep) (tea.Model, tea.Cmd) {
	requestID := model.startPendingEffect(initWizardPendingEffect{
		Kind:              initWizardEffectFileInspection,
		StepNumber:        1,
		Title:             "Inspecting configuration file",
		Message:           "Inspecting " + displayPath + ".",
		ReturnStep:        returnStep,
		ReturnCursor:      model.cursor,
		ReturnInputValue:  model.inputValue,
		ReturnInputCursor: model.inputCursor,
		TargetPath:        targetPath,
		DisplayPath:       displayPath,
		TargetType:        targetType,
	})

	return model, inspectTargetFile(model.workflow, requestID, targetPath, displayPath, targetType)
}

func (model initWizardModel) startJSONSelectorValidation(jsonPath string) (tea.Model, tea.Cmd) {
	requestID := model.startPendingEffect(initWizardPendingEffect{
		Kind:              initWizardEffectJSONSelectorValidation,
		StepNumber:        2,
		Title:             "Validating JSON value path",
		Message:           "Checking " + jsonPath + ".",
		ReturnStep:        initWizardStepManualPath,
		ReturnCursor:      model.cursor,
		ReturnInputValue:  model.inputValue,
		ReturnInputCursor: model.inputCursor,
		TargetPath:        model.selectedFile.Path,
		DisplayPath:       model.selectedFile.DisplayPath,
		TargetType:        model.selectedFile.TargetType,
		Selector:          jsonPath,
	})

	return model, validateJSONSelector(model.workflow, requestID, model.selectedFile.Path, jsonPath)
}

func (model initWizardModel) startDotenvKeyValidation(key string) (tea.Model, tea.Cmd) {
	requestID := model.startPendingEffect(initWizardPendingEffect{
		Kind:              initWizardEffectDotenvKeyValidation,
		StepNumber:        2,
		Title:             "Validating dotenv key",
		Message:           "Checking " + key + ".",
		ReturnStep:        initWizardStepManualDotenvKey,
		ReturnCursor:      model.cursor,
		ReturnInputValue:  model.inputValue,
		ReturnInputCursor: model.inputCursor,
		TargetPath:        model.selectedFile.Path,
		DisplayPath:       model.selectedFile.DisplayPath,
		TargetType:        model.selectedFile.TargetType,
		Selector:          key,
	})

	return model, validateDotenvKey(model.workflow, requestID, model.selectedFile.Path, key)
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
		model.clearError()
		model.setInputValue(model.fileFilter)
	case isRuneKey(message, 'm'):
		model.step = initWizardStepManualFile
		model.cursor = 0
		model.clearError()
		model.clearInputValue()
	case message.Type == tea.KeyEnter:
		if len(matchingCandidates) == 0 {
			return model, nil
		}

		return model.startFileCandidateInspection(matchingCandidates[model.cursor], initWizardStepFileSelect, model.fileFilter)
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
		model.clearError()
		model.clearInputValue()
		model.clampCursor(len(model.filteredFileCandidates(model.fileFilter)))
		return model, nil
	case tea.KeyEnter:
		if len(matchingCandidates) == 0 {
			model.setStepError("No matching configuration files.", fmt.Sprintf("No discovered configuration files match %q.", model.inputValue), "Edit the filter or press Esc to return to the full file list.")
			return model, nil
		}

		return model.startFileCandidateInspection(matchingCandidates[model.cursor], initWizardStepFileFilter, strings.TrimSpace(model.inputValue))
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
		model.clearError()
		model.clampCursor(len(model.filteredFileCandidates(model.inputValue)))
		return model, nil
	}
}

func (model initWizardModel) handleManualFileKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.step = initWizardStepFileSelect
		model.cursor = 0
		model.clearError()
		model.clearInputValue()
		return model, nil
	case tea.KeyEnter:
		enteredValue := strings.TrimSpace(model.inputValue)
		if enteredValue == "" {
			model.setInputRequiredError()
			return model, nil
		}

		value := enteredValue
		resolvedTargetPath := app.ResolveInitTargetPath(model.workingDirectory, value)
		targetType, ok := app.InferInitTargetType(resolvedTargetPath)
		if !ok {
			model.selectedFile = app.InitTargetFileSelection{
				Path:        resolvedTargetPath,
				DisplayPath: app.DisplayInitTargetPath(model.workingDirectory, resolvedTargetPath),
			}
			model.step = initWizardStepTypeSelect
			model.cursor = 0
			model.clearError()
			model.clearInputValue()
			return model, nil
		}

		return model.startFileInspection(resolvedTargetPath, app.DisplayInitTargetPath(model.workingDirectory, resolvedTargetPath), targetType, initWizardStepManualFile)
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.clearError()
		return model, nil
	}
}

func (model *initWizardModel) beginSelectorForSelectedFile() {
	if model.selectedFile.TargetType == app.InitTargetTypeDotenv {
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
		model.clearError()
		model.setInputValue(model.selectedFile.DisplayPath)
	case message.Type == tea.KeyEnter:
		targetType := app.InitTargetTypeJSON
		if model.cursor == 1 {
			targetType = app.InitTargetTypeDotenv
		}

		return model.startFileInspection(model.selectedFile.Path, model.selectedFile.DisplayPath, targetType, initWizardStepTypeSelect)
	}

	return model, nil
}

func (model initWizardModel) handleDotenvKeySelectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	keys := model.selectedFile.DotenvKeys
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
		model.clearError()
		model.clearInputValue()
	case message.Type == tea.KeyEsc:
		model.step = initWizardStepFileSelect
		model.cursor = 0
		model.clearError()
	case message.Type == tea.KeyEnter:
		if model.cursor < len(keys) {
			model.selectedDotenvKey = keys[model.cursor]
			model.beginManagedValueName()
			return model, nil
		}

		model.step = initWizardStepManualDotenvKey
		model.cursor = 0
		model.clearError()
		model.clearInputValue()
	}

	return model, nil
}

func (model initWizardModel) handleManualDotenvKeyKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.step = initWizardStepDotenvKeySelect
		model.cursor = 0
		model.clearError()
		model.clearInputValue()
		return model, nil
	case tea.KeyEnter:
		enteredValue := strings.TrimSpace(model.inputValue)
		if enteredValue == "" {
			model.setInputRequiredError()
			return model, nil
		}

		return model.startDotenvKeyValidation(enteredValue)
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.clearError()
		return model, nil
	}
}

func (model initWizardModel) handleManagedValueNameKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	previousStep := initWizardStepPathBrowse
	if model.selectedFile.TargetType == app.InitTargetTypeDotenv {
		previousStep = initWizardStepDotenvKeySelect
	}

	return model.handleTextInputKey(message, previousStep, func(value string) (initWizardModel, error) {
		if model.targetNameExists(value) {
			return model, fmt.Errorf("managed value name %q is already configured", value)
		}

		model.appendTarget(value)
		return model, nil
	})
}

func (model initWizardModel) handleManagedValueCheckpointKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		model.beginProfileEntry()
	case message.Type == tea.KeyEnter:
		switch model.cursor {
		case 0:
			model.beginProfileEntry()
		case 1:
			model.step = initWizardStepFileSelect
			model.cursor = 0
			model.clearError()
		case 2:
			model.removeLastTarget()
		}
	}

	return model, nil
}

func (model initWizardModel) handlePathBrowseKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	choices := len(model.browseNodes) + 2
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
		model.clearError()
		model.clearInputValue()
	case isRuneKey(message, 'm'):
		model.step = initWizardStepManualPath
		model.cursor = 0
		model.clearError()
		model.clearInputValue()
	case message.Type == tea.KeyEsc:
		if len(model.browseAncestors) > 0 {
			previousLevel := model.browseAncestors[len(model.browseAncestors)-1]
			model.browseAncestors = model.browseAncestors[:len(model.browseAncestors)-1]
			model.browseNodes = previousLevel.nodes
			model.cursor = 0
			model.clearError()
		} else {
			model.step = initWizardStepFileSelect
			model.cursor = 0
			model.clearError()
		}
	case message.Type == tea.KeyEnter:
		if model.cursor < len(model.browseNodes) {
			selectedNode := model.browseNodes[model.cursor]
			if selectedNode.Selectable {
				model.selectedJSONPath = selectedNode.JSONPath
				model.beginManagedValueName()
				return model, nil
			}

			model.browseAncestors = append(model.browseAncestors, targetBrowseLevel{
				path:  selectedNode.JSONPath,
				nodes: model.browseNodes,
			})
			model.browseNodes = selectedNode.Children
			model.cursor = 0
			model.clearError()
			return model, nil
		}

		actionIndex := len(model.browseNodes)
		if model.cursor == actionIndex {
			model.step = initWizardStepPathSearch
			model.cursor = 0
			model.clearError()
			model.inputValue = ""
			return model, nil
		}

		model.step = initWizardStepManualPath
		model.cursor = 0
		model.clearError()
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
		model.clearError()
		model.clearInputValue()
		return model, nil
	case tea.KeyEnter:
		if len(matchingPaths) == 0 {
			model.setStepError("No matching JSON values.", fmt.Sprintf("No selectable JSON paths in %s match %q.", model.selectedFile.DisplayPath, model.inputValue), "Edit the search or press Esc to browse values.")
			return model, nil
		}

		model.selectedJSONPath = matchingPaths[model.cursor]
		model.beginManagedValueName()
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
		model.clearError()
		model.clampCursor(len(model.filteredSelectableJSONPaths(model.inputValue)))
		return model, nil
	}
}

func (model initWizardModel) handleManualPathKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.step = initWizardStepPathBrowse
		model.cursor = 0
		model.clearError()
		model.clearInputValue()
		return model, nil
	case tea.KeyEnter:
		enteredValue := strings.TrimSpace(model.inputValue)
		if enteredValue == "" {
			model.setInputRequiredError()
			return model, nil
		}

		return model.startJSONSelectorValidation(enteredValue)
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.clearError()
		return model, nil
	}
}

func (model initWizardModel) handleProfileNameKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.beginManagedValueCheckpoint()
		return model, nil
	case tea.KeyEnter:
		enteredValue := strings.TrimSpace(model.inputValue)
		if enteredValue == "" {
			model.setStepError("Profile name is required.", "Profile name must not be empty.", "Enter a profile name or press Esc to return to managed values.")
			return model, nil
		}

		if model.profileNameExists(enteredValue) {
			model.setStepError("Profile name is already in use.", fmt.Sprintf("Profile name %q is already configured.", enteredValue), "Choose a different profile name.")
			return model, nil
		}

		model.draftProfile = initWizardProfileDraft{Name: enteredValue, Values: make([]app.InitProfileValue, 0, len(model.targets))}
		if len(model.targets) == 1 {
			model.draftProfile.TargetIndex = 0
			model.beginProfileValueSource()
		} else {
			model.beginProfileTargetInclude()
		}
		return model, nil
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.clearError()
		return model, nil
	}
}

func (model initWizardModel) handleProfileTargetIncludeKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isMoveUpKey(message), isMoveDownKey(message):
		model.cursor = 1 - model.cursor
	case message.Type == tea.KeyEsc:
		if model.draftProfile.TargetIndex == 0 {
			model.trimDraftProfileValuesFromTargetIndex(0)
			model.step = initWizardStepProfileName
			model.cursor = 0
			model.clearError()
			model.setInputValue(model.draftProfile.Name)
			return model, nil
		}

		model.draftProfile.TargetIndex--
		model.trimDraftProfileValuesFromTargetIndex(model.draftProfile.TargetIndex)
		model.beginProfileTargetInclude()
	case message.Type == tea.KeyEnter:
		if model.cursor == 0 {
			model.beginProfileValueSource()
			return model, nil
		}

		model.advanceDraftProfileTarget()
	}

	return model, nil
}

func (model initWizardModel) handleProfileValueSourceKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if len(model.targets) == 1 {
			model.step = initWizardStepProfileName
			model.cursor = 0
			model.clearError()
			model.setInputValue(model.draftProfile.Name)
			return model, nil
		}

		model.beginProfileTargetInclude()
	case message.Type == tea.KeyEnter:
		switch model.cursor {
		case 0, 1:
			useEnvironment := model.cursor == 1
			if model.draftProfile.UseEnvironment != useEnvironment {
				model.draftProfile.Value = ""
			}
			model.draftProfile.UseEnvironment = useEnvironment
			model.beginProfileValue()
		case 2:
			model.draftProfile.Protected = !model.draftProfile.Protected
		}
	}

	return model, nil
}

func (model initWizardModel) handleProfileValueKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		model.draftProfile.Value = model.inputValue
		model.beginProfileValueSource()
		return model, nil
	case tea.KeyEnter:
		enteredValue := strings.TrimSpace(model.inputValue)
		if enteredValue == "" {
			model.setInputRequiredError()
			return model, nil
		}

		model.draftProfile.Value = enteredValue
		model.appendDraftProfileValue()
		return model, nil
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.clearError()
		return model, nil
	}
}

func (model initWizardModel) handleProfileSummaryKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	choiceCount := 4
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
		model.beginProfileEntry()
	case message.Type == tea.KeyEnter:
		switch model.cursor {
		case 0:
			model.beginReview()
		case 1:
			model.beginProfileEntry()
		case 2:
			model.step = initWizardStepFileSelect
			model.cursor = 0
			model.clearError()
			model.clearInputValue()
		case 3:
			model.removeLastProfile()
		}
	}

	return model, nil
}

func (model initWizardModel) handleReviewKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	choiceCount := 1
	if app.InitProfilesHaveLiteralValues(model.profiles) {
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
		model.returnToProfilesFromReview()
	case message.Type == tea.KeyEnter:
		if model.cursor == 0 {
			model.complete()
			return model, tea.Quit
		}

		if app.InitProfilesHaveLiteralValues(model.profiles) && model.cursor == 1 {
			model.shouldIgnoreConfig = !model.shouldIgnoreConfig
			model.shouldIgnoreConfigSet = true
			return model, nil
		}

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
		model.clearError()
		model.clearInputValue()
		return model, nil
	case tea.KeyEnter:
		enteredValue := strings.TrimSpace(model.inputValue)
		if enteredValue == "" {
			model.setInputRequiredError()
			return model, nil
		}

		nextModel, err := submit(enteredValue)
		if err != nil {
			model.setStepError("Input could not be used.", err.Error(), "Enter a different value or press Esc to go back.")
			return model, nil
		}

		return nextModel, nil
	default:
		if !model.handleInputEditKey(message) {
			return model, nil
		}
		model.clearError()
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
