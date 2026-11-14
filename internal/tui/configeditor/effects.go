package configeditor

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/tui/setup"
)

type managedValuesAddedMsg struct {
	requestID int
	document  app.ConfigEditDocument
	err       error
}

func (model Model) handleManagedValueBatchKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch model.managedForm.batch.Update(key) {
	case setup.BatchBack:
		model.managedForm.batch.Review = false
		model.managedForm.batch.Pending = false
		model.clearManagedValueRequest()
	case setup.BatchQuit:
		model.managedForm.batch.Review = false
		model.managedForm.batch.Pending = false
		model.clearManagedValueRequest()
		return model.beginQuit()
	case setup.BatchAdd:
		model.managedForm.batch.Pending = true
		requestID := model.nextRequestID()
		model.managedForm.requestID = requestID
		document := model.document
		targets := append([]app.InitTarget(nil), model.managedForm.batch.Targets...)
		return model, func() tea.Msg {
			updated, err := model.workflow.AddManagedValues(document, targets)
			return managedValuesAddedMsg{requestID, updated, err}
		}
	}
	return model, nil
}

func (model Model) handleManagedValuesAdded(message managedValuesAddedMsg) (tea.Model, tea.Cmd) {
	if model.result != nil || !model.managedForm.batch.Pending || model.managedForm.requestID != message.requestID {
		return model, nil
	}
	model.managedForm.batch.Pending = false
	if message.err != nil {
		model.managedForm.batch.Error = message.err.Error()
		return model, nil
	}
	model.document = message.document
	model.cancelManagedValueForm()
	model.selectOverviewTab(overviewTabTargets)
	return model, nil
}

type saveCompletedMsg struct {
	configPath string
	changes    []app.ConfigEditChange
	err        error
}

type profileCapturedMsg struct {
	requestID int
	draft     app.ConfigEditProfileDraft
	err       error
}

type managedValueFilesDiscoveredMsg struct {
	requestID  int
	candidates []app.InitTargetFileCandidate
	err        error
}

type managedValueFileInspectedMsg struct {
	requestID int
	selection app.InitTargetFileSelection
	err       error
}

type managedValueSelectorValidatedMsg struct {
	requestID  int
	targetPath string
	targetType app.InitTargetType
	selector   string
	err        error
}

func saveDocument(workflow app.ConfigEditWorkflow, document app.ConfigEditDocument) tea.Cmd {
	return func() tea.Msg {
		preparedSave, err := workflow.PrepareSave(document)
		if err != nil {
			return saveCompletedMsg{err: err}
		}
		if err := preparedSave.Commit(); err != nil {
			return saveCompletedMsg{err: fmt.Errorf("commit configuration save: %w", err)}
		}

		return saveCompletedMsg{
			configPath: preparedSave.ConfigPath,
			changes:    preparedSave.Changes,
		}
	}
}

func captureCurrentProfile(workflow app.ConfigEditWorkflow, requestID int, document app.ConfigEditDocument) tea.Cmd {
	return func() tea.Msg {
		draft, err := workflow.CaptureCurrentProfileDraft(document)
		return profileCapturedMsg{requestID: requestID, draft: draft, err: err}
	}
}

func discoverManagedValueFiles(workflow app.InitWorkflow, requestID int, projectRoot string) tea.Cmd {
	return func() tea.Msg {
		candidates, err := workflow.DiscoverTargetFileCandidates(projectRoot)
		return managedValueFilesDiscoveredMsg{requestID: requestID, candidates: candidates, err: err}
	}
}

func inspectManagedValueFileCandidate(workflow app.InitWorkflow, requestID int, candidate app.InitTargetFileCandidate) tea.Cmd {
	return func() tea.Msg {
		selection, err := workflow.InspectTargetFileCandidate(candidate)
		return managedValueFileInspectedMsg{requestID: requestID, selection: selection, err: err}
	}
}

func inspectManagedValueFile(workflow app.InitWorkflow, requestID int, targetPath string, displayPath string, targetType app.InitTargetType) tea.Cmd {
	return func() tea.Msg {
		selection, err := workflow.InspectTargetFile(targetPath, displayPath, targetType)
		return managedValueFileInspectedMsg{requestID: requestID, selection: selection, err: err}
	}
}

func validateManagedValueSelector(workflow app.InitWorkflow, requestID int, targetPath string, targetType app.InitTargetType, selector string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch targetType {
		case app.InitTargetTypeDotenv:
			err = workflow.ValidateDotenvTarget(targetPath, selector)
		case app.InitTargetTypeYAML:
			err = workflow.ValidateYAMLTarget(targetPath, selector)
		case app.InitTargetTypeTOML:
			err = workflow.ValidateTOMLTarget(targetPath, selector)
		default:
			err = workflow.ValidateStringTarget(targetPath, selector)
		}

		return managedValueSelectorValidatedMsg{
			requestID:  requestID,
			targetPath: targetPath,
			targetType: targetType,
			selector:   selector,
			err:        err,
		}
	}
}
