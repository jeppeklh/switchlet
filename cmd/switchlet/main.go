package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/tui"
)

const (
	runtimeExitCode = 1
	usageExitCode   = 2
)

type commandError struct {
	message    string
	exitCode   int
	jsonOutput bool
}

func (errorValue commandError) Error() string {
	return errorValue.message
}

func main() {
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get working directory: %v\n", err)
		os.Exit(runtimeExitCode)
	}

	if err := runCommand(os.Args[1:], workingDirectory, startProgram, os.Stdin, os.Stdout); err != nil {
		if writeErr := writeCommandError(err, os.Stdout, os.Stderr); writeErr != nil {
			fmt.Fprintln(os.Stderr, writeErr)
			os.Exit(runtimeExitCode)
		}

		os.Exit(exitCodeForError(err))
	}
}

func runCommand(args []string, workingDirectory string, runProgram func(tea.Model) error, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return runInteractiveCommand(workingDirectory, runProgram)
	}

	switch args[0] {
	case "help", "-h", "--help":
		return writeHelp(output, args[1:])
	case "init":
		if wantsHelpFlag(args[1:]) {
			_, err := io.WriteString(output, initHelpText())
			return err
		}
		if len(args) != 1 {
			return usageCommandError(false, "init does not accept additional arguments\n\n%s", initHelpText())
		}

		return runInit(workingDirectory, input, output, defaultInitDependencies())
	case "list":
		return runListCommand(workingDirectory, args[1:], output)
	case "inspect":
		return runInspectCommand(workingDirectory, args[1:], output)
	case "apply":
		return runApplyCommand(workingDirectory, args[1:], output)
	default:
		return usageCommandError(false, "unknown command %q\n\n%s", args[0], usageText())
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
	case "list":
		return listHelpText(), nil
	case "inspect":
		return inspectHelpText(), nil
	case "apply":
		return applyHelpText(), nil
	default:
		return "", usageCommandError(false, "unknown help topic %q\n\n%s", topic, usageText())
	}
}

func runInteractiveCommand(workingDirectory string, runProgram func(tea.Model) error) error {
	application, err := loadApplication(workingDirectory)
	if err != nil {
		return err
	}

	if err := runProgram(tui.New(application)); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}

	return nil
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

	application := app.New(loadedConfig.Target, loadedConfig.Profiles)
	if err := application.ValidateStartup(); err != nil {
		return app.Application{}, err
	}

	return application, nil
}

func startProgram(model tea.Model) error {
	_, err := tea.NewProgram(model).Run()
	return err
}

func runListCommand(workingDirectory string, args []string, output io.Writer) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, listHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput})
	if err != nil {
		return usageCommandError(jsonOutput, "list: %v\n\n%s", err, listHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(jsonOutput, "list does not accept a profile name\n\n%s", listHelpText())
	}

	application, err := loadApplication(workingDirectory)
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	profiles := application.Profiles()
	if jsonOutput {
		return writeListJSON(output, profiles)
	}

	return writeListText(output, profiles)
}

func runInspectCommand(workingDirectory string, args []string, output io.Writer) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, inspectHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput})
	if err != nil {
		return usageCommandError(jsonOutput, "inspect: %v\n\n%s", err, inspectHelpText())
	}
	if len(positionals) != 1 {
		return usageCommandError(jsonOutput, "inspect requires exactly one profile name\n\n%s", inspectHelpText())
	}

	application, err := loadApplication(workingDirectory)
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	profileItem, err := application.InspectProfileByName(positionals[0])
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	if jsonOutput {
		return writeInspectJSON(output, profileItem)
	}

	return writeInspectText(output, profileItem)
}

func runApplyCommand(workingDirectory string, args []string, output io.Writer) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, applyHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	allowProtected := false
	dryRun := false

	positionals, err := parseArguments(args, map[string]*bool{
		"--json":            &jsonOutput,
		"--dry-run":         &dryRun,
		"--allow-protected": &allowProtected,
	})
	if err != nil {
		return usageCommandError(jsonOutput, "apply: %v\n\n%s", err, applyHelpText())
	}
	if len(positionals) != 1 {
		return usageCommandError(jsonOutput, "apply requires exactly one profile name\n\n%s", applyHelpText())
	}

	application, err := loadApplication(workingDirectory)
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	result, err := application.ApplyProfileByNameWithOptions(positionals[0], app.ApplyOptions{
		DryRun:         dryRun,
		AllowProtected: allowProtected,
	})
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	if jsonOutput {
		return writeApplyJSON(output, result)
	}

	return writeApplyText(output, result)
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
			return nil, fmt.Errorf("unsupported flag %q", arg)
		}

		*flagValue = true
	}

	return positionals, nil
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

func writeCommandError(err error, output io.Writer, errorOutput io.Writer) error {
	writer := errorOutput

	var commandFailure commandError
	if errors.As(err, &commandFailure) && commandFailure.jsonOutput {
		writer = output
	}

	_, writeErr := fmt.Fprintln(writer, err)
	return writeErr
}

func exitCodeForError(err error) int {
	var commandFailure commandError
	if errors.As(err, &commandFailure) {
		return commandFailure.exitCode
	}

	return runtimeExitCode
}

func usageCommandError(jsonOutput bool, format string, arguments ...any) error {
	return commandFailure(jsonOutput, usageExitCode, "usage", fmt.Sprintf(format, arguments...))
}

func runtimeCommandError(jsonOutput bool, err error) error {
	if !jsonOutput {
		return err
	}

	return commandFailure(true, runtimeExitCode, runtimeErrorKind(err), err.Error())
}

func commandFailure(jsonOutput bool, exitCode int, kind string, message string) error {
	if !jsonOutput {
		return commandError{message: message, exitCode: exitCode}
	}

	encodedError, err := json.Marshal(struct {
		Error commandErrorJSON `json:"error"`
	}{Error: commandErrorJSON{Kind: kind, Message: message}})
	if err != nil {
		return fmt.Errorf("serialize command error: %w", err)
	}

	return commandError{message: string(encodedError), exitCode: exitCode, jsonOutput: true}
}

func runtimeErrorKind(err error) string {
	switch {
	case errors.Is(err, app.ErrProfileNotFound):
		return "profile_not_found"
	case errors.Is(err, app.ErrProfileUnavailable):
		return "profile_unavailable"
	case errors.Is(err, app.ErrProtectedProfileRequiresApproval):
		return "protected_profile"
	case errors.Is(err, config.ErrConfigNotFound):
		return "config_not_found"
	default:
		return "runtime"
	}
}

type commandErrorJSON struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}
