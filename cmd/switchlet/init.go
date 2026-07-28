package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

type initDependencies struct {
	validateCreateLocation       func(string) error
	discoverTargetFileCandidates func(string) ([]editor.TargetFileCandidate, error)
	inspectStringTargets         func(string) ([]editor.StringTargetNode, error)
	inspectYAMLStringTargets     func(string) ([]editor.YAMLStringTargetNode, error)
	inspectTOMLStringTargets     func(string) ([]editor.TOMLStringTargetNode, error)
	inspectDotenvKeys            func(string) ([]string, error)
	validateStringTarget         func(string, string) error
	validateYAMLTarget           func(string, string) error
	validateTOMLTarget           func(string, string) error
	validateDotenvTarget         func(string, string) error
	createConfig                 func(string, []config.Target, []config.Profile) (string, config.Config, error)
	prepareReplacementConfig     func(string, []config.Target, []config.Profile) (config.PreparedReplacement, error)
	ensureConfigIgnored          func(string) (bool, error)
	validateCreatedConfig        func(config.Config) error
	removeFile                   func(string) error
}

type initPrompter struct {
	reader *bufio.Reader
	writer io.Writer
}

type initOptions struct {
	OverwriteExistingConfig bool
}

type initCreateLocationDecision struct {
	Continue                bool
	OverwriteExistingConfig bool
}

const initStepCount = 5

func defaultInitDependencies() initDependencies {
	return initDependencies{
		validateCreateLocation:       config.ValidateCreateLocation,
		discoverTargetFileCandidates: editor.DiscoverTargetFileCandidates,
		inspectStringTargets:         editor.InspectStringTargets,
		inspectYAMLStringTargets:     editor.InspectYAMLStringTargets,
		inspectTOMLStringTargets:     editor.InspectTOMLStringTargets,
		inspectDotenvKeys:            editor.InspectDotenvKeys,
		validateStringTarget:         editor.ValidateStringTarget,
		validateYAMLTarget:           editor.ValidateYAMLTarget,
		validateTOMLTarget:           editor.ValidateTOMLTarget,
		validateDotenvTarget:         editor.ValidateDotenvTarget,
		createConfig:                 config.Create,
		prepareReplacementConfig:     config.PrepareReplacement,
		ensureConfigIgnored:          config.EnsureConfigIgnored,
		validateCreatedConfig: func(loadedConfig config.Config) error {
			return app.NewWithTargets(loadedConfig.Targets, loadedConfig.Profiles).ValidateStartup()
		},
		removeFile: os.Remove,
	}
}

func runInit(workingDirectory string, input io.Reader, output io.Writer, dependencies initDependencies) error {
	return runInitWithOptions(workingDirectory, input, output, dependencies, initOptions{})
}

func runInitWithOptions(workingDirectory string, input io.Reader, output io.Writer, dependencies initDependencies, options initOptions) error {
	createDecision, err := resolveInitCreateLocation(workingDirectory, input, output, dependencies, options)
	if err != nil {
		return err
	}
	if !createDecision.Continue {
		return nil
	}

	if shouldUseInitWizard(input, output) {
		result, err := runInitWizard(workingDirectory, input, output, dependencies, createDecision.OverwriteExistingConfig)
		if err != nil {
			return err
		}
		if result.Cancelled {
			_, err := fmt.Fprintln(output, "\nInitialization cancelled.")
			return err
		}

		return createInitConfiguration(workingDirectory, output, result.Targets, result.Profiles, result.ShouldIgnoreConfig, createDecision.OverwriteExistingConfig, dependencies)
	}

	return runPromptInit(workingDirectory, input, output, createDecision.OverwriteExistingConfig, dependencies)
}

func resolveInitCreateLocation(workingDirectory string, input io.Reader, output io.Writer, dependencies initDependencies, options initOptions) (initCreateLocationDecision, error) {
	err := dependencies.validateCreateLocation(workingDirectory)
	if err == nil {
		return initCreateLocationDecision{Continue: true}, nil
	}

	var existingConfigError config.ExistingConfigError
	if !errors.As(err, &existingConfigError) {
		return initCreateLocationDecision{}, err
	}

	currentConfigPath, err := currentDirectoryConfigPath(workingDirectory)
	if err != nil {
		return initCreateLocationDecision{}, err
	}
	if existingConfigError.ConfigPath != currentConfigPath {
		return initCreateLocationDecision{}, existingParentConfigMessage(existingConfigError.ConfigPath)
	}

	if options.OverwriteExistingConfig {
		return initCreateLocationDecision{Continue: true, OverwriteExistingConfig: true}, nil
	}

	if !shouldUseInitWizard(input, output) {
		return initCreateLocationDecision{}, existingCurrentConfigMessage(existingConfigError.ConfigPath)
	}

	confirmationResult, err := runInitOverwriteConfirmation(input, output, existingConfigError.ConfigPath)
	if err != nil {
		return initCreateLocationDecision{}, err
	}
	if confirmationResult.Cancelled {
		_, err := fmt.Fprintln(output, "\nInitialization cancelled.")
		return initCreateLocationDecision{Continue: false}, err
	}
	if !confirmationResult.Replace {
		if _, err := fmt.Fprintln(output, "\nNothing changed."); err != nil {
			return initCreateLocationDecision{}, err
		}
		if _, err := fmt.Fprintln(output, "Run `switchlet` to use the existing configuration."); err != nil {
			return initCreateLocationDecision{}, err
		}
		return initCreateLocationDecision{Continue: false}, nil
	}

	return initCreateLocationDecision{Continue: true, OverwriteExistingConfig: true}, nil
}

