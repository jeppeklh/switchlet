package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

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
		if commandFailure.message == "" {
			return nil
		}
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

func silentCommandError(exitCode int, outputOptions commandOutputOptions, jsonOutput bool) error {
	return commandError{exitCode: exitCode, jsonOutput: jsonOutput, outputOptions: outputOptions}
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

	var recoveryErr editor.RecoveryError
	if errors.As(err, &recoveryErr) {
		return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatRecoveryErrorMessage(profileName, recoveryErr, textProjectRoot(jsonOutput, projectRoot)))
	}

	var preflightErr editor.PreflightError
	if errors.As(err, &preflightErr) {
		return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatPreflightErrorMessage(profileName, preflightErr, textProjectRoot(jsonOutput, projectRoot)))
	}

	var targetErr editor.TargetError
	if errors.As(err, &targetErr) {
		return runtimeCommandErrorWithMessage(outputOptions, jsonOutput, err, formatTargetErrorMessage(profileName, targetErr, textProjectRoot(jsonOutput, projectRoot)))
	}

	return runtimeCommandError(outputOptions, jsonOutput, err)
}

func commandFailure(outputOptions commandOutputOptions, jsonOutput bool, exitCode int, kind string, message string) error {
	if !jsonOutput {
		return commandError{message: message, exitCode: exitCode, outputOptions: outputOptions}
	}

	encodedError, err := json.Marshal(struct {
		SchemaVersion int              `json:"schemaVersion"`
		Error         commandErrorJSON `json:"error"`
	}{SchemaVersion: commandJSONSchemaVersion, Error: commandErrorJSON{Kind: kind, Message: message}})
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

func noProfileUsageCommandError(workingDirectory string, commandName string, outputOptions commandOutputOptions, jsonOutput bool, options ...projectLoadOptions) error {
	application, err := loadApplication(workingDirectory, options...)
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
