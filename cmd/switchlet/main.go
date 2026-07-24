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
		return run(workingDirectory, runProgram)
	}

	switch args[0] {
	case "help", "-h", "--help":
		_, err := io.WriteString(output, usageText())
		return err
	case "init":
		if len(args) != 1 {
			return usageCommandError(false, "init does not accept additional arguments\n\n%s", usageText())
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

func run(workingDirectory string, runProgram func(tea.Model) error) error {
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
	jsonOutput := containsJSONFlag(args)

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput})
	if err != nil {
		return usageCommandError(jsonOutput, "list: %v\n\n%s", err, usageText())
	}
	if len(positionals) != 0 {
		return usageCommandError(jsonOutput, "list does not accept a profile name\n\n%s", usageText())
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
	jsonOutput := containsJSONFlag(args)

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput})
	if err != nil {
		return usageCommandError(jsonOutput, "inspect: %v\n\n%s", err, usageText())
	}
	if len(positionals) != 1 {
		return usageCommandError(jsonOutput, "inspect requires exactly one profile name\n\n%s", usageText())
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
	jsonOutput := containsJSONFlag(args)
	allowProtected := false
	dryRun := false

	positionals, err := parseArguments(args, map[string]*bool{
		"--json":            &jsonOutput,
		"--dry-run":         &dryRun,
		"--allow-protected": &allowProtected,
	})
	if err != nil {
		return usageCommandError(jsonOutput, "apply: %v\n\n%s", err, usageText())
	}
	if len(positionals) != 1 {
		return usageCommandError(jsonOutput, "apply requires exactly one profile name\n\n%s", usageText())
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

func writeListText(output io.Writer, profiles []app.ProfileItem) error {
	for _, profileItem := range profiles {
		indicators := make([]string, 0, 2)
		if profileItem.Protected {
			indicators = append(indicators, "protected")
		}
		if !profileItem.Available {
			indicators = append(indicators, "unavailable")
		}

		line := profileItem.Name
		if len(indicators) > 0 {
			line += " [" + strings.Join(indicators, ", ") + "]"
		}

		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
		if profileItem.UnavailableReason != "" {
			if _, err := fmt.Fprintf(output, "  reason: %s\n", profileItem.UnavailableReason); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeInspectText(output io.Writer, profileItem app.ProfileItem) error {
	if _, err := fmt.Fprintf(output, "Profile: %s\n", profileItem.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Availability: %s\n", availabilityLabel(profileItem)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Source: %s\n", sourceLabel(profileItem.Source)); err != nil {
		return err
	}
	if profileItem.EnvironmentVariableName != "" {
		if _, err := fmt.Fprintf(output, "Environment variable: %s\n", profileItem.EnvironmentVariableName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(output, "Protection: %s\n\n", protectionLabel(profileItem)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Masked value:\n%s\n", maskedValueLabel(profileItem)); err != nil {
		return err
	}
	if profileItem.UnavailableReason != "" {
		if _, err := fmt.Fprintf(output, "\nResolution error:\n%s\n", profileItem.UnavailableReason); err != nil {
			return err
		}
	}

	return nil
}

func writeApplyText(output io.Writer, result app.Result) error {
	if result.DryRun {
		if _, err := fmt.Fprintf(output, "Dry run successful: %s\n\nValidated target:\n%s\n\nNo changes were written.\n", result.ProfileName, result.TargetPath); err != nil {
			return err
		}

		return nil
	}

	if _, err := fmt.Fprintf(output, "Applied profile: %s\n\nUpdated target:\n%s\n", result.ProfileName, result.TargetPath); err != nil {
		return err
	}

	return nil
}

func writeListJSON(output io.Writer, profiles []app.ProfileItem) error {
	encodedProfiles := make([]profileJSON, 0, len(profiles))
	for _, profileItem := range profiles {
		encodedProfiles = append(encodedProfiles, profileJSONFromItem(profileItem))
	}

	return writeJSON(output, struct {
		Profiles []profileJSON `json:"profiles"`
	}{Profiles: encodedProfiles})
}

func writeInspectJSON(output io.Writer, profileItem app.ProfileItem) error {
	return writeJSON(output, struct {
		Profile profileJSON `json:"profile"`
	}{Profile: profileJSONFromItem(profileItem)})
}

func writeApplyJSON(output io.Writer, result app.Result) error {
	return writeJSON(output, struct {
		Result applyResultJSON `json:"result"`
	}{Result: applyResultJSON{
		ProfileName: result.ProfileName,
		TargetPath:  result.TargetPath,
		TargetFile:  result.TargetFile,
		Protected:   result.Protected,
		DryRun:      result.DryRun,
	}})
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
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

func availabilityLabel(profileItem app.ProfileItem) string {
	if profileItem.Available {
		return "Available"
	}

	return "Unavailable"
}

func sourceLabel(source app.ProfileSource) string {
	switch source {
	case app.ProfileSourceEnvironment:
		return "Environment variable"
	case app.ProfileSourceLiteral:
		return "Literal"
	default:
		return "Unknown"
	}
}

func protectionLabel(profileItem app.ProfileItem) string {
	if profileItem.Protected {
		return "Protected"
	}

	return "Not protected"
}

func maskedValueLabel(profileItem app.ProfileItem) string {
	if !profileItem.Available {
		return "Unavailable"
	}
	if profileItem.MaskedValue == "" {
		return "<empty>"
	}

	return profileItem.MaskedValue
}

type profileJSON struct {
	Name                    string            `json:"name"`
	Protected               bool              `json:"protected"`
	Available               bool              `json:"available"`
	Source                  app.ProfileSource `json:"source"`
	EnvironmentVariableName string            `json:"environmentVariableName"`
	MaskedValue             string            `json:"maskedValue"`
	UnavailableReason       string            `json:"unavailableReason"`
}

type applyResultJSON struct {
	ProfileName string `json:"profileName"`
	TargetPath  string `json:"targetPath"`
	TargetFile  string `json:"targetFile"`
	Protected   bool   `json:"protected"`
	DryRun      bool   `json:"dryRun"`
}

type commandErrorJSON struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func profileJSONFromItem(profileItem app.ProfileItem) profileJSON {
	return profileJSON{
		Name:                    profileItem.Name,
		Protected:               profileItem.Protected,
		Available:               profileItem.Available,
		Source:                  profileItem.Source,
		EnvironmentVariableName: profileItem.EnvironmentVariableName,
		MaskedValue:             profileItem.MaskedValue,
		UnavailableReason:       profileItem.UnavailableReason,
	}
}

func usageText() string {
	return `Usage:
	  switchlet                                      Launch the profile switcher
	  switchlet init                                 Create a new .switchlet.yaml in the current directory
	  switchlet list [--json]                        List configured profiles without launching the TUI
	  switchlet inspect <profile-name> [--json]      Inspect one configured profile by name
	  switchlet apply <profile-name> [flags]         Apply one configured profile by name
	  switchlet help                                 Show this help text

	Apply flags:
	  --dry-run            Validate the apply operation without writing the target file
	  --allow-protected    Explicitly allow non-interactive use of a protected profile
	  --json               Write machine-readable JSON for non-interactive commands
`
}
