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
