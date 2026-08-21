package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

type statusCommandOptions struct {
	jsonOutput      bool
	shortOutput     bool
	nameOutput      bool
	expectedProfile string
}

type applyCommandOptions struct {
	jsonOutput                 bool
	dryRun                     bool
	allowProtected             bool
	expectedCurrentProfile     string
	expectCurrentBeforeProfile bool
	profileAfterDelimiter      bool
}

type statusExpectationResult struct {
	ExpectedProfile  string
	Matched          bool
	ObservedStatus   app.StatusComparisonStatus
	ObservedProfiles []string
	Message          string
}

type doctorReport struct {
	ConfigPath  string
	ProjectRoot string
	Checks      []app.HealthCheck
}

func runListCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, listHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	commandLoadOptions := loadOptions

	positionals, err := parseProjectArguments(args, map[string]*bool{"--json": &jsonOutput}, &outputOptions, &commandLoadOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "list: %v\n\n%s", err, listHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(outputOptions, jsonOutput, "list does not accept a profile name\n\n%s", listHelpText())
	}

	project, err := loadProject(workingDirectory, commandLoadOptions)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	profiles := project.Application.Profiles()
	if jsonOutput {
		return writeListJSON(output, profiles)
	}

	return writeListText(output, profiles)
}

func runInspectCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, inspectHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	commandLoadOptions := loadOptions

	positionals, err := parseProjectArguments(args, map[string]*bool{"--json": &jsonOutput}, &outputOptions, &commandLoadOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "inspect: %v\n\n%s", err, inspectHelpText())
	}
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "inspect", outputOptions, jsonOutput, commandLoadOptions)
	}
	if len(positionals) != 1 {
		return usageCommandError(outputOptions, jsonOutput, "inspect requires exactly one profile name\n\n%s", inspectHelpText())
	}

	project, err := loadProject(workingDirectory, commandLoadOptions)
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

func runApplyCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, applyHelpText())
		return err
	}

	commandLoadOptions := loadOptions
	options, positionals, err := parseApplyCommandArguments(args, &outputOptions, &commandLoadOptions)
	if err != nil {
		return usageCommandError(outputOptions, options.jsonOutput, "apply: %v\n\n%s", err, applyHelpText())
	}
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "apply", outputOptions, options.jsonOutput, commandLoadOptions)
	}
	if len(positionals) != 1 {
		return usageCommandError(outputOptions, options.jsonOutput, "apply requires exactly one profile name\n\n%s", applyHelpText())
	}
	if options.expectedCurrentProfile != "" && options.expectCurrentBeforeProfile && !options.profileAfterDelimiter {
		return usageCommandError(outputOptions, options.jsonOutput, "apply: --expect-current must follow the profile name\n\n%s", applyHelpText())
	}

	project, err := loadProject(workingDirectory, commandLoadOptions)
	if err != nil {
		return runtimeCommandError(outputOptions, options.jsonOutput, err)
	}

	profileName := positionals[0]
	if options.expectedCurrentProfile != "" {
		profiles := project.Application.Profiles()
		if !profileNameExists(profiles, profileName) {
			return runtimeCommandErrorWithMessage(outputOptions, options.jsonOutput, app.ErrProfileNotFound, formatMissingProfileMessage(profileName, profiles))
		}

		status, err := project.Application.CompareStatus()
		if err != nil {
			return comparisonCommandError(outputOptions, options.jsonOutput, "apply", err, project.ProjectRoot)
		}

		expectation := evaluateStatusExpectation(status, profiles, options.expectedCurrentProfile)
		if !expectation.Matched {
			return runtimeCommandErrorWithMessage(outputOptions, options.jsonOutput, errors.New(expectation.Message), formatApplyExpectationMessage(profileName, expectation))
		}
	}

	result, err := project.Application.ApplyProfileByNameWithOptions(profileName, app.ApplyOptions{
		DryRun:         options.dryRun,
		AllowProtected: options.allowProtected,
	})
	if err != nil {
		return applyCommandError(outputOptions, options.jsonOutput, project.Application, profileName, err, project.ProjectRoot)
	}

	if options.jsonOutput {
		return writeApplyJSON(output, result)
	}

	return writeApplyText(output, result, project.ProjectRoot)
}

func runStatusCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, statusHelpText())
		return err
	}

	commandLoadOptions := loadOptions
	options, positionals, err := parseStatusCommandArguments(args, &outputOptions, &commandLoadOptions)
	if err != nil {
		return usageCommandError(outputOptions, options.jsonOutput, "status: %v\n\n%s", err, statusHelpText())
	}
	if options.jsonOutput && options.nameOutput {
		return usageCommandError(outputOptions, options.jsonOutput, "status --name cannot be combined with --json\n\n%s", statusHelpText())
	}
	if options.jsonOutput && options.shortOutput {
		return usageCommandError(outputOptions, options.jsonOutput, "status --short cannot be combined with --json\n\n%s", statusHelpText())
	}
	if options.nameOutput && options.shortOutput {
		return usageCommandError(outputOptions, options.jsonOutput, "status --name cannot be combined with --short\n\n%s", statusHelpText())
	}
	if options.expectedProfile != "" && options.nameOutput {
		return usageCommandError(outputOptions, options.jsonOutput, "status --expect cannot be combined with --name\n\n%s", statusHelpText())
	}
	if options.expectedProfile != "" && options.shortOutput {
		return usageCommandError(outputOptions, options.jsonOutput, "status --expect cannot be combined with --short\n\n%s", statusHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(outputOptions, options.jsonOutput, "status does not accept a profile name\n\n%s", statusHelpText())
	}

	project, err := loadProject(workingDirectory, commandLoadOptions)
	if err != nil {
		return runtimeCommandError(outputOptions, options.jsonOutput, err)
	}

	status, err := project.Application.CompareStatus()
	if err != nil {
		return comparisonCommandError(outputOptions, options.jsonOutput, "status", err, project.ProjectRoot)
	}
	expectation := statusExpectationResult{}
	if options.expectedProfile != "" {
		expectation = evaluateStatusExpectation(status, project.Application.Profiles(), options.expectedProfile)
	}

	if options.jsonOutput {
		if err := writeStatusJSONWithExpectation(output, status, expectation); err != nil {
			return err
		}
		if options.expectedProfile != "" && !expectation.Matched {
			return silentCommandError(runtimeExitCode, outputOptions, true)
		}

		return nil
	}
	if options.shortOutput {
		return writeStatusShortText(output, status)
	}
	if options.nameOutput {
		return writeStatusNameText(output, status, outputOptions)
	}

	if err := writeStatusText(output, status, project.ProjectRoot, outputOptions); err != nil {
		return err
	}
	if options.expectedProfile != "" {
		if err := writeStatusExpectationText(output, expectation, outputOptions); err != nil {
			return err
		}
		if !expectation.Matched {
			return silentCommandError(runtimeExitCode, outputOptions, false)
		}
	}

	return nil
}

func runDiffCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, diffHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	commandLoadOptions := loadOptions
	patchOutput := false
	exitCode := false

	positionals, err := parseProjectArguments(args, map[string]*bool{"--json": &jsonOutput, "--patch": &patchOutput, "--exit-code": &exitCode}, &outputOptions, &commandLoadOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "diff: %v\n\n%s", err, diffHelpText())
	}
	if jsonOutput && patchOutput {
		return usageCommandError(outputOptions, jsonOutput, "diff --patch cannot be combined with --json\n\n%s", diffHelpText())
	}
	if len(positionals) == 0 {
		return noProfileUsageCommandError(workingDirectory, "diff", outputOptions, jsonOutput, commandLoadOptions)
	}
	if len(positionals) != 1 {
		return usageCommandError(outputOptions, jsonOutput, "diff requires exactly one profile name\n\n%s", diffHelpText())
	}

	project, err := loadProject(workingDirectory, commandLoadOptions)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	profileName := positionals[0]
	if patchOutput {
		preview, err := project.Application.ManagedPatchPreviewByName(profileName, app.PreviewOptions{})
		if err != nil {
			return diffCommandError(outputOptions, false, project.Application, profileName, err, project.ProjectRoot)
		}

		if err := writeManagedPatchText(output, preview, project.ProjectRoot); err != nil {
			return err
		}
		if exitCode && managedPatchPreviewHasDiffExitFailure(preview) {
			return silentCommandError(runtimeExitCode, outputOptions, false)
		}

		return nil
	}

	diff, err := project.Application.DiffProfileByName(profileName)
	if err != nil {
		return diffCommandError(outputOptions, jsonOutput, project.Application, profileName, err, project.ProjectRoot)
	}

	if jsonOutput {
		if err := writeDiffJSON(output, diff); err != nil {
			return err
		}
		if exitCode && diffHasExitFailure(diff) {
			return silentCommandError(runtimeExitCode, outputOptions, true)
		}

		return nil
	}

	if err := writeDiffText(output, diff, project.ProjectRoot, outputOptions); err != nil {
		return err
	}
	if exitCode && diffHasExitFailure(diff) {
		return silentCommandError(runtimeExitCode, outputOptions, false)
	}

	return nil
}

func runDoctorCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, doctorHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	strict := false
	commandLoadOptions := loadOptions
	positionals, err := parseProjectArguments(args, map[string]*bool{"--json": &jsonOutput, "--strict": &strict}, &outputOptions, &commandLoadOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "doctor: %v\n\n%s", err, doctorHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(outputOptions, jsonOutput, "doctor does not accept a positional argument\n\n%s", doctorHelpText())
	}

	report := buildDoctorReport(workingDirectory, commandLoadOptions)
	if jsonOutput {
		if err := writeDoctorJSON(output, report); err != nil {
			return err
		}
		if doctorReportHasFailures(report) || (strict && doctorReportHasWarnings(report)) {
			return silentCommandError(runtimeExitCode, outputOptions, true)
		}

		return nil
	}

	if err := writeDoctorText(output, report, outputOptions); err != nil {
		return err
	}
	if doctorReportHasFailures(report) || (strict && doctorReportHasWarnings(report)) {
		return silentCommandError(runtimeExitCode, outputOptions, false)
	}

	return nil
}

func runTargetsCommand(workingDirectory string, args []string, output io.Writer, outputOptions commandOutputOptions, loadOptions projectLoadOptions) error {
	if wantsHelpFlag(args) {
		_, err := io.WriteString(output, targetsHelpText())
		return err
	}

	jsonOutput := containsJSONFlag(args)
	commandLoadOptions := loadOptions
	positionals, err := parseProjectArguments(args, map[string]*bool{"--json": &jsonOutput}, &outputOptions, &commandLoadOptions)
	if err != nil {
		return usageCommandError(outputOptions, jsonOutput, "targets: %v\n\n%s", err, targetsHelpText())
	}
	if len(positionals) != 0 {
		return usageCommandError(outputOptions, jsonOutput, "targets does not accept a positional argument\n\n%s", targetsHelpText())
	}

	project, err := loadProject(workingDirectory, commandLoadOptions)
	if err != nil {
		return runtimeCommandError(outputOptions, jsonOutput, err)
	}

	targets := project.Application.Targets()
	if jsonOutput {
		return writeTargetsJSON(output, targets)
	}

	return writeTargetsText(output, targets, project.ProjectRoot, outputOptions)
}

