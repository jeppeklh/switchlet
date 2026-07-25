package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	legacyVersion           = 1
	currentVersion          = 2
	namedTargetVersion      = 3
	compatibilityTargetName = "default"
)

func validateConfig(configPath string, parsed fileConfig) (Config, error) {
	if parsed.Version == nil {
		return Config{}, fmt.Errorf("version must be set")
	}

	version := *parsed.Version
	if !isSupportedVersion(version) {
		return Config{}, fmt.Errorf("unsupported version %d", version)
	}

	projectRoot := filepath.Dir(configPath)
	if version == namedTargetVersion {
		return validateVersionThreeConfig(version, projectRoot, parsed)
	}

	target, err := validateCompatibilityTarget(parsed.Target, projectRoot, version)
	if err != nil {
		return Config{}, err
	}

	profiles, err := validateCompatibilityProfiles(parsed.Profiles)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		Version:  version,
		Target:   target,
		Targets:  []Target{target},
		Profiles: profiles,
	}

	return config, nil
}

func isSupportedVersion(version int) bool {
	return version == legacyVersion || version == currentVersion || version == namedTargetVersion
}

func validateVersionThreeConfig(version int, projectRoot string, parsed fileConfig) (Config, error) {
	targets, err := validateVersionThreeTargets(parsed.Targets, projectRoot)
	if err != nil {
		return Config{}, err
	}

	profiles, err := validateVersionThreeProfiles(parsed.Profiles, targets)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Version:  version,
		Targets:  targets,
		Profiles: profiles,
	}, nil
}

func validateCompatibilityTarget(parsedTarget fileTarget, projectRoot string, version int) (Target, error) {
	targetFile, err := resolveTargetFile(parsedTarget.File, projectRoot, "target.file")
	if err != nil {
		return Target{}, err
	}

	switch version {
	case legacyVersion:
		return validateLegacyTarget(parsedTarget, targetFile)
	case currentVersion:
		return validateVersionTwoTarget(parsedTarget, targetFile)
	default:
		return Target{}, fmt.Errorf("unsupported version %d", version)
	}
}

func resolveTargetFile(rawTargetFile string, projectRoot string, fieldName string) (string, error) {
	targetFile := strings.TrimSpace(rawTargetFile)
	if targetFile == "" {
		return "", fmt.Errorf("%s must be set", fieldName)
	}

	if !filepath.IsAbs(targetFile) {
		targetFile = filepath.Join(projectRoot, targetFile)
	}

	return filepath.Clean(targetFile), nil
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
		Name:     compatibilityTargetName,
		File:     targetFile,
		Type:     TargetTypeJSON,
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
		Name:     compatibilityTargetName,
		File:     targetFile,
		Type:     TargetTypeJSON,
		JSONPath: jsonPath,
	}, nil
}

func validateVersionThreeTargets(parsedTargets []fileTarget, projectRoot string) ([]Target, error) {
	if len(parsedTargets) == 0 {
		return nil, fmt.Errorf("at least one target must be configured")
	}

	targets := make([]Target, 0, len(parsedTargets))
	seenNames := make(map[string]struct{}, len(parsedTargets))
	seenLocations := make(map[string]string, len(parsedTargets))

	for index, parsedTarget := range parsedTargets {
		target, err := validateVersionThreeTarget(index, parsedTarget, projectRoot)
		if err != nil {
			return nil, err
		}

		if _, exists := seenNames[target.Name]; exists {
			return nil, fmt.Errorf("duplicate target name %q", target.Name)
		}
		seenNames[target.Name] = struct{}{}

		locationKey := targetLocationKey(target)
		if existingTargetName, exists := seenLocations[locationKey]; exists {
			return nil, fmt.Errorf("targets[%d] duplicates target location used by target %q", index, existingTargetName)
		}
		seenLocations[locationKey] = target.Name

		targets = append(targets, target)
	}

	return targets, nil
}

