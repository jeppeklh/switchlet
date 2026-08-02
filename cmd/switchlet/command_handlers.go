package main

import (
	"errors"
	"io"

	"github.com/jeppeklh/switchlet/internal/app"
)

func runListCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, listHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput}, &outputOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "list: %v\n\n%s", err, listHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(outputOptions, jsonOutput, "list does not accept a profile name\n\n%s", listHelpText())
	}

	project, err := loadProject(workingDirectory)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	profiles := project.Application.Profiles()
	if jsonOutput {
		return writeListJSON(output, profiles)
	}

	return writeListText(output, profiles)
}

func runInspectCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, inspectHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput}, &outputOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "inspect: %v\n\n%s", err, inspectHelpText())
	}
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "inspect", outputOptions, jsonOutput)
	}
	if len(positionals) != 1 {
		return usageCommandError(outputOptions, jsonOutput, "inspect requires exactly one profile name\n\n%s", inspectHelpText())
	}

	project, err := loadProject(workingDirectory)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	profileName := positionals[0]
	profileItem, err := project.Application.InspectProfileByName(profileName)
	if err != nil {
		if errors.Is(err, app.ErrProfileNotFound) {
			return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatMissingProfileMessage(profileName, project.Application.Profiles()))
		}

		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	if jsonOutput {
		return writeInspectJSON(output, profileItem)
	}

	return writeInspectText(output, profileItem, project.ProjectRoot)
}

func runApplyCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions) error {
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
	}, &outputOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "apply: %v\n\n%s", err, applyHelpText())
	}
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "apply", outputOptions, jsonOutput)
	}
	if len(positionals) != 1 {
		return usageCommandError(outputOptions, jsonOutput, "apply requires exactly one profile name\n\n%s", applyHelpText())
	}

	project, err := loadProject(workingDirectory)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	profileName := positionals[0]
	result, err := project.Application.ApplyProfileByNameWithOptions(profileName, app.ApplyOptions{
		DryRun:         dryRun,
		AllowProtected: allowProtected,
	})
	if err != nil {
		return applyCommandError(outputOptions, jsonOutput, project.Application, profileName, err, project.ProjectRoot)
	}

	if jsonOutput {
		return writeApplyJSON(output, result)
	}

	return writeApplyText(output, result, project.ProjectRoot)
}

func runStatusCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, statusHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	shortOutput := false

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput, "--short": &shortOutput}, &outputOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "status: %v\n\n%s", err, statusHelpText())
	}
	if jsonOutput && shortOutput {
		return usageCommandError(outputOptions, jsonOutput, "status --short cannot be combined with --json\n\n%s", statusHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(outputOptions, jsonOutput, "status does not accept a profile name\n\n%s", statusHelpText())
	}

	project, err := loadProject(workingDirectory)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	status, err := project.Application.CompareStatus()
	if err != nil {
		return comparisonCommandError(outputOptions, jsonOutput, "status", err, project.ProjectRoot)
	}

	if jsonOutput {
		return writeStatusJSON(output, status)
	}
	if shortOutput {
		return writeStatusShortText(output, status)
	}

	return writeStatusText(output, status, project.ProjectRoot, outputOptions)
}

func runDiffCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, diffHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	patchOutput := false

	positionals, err := parseArguments(args, map[string]*bool{"--json": &jsonOutput, "--patch": &patchOutput}, &outputOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "diff: %v\n\n%s", err, diffHelpText())
	}
	if jsonOutput && patchOutput {
		return usageCommandError(outputOptions, jsonOutput, "diff --patch cannot be combined with --json\n\n%s", diffHelpText())
	}
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "diff", outputOptions, jsonOutput)
	}
	if len(positionals) != 1 {
		return usageCommandError(outputOptions, jsonOutput, "diff requires exactly one profile name\n\n%s", diffHelpText())
	}

	project, err := loadProject(workingDirectory)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	profileName := positionals[0]
	if patchOutput {
		preview, err := project.Application.ManagedPatchPreviewByName(profileName, app.PreviewOptions{})
		if err != nil {
			return diffCommandError(outputOptions, false, project.Application, profileName, err, project.ProjectRoot)
		}

		return writeManagedPatchText(output, preview, project.ProjectRoot)
	}

	diff, err := project.Application.DiffProfileByName(profileName)
	if err != nil {
		return diffCommandError(outputOptions, jsonOutput, project.Application, profileName, err, project.ProjectRoot)
	}

	if jsonOutput {
		return writeDiffJSON(output, diff)
	}

	return writeDiffText(output, diff, project.ProjectRoot, outputOptions)
}
