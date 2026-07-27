package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
	"github.com/jeppeklh/switchlet/internal/tui"
)

const (
	runtimeExitCode = 1
	usageExitCode   = 2
)

var (
	commandNames   = []string{"help", "init", "list", "inspect", "apply"}
	helpTopicNames = []string{"init", "list", "inspect", "apply"}
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

	if err := runCommand(os.Args[1:], workingDirectory, startFullScreenProgram(os.Stdout), os.Stdin, os.Stdout); err != nil {
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
		return usageCommandError(false, "%s\n\n%s", unknownCommandMessage(args[0]), usageText())
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
		return "", usageCommandError(false, "%s\n\n%s", unknownHelpTopicMessage(topic), usageText())
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

	application := app.NewWithTargets(loadedConfig.Targets, loadedConfig.Profiles)
	if err := application.ValidateStartup(); err != nil {
		return app.Application{}, err
	}

	return application, nil
}

func startFullScreenProgram(output io.Writer) func(tea.Model) error {
	return func(model tea.Model) error {
		finalModel, err := runFullScreenTerminalProgram(model)
		if err != nil {
			return err
		}

		if model, ok := finalModel.(tui.Model); ok {
			if finalMessage := model.FinalMessage(); finalMessage != "" {
				if _, err := fmt.Fprint(output, finalMessage); err != nil {
					return fmt.Errorf("write final terminal message: %w", err)
				}
			}
		}

		return nil
	}
}

func runFullScreenTerminalProgram(model tea.Model, options ...tea.ProgramOption) (tea.Model, error) {
	programOptions := make([]tea.ProgramOption, 0, len(options)+1)
	programOptions = append(programOptions, options...)
	programOptions = append(programOptions, tea.WithAltScreen())

	return tea.NewProgram(model, programOptions...).Run()
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
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "inspect", jsonOutput)
	}
	if len(positionals) != 1 {
		return usageCommandError(jsonOutput, "inspect requires exactly one profile name\n\n%s", inspectHelpText())
	}

	application, err := loadApplication(workingDirectory)
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	profileName := positionals[0]
	profileItem, err := application.InspectProfileByName(profileName)
	if err != nil {
		if errors.Is(err, app.ErrProfileNotFound) {
			return runtimeCommandErrorWithMessage(jsonOutput, err, formatMissingProfileMessage(profileName, application.Profiles()))
		}

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
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "apply", jsonOutput)
	}
	if len(positionals) != 1 {
		return usageCommandError(jsonOutput, "apply requires exactly one profile name\n\n%s", applyHelpText())
	}

	application, err := loadApplication(workingDirectory)
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	profileName := positionals[0]
	result, err := application.ApplyProfileByNameWithOptions(profileName, app.ApplyOptions{
		DryRun:         dryRun,
		AllowProtected: allowProtected,
	})
	if err != nil {
		return applyCommandError(jsonOutput, application, profileName, err)
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
			return nil, unsupportedFlagError(arg, allowedFlagNames(allowedFlags))
		}

		*flagValue = true
	}

	return positionals, nil
}

func noProfileUsageCommandError(workingDirectory string, commandName string, jsonOutput bool) error {
	application, err := loadApplication(workingDirectory)
	if err != nil {
		return usageCommandError(jsonOutput, "No profile specified.\n\n%s", profileCommandHelpText(commandName))
	}

	profiles := application.Profiles()
	if len(profiles) == 0 {
		return usageCommandError(jsonOutput, "No profile specified.\n\n%s", profileCommandHelpText(commandName))
	}

	return usageCommandError(jsonOutput, "%s", formatNoProfileMessage(commandName, profiles))
}

func profileCommandHelpText(commandName string) string {
	switch commandName {
	case "apply":
		return applyHelpText()
	case "inspect":
		return inspectHelpText()
	default:
		return usageText()
	}
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
	return runtimeCommandErrorWithMessage(jsonOutput, err, err.Error())
}

func runtimeCommandErrorWithMessage(jsonOutput bool, err error, message string) error {
	if !jsonOutput {
		return commandError{message: message, exitCode: runtimeExitCode}
	}

	return commandFailure(true, runtimeExitCode, runtimeErrorKind(err), message)
}