func parseApplyCommandArguments(args []string, outputOptions *commandOutputOptions, loadOptions *projectLoadOptions) (applyCommandOptions, []string, error) {
	options := applyCommandOptions{jsonOutput: containsJSONFlag(args)}
	positionals := make([]string, 0, len(args))

	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			options.profileAfterDelimiter = true
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if arg == noColorFlag {
			if outputOptions != nil {
				outputOptions.NoColor = true
			}
			continue
		}
		if arg == configFlag {
			if index+1 >= len(args) {
				return options, nil, fmt.Errorf("%s requires a path", configFlag)
			}
			if err := setConfigPathOption(loadOptions, args[index+1]); err != nil {
				return options, nil, err
			}
			index++
			continue
		}
		if value, ok := strings.CutPrefix(arg, configFlag+"="); ok {
			if err := setConfigPathOption(loadOptions, value); err != nil {
				return options, nil, err
			}
			continue
		}
		if arg == "--json" {
			options.jsonOutput = true
			continue
		}
		if arg == "--dry-run" {
			options.dryRun = true
			continue
		}
		if arg == "--allow-protected" {
			options.allowProtected = true
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--expect-current="); ok {
			if options.expectedCurrentProfile != "" {
				return options, nil, fmt.Errorf("--expect-current can be provided only once")
			}
			if value == "" {
				return options, nil, fmt.Errorf("--expect-current requires a profile name")
			}
			options.expectedCurrentProfile = value
			options.expectCurrentBeforeProfile = len(positionals) == 0
			continue
		}
		if arg == "--expect-current" {
			if options.expectedCurrentProfile != "" {
				return options, nil, fmt.Errorf("--expect-current can be provided only once")
			}
			if index+1 >= len(args) || args[index+1] == "" {
				return options, nil, fmt.Errorf("--expect-current requires a profile name")
			}
			options.expectedCurrentProfile = args[index+1]
			options.expectCurrentBeforeProfile = len(positionals) == 0
			index++
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			return options, nil, unsupportedFlagError(arg, []string{"--allow-protected", "--config", "--dry-run", "--expect-current", "--json", "--no-color"})
		}

		positionals = append(positionals, arg)
	}

	return options, positionals, nil
}

func parseStatusCommandArguments(args []string, outputOptions *commandOutputOptions, loadOptions *projectLoadOptions) (statusCommandOptions, []string, error) {
	options := statusCommandOptions{jsonOutput: containsJSONFlag(args)}
	positionals := make([]string, 0, len(args))

	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if arg == noColorFlag {
			if outputOptions != nil {
				outputOptions.NoColor = true
			}
			continue
		}
		if arg == configFlag {
			if index+1 >= len(args) {
				return options, nil, fmt.Errorf("%s requires a path", configFlag)
			}
			if err := setConfigPathOption(loadOptions, args[index+1]); err != nil {
				return options, nil, err
			}
			index++
			continue
		}
		if value, ok := strings.CutPrefix(arg, configFlag+"="); ok {
			if err := setConfigPathOption(loadOptions, value); err != nil {
				return options, nil, err
			}
			continue
		}
		if arg == "--json" {
			options.jsonOutput = true
			continue
		}
		if arg == "--short" {
			options.shortOutput = true
			continue
		}
		if arg == "--name" {
			options.nameOutput = true
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--expect="); ok {
			if options.expectedProfile != "" {
				return options, nil, fmt.Errorf("--expect can be provided only once")
			}
			if value == "" {
				return options, nil, fmt.Errorf("--expect requires a profile name")
			}
			options.expectedProfile = value
			continue
		}
		if arg == "--expect" {
			if options.expectedProfile != "" {
				return options, nil, fmt.Errorf("--expect can be provided only once")
			}
			if index+1 >= len(args) || args[index+1] == "" {
				return options, nil, fmt.Errorf("--expect requires a profile name")
			}
			options.expectedProfile = args[index+1]
			index++
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			return options, nil, unsupportedFlagError(arg, []string{"--config", "--expect", "--json", "--name", "--no-color", "--short"})
		}

		positionals = append(positionals, arg)
	}

	return options, positionals, nil
}

func evaluateStatusExpectation(status app.StatusComparison, profiles []app.ProfileItem, expectedProfile string) statusExpectationResult {
	expectation := statusExpectationResult{
		ExpectedProfile:  expectedProfile,
		ObservedStatus:   status.Status,
		ObservedProfiles: statusCompleteProfileNames(status),
	}

	if !profileNameExists(profiles, expectedProfile) {
		expectation.Message = fmt.Sprintf("expected profile %q is not configured", expectedProfile)
		return expectation
	}

	switch status.Status {
	case app.StatusComparisonMatched:
		if status.CurrentProfile == expectedProfile {
			expectation.Matched = true
			expectation.Message = fmt.Sprintf("current complete profile is %q", expectedProfile)
			return expectation
		}

		expectation.Message = fmt.Sprintf("expected profile %q, but current complete profile is %q", expectedProfile, status.CurrentProfile)
	case app.StatusComparisonAmbiguous:
		expectation.Message = fmt.Sprintf("expected profile %q, but current state is ambiguous", expectedProfile)
	default:
		expectation.Message = fmt.Sprintf("expected profile %q, but no complete profile matches current managed values", expectedProfile)
	}

	return expectation
}

