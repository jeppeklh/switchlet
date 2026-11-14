package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func promptProfiles(prompter initPrompter, projectRoot string, targets []config.Target) ([]config.Profile, error) {
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

		profile, err := promptProfileCreation(prompter, projectRoot, name, targets, profiles)
		if err != nil {
			return nil, err
		}

		protected, err := prompter.promptYesNo(formatYesNoPrompt("Require confirmation before applying this profile?", profile.Protected), profile.Protected)
		if err != nil {
			return nil, err
		}
		profile.Protected = protected

		profiles = append(profiles, profile)
		seenNames[name] = struct{}{}

		addAnother, err := prompter.promptYesNo(formatYesNoPrompt("Add another profile?", false), false)
		if err != nil {
			return nil, err
		}
		if !addAnother {
			return profiles, nil
		}
	}
}

// promptProfileCreation keeps the one-value literal/environment choices in place.
// Capture and copy are additional choices, not mandatory steps on that path.
func promptProfileCreation(prompter initPrompter, projectRoot string, name string, targets []config.Target, profiles []config.Profile) (config.Profile, error) {
	choices := []string{"Enter profile values"}
	if len(targets) == 1 {
		choices = []string{"Use a literal value", "Use an environment variable"}
	}
	freshChoices := len(choices)
	choices = append(choices, "Capture current managed values")
	for _, profile := range profiles {
		choices = append(choices, "Copy "+profile.Name)
	}
	choice, err := prompter.promptChoiceIndex("Start profile:", choices)
	if err != nil {
		return config.Profile{}, err
	}
	if choice < freshChoices {
		var values []config.ProfileValue
		if len(targets) == 1 {
			if _, err := fmt.Fprintf(prompter.writer, "Expected value type: %s\n", config.EffectiveValueType(targets[0].ValueType)); err != nil {
				return config.Profile{}, err
			}
			value, err := promptProfileValueSource(prompter, targets[0], choices[choice])
			if err != nil {
				return config.Profile{}, err
			}
			values = []config.ProfileValue{value}
		} else {
			values, err = promptProfileValues(prompter, name, targets)
			if err != nil {
				return config.Profile{}, err
			}
		}
		return config.Profile{Name: name, Values: values}, nil
	}
	workflow := app.DefaultConfigEditWorkflow()
	document := app.ConfigEditDocument{ProjectRoot: projectRoot, Targets: targets, Profiles: profiles}
	var draft app.ConfigEditProfileDraft
	if choice == freshChoices {
		draft, err = workflow.CaptureCurrentProfileDraft(document)
	} else {
		draft, err = workflow.DuplicateProfileDraft(document, profiles[choice-freshChoices-1].Name, name)
	}
	if err != nil {
		return config.Profile{}, err
	}
	draft.Name = name
	draft, err = promptProfileDraftEdits(prompter, targets, draft)
	if err != nil {
		return config.Profile{}, err
	}
	updated, err := workflow.AddProfileDraft(document, draft)
	if err != nil {
		return config.Profile{}, err
	}
	return updated.Profiles[len(updated.Profiles)-1], nil
}

func promptProfileDraftEdits(prompter initPrompter, targets []config.Target, draft app.ConfigEditProfileDraft) (app.ConfigEditProfileDraft, error) {
	edit, err := prompter.promptYesNo(formatYesNoPrompt("Edit selected profile values?", false), false)
	if err != nil {
		return app.ConfigEditProfileDraft{}, err
	}
	if edit {
		for index := range draft.Values {
			value := &draft.Values[index]
			change, err := prompter.promptYesNo(formatYesNoPrompt("Edit "+value.TargetName+"?", false), false)
			if err != nil {
				return app.ConfigEditProfileDraft{}, err
			}
			if !change {
				continue
			}
			include, err := prompter.promptYesNo(formatYesNoPrompt("Include "+value.TargetName+"?", value.Included), value.Included)
			if err != nil {
				return app.ConfigEditProfileDraft{}, err
			}
			value.Included = include
			if !include {
				continue
			}
			entered, err := promptProfileValue(prompter, draft.Name, targets[index])
			if err != nil {
				return app.ConfigEditProfileDraft{}, err
			}
			value.Source = app.ProfileSourceLiteral
			value.EnvironmentVariableName = ""
			value.LiteralValue = ""
			if entered.Value != nil {
				value.LiteralValue = *entered.Value
			}
			if entered.ValueFromEnv != nil {
				value.Source = app.ProfileSourceEnvironment
				value.EnvironmentVariableName = *entered.ValueFromEnv
			}
		}
	}
	return draft, nil
}

