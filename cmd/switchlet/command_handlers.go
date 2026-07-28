package main

import (
	"errors"
	"io"

	"github.com/jeppeklh/switchlet/internal/app"
)

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

func runStatusCommand(workingDirectory string, args []string, output io.Writer) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, statusHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput})
	if err != nil {
		return usageCommandError(jsonOutput, "status: %v\n\n%s", err, statusHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(jsonOutput, "status does not accept a profile name\n\n%s", statusHelpText())
	}

	application, err := loadApplication(workingDirectory)
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	status, err := application.CompareStatus()
	if err != nil {
		return comparisonCommandError(jsonOutput, "status", err)
	}

	if jsonOutput {
		return writeStatusJSON(output, status)
	}

	return writeStatusText(output, status)
}

func runDiffCommand(workingDirectory string, args []string, output io.Writer) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, diffHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	patchOutput := false

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput, "--patch": &patchOutput})
	if err != nil {
		return usageCommandError(jsonOutput, "diff: %v\n\n%s", err, diffHelpText())
	}
	if jsonOutput && patchOutput {
		return usageCommandError(jsonOutput, "diff --patch cannot be combined with --json\n\n%s", diffHelpText())
	}
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "diff", jsonOutput)
	}
	if len(positionals) != 1 {
		return usageCommandError(jsonOutput, "diff requires exactly one profile name\n\n%s", diffHelpText())
	}

	application, err := loadApplication(workingDirectory)
	if err != nil {
		return runtimeCommandError(jsonOutput, err)
	}

	profileName := positionals[0]
	if patchOutput {
		preview, err := application.ManagedPatchPreviewByName(profileName, app.PreviewOptions{ValueVisibility: app.ValueVisibilityShown})
		if err != nil {
			return diffCommandError(false, application, profileName, err)
		}

		return writeManagedPatchText(output, preview)
	}

	diff, err := application.DiffProfileByName(profileName)
	if err != nil {
		return diffCommandError(jsonOutput, application, profileName, err)
	}

	if jsonOutput {
		return writeDiffJSON(output, diff)
	}

	return writeDiffText(output, diff)
}