func profileNameExists(profiles []app.ProfileItem, profileName string) bool {
	for _, profileItem := range profiles {
		if profileItem.Name == profileName {
			return true
		}
	}

	return false
}

func statusCompleteProfileNames(status app.StatusComparison) []string {
	names := make([]string, 0, len(status.Matches))
	for _, match := range status.Matches {
		names = append(names, match.ProfileName)
	}

	return names
}

func diffHasExitFailure(diff app.ProfileDiff) bool {
	return len(diff.WouldUpdate) > 0 || len(diff.Unavailable) > 0
}

func managedPatchPreviewHasDiffExitFailure(preview app.ManagedPatchPreview) bool {
	for _, fileGroup := range preview.Files {
		for _, hunk := range fileGroup.Hunks {
			if hunk.Status == app.ManagedPatchStatusWouldUpdate || hunk.Status == app.ManagedPatchStatusUnavailable {
				return true
			}
		}
	}

	return false
}

func buildDoctorReport(workingDirectory string, loadOptions projectLoadOptions) doctorReport {
	report := doctorReport{}

	configPath, err := selectedConfigPath(workingDirectory, loadOptions)
	if err != nil {
		report.Checks = append(report.Checks, app.HealthCheck{
			Name:    "configuration_discovery",
			Status:  app.HealthCheckFailed,
			Message: doctorConfigurationDiscoveryMessage(err),
			Hint:    doctorConfigurationDiscoveryHint(err),
		})
		return report
	}

	report.ConfigPath = configPath
	report.ProjectRoot = filepath.Dir(configPath)
	discoveryMessage := "found .switchlet.yaml"
	if loadOptions.ConfigPath != "" {
		discoveryMessage = "using explicit configuration file"
	}
	report.Checks = append(report.Checks, app.HealthCheck{
		Name:    "configuration_discovery",
		Status:  app.HealthCheckOK,
		Message: discoveryMessage,
	})

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		report.Checks = append(report.Checks, app.HealthCheck{
			Name:    "configuration_loading",
			Status:  app.HealthCheckFailed,
			Message: err.Error(),
			Hint:    "Fix .switchlet.yaml, then run `switchlet doctor` again.",
		})
		return report
	}

	report.Checks = append(report.Checks, app.HealthCheck{
		Name:    "configuration_loading",
		Status:  app.HealthCheckOK,
		Message: fmt.Sprintf("loaded version %d configuration with %d target(s) and %d profile(s)", loadedConfig.Version, len(loadedConfig.Targets), len(loadedConfig.Profiles)),
	})

	application := app.NewWithTargets(loadedConfig.Targets, loadedConfig.Profiles)
	report.Checks = append(report.Checks, application.HealthChecks()...)

	return report
}

func doctorConfigurationDiscoveryMessage(err error) string {
	if errors.Is(err, config.ErrConfigNotFound) {
		return "No .switchlet.yaml found."
	}

	return formatRuntimeErrorMessage(err, err.Error())
}

func doctorConfigurationDiscoveryHint(err error) string {
	if errors.Is(err, config.ErrConfigNotFound) {
		return "Run `switchlet init` to create one, or run Switchlet from a configured project."
	}

	return "Check the configured path or working directory, then run `switchlet doctor` again."
}

func doctorReportHasFailures(report doctorReport) bool {
	for _, check := range report.Checks {
		if check.Status == app.HealthCheckFailed {
			return true
		}
	}

	return false
}

func doctorReportHasWarnings(report doctorReport) bool {
	for _, check := range report.Checks {
		if check.Status == app.HealthCheckWarning {
			return true
		}
	}

	return false
}
