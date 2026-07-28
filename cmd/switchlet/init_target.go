package main

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/config"
)

const (
	chooseDifferentFileLabel   = "Back to file selection"
	clearTargetFileFilterLabel = "Clear filter"
)

type targetFileSelection struct {
	path        string
	displayPath string
	targetType  config.TargetType
	nodes       []targetSelectorNode
	dotenvKeys  []string
}

type targetSelectorNode struct {
	name       string
	selector   string
	selectable bool
	children   []targetSelectorNode
}

func promptTarget(prompter initPrompter, workingDirectory string, dependencies initDependencies) (config.Target, error) {
	targets, err := promptTargets(prompter, workingDirectory, dependencies)
	if err != nil {
		return config.Target{}, err
	}

	return targets[0], nil
}

func promptTargets(prompter initPrompter, workingDirectory string, dependencies initDependencies) ([]config.Target, error) {
	targets := make([]config.Target, 0, 1)
	seenNames := make(map[string]struct{})

	for {
		target, err := promptNamedTarget(prompter, workingDirectory, seenNames, dependencies)
		if err != nil {
			return nil, err
		}

		targets = append(targets, target)
		seenNames[target.Name] = struct{}{}

		addAnother, err := prompter.promptYesNo(formatYesNoPrompt("Add another managed value?", false), false)
		if err != nil {
			return nil, err
		}
		if !addAnother {
			return targets, nil
		}

		if err := writeInitStep(prompter.writer, 1, "Choose configuration file",
			"Pick the next JSON, YAML, TOML, or dotenv file containing a value Switchlet should manage.",
			"You can also enter a file path manually.",
		); err != nil {
			return nil, err
		}
	}
}

func promptNamedTarget(prompter initPrompter, workingDirectory string, seenNames map[string]struct{}, dependencies initDependencies) (config.Target, error) {
	for {
		selectedFile, err := promptTargetFile(prompter, workingDirectory, dependencies)
		if err != nil {
			return config.Target{}, err
		}

		if err := writeInitStep(prompter.writer, 2, targetSelectorStepTitle(selectedFile.targetType),
			fmt.Sprintf("Selected file: %s", selectedFile.displayPath),
			fmt.Sprintf("Detected format: %s", targetTypeDisplayName(selectedFile.targetType)),
			targetSelectorStepGuidance(selectedFile.targetType),
		); err != nil {
			return config.Target{}, err
		}

		selector, chooseDifferentFile, err := promptTargetSelector(prompter, selectedFile, dependencies)
		if err != nil {
			return config.Target{}, err
		}
		if chooseDifferentFile {
			continue
		}

		if err := writeInitStep(prompter.writer, 3, "Name this managed value",
			fmt.Sprintf("Selected file: %s", selectedFile.displayPath),
			fmt.Sprintf("Selected value: %s", selector),
			"Profiles refer to this short name.",
		); err != nil {
			return config.Target{}, err
		}

		name, err := promptTargetName(prompter, seenNames)
		if err != nil {
			return config.Target{}, err
		}

		target := config.Target{
			Name: name,
			File: selectedFile.path,
			Type: selectedFile.targetType,
		}
		switch selectedFile.targetType {
		case config.TargetTypeDotenv:
			target.Key = selector
		case config.TargetTypeYAML:
			target.YAMLPath = selector
		case config.TargetTypeTOML:
			target.TOMLPath = selector
		default:
			target.JSONPath = selector
		}

		return target, nil
	}
}

func targetSelectorStepTitle(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeDotenv:
		return "Choose dotenv value"
	case config.TargetTypeYAML:
		return "Choose YAML value"
	case config.TargetTypeTOML:
		return "Choose TOML value"
	default:
		return "Choose JSON value"
	}
}

func targetSelectorStepGuidance(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeDotenv:
		return "Choose an existing dotenv key that appears once. Switchlet does not create missing keys."
	case config.TargetTypeYAML:
		return "Choose an existing string-valued YAML path. Switchlet does not create missing values. Browse the mapping hierarchy, search when the file has many selectable values, or enter a path manually."
	case config.TargetTypeTOML:
		return "Choose an existing string-valued TOML path. Switchlet does not create missing values. Browse the table hierarchy, search when the file has many selectable values, or enter a path manually."
	default:
		return "Choose an existing string-valued JSON path. Switchlet does not create missing values. Browse the hierarchy, search when the file has many selectable values, or enter a path manually."
	}
}

func targetSelectorLabel(target config.Target) (string, string) {
	switch target.Type {
	case config.TargetTypeDotenv:
		return "Key", target.Key
	case config.TargetTypeYAML:
		return "YAML path", target.YAMLPath
	case config.TargetTypeTOML:
		return "TOML path", target.TOMLPath
	default:
		return "JSON path", target.JSONPath
	}
}

func targetTypeDisplayName(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeJSON:
		return "JSON"
	case config.TargetTypeYAML:
		return "YAML"
	case config.TargetTypeTOML:
		return "TOML"
	case config.TargetTypeDotenv:
		return "dotenv"
	default:
		return string(targetType)
	}
}

func promptTargetName(prompter initPrompter, seenNames map[string]struct{}) (string, error) {
	for {
		name, err := prompter.promptNonEmptyLine("Managed value name: ")
		if err != nil {
			return "", err
		}

		if _, exists := seenNames[name]; exists {
			if _, err := fmt.Fprintf(prompter.writer, "Error: managed value name %q is already configured.\n", name); err != nil {
				return "", err
			}
			continue
		}

		return name, nil
	}
}

func writePromptError(prompter initPrompter, err error) error {
	_, writeErr := fmt.Fprintf(prompter.writer, "Error: %v\n", err)
	return writeErr
}