func currentDirectoryConfigPath(workingDirectory string) (string, error) {
	resolvedWorkingDirectory, err := filepath.Abs(workingDirectory)
	if err != nil {
		return "", fmt.Errorf("resolve working directory %q: %w", workingDirectory, err)
	}

	return filepath.Join(resolvedWorkingDirectory, ".switchlet.yaml"), nil
}

func existingCurrentConfigMessage(configPath string) error {
	return fmt.Errorf("Switchlet is already configured.\n\nExisting configuration:\n  %s\n\nNothing changed.\n\nRun `switchlet` to use it, or run:\n  switchlet init --overwrite\nto replace it.", configPath)
}

func existingParentConfigMessage(configPath string) error {
	return fmt.Errorf("Switchlet found an existing configuration in a parent directory.\n\nExisting configuration:\n  %s\n\nRun `switchlet init` from that directory if you want to replace it.", configPath)
}

func runPromptInit(workingDirectory string, input io.Reader, output io.Writer, overwriteExistingConfig bool, dependencies initDependencies) error {
	prompter := initPrompter{
		reader: bufio.NewReader(input),
		writer: output,
	}

	if _, err := fmt.Fprintln(output, "Switchlet init"); err != nil {
		return err
	}
	if err := writeInitStep(output, 1, "Choose configuration file",
		"Pick the JSON, YAML, TOML, or dotenv file containing a value Switchlet should manage.",
		"When many files are discovered, narrow the list by name or path.",
		"You can also enter a file path manually.",
	); err != nil {
		return err
	}

	targets, err := promptTargets(prompter, workingDirectory, dependencies)
	if err != nil {
		return err
	}

	if err := writeInitStep(output, 4, "Add profiles",
		"Add one or more named profiles for the managed values.",
		"Each profile can use a literal value or an environment variable name.",
	); err != nil {
		return err
	}

	profiles, err := promptProfiles(prompter, targets)
	if err != nil {
		return err
	}

	reviewDescription := "Review the managed values and profiles below before creating .switchlet.yaml."
	createPrompt := "Create .switchlet.yaml now?"
	if overwriteExistingConfig {
		reviewDescription = "Review the managed values and profiles below before replacing .switchlet.yaml."
		createPrompt = "Replace .switchlet.yaml now?"
	}

	if err := writeInitStep(output, 5, "Review and create configuration", reviewDescription); err != nil {
		return err
	}

	if err := printInitSummary(output, workingDirectory, targets, profiles); err != nil {
		return err
	}

	confirmed, err := prompter.promptYesNo(formatYesNoPrompt(createPrompt, true), true)
	if err != nil {
		return err
	}
	if !confirmed {
		_, err := fmt.Fprintln(output, "Initialization cancelled.")
		return err
	}

	shouldIgnoreConfig := false
	if app.InitProfilesHaveLiteralValues(profiles) {
		if _, err := fmt.Fprintln(output, "\nLiteral profile values are stored directly in .switchlet.yaml."); err != nil {
			return err
		}

		shouldIgnoreConfig, err = prompter.promptYesNo(formatYesNoPrompt("Add .switchlet.yaml to the project .gitignore?", true), true)
		if err != nil {
			return err
		}
	}

	return createInitConfiguration(workingDirectory, output, targets, profiles, shouldIgnoreConfig, overwriteExistingConfig, dependencies)
}

func createInitConfiguration(workingDirectory string, output io.Writer, targets []config.Target, profiles []config.Profile, shouldIgnoreConfig bool, overwriteExistingConfig bool, dependencies initDependencies) error {
	if overwriteExistingConfig {
		return replaceInitConfiguration(workingDirectory, output, targets, profiles, shouldIgnoreConfig, dependencies)
	}

	configPath, loadedConfig, err := dependencies.createConfig(workingDirectory, targets, profiles)
	if err != nil {
		return err
	}

	if err := dependencies.validateCreatedConfig(loadedConfig); err != nil {
		if removeErr := dependencies.removeFile(configPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return fmt.Errorf("validate created configuration file %q: %w; remove configuration file %q: %v", configPath, err, configPath, removeErr)
		}

		return fmt.Errorf("validate created configuration file %q: %w", configPath, err)
	}

	return finishInitConfiguration(workingDirectory, output, configPath, shouldIgnoreConfig, "Created configuration", dependencies)
}

