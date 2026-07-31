package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/tui"
	"github.com/jeppeklh/switchlet/internal/tui/configeditor"
)

const (
	runtimeExitCode = 1
	usageExitCode   = 2
)

type terminalProgramRunner func(tea.Model) (tea.Model, error)

type configEditorRunner func(string, io.Reader, io.Writer) (configeditor.Result, error)

var (
	commandNames   = []string{"help", "init", "config", "list", "inspect", "apply", "status", "diff"}
	helpTopicNames = []string{"init", "config", "list", "inspect", "apply", "status", "diff"}
)

func main() {
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get working directory: %v\n", err)
		os.Exit(runtimeExitCode)
	}

	if err := runCommandWithTerminalRunner(os.Args[1:], workingDirectory, startFullScreenProgram(os.Stdout), os.Stdin, os.Stdout); err != nil {
		if writeErr := writeCommandError(err, os.Stdout, os.Stderr); writeErr != nil {
			fmt.Fprintln(os.Stderr, writeErr)
			os.Exit(runtimeExitCode)
		}

		os.Exit(exitCodeForError(err))
	}
}

func runCommand(args []string, workingDirectory string, runProgram func(tea.Model) error, input io.Reader, output io.Writer) error {
	return runCommandWithTerminalRunner(args, workingDirectory, adaptTerminalProgramRunner(runProgram), input, output)
}

func runCommandWithTerminalRunner(args []string, workingDirectory string, runProgram terminalProgramRunner, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return runInteractiveCommandWithTerminalRunner(workingDirectory, runProgram, input, output)
	}

	switch args[0] {
	case "help", "-h", "--help":
		return writeHelp(output, args[1:])
	case "init":
		if wantsHelpFlag(args[1:]) {
			_, err := io.WriteString(output, initHelpText())
			return err
		}

		overwriteExistingConfig := false
		positionals, err := parseArguments(args[1:], map[string]*bool{"--overwrite": &overwriteExistingConfig})
		if err != nil {
			return usageCommandError(false, "init: %v\n\n%s", err, initHelpText())
		}
		if len(positionals) != 0 {
			return usageCommandError(false, "init does not accept a positional argument\n\n%s", initHelpText())
		}

		return runInitWithOptions(workingDirectory, input, output, defaultInitDependencies(), initOptions{OverwriteExistingConfig: overwriteExistingConfig})
	case "config":
		if wantsHelpFlag(args[1:]) {
			_, err := io.WriteString(output, configHelpText())
			return err
		}

		positionals, err := parseArguments(args[1:], map[string]*bool{})
		if err != nil {
			return usageCommandError(false, "config: %v\n\n%s", err, configHelpText())
		}
		if len(positionals) != 0 {
			return usageCommandError(false, "config does not accept a positional argument\n\n%s", configHelpText())
		}

		return runConfigCommand(workingDirectory, input, output)
	case "list":
		return runListCommand(workingDirectory, args[1:], output)
	case "inspect":
		return runInspectCommand(workingDirectory, args[1:], output)
	case "apply":
		return runApplyCommand(workingDirectory, args[1:], output)
	case "status":
		return runStatusCommand(workingDirectory, args[1:], output)
	case "diff":
		return runDiffCommand(workingDirectory, args[1:], output)
	default:
		return usageCommandError(false, "%s\n\n%s", unknownCommandMessage(args[0]), usageText())
	}
}

func adaptTerminalProgramRunner(runProgram func(tea.Model) error) terminalProgramRunner {
	return func(model tea.Model) (tea.Model, error) {
		return model, runProgram(model)
	}
}

func writeHelp(output io.Writer, args []string) error {
	if len(args) == 0 {
		_, err := io.WriteString(output, usageText())
		return err
	}
	if len(args) != 1 {
		return usageCommandError(false, "help accepts at most one command name\n\n%s", usageText())
	}

	helpText, err := helpTextForTopic(args[0])
	if err != nil {
		return err
	}

	_, err = io.WriteString(output, helpText)
	return err
}

