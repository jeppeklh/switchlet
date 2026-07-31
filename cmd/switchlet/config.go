package main

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/tui/configeditor"
)

func runConfigCommand(workingDirectory string, input io.Reader, output io.Writer) error {
	if !shouldUseConfigEditor(input, output) {
		return fmt.Errorf("switchlet config is interactive-only in Version 0.19 and requires stdin and stdout to be terminals")
	}

	result, err := runConfigEditorForWorkingDirectory(workingDirectory, input, output)
	if err != nil {
		return err
	}

	switch {
	case result.Saved:
		_, err = fmt.Fprintf(output, "\nSaved configuration: %s\n", result.ConfigPath)
	case result.Cancelled:
		_, err = fmt.Fprintln(output, "\nConfiguration editing cancelled.")
	default:
		_, err = fmt.Fprintln(output, "\nNo configuration changes saved.")
	}

	return err
}

func runConfigEditorForWorkingDirectory(workingDirectory string, input io.Reader, output io.Writer) (configeditor.Result, error) {
	configPath, err := config.Discover(workingDirectory)
	if err != nil {
		return configeditor.Result{}, err
	}

	workflow := app.DefaultConfigEditWorkflow()
	document, err := workflow.LoadDocument(configPath)
	if err != nil {
		return configeditor.Result{}, err
	}

	return runConfigEditor(document, workflow, input, output)
}

func runConfigEditor(document app.ConfigEditDocument, workflow app.ConfigEditWorkflow, input io.Reader, output io.Writer) (configeditor.Result, error) {
	model := configeditor.NewModel(document, workflow)
	finalModel, err := runFullScreenTerminalProgram(model, tea.WithInput(input), tea.WithOutput(output))
	if err != nil {
		return configeditor.Result{}, fmt.Errorf("run config editor: %w", err)
	}

	completedModel, ok := finalModel.(interface {
		Result() (configeditor.Result, bool)
	})
	if !ok {
		return configeditor.Result{}, fmt.Errorf("run config editor: unexpected final model type %T", finalModel)
	}
	result, ok := completedModel.Result()
	if !ok {
		return configeditor.Result{}, fmt.Errorf("run config editor: finished without a result")
	}

	return result, nil
}

func shouldUseConfigEditor(input io.Reader, output io.Writer) bool {
	return shouldUseInitWizard(input, output)
}