func replaceInitConfiguration(workingDirectory string, output io.Writer, targets []config.Target, profiles []config.Profile, shouldIgnoreConfig bool, dependencies initDependencies) error {
	replacement, err := dependencies.prepareReplacementConfig(workingDirectory, targets, profiles)
	if err != nil {
		return err
	}

	configPath := replacement.ConfigPath()
	if err := dependencies.validateCreatedConfig(replacement.Config()); err != nil {
		return fmt.Errorf("validate replacement configuration file %q: %w", configPath, err)
	}

	if err := replacement.Commit(); err != nil {
		return fmt.Errorf("replace configuration file %q: %w", configPath, err)
	}

	return finishInitConfiguration(workingDirectory, output, configPath, shouldIgnoreConfig, "Replaced configuration", dependencies)
}

func finishInitConfiguration(workingDirectory string, output io.Writer, configPath string, shouldIgnoreConfig bool, successLabel string, dependencies initDependencies) error {
	if _, err := fmt.Fprintf(output, "\n%s: %s\n", successLabel, configPath); err != nil {
		return err
	}
	if shouldIgnoreConfig {
		if dependencies.ensureConfigIgnored == nil {
			return fmt.Errorf("configuration file %q was written, but project .gitignore protection is not configured", configPath)
		}

		changed, err := dependencies.ensureConfigIgnored(workingDirectory)
		if err != nil {
			return fmt.Errorf("configuration file %q was written, but update project .gitignore: %w", configPath, err)
		}

		message := "Project .gitignore already ignores .switchlet.yaml."
		if changed {
			message = "Updated project .gitignore to ignore .switchlet.yaml."
		}
		if _, err := fmt.Fprintln(output, message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(output, "Run `switchlet` to choose and apply a profile.")
	return err
}

func (prompter initPrompter) promptNonEmptyLine(prompt string) (string, error) {
	for {
		value, err := prompter.promptLine(prompt)
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}

		if _, err := fmt.Fprintln(prompter.writer, "Error: value must not be empty."); err != nil {
			return "", err
		}
	}
}

func (prompter initPrompter) promptChoice(prompt string, choices []string) (string, error) {
	selectedIndex, err := prompter.promptChoiceIndex(prompt, choices)
	if err != nil {
		return "", err
	}

	return choices[selectedIndex], nil
}

func writeInitStep(output io.Writer, stepNumber int, title string, details ...string) error {
	if _, err := fmt.Fprintf(output, "\nStep %d of %d: %s\n", stepNumber, initStepCount, title); err != nil {
		return err
	}

	for _, detail := range details {
		if _, err := fmt.Fprintln(output, detail); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(output, "")
	return err
}

func formatYesNoPrompt(label string, defaultValue bool) string {
	defaultLabel := "[y/N]"
	if defaultValue {
		defaultLabel = "[Y/n]"
	}

	return fmt.Sprintf("%s %s: ", label, defaultLabel)
}

func (prompter initPrompter) promptChoiceIndex(prompt string, choices []string) (int, error) {
	if _, err := fmt.Fprintln(prompter.writer, prompt); err != nil {
		return 0, err
	}
	for index, choice := range choices {
		if _, err := fmt.Fprintf(prompter.writer, "  %d. %s\n", index+1, choice); err != nil {
			return 0, err
		}
	}

	for {
		enteredValue, err := prompter.promptNonEmptyLine("Choice: ")
		if err != nil {
			return 0, err
		}

		selectedIndex, err := strconv.Atoi(enteredValue)
		if err == nil && selectedIndex >= 1 && selectedIndex <= len(choices) {
			return selectedIndex - 1, nil
		}

		if _, err := fmt.Fprintf(prompter.writer, "Error: enter a number between 1 and %d.\n", len(choices)); err != nil {
			return 0, err
		}
	}
}

func (prompter initPrompter) promptYesNo(prompt string, defaultValue bool) (bool, error) {
	for {
		enteredValue, err := prompter.promptLine(prompt)
		if err != nil {
			return false, err
		}

		switch strings.ToLower(enteredValue) {
		case "":
			return defaultValue, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			if _, err := fmt.Fprintln(prompter.writer, "Error: enter y or n."); err != nil {
				return false, err
			}
		}
	}
}

func (prompter initPrompter) promptLine(prompt string) (string, error) {
	if _, err := io.WriteString(prompter.writer, prompt); err != nil {
		return "", err
	}

	line, err := prompter.reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line != "" {
			return strings.TrimSpace(line), nil
		}

		return "", fmt.Errorf("read input: %w", err)
	}

	return strings.TrimSpace(line), nil
}