func helpTextForTopic(topic string) (string, error) {
	switch topic {
	case "init":
		return initHelpText(), nil
	case "config":
		return configHelpText(), nil
	case "list":
		return listHelpText(), nil
	case "inspect":
		return inspectHelpText(), nil
	case "apply":
		return applyHelpText(), nil
	case "status":
		return statusHelpText(), nil
	case "diff":
		return diffHelpText(), nil
	default:
		return "", usageCommandError(false, "%s\n\n%s", unknownHelpTopicMessage(topic), usageText())
	}
}

func runInteractiveCommand(workingDirectory string, runProgram func(tea.Model) error) error {
	return runInteractiveCommandWithTerminalRunner(workingDirectory, adaptTerminalProgramRunner(runProgram), nil, nil)
}

func runInteractiveCommandWithTerminalRunner(workingDirectory string, runProgram terminalProgramRunner, input io.Reader, output io.Writer) error {
	return runInteractiveCommandWithConfigEditor(workingDirectory, runProgram, input, output, runConfigEditorForWorkingDirectory)
}

func runInteractiveCommandWithConfigEditor(workingDirectory string, runProgram terminalProgramRunner, input io.Reader, output io.Writer, runEditor configEditorRunner) error {
	application, err := loadApplication(workingDirectory)
	if err != nil {
		return err
	}

	for {
		finalModel, err := runProgram(tui.New(application))
		if err != nil {
			return fmt.Errorf("run terminal UI: %w", err)
		}

		configRequest, ok := finalModel.(interface{ ConfigRequested() bool })
		if !ok || !configRequest.ConfigRequested() {
			return nil
		}

		result, err := runEditor(workingDirectory, input, output)
		if err != nil {
			return err
		}
		if !result.Saved {
			continue
		}

		application, err = loadApplication(workingDirectory)
		if err != nil {
			if _, runErr := runProgram(tui.NewReloadError(err)); runErr != nil {
				return fmt.Errorf("run terminal UI: %w", runErr)
			}

			return nil
		}
	}
}

func loadApplication(workingDirectory string) (app.Application, error) {
	configPath, err := config.Discover(workingDirectory)
	if err != nil {
		return app.Application{}, err
	}

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		return app.Application{}, err
	}

	application := app.NewWithTargets(loadedConfig.Targets, loadedConfig.Profiles)
	if err := application.ValidateStartup(); err != nil {
		return app.Application{}, err
	}

	return application, nil
}

func startFullScreenProgram(output io.Writer) terminalProgramRunner {
	return func(model tea.Model) (tea.Model, error) {
		finalModel, err := runFullScreenTerminalProgram(model)
		if err != nil {
			return nil, err
		}

		if model, ok := finalModel.(tui.Model); ok {
			if finalMessage := model.FinalMessage(); finalMessage != "" {
				if _, err := fmt.Fprint(output, finalMessage); err != nil {
					return finalModel, fmt.Errorf("write final terminal message: %w", err)
				}
			}
		}

		return finalModel, nil
	}
}

func runFullScreenTerminalProgram(model tea.Model, options ...tea.ProgramOption) (tea.Model, error) {
	programOptions := make([]tea.ProgramOption, 0, len(options)+1)
	programOptions = append(programOptions, options...)
	programOptions = append(programOptions, tea.WithAltScreen())

	return tea.NewProgram(model, programOptions...).Run()
}

func parseArguments(args []string, allowedFlags map[string]*bool) ([]string, error) {
	positionals := make([]string, 0, len(args))
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		flagValue, ok := allowedFlags[arg]
		if !ok {
			return nil, unsupportedFlagError(arg, allowedFlagNames(allowedFlags))
		}

		*flagValue = true
	}

	return positionals, nil
}

func allowedFlagNames(allowedFlags map[string]*bool) []string {
	flagNames := make([]string, 0, len(allowedFlags))
	for flagName := range allowedFlags {
		flagNames = append(flagNames, flagName)
	}
	sort.Strings(flagNames)

	return flagNames
}

func containsJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}

	return false
}

func wantsHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}

	return false
}