func applyCommandError(jsonOutput bool, application app.Application, profileName string, err error) error {
	if errors.Is(err, app.ErrProfileNotFound) {
		return runtimeCommandErrorWithMessage(jsonOutput, err, formatMissingProfileMessage(profileName, application.Profiles()))
	}
	if errors.Is(err, app.ErrProfileUnavailable) {
		profileItem, inspectErr := application.InspectProfileByName(profileName)
		if inspectErr == nil {
			return runtimeCommandErrorWithMessage(jsonOutput, err, formatUnavailableProfileMessage(profileItem))
		}
	}

	var targetErr editor.TargetError
	if errors.As(err, &targetErr) {
		return runtimeCommandErrorWithMessage(jsonOutput, err, formatTargetErrorMessage(profileName, targetErr))
	}

	return runtimeCommandError(jsonOutput, err)
}

func unknownCommandMessage(commandName string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "unknown command %q", commandName)
	if suggestion := suggestedName(commandName, commandNames); suggestion != "" {
		fmt.Fprintf(&builder, "\n\nDid you mean %q?", suggestion)
	}

	return builder.String()
}

func unknownHelpTopicMessage(topic string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "unknown help topic %q", topic)
	if suggestion := suggestedName(topic, helpTopicNames); suggestion != "" {
		fmt.Fprintf(&builder, "\n\nDid you mean %q?", suggestion)
	}

	return builder.String()
}

func unsupportedFlagError(flag string, allowedFlags []string) error {
	var builder strings.Builder
	fmt.Fprintf(&builder, "unsupported flag %q", flag)
	if suggestion := suggestedName(flag, allowedFlags); suggestion != "" {
		fmt.Fprintf(&builder, "\n\nDid you mean %q?", suggestion)
	}

	return errors.New(builder.String())
}

