package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

type commandError struct {
	message       string
	exitCode      int
	jsonOutput    bool
	outputOptions commandOutputOptions
}

func (errorValue commandError) Error() string {
	return errorValue.message
}

func writeCommandError(err error, output io.Writer, errorOutput io.Writer) error {
	writer := errorOutput

	var commandFailure commandError
	if errors.As(err, &commandFailure) {
		if commandFailure.jsonOutput {
			_, writeErr := fmt.Fprintln(output, err)
			return writeErr
		}

		_, writeErr := fmt.Fprintln(writer, renderCommandErrorText(commandFailure.message, commandFailure.outputOptions))
		return writeErr
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

func usageCommandError(outputOptions commandOutputOptions, jsonOutput bool, format string, arguments ...any) error {
	return commandFailure(outputOptions, jsonOutput, usageExitCode, "usage", fmt.Sprintf(format, arguments...))
}

func runtimeCommandError(outputOptions commandOutputOptions, jsonOutput bool, err error) error {
	return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatRuntimeErrorMessage(err, err.Error()))
}

func runtimeCommandErrorWithMessage(outputOptions commandOutputOptions, jsonOutput bool, err error, message string) error {
	message = formatRuntimeErrorMessage(err, message)
	if !jsonOutput {
		return commandError{message: message, exitCode: runtimeExitCode, outputOptions: outputOptions}
	}

	return commandFailure(outputOptions, true, runtimeExitCode, runtimeErrorKind(err), message)
}

func mainPickerRequiresTerminalError(outputOptions commandOutputOptions) error {
	return commandError{message: mainPickerRequiresTerminalMessage(), exitCode: runtimeExitCode, outputOptions: outputOptions}
}

func mainPickerRequiresTerminalMessage() string {
	return "`switchlet` without a command launches the interactive profile picker.\n\nstdin and stdout must be interactive terminals.\n\nFor non-interactive use, run:\n  switchlet list\n  switchlet status\n  switchlet apply <profile-name>"
}

func applyCommandError(outputOptions commandOutputOptions, jsonOutput bool, application app.Application, profileName string, err error, projectRoot string) error {
	if errors.Is(err, app.ErrProfileNotFound) {
		return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatMissingProfileMessage(profileName, application.Profiles()))
	}
	if errors.Is(err, app.ErrProfileUnavailable) {
		profileItem, inspectErr := application.InspectProfileByName(profileName)
		if inspectErr == nil {
			return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatUnavailableProfileMessage(profileItem))
		}
	}

	var targetErr editor.TargetError
	if errors.As(err, &targetErr) {
		return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatTargetErrorMessage(profileName, targetErr, textProjectRoot(jsonOutput, projectRoot)))
	}

	return runtimeCommandError(outputOptions, jsonOutput, err)
}

func formatRuntimeErrorMessage(err error, message string) string {
	if errors.Is(err, config.ErrConfigNotFound) {
		return "No .switchlet.yaml found.\n\nRun `switchlet init` to create one, or run Switchlet from a configured project."
	}

	return message
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

func commandFailure(outputOptions commandOutputOptions, jsonOutput bool, exitCode int, kind string, message string) error {
	if !jsonOutput {
		return commandError{message: message, exitCode: exitCode, outputOptions: outputOptions}
	}

	encodedError, err := json.Marshal(struct {
		Error commandErrorJSON `json:"error"`
	}{Error: commandErrorJSON{Kind: kind, Message: message}})
	if err != nil {
		return fmt.Errorf("serialize command error: %w", err)
	}

	return commandError{message: string(encodedError), exitCode: exitCode, jsonOutput: true, outputOptions: outputOptions}
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

func noProfileUsageCommandError(workingDirectory string, commandName string, outputOptions commandOutputOptions, jsonOutput bool) error {
	application, err := loadApplication(workingDirectory)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "No profile specified.\n\n%s", profileCommandHelpText(commandName))
	}

	profiles := application.Profiles()
	if len(profiles) == 0 {
		return usageCommandError(outputOptions, jsonOutput, "No profile specified.\n\n%s", profileCommandHelpText(commandName))
	}

	return usageCommandError(outputOptions, jsonOutput, "%s", formatNoProfileMessage(commandName, profiles))
}

func profileCommandHelpText(commandName string) string {
	switch commandName {
	case "apply":
		return applyHelpText()
	case "inspect":
		return inspectHelpText()
	case "diff":
		return diffHelpText()
	default:
		return usageText()
	}
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
	case "diff":
		return fmt.Sprintf("switchlet diff %s", profileName)
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
	if value != "" && !strings.ContainsAny(value, " \t\n\r\"\\$`'!#&();<>|*?[]{}~") {
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

	fmt.Fprintf(&builder, "\n\nHint:\nRun `switchlet inspect %s` to review profile values.", quoteCommandArgument(profileItem.Name))
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

func formatTargetErrorMessage(profileName string, targetErr editor.TargetError, projectRoot string) string {
	target := targetErr.Target
	var builder strings.Builder
	fmt.Fprintf(&builder, "Could not prepare target %q.", targetNameLabel(target.Name))

	if target.File != "" {
		fmt.Fprintf(&builder, "\n\nFile:\n%s", displayProjectPath(projectRoot, target.File))
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

	fmt.Fprintf(&builder, "\n\nHint:\nRun `switchlet inspect %s` to review planned targets.", quoteCommandArgument(profileName))
	return builder.String()
}

func comparisonCommandError(outputOptions commandOutputOptions, jsonOutput bool, commandName string, err error, projectRoot string) error {
	var targetErr editor.TargetError
	if errors.As(err, &targetErr) {
		return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatTargetReadErrorMessage(commandName, targetErr, textProjectRoot(jsonOutput, projectRoot)))
	}

	return runtimeCommandError(outputOptions, jsonOutput, err)
}

func diffCommandError(outputOptions commandOutputOptions, jsonOutput bool, application app.Application, profileName string, err error, projectRoot string) error {
	if errors.Is(err, app.ErrProfileNotFound) {
		return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatMissingProfileMessage(profileName, application.Profiles()))
	}

	return comparisonCommandError(outputOptions, jsonOutput, "diff", err, projectRoot)
}

func formatTargetReadErrorMessage(commandName string, targetErr editor.TargetError, projectRoot string) string {
	target := targetErr.Target
	var builder strings.Builder
	fmt.Fprintf(&builder, "Could not read target %q while running switchlet %s.", targetNameLabel(target.Name), commandName)

	if target.File != "" {
		fmt.Fprintf(&builder, "\n\nFile:\n%s", displayProjectPath(projectRoot, target.File))
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

	return builder.String()
}

func textProjectRoot(jsonOutput bool, projectRoot string) string {
	if jsonOutput {
		return ""
	}

	return projectRoot
}

func selectorValue(target config.Target) string {
	switch target.Type {
	case config.TargetTypeDotenv:
		return target.Key
	case config.TargetTypeYAML:
		return target.YAMLPath
	case config.TargetTypeTOML:
		return target.TOMLPath
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
