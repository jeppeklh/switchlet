package profile

import (
	"errors"
	"fmt"
	"os"

	"github.com/jeppeklh/switchlet/internal/config"
)

// ResolveProfile resolves one configured profile into a value that can be displayed or applied.
func ResolveProfile(configuredProfile config.Profile) ResolvedProfile {
	resolvedProfile := ResolvedProfile{
		Name:      configuredProfile.Name,
		Protected: configuredProfile.Protected,
	}

	configuredValues := configuredProfile.Values
	if len(configuredValues) == 0 {
		configuredValues = compatibilityProfileValues(configuredProfile)
	}
	if len(configuredValues) == 0 {
		resolvedProfile.ResolutionError = fmt.Errorf("profile %q does not define a value", configuredProfile.Name)
		return resolvedProfile
	}

	valueErrors := make([]error, 0)
	resolvedValues := make([]ResolvedValue, 0, len(configuredValues))
	for _, configuredValue := range configuredValues {
		resolvedValue := resolveProfileValue(configuredProfile.Name, configuredValue)
		if resolvedValue.ResolutionError != nil {
			valueErrors = append(valueErrors, resolvedValue.ResolutionError)
		}

		resolvedValues = append(resolvedValues, resolvedValue)
	}

	resolvedProfile.Values = resolvedValues
	resolvedProfile.Source = resolvedValueSource(resolvedValues)
	if len(resolvedValues) == 1 {
		resolvedProfile.EnvironmentVariableName = resolvedValues[0].EnvironmentVariableName
		resolvedProfile.Value = resolvedValues[0].Value
		resolvedProfile.MaskedValue = resolvedValues[0].MaskedValue
	}
	if len(valueErrors) > 0 {
		resolvedProfile.ResolutionError = errors.Join(valueErrors...)
	}

	return resolvedProfile
}

func compatibilityProfileValues(configuredProfile config.Profile) []config.ProfileValue {
	switch {
	case configuredProfile.Value != nil:
		return []config.ProfileValue{{Value: configuredProfile.Value}}
	case configuredProfile.ValueFromEnv != nil:
		return []config.ProfileValue{{ValueFromEnv: configuredProfile.ValueFromEnv}}
	default:
		return nil
	}
}

func resolveProfileValue(profileName string, configuredValue config.ProfileValue) ResolvedValue {
	resolvedValue := ResolvedValue{Target: configuredValue.Target}

	switch {
	case configuredValue.Value != nil:
		resolvedValue.Source = ValueSourceLiteral
		resolvedValue.Value = *configuredValue.Value
		if resolvedValue.Value == "" {
			resolvedValue.ResolutionError = fmt.Errorf("%s is empty: %w", profileValueDescription(profileName, configuredValue.Target), ErrProfileValueEmpty)
			break
		}

		resolvedValue.MaskedValue = MaskManagedValue(resolvedValue.Value, ManagedValueMaskContext{TargetName: configuredValue.Target})
	case configuredValue.ValueFromEnv != nil:
		resolvedValue.Source = ValueSourceEnvironment
		resolvedValue.EnvironmentVariableName = *configuredValue.ValueFromEnv
		resolvedValue.Value, resolvedValue.ResolutionError = resolveEnvironmentValue(
			profileName,
			configuredValue.Target,
			resolvedValue.EnvironmentVariableName,
		)
		if resolvedValue.ResolutionError == nil {
			resolvedValue.MaskedValue = MaskManagedValue(resolvedValue.Value, ManagedValueMaskContext{
				TargetName:              configuredValue.Target,
				EnvironmentVariableName: resolvedValue.EnvironmentVariableName,
			})
		}
	default:
		resolvedValue.ResolutionError = fmt.Errorf("%s does not define a value", profileValueDescription(profileName, configuredValue.Target))
	}

	return resolvedValue
}

func resolveEnvironmentValue(profileName string, targetName string, variableName string) (string, error) {
	value, exists := os.LookupEnv(variableName)
	if !exists {
		return "", fmt.Errorf("%s environment variable %q is not set: %w", profileValueDescription(profileName, targetName), variableName, ErrEnvironmentVariableNotSet)
	}
	if value == "" {
		return "", fmt.Errorf("%s environment variable %q is empty: %w", profileValueDescription(profileName, targetName), variableName, ErrEnvironmentVariableEmpty)
	}

	return value, nil
}

func resolvedValueSource(values []ResolvedValue) ValueSource {
	if len(values) == 0 {
		return ""
	}

	source := values[0].Source
	for _, value := range values[1:] {
		if value.Source != source {
			return ValueSourceMixed
		}
	}

	return source
}

func profileValueDescription(profileName string, targetName string) string {
	if targetName == "" {
		return fmt.Sprintf("profile %q value", profileName)
	}

	return fmt.Sprintf("profile %q value for target %q", profileName, targetName)
}