func allowedFlagNames(allowedFlags map[string]*bool) []string {
	flagNames := make([]string, 0, len(allowedFlags))
	for flagName := range allowedFlags {
		flagNames = append(flagNames, flagName)
	}
	sort.Strings(flagNames)

	return flagNames
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

func formatMissingProfileMessage(profileName string, profiles []app.ProfileItem) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Profile %q does not exist.", profileName)

	if len(profiles) > 0 {
		builder.WriteString("\n\nAvailable profiles:\n")
		for _, profileItem := range profiles {
			fmt.Fprintf(&builder, "- %s\n", profileItem.Name)
		}
	}

	if suggestion := suggestedProfileName(profileName, profiles); suggestion != "" {
		fmt.Fprintf(&builder, "\nDid you mean %q?", suggestion)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func formatNoProfileMessage(commandName string, profiles []app.ProfileItem) string {
	var builder strings.Builder
	builder.WriteString("No profile specified.")
	builder.WriteString("\n\nAvailable profiles:\n")
	for _, profileItem := range profiles {
		fmt.Fprintf(&builder, "- %s\n", profileGuidanceLabel(profileItem))
	}

	if tryCommand := tryCommandForNoProfile(commandName, profiles); tryCommand != "" {
		fmt.Fprintf(&builder, "\nTry:\n%s", tryCommand)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func profileGuidanceLabel(profileItem app.ProfileItem) string {
	indicators := make([]string, 0, 3)
	if profileItem.Protected {
		indicators = append(indicators, "protected")
	}
	if profileItem.Partial {
		indicators = append(indicators, "partial")
	}
	if !profileItem.Available {
		indicators = append(indicators, "unavailable")
	}
	if len(indicators) == 0 {
		return profileItem.Name
	}

	return fmt.Sprintf("%s [%s]", profileItem.Name, strings.Join(indicators, "] ["))
}

func tryCommandForNoProfile(commandName string, profiles []app.ProfileItem) string {
	profileItem, ok := exampleProfileForCommand(commandName, profiles)
	if !ok {
		return ""
	}

	profileName := quoteCommandArgument(profileItem.Name)
	switch commandName {
	case "apply":
		command := fmt.Sprintf("switchlet apply %s --dry-run", profileName)
		if profileItem.Protected {
			command += " --allow-protected"
		}
		return command
	case "inspect":
		return fmt.Sprintf("switchlet inspect %s", profileName)
	default:
		return ""
	}
}

func exampleProfileForCommand(commandName string, profiles []app.ProfileItem) (app.ProfileItem, bool) {
	if len(profiles) == 0 {
		return app.ProfileItem{}, false
	}

	if commandName == "apply" {
		for _, profileItem := range profiles {
			if profileItem.Available && !profileItem.Protected {
				return profileItem, true
			}
		}
		for _, profileItem := range profiles {
			if profileItem.Available {
				return profileItem, true
			}
		}
		for _, profileItem := range profiles {
			if !profileItem.Protected {
				return profileItem, true
			}
		}
	}

	return profiles[0], true
}

func quoteCommandArgument(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n\r\"\\$`") {
		return value
	}

	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
		"`", "\\`",
	)
	return `"` + replacer.Replace(value) + `"`
}

func formatUnavailableProfileMessage(profileItem app.ProfileItem) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Profile %q is unavailable.", profileItem.Name)

	unavailableValues := unavailableProfileValues(profileItem)
	if len(unavailableValues) > 0 {
		builder.WriteString("\n\nUnavailable values:")
		for _, valueItem := range unavailableValues {
			fmt.Fprintf(&builder, "\n- %s", targetNameLabel(valueItem.TargetName))
			if valueItem.EnvironmentVariableName != "" {
				fmt.Fprintf(&builder, "\n  environment variable: %s", valueItem.EnvironmentVariableName)
			}
			if valueItem.UnavailableReason != "" {
				fmt.Fprintf(&builder, "\n  reason: %s", valueItem.UnavailableReason)
			}
		}
	} else if profileItem.UnavailableReason != "" {
		fmt.Fprintf(&builder, "\n\nReason:\n%s", profileItem.UnavailableReason)
	}

	fmt.Fprintf(&builder, "\n\nHint:\nRun `switchlet inspect %s` to review profile values.", profileItem.Name)
	return builder.String()
}

func unavailableProfileValues(profileItem app.ProfileItem) []app.ProfileValueItem {
	unavailableValues := make([]app.ProfileValueItem, 0)
	for _, valueItem := range profileItem.Values {
		if !valueItem.Available {
			unavailableValues = append(unavailableValues, valueItem)
		}
	}

	return unavailableValues
}

func formatTargetErrorMessage(profileName string, targetErr editor.TargetError) string {
	target := targetErr.Target
	var builder strings.Builder
	fmt.Fprintf(&builder, "Could not prepare target %q.", targetNameLabel(target.Name))

	if target.File != "" {
		fmt.Fprintf(&builder, "\n\nFile:\n%s", target.File)
	}
	if target.Type != "" {
		fmt.Fprintf(&builder, "\n\nType:\n%s", target.Type)
	}
	if selector := selectorValue(target); selector != "" {
		fmt.Fprintf(&builder, "\n\nSelector:\n%s", selector)
	}
	if targetErr.Err != nil {
		fmt.Fprintf(&builder, "\n\nReason:\n%s", targetErr.Err)
	}

	fmt.Fprintf(&builder, "\n\nHint:\nRun `switchlet inspect %s` to review planned targets.", profileName)
	return builder.String()
}

func selectorValue(target config.Target) string {
	switch target.Type {
	case config.TargetTypeDotenv:
		return target.Key
	case config.TargetTypeYAML:
		return target.YAMLPath
	default:
		return target.JSONPath
	}
}

func suggestedProfileName(profileName string, profiles []app.ProfileItem) string {
	candidates := make([]string, 0, len(profiles))
	for _, profileItem := range profiles {
		candidates = append(candidates, profileItem.Name)
	}

	return suggestedName(profileName, candidates)
}

func suggestedName(requestedName string, candidates []string) string {
	requested := strings.ToLower(strings.TrimSpace(requestedName))
	if requested == "" {
		return ""
	}

	bestDistance := -1
	bestSuggestion := ""
	bestMatches := 0
	for _, candidateName := range candidates {
		candidate := strings.ToLower(candidateName)
		distance := levenshteinDistance(requested, candidate)
		if distance > suggestionThreshold(requested, candidate) {
			continue
		}
		if bestDistance < 0 || distance < bestDistance {
			bestDistance = distance
			bestSuggestion = candidateName
			bestMatches = 1
			continue
		}
		if distance == bestDistance {
			bestMatches++
		}
	}

	if bestMatches == 1 {
		return bestSuggestion
	}

	return ""
}

func suggestionThreshold(requested string, candidate string) int {
	maxLength := len([]rune(requested))
	if candidateLength := len([]rune(candidate)); candidateLength > maxLength {
		maxLength = candidateLength
	}
	if maxLength <= 3 {
		return 0
	}
	if maxLength <= 6 {
		return 1
	}

	return 2
}

func levenshteinDistance(left string, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	previous := make([]int, len(rightRunes)+1)
	current := make([]int, len(rightRunes)+1)

	for index := range previous {
		previous[index] = index
	}

	for leftIndex, leftRune := range leftRunes {
		current[0] = leftIndex + 1
		for rightIndex, rightRune := range rightRunes {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}

			current[rightIndex+1] = minInt(
				current[rightIndex]+1,
				previous[rightIndex+1]+1,
				previous[rightIndex]+cost,
			)
		}

		previous, current = current, previous
	}

	return previous[len(rightRunes)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}

	return minimum
}
