package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

func mainPickerRequiresTerminalMessage() string {
	return "`switchlet` without a command launches the interactive profile picker.\n\nstdin and stdout must be interactive terminals.\n\nFor non-interactive use, run:\n  switchlet list\n  switchlet status\n  switchlet apply <profile-name>"
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

	switch commandName {
	case "apply":
		if strings.HasPrefix(profileItem.Name, "-") {
			command := "switchlet apply --dry-run"
			if profileItem.Protected {
				command += " --allow-protected"
			}
			return fmt.Sprintf("%s -- %s", command, quoteCommandArgument(profileItem.Name))
		}

		command := fmt.Sprintf("switchlet apply %s --dry-run", quoteCommandArgument(profileItem.Name))
		if profileItem.Protected {
			command += " --allow-protected"
		}
		return command
	case "inspect":
		return profileCommandSuggestion("switchlet inspect", profileItem.Name)
	case "diff":
		return profileCommandSuggestion("switchlet diff", profileItem.Name)
	default:
		return ""
	}
}

func profileCommandSuggestion(commandName string, profileName string) string {
	if strings.HasPrefix(profileName, "-") {
		return fmt.Sprintf("%s -- %s", commandName, quoteCommandArgument(profileName))
	}

	return fmt.Sprintf("%s %s", commandName, quoteCommandArgument(profileName))
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

	fmt.Fprintf(&builder, "\n\nHint:\nRun `%s` to review profile values.", profileCommandSuggestion("switchlet inspect", profileItem.Name))
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

	fmt.Fprintf(&builder, "\n\nHint:\nRun `%s` to review planned targets.", profileCommandSuggestion("switchlet inspect", profileName))
	return builder.String()
}

func formatPreflightErrorMessage(profileName string, preflightErr editor.PreflightError, projectRoot string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Could not verify target-file writes before applying profile %q.", profileName)

	if preflightErr.TargetFile != "" {
		fmt.Fprintf(&builder, "\n\nFile:\n%s", displayProjectPath(projectRoot, preflightErr.TargetFile))
	}
	if preflightErr.Err != nil {
		fmt.Fprintf(&builder, "\n\nReason:\n%s", preflightErr.Err)
	}

	builder.WriteString("\n\nNo target files were replaced.")
	builder.WriteString("\n\nHint:\nCheck permissions and available space for the listed target file, then retry the apply.")
	return builder.String()
}

func formatRecoveryErrorMessage(profileName string, recoveryErr editor.RecoveryError, projectRoot string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Apply failed while writing target files for profile %q.", profileName)

	if recoveryErr.FailedFile != "" {
		fmt.Fprintf(&builder, "\n\nFailed file:\n%s", displayProjectPath(projectRoot, recoveryErr.FailedFile))
	}
	if len(recoveryErr.RestoredFiles) > 0 {
		builder.WriteString("\n\nRestored files:")
		writeDisplayFileList(&builder, projectRoot, recoveryErr.RestoredFiles)
	}
	if len(recoveryErr.UnrestoredFiles) > 0 {
		builder.WriteString("\n\nFiles that may still be updated:")
		writeDisplayFileList(&builder, projectRoot, recoveryErr.UnrestoredFiles)
	} else if len(recoveryErr.ReplacedFiles) > 0 {
		builder.WriteString("\n\nPrior replacements were restored.")
	}
	if recoveryErr.Err != nil {
		fmt.Fprintf(&builder, "\n\nWrite failure:\n%s", recoveryErr.Err)
	}
	if recoveryErr.RestoreErr != nil {
		fmt.Fprintf(&builder, "\n\nRestoration failure:\n%s", recoveryErr.RestoreErr)
	}

	if len(recoveryErr.UnrestoredFiles) > 0 {
		builder.WriteString("\n\nHint:\nReview the listed files before running the application.")
		return builder.String()
	}

	builder.WriteString("\n\nHint:\nThe failed apply did not leave earlier target files updated. Fix the write failure and retry.")
	return builder.String()
}

func writeDisplayFileList(builder *strings.Builder, projectRoot string, files []string) {
	for _, file := range files {
		fmt.Fprintf(builder, "\n- %s", displayProjectPath(projectRoot, file))
	}
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