func validateVersionThreeTarget(index int, parsedTarget fileTarget, projectRoot string) (Target, error) {
	name := strings.TrimSpace(parsedTarget.Name)
	if name == "" {
		return Target{}, fmt.Errorf("targets[%d].name must be set", index)
	}
	if name != parsedTarget.Name {
		return Target{}, fmt.Errorf("targets[%d].name must not contain leading or trailing whitespace", index)
	}

	targetFile, err := resolveTargetFile(parsedTarget.File, projectRoot, fmt.Sprintf("targets[%d].file", index))
	if err != nil {
		return Target{}, err
	}

	targetType, err := validateVersionThreeTargetType(index, parsedTarget.Type, targetFile)
	if err != nil {
		return Target{}, err
	}

	switch targetType {
	case TargetTypeJSON:
		return validateVersionThreeJSONTarget(index, name, targetFile, parsedTarget)
	case TargetTypeDotenv:
		return validateVersionThreeDotenvTarget(index, name, targetFile, parsedTarget)
	default:
		return Target{}, fmt.Errorf("targets[%d].type %q is not supported", index, targetType)
	}
}

func validateVersionThreeTargetType(index int, rawTargetType string, targetFile string) (TargetType, error) {
	targetType := strings.TrimSpace(rawTargetType)
	if targetType == "" {
		inferredType, ok := InferTargetType(targetFile)
		if !ok {
			return "", fmt.Errorf("targets[%d].type must be set because target type cannot be inferred from file %q", index, targetFile)
		}

		return inferredType, nil
	}

	switch TargetType(targetType) {
	case TargetTypeJSON:
		return TargetTypeJSON, nil
	case TargetTypeDotenv:
		return TargetTypeDotenv, nil
	default:
		return "", fmt.Errorf("targets[%d].type %q is not supported", index, targetType)
	}
}

// InferTargetType returns the target type that can be safely inferred from a
// configured file name.
func InferTargetType(targetFile string) (TargetType, bool) {
	fileName := strings.ToLower(filepath.Base(targetFile))
	if strings.HasSuffix(fileName, ".json") {
		return TargetTypeJSON, true
	}
	if fileName == ".env" || strings.HasPrefix(fileName, ".env.") {
		return TargetTypeDotenv, true
	}

	return "", false
}

func validateVersionThreeJSONTarget(index int, name string, targetFile string, parsedTarget fileTarget) (Target, error) {
	jsonPath := strings.TrimSpace(parsedTarget.JSONPath)
	if jsonPath == "" {
		return Target{}, fmt.Errorf("targets[%d].jsonPath must be set for json targets", index)
	}
	if strings.TrimSpace(parsedTarget.Key) != "" {
		return Target{}, fmt.Errorf("targets[%d].key is only supported for dotenv targets", index)
	}
	if _, err := ParseJSONPath(jsonPath); err != nil {
		return Target{}, fmt.Errorf("targets[%d].jsonPath is invalid: %w", index, err)
	}

	return Target{
		Name:     name,
		File:     targetFile,
		Type:     TargetTypeJSON,
		JSONPath: jsonPath,
	}, nil
}

func validateVersionThreeDotenvTarget(index int, name string, targetFile string, parsedTarget fileTarget) (Target, error) {
	if strings.TrimSpace(parsedTarget.JSONPath) != "" {
		return Target{}, fmt.Errorf("targets[%d].jsonPath is only supported for json targets", index)
	}

	key := strings.TrimSpace(parsedTarget.Key)
	if key == "" {
		return Target{}, fmt.Errorf("targets[%d].key must be set for dotenv targets", index)
	}
	if !isValidDotenvKey(parsedTarget.Key) {
		return Target{}, fmt.Errorf("targets[%d].key is invalid: must match [A-Za-z_][A-Za-z0-9_]*", index)
	}

	return Target{
		Name: name,
		File: targetFile,
		Type: TargetTypeDotenv,
		Key:  parsedTarget.Key,
	}, nil
}

