package configeditor

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type saveCompletedMsg struct {
	configPath string
	changes    []app.ConfigEditChange
	err        error
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
