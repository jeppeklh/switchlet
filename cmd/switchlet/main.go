package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

const (
	runtimeExitCode = 1
	usageExitCode   = 2
	noColorFlag     = "--no-color"
)

type terminalProgramRunner func(tea.Model) (tea.Model, error)

var (
	commandNames   = []string{"help", "init", "config", "list", "inspect", "apply", "status", "diff", "version", "completion"}
	helpTopicNames = []string{"init", "config", "list", "inspect", "apply", "status", "diff", "version", "completion"}
	buildVersion   = ""
)

type loadedProject struct {
	Application app.Application
	ProjectRoot string
}

type commandOutputOptions struct {
	NoColor bool
}

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
	outputOptions := defaultCommandOutputOptions()
	args = parseLeadingCommandOutputOptions(args, &outputOptions)

	if len(args) == 0 {
		if !terminalInteractionAvailable(input, output) {
			return mainPickerRequiresTerminalError(outputOptions)
		}

		return runInteractiveCommandWithTerminalRunner(workingDirectory, runProgram, input, output)
	}

	switch args[0] {
	case profileCompletionCommandName:
		return writeProfileNameCompletions(output, workingDirectory)
	case "help", "-h", "--help":
		return writeHelp(output, args[1:], outputOptions)
	case "--version":
		if len(args) != 1 {
			return usageCommandError(outputOptions, false, "--version does not accept arguments\n\n%s", usageText())
		}

		return writeVersion(output)
	case "version":
		if wantsHelpFlag(args[1:]) {
			_, err := io.WriteString(output, versionHelpText())
			return err
		}

		positionals, err := parseArguments(args[1:], map[string]*bool{}, &outputOptions)
		if err != nil {
			return usageCommandError(outputOptions, false, "version: %v\n\n%s", err, versionHelpText())
		}
		if len(positionals) != 0 {
			return usageCommandError(outputOptions, false, "version does not accept a positional argument\n\n%s", versionHelpText())
		}

		return writeVersion(output)
	case "completion":
		if wantsHelpFlag(args[1:]) {
			_, err := io.WriteString(output, completionHelpText())
			return err
		}

		positionals, err := parseArguments(args[1:], map[string]*bool{}, &outputOptions)
		if err != nil {
			return usageCommandError(outputOptions, false, "completion: %v\n\n%s", err, completionHelpText())
		}
		if len(positionals) == 0 {
			return usageCommandError(outputOptions, false, "completion requires a shell name\n\n%s", completionHelpText())
		}
		if len(positionals) != 1 {
			return usageCommandError(outputOptions, false, "completion requires exactly one shell name\n\n%s", completionHelpText())
		}

		return writeCompletionScript(output, positionals[0], outputOptions)
	case "init":
		if wantsHelpFlag(args[1:]) {
			_, err := io.WriteString(output, initHelpText())
			return err
		}

		overwriteExistingConfig := false
		positionals, err := parseArguments(args[1:], map[string]*bool{"--overwrite": &overwriteExistingConfig}, &outputOptions)
		if err != nil {
			return usageCommandError(outputOptions, false, "init: %v\n\n%s", err, initHelpText())
		}
		if len(positionals) != 0 {
			return usageCommandError(outputOptions, false, "init does not accept a positional argument\n\n%s", initHelpText())
		}

		return runInitWithOptions(workingDirectory, input, output, defaultInitDependencies(), initOptions{OverwriteExistingConfig: overwriteExistingConfig})
	case "config":
		if wantsHelpFlag(args[1:]) {
			_, err := io.WriteString(output, configHelpText())
			return err
		}

		positionals, err := parseArguments(args[1:], map[string]*bool{}, &outputOptions)
		if err != nil {
			return usageCommandError(outputOptions, false, "config: %v\n\n%s", err, configHelpText())
		}
		if len(positionals) != 0 {
			return usageCommandError(outputOptions, false, "config does not accept a positional argument\n\n%s", configHelpText())
		}

		return runConfigCommand(workingDirectory, input, output)
	case "list":
		return runListCommand(workingDirectory, args[1:], output, outputOptions)
	case "inspect":
		return runInspectCommand(workingDirectory, args[1:], output, outputOptions)
	case "apply":
		return runApplyCommand(workingDirectory, args[1:], output, outputOptions)
	case "status":
		return runStatusCommand(workingDirectory, args[1:], output, outputOptions)
	case "diff":
		return runDiffCommand(workingDirectory, args[1:], output, outputOptions)
	default:
		return usageCommandError(outputOptions, false, "%s\n\n%s", unknownCommandMessage(args[0]), usageText())
	}
}