func promptProfileValues(prompter initPrompter, profileName string, targets []config.Target) ([]config.ProfileValue, error) {
	for {
		values := make([]config.ProfileValue, 0, len(targets))
		for _, target := range targets {
			includeTarget := true
			if len(targets) > 1 {
				var err error
				includeTarget, err = prompter.promptYesNo(formatYesNoPrompt(fmt.Sprintf("Set %s in %s?", target.Name, profileName), true), true)
				if err != nil {
					return nil, err
				}
			}
			if !includeTarget {
				if _, err := fmt.Fprintf(prompter.writer, "Leaving %s unchanged in %s.\n", target.Name, profileName); err != nil {
					return nil, err
				}
				continue
			}

			value, err := promptProfileValue(prompter, profileName, target)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}

		if len(values) > 0 {
			return values, nil
		}

		if _, err := fmt.Fprintln(prompter.writer, "Error: a profile must include at least one managed value."); err != nil {
			return nil, err
		}
	}
}

func promptProfileValue(prompter initPrompter, profileName string, target config.Target) (config.ProfileValue, error) {
	if _, err := fmt.Fprintf(prompter.writer, "\nSet %s for %s\n", target.Name, profileName); err != nil {
		return config.ProfileValue{}, err
	}
	if target.ValueType != "" {
		if _, err := fmt.Fprintf(prompter.writer, "Expected value type: %s\n", target.ValueType); err != nil {
			return config.ProfileValue{}, err
		}
	}

	sourceChoice, err := prompter.promptChoice("How should Switchlet store this managed value?", []string{"Use a literal value", "Use an environment variable"})
	if err != nil {
		return config.ProfileValue{}, err
	}

	return promptProfileValueSource(prompter, target, sourceChoice)
}

func promptProfileValueSource(prompter initPrompter, target config.Target, sourceChoice string) (config.ProfileValue, error) {
	value := config.ProfileValue{Target: target.Name, ValueType: target.ValueType}
	switch sourceChoice {
	case "Use a literal value":
		for {
			literalValue, err := prompter.promptNonEmptyLine("Literal value: ")
			if err != nil {
				return config.ProfileValue{}, err
			}
			normalized, err := config.ParseScalar(target.ValueType, literalValue)
			if err != nil {
				if _, writeErr := fmt.Fprintf(prompter.writer, "Error: target %q: %s\n", target.Name, err); writeErr != nil {
					return config.ProfileValue{}, writeErr
				}
				continue
			}
			value.Value = stringValuePointer(normalized.Text())
			break
		}

	case "Use an environment variable":
		environmentVariableName, err := prompter.promptNonEmptyLine("Environment variable name: ")
		if err != nil {
			return config.ProfileValue{}, err
		}
		value.ValueFromEnv = stringValuePointer(environmentVariableName)
	default:
		return config.ProfileValue{}, fmt.Errorf("unsupported profile source %q", sourceChoice)
	}

	return value, nil
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

func printInitSummary(output io.Writer, workingDirectory string, targets []config.Target, profiles []config.Profile) error {
	if _, err := fmt.Fprintln(output, "Summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  Configuration file: %s\n", filepath.Join(workingDirectory, ".switchlet.yaml")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "  Managed values:"); err != nil {
		return err
	}
	for _, target := range targets {
		if _, err := fmt.Fprintf(output, "    - %s [%s] -> %s\n", target.Name, target.Type, app.DisplayInitTargetPath(workingDirectory, target.File)); err != nil {
			return err
		}
		if target.ValueType != "" {
			if _, err := fmt.Fprintf(output, "      Value type: %s\n", target.ValueType); err != nil {
				return err
			}
		}
		selectorName, selector := targetSelectorLabel(target)
		if _, err := fmt.Fprintf(output, "      %s: %s\n", selectorName, selector); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(output, "  Profiles:"); err != nil {
		return err
	}

	for _, profile := range profiles {
		description := profileValueSummary(profile)
		if profile.Protected {
			description += ", protected"
		}

		if _, err := fmt.Fprintf(output, "    - %s (%s)\n", profile.Name, description); err != nil {
			return err
		}
	}

	return nil
}

func profileValueSummary(profile config.Profile) string {
	if len(profile.Values) == 0 {
		if profile.ValueFromEnv != nil {
			return fmt.Sprintf("env %s", *profile.ValueFromEnv)
		}
		return "literal"
	}

	literalCount := 0
	environmentCount := 0
	for _, value := range profile.Values {
		if value.ValueFromEnv != nil {
			environmentCount++
		} else {
			literalCount++
		}
	}

	description := fmt.Sprintf("%d managed value", len(profile.Values))
	if len(profile.Values) != 1 {
		description += "s"
	}
	switch {
	case literalCount > 0 && environmentCount > 0:
		description += ", mixed"
	case environmentCount > 0:
		description += ", env"
	default:
		description += ", literal"
	}

	return description
}

func stringValuePointer(value string) *string {
	return &value
}
