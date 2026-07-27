package initwizard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type fileInspectedMsg struct {
	requestID   int
	targetPath  string
	displayPath string
	targetType  app.InitTargetType
	selection   app.InitTargetFileSelection
	err         error
}

type jsonSelectorValidatedMsg struct {
	requestID  int
	targetPath string
	jsonPath   string
	err        error
}

type yamlSelectorValidatedMsg struct {
	requestID  int
	targetPath string
	yamlPath   string
	err        error
}

type tomlSelectorValidatedMsg struct {
	requestID  int
	targetPath string
	tomlPath   string
	err        error
}

type dotenvKeyValidatedMsg struct {
	requestID  int
	targetPath string
	key        string
	err        error
}

func inspectTargetFileCandidate(workflow app.InitWorkflow, requestID int, candidate app.InitTargetFileCandidate) tea.Cmd {
	return func() tea.Msg {
		selection, err := workflow.InspectTargetFileCandidate(candidate)
		return fileInspectedMsg{
			requestID:   requestID,
			targetPath:  candidate.Path,
			displayPath: candidate.RelativePath,
			targetType:  candidate.Type,
			selection:   selection,
			err:         err,
		}
	}
}

func inspectTargetFile(workflow app.InitWorkflow, requestID int, targetPath string, displayPath string, targetType app.InitTargetType) tea.Cmd {
	return func() tea.Msg {
		selection, err := workflow.InspectTargetFile(targetPath, displayPath, targetType)
		return fileInspectedMsg{
			requestID:   requestID,
			targetPath:  targetPath,
			displayPath: displayPath,
			targetType:  targetType,
			selection:   selection,
			err:         err,
		}
	}
}

func validateJSONSelector(workflow app.InitWorkflow, requestID int, targetPath string, jsonPath string) tea.Cmd {
	return func() tea.Msg {
		return jsonSelectorValidatedMsg{
			requestID:  requestID,
			targetPath: targetPath,
			jsonPath:   jsonPath,
			err:        workflow.ValidateStringTarget(targetPath, jsonPath),
		}
	}
}

func validateYAMLSelector(workflow app.InitWorkflow, requestID int, targetPath string, yamlPath string) tea.Cmd {
	return func() tea.Msg {
		return yamlSelectorValidatedMsg{
			requestID:  requestID,
			targetPath: targetPath,
			yamlPath:   yamlPath,
			err:        workflow.ValidateYAMLTarget(targetPath, yamlPath),
		}
	}
}

func validateTOMLSelector(workflow app.InitWorkflow, requestID int, targetPath string, tomlPath string) tea.Cmd {
	return func() tea.Msg {
		return tomlSelectorValidatedMsg{
			requestID:  requestID,
			targetPath: targetPath,
			tomlPath:   tomlPath,
			err:        workflow.ValidateTOMLTarget(targetPath, tomlPath),
		}
	}
}

func validateDotenvKey(workflow app.InitWorkflow, requestID int, targetPath string, key string) tea.Cmd {
	return func() tea.Msg {
		return dotenvKeyValidatedMsg{
			requestID:  requestID,
			targetPath: targetPath,
			key:        key,
			err:        workflow.ValidateDotenvTarget(targetPath, key),
		}
	}
}

func (model *initWizardModel) nextEffectRequestID() int {
	model.effectRequestID++
	return model.effectRequestID
}

func (model initWizardModel) isPending() bool {
	return model.pendingEffect != nil
}

func (model *initWizardModel) startPendingEffect(pendingEffect initWizardPendingEffect) int {
	pendingEffect.RequestID = model.nextEffectRequestID()
	model.pendingEffect = &pendingEffect
	model.clearError()
	return pendingEffect.RequestID
}

func (model *initWizardModel) restorePendingContext(pendingEffect initWizardPendingEffect) {
	model.step = pendingEffect.ReturnStep
	model.cursor = pendingEffect.ReturnCursor
	model.inputValue = pendingEffect.ReturnInputValue
	model.inputCursor = pendingEffect.ReturnInputCursor
	model.pendingEffect = nil
}

func (model *initWizardModel) cancelPendingEffect() {
	if model.pendingEffect == nil {
		return
	}

	pendingEffect := *model.pendingEffect
	model.restorePendingContext(pendingEffect)
	model.clearError()
}

func (model initWizardModel) staleFileInspected(message fileInspectedMsg) bool {
	if model.pendingEffect == nil || model.pendingEffect.Kind != initWizardEffectFileInspection {
		return true
	}
	if model.pendingEffect.RequestID != message.requestID {
		return true
	}
	if model.pendingEffect.TargetPath != message.targetPath {
		return true
	}
	if model.pendingEffect.DisplayPath != message.displayPath {
		return true
	}

	return model.pendingEffect.TargetType != message.targetType
}

func (model initWizardModel) staleJSONSelectorValidated(message jsonSelectorValidatedMsg) bool {
	if model.pendingEffect == nil || model.pendingEffect.Kind != initWizardEffectJSONSelectorValidation {
		return true
	}

	return model.pendingEffect.RequestID != message.requestID || model.pendingEffect.TargetPath != message.targetPath || model.pendingEffect.Selector != message.jsonPath
}

func (model initWizardModel) staleYAMLSelectorValidated(message yamlSelectorValidatedMsg) bool {
	if model.pendingEffect == nil || model.pendingEffect.Kind != initWizardEffectYAMLSelectorValidation {
		return true
	}

	return model.pendingEffect.RequestID != message.requestID || model.pendingEffect.TargetPath != message.targetPath || model.pendingEffect.Selector != message.yamlPath
}

func (model initWizardModel) staleTOMLSelectorValidated(message tomlSelectorValidatedMsg) bool {
	if model.pendingEffect == nil || model.pendingEffect.Kind != initWizardEffectTOMLSelectorValidation {
		return true
	}

	return model.pendingEffect.RequestID != message.requestID || model.pendingEffect.TargetPath != message.targetPath || model.pendingEffect.Selector != message.tomlPath
}

func (model initWizardModel) staleDotenvKeyValidated(message dotenvKeyValidatedMsg) bool {
	if model.pendingEffect == nil || model.pendingEffect.Kind != initWizardEffectDotenvKeyValidation {
		return true
	}

	return model.pendingEffect.RequestID != message.requestID || model.pendingEffect.TargetPath != message.targetPath || model.pendingEffect.Selector != message.key
}
