package profile

import (
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

	switch {
	case configuredProfile.Value != nil:
		resolvedProfile.Source = ValueSourceLiteral
		resolvedProfile.Value = *configuredProfile.Value
		if resolvedProfile.Value == "" {
			resolvedProfile.ResolutionError = fmt.Errorf("profile %q value is empty: %w", configuredProfile.Name, ErrProfileValueEmpty)
			break
		}
		resolvedProfile.MaskedValue = MaskConnectionString(resolvedProfile.Value)
	case configuredProfile.ValueFromEnv != nil:
		resolvedProfile.Source = ValueSourceEnvironment
		resolvedProfile.EnvironmentVariableName = *configuredProfile.ValueFromEnv
		resolvedProfile.Value, resolvedProfile.ResolutionError = resolveEnvironmentValue(
			configuredProfile.Name,
			resolvedProfile.EnvironmentVariableName,
		)
		if resolvedProfile.ResolutionError == nil {
			resolvedProfile.MaskedValue = MaskConnectionString(resolvedProfile.Value)
		}
	default:
		resolvedProfile.ResolutionError = fmt.Errorf("profile %q does not define a value", configuredProfile.Name)
	}

	return resolvedProfile
}

func resolveEnvironmentValue(profileName string, variableName string) (string, error) {
	value, exists := os.LookupEnv(variableName)
	if !exists {
		return "", fmt.Errorf("profile %q environment variable %q is not set: %w", profileName, variableName, ErrEnvironmentVariableNotSet)
	}
	if value == "" {
		return "", fmt.Errorf("profile %q environment variable %q is empty: %w", profileName, variableName, ErrEnvironmentVariableEmpty)
	}

	return value, nil
}