func defaultCommandOutputOptions() commandOutputOptions {
	return commandOutputOptions{NoColor: os.Getenv("NO_COLOR") != ""}
}

func parseLeadingCommandOutputOptions(args []string, outputOptions *commandOutputOptions) []string {
	for len(args) > 0 && args[0] == noColorFlag {
		outputOptions.NoColor = true
		args = args[1:]
	}

	return args
}

func adaptTerminalProgramRunner(runProgram func(tea.Model) error) terminalProgramRunner {
	return func(model tea.Model) (tea.Model, error) {
		return model, runProgram(model)
	}
}

func writeHelp(output io.Writer, args []string, outputOptions commandOutputOptions) error {
	if len(args) == 0 {
		_, err := io.WriteString(output, usageText())
		return err
	}
	if len(args) != 1 {
		return usageCommandError(outputOptions, false, "help accepts at most one command name\n\n%s", usageText())
	}

	helpText, err := helpTextForTopic(args[0], outputOptions)
	if err != nil {
		return err
	}

	_, err = io.WriteString(output, helpText)
	return err
}

func helpTextForTopic(topic string, outputOptions commandOutputOptions) (string, error) {
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
	case "version":
		return versionHelpText(), nil
	case "completion":
		return completionHelpText(), nil
	default:
		return "", usageCommandError(outputOptions, false, "%s\n\n%s", unknownHelpTopicMessage(topic), usageText())
	}
}

func writeVersion(output io.Writer) error {
	_, err := fmt.Fprintln(output, versionLine())
	return err
}

func versionLine() string {
	return "switchlet " + resolvedVersion()
}

func resolvedVersion() string {
	if version := strings.TrimSpace(buildVersion); version != "" {
		return version
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if ok && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
		return buildInfo.Main.Version
	}

	return "dev"
}

func runInteractiveCommand(workingDirectory string, runProgram func(tea.Model) error) error {
	return runInteractiveCommandWithTerminalRunner(workingDirectory, adaptTerminalProgramRunner(runProgram), nil, nil)
}

func runInteractiveCommandWithTerminalRunner(workingDirectory string, runProgram terminalProgramRunner, input io.Reader, output io.Writer) error {
	project, err := loadProject(workingDirectory)
	if err != nil {
		return err
	}

	if _, err := runProgram(newInteractiveSessionModel(workingDirectory, project.Application, project.ProjectRoot)); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}

	return nil
}

func loadApplication(workingDirectory string) (app.Application, error) {
	project, err := loadProject(workingDirectory)
	if err != nil {
		return app.Application{}, err
	}

	return project.Application, nil
}

func loadProject(workingDirectory string) (loadedProject, error) {
	configPath, err := config.Discover(workingDirectory)
	if err != nil {
		return loadedProject{}, err
	}

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		return loadedProject{}, err
	}

	application := app.NewWithTargets(loadedConfig.Targets, loadedConfig.Profiles)
	if err := application.ValidateStartup(); err != nil {
		return loadedProject{}, err
	}

	return loadedProject{Application: application, ProjectRoot: filepath.Dir(configPath)}, nil
}

func startFullScreenProgram(output io.Writer) terminalProgramRunner {
	return func(model tea.Model) (tea.Model, error) {
		finalModel, err := runFullScreenTerminalProgram(model)
		if err != nil {
			return nil, err
		}

		if model, ok := finalModel.(interface{ FinalMessage() string }); ok {
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

func parseArguments(args []string, allowedFlags map[string]*bool, outputOptions *commandOutputOptions) ([]string, error) {
	args = parseCommandOutputOptions(args, outputOptions)

	positionals := make([]string, 0, len(args))
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		flagValue, ok := allowedFlags[arg]
		if !ok {
			return nil, unsupportedFlagError(arg, allowedFlagNames(allowedFlags, outputOptions))
		}

		*flagValue = true
	}

	return positionals, nil
}

func parseCommandOutputOptions(args []string, outputOptions *commandOutputOptions) []string {
	if outputOptions == nil {
		return args
	}

	parsedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == noColorFlag {
			outputOptions.NoColor = true
			continue
		}

		parsedArgs = append(parsedArgs, arg)
	}

	return parsedArgs
}

func allowedFlagNames(allowedFlags map[string]*bool, outputOptions *commandOutputOptions) []string {
	flagNames := make([]string, 0, len(allowedFlags))
	for flagName := range allowedFlags {
		flagNames = append(flagNames, flagName)
	}
	if outputOptions != nil {
		flagNames = append(flagNames, noColorFlag)
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
