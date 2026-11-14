package initwizard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/tui/setup"
)

type batchValidatedMsg struct {
	requestID int
	targets   []app.InitTarget
	err       error
}

func (model initWizardModel) handleBatchKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch model.batch.Update(key) {
	case setup.BatchBack:
		model.batch.Review = false
		model.batch.Pending = false
		model.batchRequestID = 0
	case setup.BatchQuit:
		model.cancel()
		return model, tea.Quit
	case setup.BatchAdd:
		model.batch.Pending = true
		model.batchRequestID = model.nextEffectRequestID()
		requestID := model.batchRequestID
		existing := append([]app.InitTarget(nil), model.targets...)
		additions := append([]app.InitTarget(nil), model.batch.Targets...)
		return model, func() tea.Msg {
			targets, err := model.workflow.ValidateManagedValueBatch(model.workingDirectory, existing, additions)
			return batchValidatedMsg{requestID, targets, err}
		}
	}
	return model, nil
}

func (model initWizardModel) handleBatchValidated(message batchValidatedMsg) (tea.Model, tea.Cmd) {
	if model.result != nil || !model.batch.Pending || model.batchRequestID != message.requestID {
		return model, nil
	}
	model.batch.Pending = false
	if message.err != nil {
		model.batch.Error = message.err.Error()
		return model, nil
	}
	model.targets = message.targets
	model.batch = setup.Batch{}
	model.beginManagedValueCheckpoint()
	return model, nil
}

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
