package main

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/tui/initwizard"
)

func runInitWizard(workingDirectory string, input io.Reader, output io.Writer, dependencies initDependencies) (initwizard.Result, error) {
	model, err := initwizard.NewModel(workingDirectory, initWorkflowFromDependencies(dependencies))
	if err != nil {
		return initwizard.Result{}, err
	}

	finalModel, err := runFullScreenTerminalProgram(model, tea.WithInput(input), tea.WithOutput(output))
	if err != nil {
		return initwizard.Result{}, fmt.Errorf("run init wizard: %w", err)
	}

	completedModel, ok := finalModel.(interface {
		Result() (initwizard.Result, bool)
	})
	if !ok {
		return initwizard.Result{}, fmt.Errorf("run init wizard: unexpected final model type %T", finalModel)
	}
	result, ok := completedModel.Result()
	if !ok {
		return initwizard.Result{}, fmt.Errorf("run init wizard: finished without a result")
	}

	return result, nil
}

func initWorkflowFromDependencies(dependencies initDependencies) app.InitWorkflow {
	return app.NewInitWorkflow(app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: dependencies.discoverTargetFileCandidates,
		InspectStringTargets:         dependencies.inspectStringTargets,
		InspectYAMLStringTargets:     dependencies.inspectYAMLStringTargets,
		InspectTOMLStringTargets:     dependencies.inspectTOMLStringTargets,
		InspectDotenvKeys:            dependencies.inspectDotenvKeys,
		ValidateStringTarget:         dependencies.validateStringTarget,
		ValidateYAMLTarget:           dependencies.validateYAMLTarget,
		ValidateTOMLTarget:           dependencies.validateTOMLTarget,
		ValidateDotenvTarget:         dependencies.validateDotenvTarget,
	})
}

func shouldUseInitWizard(input io.Reader, output io.Writer) bool {
	inputFile, ok := input.(*os.File)
	if !ok {
		return false
	}

	outputFile, ok := output.(*os.File)
	if !ok {
		return false
	}

	return isCharacterDevice(inputFile) && isCharacterDevice(outputFile)
}

func isCharacterDevice(file *os.File) bool {
	fileInfo, err := file.Stat()
	if err != nil {
		return false
	}

	return fileInfo.Mode()&os.ModeCharDevice != 0
}
