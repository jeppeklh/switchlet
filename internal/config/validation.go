package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const supportedVersion = 1

func validateConfig(configPath string, parsed fileConfig) (Config, error) {
	if parsed.Version == nil {
		return Config{}, fmt.Errorf("version must be set")
	}
	if *parsed.Version != supportedVersion {
		return Config{}, fmt.Errorf("unsupported version %d", *parsed.Version)
	}

	projectRoot := filepath.Dir(configPath)

	target, err := validateTarget(parsed.Target, projectRoot)
	if err != nil {
		return Config{}, err
	}

	profiles, err := validateProfiles(parsed.Profiles)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ConfigPath:  configPath,
		ProjectRoot: projectRoot,
		Version:     *parsed.Version,
		Target:      target,
		Profiles:    profiles,
	}

	if err := validateTargetFile(config.Target.File, config.Target.ConnectionName); err != nil {
		return Config{}, err
	}

	return config, nil
}

func validateTarget(parsedTarget fileTarget, projectRoot string) (Target, error) {
	targetFile := strings.TrimSpace(parsedTarget.File)
	if targetFile == "" {
		return Target{}, fmt.Errorf("target.file must be set")
	}

	connectionName := strings.TrimSpace(parsedTarget.ConnectionName)
	if connectionName == "" {
		return Target{}, fmt.Errorf("target.connectionName must be set")
	}

	if !filepath.IsAbs(targetFile) {
		targetFile = filepath.Join(projectRoot, targetFile)
	}

	return Target{
		File:           filepath.Clean(targetFile),
		ConnectionName: connectionName,
	}, nil
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

func validateTargetFile(targetPath string, connectionName string) error {
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("target file %q: %w", targetPath, err)
	}
	if targetInfo.IsDir() {
		return fmt.Errorf("target file %q is a directory", targetPath)
	}

	contents, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read target file %q: %w", targetPath, err)
	}

	var decoded any
	if err := json.Unmarshal(contents, &decoded); err != nil {
		return fmt.Errorf("target file %q contains invalid JSON: %w", targetPath, err)
	}

	rootObject, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("target file %q must contain a JSON object at the root", targetPath)
	}

	connectionStringsValue, ok := rootObject["ConnectionStrings"]
	if !ok {
		return fmt.Errorf("target file %q must contain a ConnectionStrings object", targetPath)
	}

	connectionStringsObject, ok := connectionStringsValue.(map[string]any)
	if !ok {
		return fmt.Errorf("target file %q ConnectionStrings must be an object", targetPath)
	}

	connectionValue, ok := connectionStringsObject[connectionName]
	if !ok {
		return fmt.Errorf("target file %q does not contain connection string %q", targetPath, connectionName)
	}

	if _, ok := connectionValue.(string); !ok {
		return fmt.Errorf("target file %q connection string %q must be a string", targetPath, connectionName)
	}

	return nil
}