func targetLocationKey(target Target) string {
	selector := target.JSONPath
	if target.Type == TargetTypeDotenv {
		selector = target.Key
	}

	return string(target.Type) + "\x00" + filepath.Clean(target.File) + "\x00" + selector
}

func isValidDotenvKey(key string) bool {
	if key == "" {
		return false
	}

	for index := 0; index < len(key); index++ {
		character := key[index]
		if index == 0 {
			if !isDotenvKeyStart(character) {
				return false
			}
			continue
		}

		if !isDotenvKeyPart(character) {
			return false
		}
	}

	return true
}

func isDotenvKeyStart(character byte) bool {
	return character == '_' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

func isDotenvKeyPart(character byte) bool {
	return isDotenvKeyStart(character) || character >= '0' && character <= '9'
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

func validateCompatibilityProfiles(parsedProfiles []fileProfile) ([]Profile, error) {
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
			profile.Values = []ProfileValue{{Target: compatibilityTargetName, Value: &literalValue}}
		} else {
			environmentVariableName := strings.TrimSpace(*parsedProfile.ValueFromEnv)
			if environmentVariableName == "" {
				return nil, fmt.Errorf("profile %q valueFromEnv must be set", name)
			}

			profile.ValueFromEnv = &environmentVariableName
			profile.Values = []ProfileValue{{Target: compatibilityTargetName, ValueFromEnv: &environmentVariableName}}
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func validateVersionThreeProfiles(parsedProfiles []fileProfile, targets []Target) ([]Profile, error) {
	if len(parsedProfiles) == 0 {
		return nil, fmt.Errorf("at least one profile must be configured")
	}

	targetNames := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetNames[target.Name] = struct{}{}
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

		if parsedProfile.Value != nil || parsedProfile.ValueFromEnv != nil {
			return nil, fmt.Errorf("profile %q must define target values under values in version 3", name)
		}
		if len(parsedProfile.Values) == 0 {
			return nil, fmt.Errorf("profile %q must include at least one value", name)
		}

		values, err := validateVersionThreeProfileValues(name, parsedProfile.Values, targetNames)
		if err != nil {
			return nil, err
		}

		profiles = append(profiles, Profile{
			Name:      name,
			Values:    values,
			Protected: parsedProfile.Protected,
		})
	}

	return profiles, nil
}

func validateVersionThreeProfileValues(profileName string, parsedValues []fileProfileValue, targetNames map[string]struct{}) ([]ProfileValue, error) {
	values := make([]ProfileValue, 0, len(parsedValues))
	seenTargets := make(map[string]struct{}, len(parsedValues))

	for index, parsedValue := range parsedValues {
		targetName := parsedValue.Target
		if strings.TrimSpace(targetName) == "" {
			return nil, fmt.Errorf("profile %q values[%d].target must be set", profileName, index)
		}
		if _, exists := targetNames[targetName]; !exists {
			return nil, fmt.Errorf("profile %q values[%d].target %q is not configured", profileName, index, targetName)
		}
		if _, exists := seenTargets[targetName]; exists {
			return nil, fmt.Errorf("profile %q has duplicate value for target %q", profileName, targetName)
		}
		seenTargets[targetName] = struct{}{}

		hasValue := parsedValue.Value != nil
		hasValueFromEnv := parsedValue.ValueFromEnv != nil
		if hasValue == hasValueFromEnv {
			return nil, fmt.Errorf("profile %q value for target %q must define exactly one of value or valueFromEnv", profileName, targetName)
		}

		value := ProfileValue{Target: targetName}
		if hasValue {
			literalValue := *parsedValue.Value
			value.Value = &literalValue
		} else {
			environmentVariableName := strings.TrimSpace(*parsedValue.ValueFromEnv)
			if environmentVariableName == "" {
				return nil, fmt.Errorf("profile %q value for target %q valueFromEnv must be set", profileName, targetName)
			}

			value.ValueFromEnv = &environmentVariableName
		}

		values = append(values, value)
	}

	return values, nil
}
