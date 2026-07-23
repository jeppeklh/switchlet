package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateCreateLocation verifies that a project directory can host a new
// Switchlet configuration file.
func ValidateCreateLocation(projectRoot string) error {
	_, _, err := resolveCreateLocation(projectRoot)
	return err
}

// Create writes a new Switchlet configuration file, then loads it back to
// verify that the generated YAML is valid.
func Create(projectRoot string, target Target, profiles []Profile) (configPath string, loadedConfig Config, returnErr error) {
	resolvedProjectRoot, configPath, err := resolveCreateLocation(projectRoot)
	if err != nil {
		return "", Config{}, err
	}

	contents, err := marshalCreatedConfig(resolvedProjectRoot, target, profiles)
	if err != nil {
		return "", Config{}, err
	}

	removeCreatedFile := true
	defer func() {
		if returnErr == nil || !removeCreatedFile {
			return
		}

		if err := os.Remove(configPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			returnErr = fmt.Errorf("%w; remove configuration file %q: %v", returnErr, configPath, err)
		}
	}()

	if err := writeCreatedConfig(configPath, contents); err != nil {
		return "", Config{}, err
	}

	loadedConfig, err = Load(configPath)
	if err != nil {
		return configPath, Config{}, fmt.Errorf("verify configuration file %q: %w", configPath, err)
	}

	removeCreatedFile = false

	return configPath, loadedConfig, nil
}

func resolveCreateLocation(projectRoot string) (string, string, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return "", "", fmt.Errorf("project root must be set")
	}

	resolvedProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve project root %q: %w", projectRoot, err)
	}

	projectRootInfo, err := os.Stat(resolvedProjectRoot)
	if err != nil {
		return "", "", fmt.Errorf("stat project root %q: %w", resolvedProjectRoot, err)
	}
	if !projectRootInfo.IsDir() {
		return "", "", fmt.Errorf("project root %q is not a directory", resolvedProjectRoot)
	}

	existingConfigPath, err := Discover(resolvedProjectRoot)
	switch {
	case err == nil:
		return "", "", fmt.Errorf("cannot create %s in %q: discovered existing configuration file %q", configFileName, resolvedProjectRoot, existingConfigPath)
	case errors.Is(err, ErrConfigNotFound):
	default:
		return "", "", fmt.Errorf("check for existing configuration from %q: %w", resolvedProjectRoot, err)
	}

	return resolvedProjectRoot, filepath.Join(resolvedProjectRoot, configFileName), nil
}

func marshalCreatedConfig(projectRoot string, target Target, profiles []Profile) ([]byte, error) {
	configuredProfiles := make([]fileProfile, 0, len(profiles))
	for _, profile := range profiles {
		configuredProfile := fileProfile{
			Name:      profile.Name,
			Protected: profile.Protected,
		}

		if profile.Value != nil {
			literalValue := *profile.Value
			configuredProfile.Value = &literalValue
		}
		if profile.ValueFromEnv != nil {
			environmentVariableName := *profile.ValueFromEnv
			configuredProfile.ValueFromEnv = &environmentVariableName
		}

		configuredProfiles = append(configuredProfiles, configuredProfile)
	}

	configFile := fileConfig{
		Version: intPointer(supportedVersion),
		Target: fileTarget{
			File:           configTargetPath(projectRoot, target.File),
			ConnectionName: target.ConnectionName,
		},
		Profiles: configuredProfiles,
	}

	contents, err := yaml.Marshal(configFile)
	if err != nil {
		return nil, fmt.Errorf("serialize configuration file: %w", err)
	}
	if len(contents) == 0 || contents[len(contents)-1] != '\n' {
		contents = append(contents, '\n')
	}

	return contents, nil
}

func configTargetPath(projectRoot string, targetPath string) string {
	if strings.TrimSpace(targetPath) == "" {
		return targetPath
	}

	resolvedTargetPath := targetPath
	if !filepath.IsAbs(resolvedTargetPath) {
		resolvedTargetPath = filepath.Join(projectRoot, resolvedTargetPath)
	}
	resolvedTargetPath = filepath.Clean(resolvedTargetPath)

	relativeTargetPath, err := filepath.Rel(projectRoot, resolvedTargetPath)
	if err == nil {
		return filepath.ToSlash(relativeTargetPath)
	}

	return filepath.ToSlash(resolvedTargetPath)
}

func writeCreatedConfig(configPath string, contents []byte) (returnErr error) {
	configFile, err := os.OpenFile(configPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create configuration file %q: %w", configPath, err)
	}

	defer func() {
		if err := configFile.Close(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("close configuration file %q: %w", configPath, err)
		}
	}()

	if _, err := configFile.Write(contents); err != nil {
		return fmt.Errorf("write configuration file %q: %w", configPath, err)
	}

	return nil
}

func intPointer(value int) *int {
	return &value
}
