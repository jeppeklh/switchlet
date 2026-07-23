package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	legacyVersion  = 1
	currentVersion = 2
)

func validateConfig(configPath string, parsed fileConfig) (Config, error) {
	if parsed.Version == nil {
		return Config{}, fmt.Errorf("version must be set")
	}
	if *parsed.Version != legacyVersion && *parsed.Version != currentVersion {
		return Config{}, fmt.Errorf("unsupported version %d", *parsed.Version)
	}

	projectRoot := filepath.Dir(configPath)

	target, err := validateTarget(parsed.Target, projectRoot, *parsed.Version)
	if err != nil {
		return Config{}, err
	}

	profiles, err := validateProfiles(parsed.Profiles)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		Version:  *parsed.Version,
		Target:   target,
		Profiles: profiles,
	}

	return config, nil
}

func validateTarget(parsedTarget fileTarget, projectRoot string, version int) (Target, error) {
	targetFile := strings.TrimSpace(parsedTarget.File)
	if targetFile == "" {
		return Target{}, fmt.Errorf("target.file must be set")
	}

	if !filepath.IsAbs(targetFile) {
		targetFile = filepath.Join(projectRoot, targetFile)
	}
	targetFile = filepath.Clean(targetFile)

	switch version {
	case legacyVersion:
		return validateLegacyTarget(parsedTarget, targetFile)
	case currentVersion:
		return validateVersionTwoTarget(parsedTarget, targetFile)
	default:
		return Target{}, fmt.Errorf("unsupported version %d", version)
	}
}

func validateLegacyTarget(parsedTarget fileTarget, targetFile string) (Target, error) {
	if strings.TrimSpace(parsedTarget.JSONPath) != "" {
		return Target{}, fmt.Errorf("target.jsonPath is not supported in version 1; use target.connectionName")
	}

	connectionName := strings.TrimSpace(parsedTarget.ConnectionName)
	if connectionName == "" {
		return Target{}, fmt.Errorf("target.connectionName must be set")
	}
	if strings.Contains(connectionName, ".") {
		return Target{}, fmt.Errorf("target.connectionName %q cannot be mapped to a JSON path because dots are not supported inside path segments", connectionName)
	}

	return Target{
		File:     targetFile,
		JSONPath: legacyConnectionJSONPath(connectionName),
	}, nil
}

func validateVersionTwoTarget(parsedTarget fileTarget, targetFile string) (Target, error) {
	if strings.TrimSpace(parsedTarget.ConnectionName) != "" {
		return Target{}, fmt.Errorf("target.connectionName is not supported in version 2; use target.jsonPath")
	}

	jsonPath := strings.TrimSpace(parsedTarget.JSONPath)
	if jsonPath == "" {
		return Target{}, fmt.Errorf("target.jsonPath must be set")
	}
	if _, err := ParseJSONPath(jsonPath); err != nil {
		return Target{}, fmt.Errorf("target.jsonPath is invalid: %w", err)
	}

	return Target{
		File:     targetFile,
		JSONPath: jsonPath,
	}, nil
}

// ParseJSONPath validates and splits a dot-separated JSON object-property path.
func ParseJSONPath(jsonPath string) ([]string, error) {
	trimmedPath := strings.TrimSpace(jsonPath)
	if trimmedPath == "" {
		return nil, fmt.Errorf("path must be set")
	}

	rawSegments := strings.Split(trimmedPath, ".")
	segments := make([]string, 0, len(rawSegments))
	for _, rawSegment := range rawSegments {
		if rawSegment == "" {
			return nil, fmt.Errorf("path must contain non-empty dot-separated segments")
		}
		if strings.TrimSpace(rawSegment) != rawSegment {
			return nil, fmt.Errorf("segment %q must not contain leading or trailing whitespace", rawSegment)
		}

		segments = append(segments, rawSegment)
	}

	return segments, nil
}

func legacyConnectionJSONPath(connectionName string) string {
	return "ConnectionStrings." + connectionName
}

func validateProfiles(parsedProfiles []fileProfile) ([]Profile, error) {
	if len(parsedProfiles) == 0 {
		return nil, fmt.Errorf("at least one profile must be configured")
	}

	profiles := make([]Profile, 0, len(parsedProfiles))
	seenNames := make(map[string]struct{}, len(parsedProfiles))

	for index, parsedProfile := range parsedProfiles {
		name := strings.TrimSpace(parsedProfile.Name)
		if name == "" {
			return nil, fmt.Errorf("profiles[%d].name must be set", index)
		}

		if _, exists := seenNames[name]; exists {
			return nil, fmt.Errorf("duplicate profile name %q", name)
		}
		seenNames[name] = struct{}{}

		hasValue := parsedProfile.Value != nil
		hasValueFromEnv := parsedProfile.ValueFromEnv != nil
		if hasValue == hasValueFromEnv {
			return nil, fmt.Errorf("profile %q must define exactly one of value or valueFromEnv", name)
		}

		profile := Profile{
			Name:      name,
			Protected: parsedProfile.Protected,
		}

		if hasValue {
			literalValue := *parsedProfile.Value
			profile.Value = &literalValue
		} else {
			environmentVariableName := strings.TrimSpace(*parsedProfile.ValueFromEnv)
			if environmentVariableName == "" {
				return nil, fmt.Errorf("profile %q valueFromEnv must be set", name)
			}

			profile.ValueFromEnv = &environmentVariableName
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}
