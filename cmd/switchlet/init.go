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
	validateCreateLocation func(string) error
	validateStringTarget   func(string, string) error
	createConfig           func(string, config.Target, []config.Profile) (string, config.Config, error)
	validateCreatedConfig  func(config.Config) error
	removeFile             func(string) error
}

type initPrompter struct {
	reader *bufio.Reader
	writer io.Writer
}

func defaultInitDependencies() initDependencies {
	return initDependencies{
		validateCreateLocation: config.ValidateCreateLocation,
		validateStringTarget:   editor.ValidateStringTarget,
		createConfig:           config.Create,
		validateCreatedConfig: func(loadedConfig config.Config) error {
			return app.New(loadedConfig.Target, loadedConfig.Profiles).ValidateStartup()
		},
		removeFile: os.Remove,
	}
}

func runInit(workingDirectory string, input io.Reader, output io.Writer, dependencies initDependencies) error {
	if err := dependencies.validateCreateLocation(workingDirectory); err != nil {
		return err
	}

	prompter := initPrompter{
		reader: bufio.NewReader(input),
		writer: output,
	}

	if _, err := fmt.Fprintln(output, "Switchlet init"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, ""); err != nil {
		return err
	}

	target, err := promptTarget(prompter, workingDirectory, dependencies)
	if err != nil {
		return err
	}

	profiles, err := promptProfiles(prompter)
	if err != nil {
		return err
	}

	if err := printInitSummary(output, workingDirectory, target, profiles); err != nil {
		return err
	}

	confirmed, err := prompter.promptYesNo("Create .switchlet.yaml? [y/N]: ", false)
	if err != nil {
		return err
	}
	if !confirmed {
		_, err := fmt.Fprintln(output, "Initialization cancelled.")
		return err
	}

	configPath, loadedConfig, err := dependencies.createConfig(workingDirectory, target, profiles)
	if err != nil {
		return err
	}

	if err := dependencies.validateCreatedConfig(loadedConfig); err != nil {
		if removeErr := dependencies.removeFile(configPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return fmt.Errorf("validate created configuration file %q: %w; remove configuration file %q: %v", configPath, err, configPath, removeErr)
		}

		return fmt.Errorf("validate created configuration file %q: %w", configPath, err)
	}

	if _, err := fmt.Fprintf(output, "\nCreated configuration: %s\n", configPath); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "Run `switchlet` to choose and apply a profile.")
	return err
}

func promptTarget(prompter initPrompter, workingDirectory string, dependencies initDependencies) (config.Target, error) {
	for {
		targetPath, err := prompter.promptNonEmptyLine("Target JSON file path: ")
		if err != nil {
			return config.Target{}, err
		}

		resolvedTargetPath := targetPath
		if !filepath.IsAbs(resolvedTargetPath) {
			resolvedTargetPath = filepath.Join(workingDirectory, resolvedTargetPath)
		}
		resolvedTargetPath = filepath.Clean(resolvedTargetPath)

		jsonPath, err := prompter.promptNonEmptyLine("Target JSON path: ")
		if err != nil {
			return config.Target{}, err
		}

		if err := dependencies.validateStringTarget(resolvedTargetPath, jsonPath); err != nil {
			if _, writeErr := fmt.Fprintf(prompter.writer, "Error: %v\n\n", err); writeErr != nil {
				return config.Target{}, writeErr
			}
			continue
		}

		return config.Target{
			File:     resolvedTargetPath,
			JSONPath: jsonPath,
		}, nil
	}
}

func promptProfiles(prompter initPrompter) ([]config.Profile, error) {
	profiles := make([]config.Profile, 0, 1)
	seenNames := make(map[string]struct{})

	for {
		profileNumber := len(profiles) + 1
		if _, err := fmt.Fprintf(prompter.writer, "\nProfile %d\n", profileNumber); err != nil {
			return nil, err
		}

		name, err := promptProfileName(prompter, seenNames)
		if err != nil {
			return nil, err
		}

		sourceChoice, err := prompter.promptChoice("Select profile source:", []string{"Literal value", "Environment variable"})
		if err != nil {
			return nil, err
		}

		profile := config.Profile{Name: name}
		switch sourceChoice {
		case "Literal value":
			literalValue, err := prompter.promptNonEmptyLine("Value: ")
			if err != nil {
				return nil, err
			}
			profile.Value = stringValuePointer(literalValue)
		case "Environment variable":
			environmentVariableName, err := prompter.promptNonEmptyLine("Environment variable name: ")
			if err != nil {
				return nil, err
			}
			profile.ValueFromEnv = stringValuePointer(environmentVariableName)
		default:
			return nil, fmt.Errorf("unsupported profile source %q", sourceChoice)
		}

		protected, err := prompter.promptYesNo("Protected profile? [y/N]: ", false)
		if err != nil {
			return nil, err
		}
		profile.Protected = protected

		profiles = append(profiles, profile)
		seenNames[name] = struct{}{}

		addAnother, err := prompter.promptYesNo("Add another profile? [y/N]: ", false)
		if err != nil {
			return nil, err
		}
		if !addAnother {
			return profiles, nil
		}
	}
}

func promptProfileName(prompter initPrompter, seenNames map[string]struct{}) (string, error) {
	for {
		name, err := prompter.promptNonEmptyLine("Profile name: ")
		if err != nil {
			return "", err
		}

		if _, exists := seenNames[name]; exists {
			if _, err := fmt.Fprintf(prompter.writer, "Error: profile name %q is already configured.\n", name); err != nil {
				return "", err
			}
			continue
		}

		return name, nil
	}
}

func printInitSummary(output io.Writer, workingDirectory string, target config.Target, profiles []config.Profile) error {
	relativeTargetPath, err := filepath.Rel(workingDirectory, target.File)
	if err != nil {
		relativeTargetPath = target.File
	}

	if _, err := fmt.Fprintln(output, "\nSummary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  Configuration file: %s\n", filepath.Join(workingDirectory, ".switchlet.yaml")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  Target file: %s\n", relativeTargetPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  Target JSON path: %s\n", target.JSONPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "  Profiles:"); err != nil {
		return err
	}

	for _, profile := range profiles {
		description := "literal"
		if profile.ValueFromEnv != nil {
			description = fmt.Sprintf("env %s", *profile.ValueFromEnv)
		}
		if profile.Protected {
			description += ", protected"
		}

		if _, err := fmt.Fprintf(output, "    - %s (%s)\n", profile.Name, description); err != nil {
			return err
		}
	}

	return nil
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
	if _, err := fmt.Fprintln(prompter.writer, prompt); err != nil {
		return "", err
	}
	for index, choice := range choices {
		if _, err := fmt.Fprintf(prompter.writer, "  %d. %s\n", index+1, choice); err != nil {
			return "", err
		}
	}

	for {
		enteredValue, err := prompter.promptNonEmptyLine("Choice: ")
		if err != nil {
			return "", err
		}

		selectedIndex, err := strconv.Atoi(enteredValue)
		if err == nil && selectedIndex >= 1 && selectedIndex <= len(choices) {
			return choices[selectedIndex-1], nil
		}

		if _, err := fmt.Fprintf(prompter.writer, "Error: enter a number between 1 and %d.\n", len(choices)); err != nil {
			return "", err
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

func stringValuePointer(value string) *string {
	return &value
}
