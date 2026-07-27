package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const gitignoreFileName = ".gitignore"

// ValidateCreateLocation verifies that a project directory can host a new
// Switchlet configuration file.
func ValidateCreateLocation(projectRoot string) error {
	_, _, err := resolveCreateLocation(projectRoot)
	return err
}

// Create writes a new Version 3 Switchlet configuration file, then loads it
// back to verify that the generated YAML is valid.
func Create(projectRoot string, targets []Target, profiles []Profile) (configPath string, loadedConfig Config, returnErr error) {
	resolvedProjectRoot, configPath, err := resolveCreateLocation(projectRoot)
	if err != nil {
		return "", Config{}, err
	}

	contents, err := marshalCreatedConfig(resolvedProjectRoot, targets, profiles)
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

// EnsureConfigIgnored creates or updates the project-root .gitignore so it
// ignores .switchlet.yaml. It returns true when the .gitignore file was created
// or modified.
func EnsureConfigIgnored(projectRoot string) (bool, error) {
	resolvedProjectRoot, err := resolveProjectRoot(projectRoot)
	if err != nil {
		return false, err
	}

	gitignorePath := filepath.Join(resolvedProjectRoot, gitignoreFileName)
	gitignoreInfo, err := os.Stat(gitignorePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.WriteFile(gitignorePath, []byte(configFileName+"\n"), 0o644); err != nil {
			return false, fmt.Errorf("create gitignore file %q: %w", gitignorePath, err)
		}

		return true, nil
	case err != nil:
		return false, fmt.Errorf("stat gitignore file %q: %w", gitignorePath, err)
	case gitignoreInfo.IsDir():
		return false, fmt.Errorf("gitignore path %q is a directory", gitignorePath)
	}

	contents, err := os.ReadFile(gitignorePath)
	if err != nil {
		return false, fmt.Errorf("read gitignore file %q: %w", gitignorePath, err)
	}
	if gitignoreContainsConfigEntry(contents) {
		return false, nil
	}

	updatedContents := appendConfigIgnoreEntry(contents)
	if err := os.WriteFile(gitignorePath, updatedContents, gitignoreInfo.Mode().Perm()); err != nil {
		return false, fmt.Errorf("write gitignore file %q: %w", gitignorePath, err)
	}

	return true, nil
}

func resolveProjectRoot(projectRoot string) (string, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return "", fmt.Errorf("project root must be set")
	}

	resolvedProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root %q: %w", projectRoot, err)
	}

	projectRootInfo, err := os.Stat(resolvedProjectRoot)
	if err != nil {
		return "", fmt.Errorf("stat project root %q: %w", resolvedProjectRoot, err)
	}
	if !projectRootInfo.IsDir() {
		return "", fmt.Errorf("project root %q is not a directory", resolvedProjectRoot)
	}

	return resolvedProjectRoot, nil
}

func resolveCreateLocation(projectRoot string) (string, string, error) {
	resolvedProjectRoot, err := resolveProjectRoot(projectRoot)
	if err != nil {
		return "", "", err
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

func gitignoreContainsConfigEntry(contents []byte) bool {
	for _, line := range strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == configFileName || trimmedLine == "/"+configFileName {
			return true
		}
	}

	return false
}

func appendConfigIgnoreEntry(contents []byte) []byte {
	updatedContents := append([]byte(nil), contents...)
	lineEnding := detectLineEnding(updatedContents)

	if len(updatedContents) > 0 && !bytes.HasSuffix(updatedContents, []byte("\n")) {
		updatedContents = append(updatedContents, []byte(lineEnding)...)
	}

	updatedContents = append(updatedContents, []byte(configFileName)...)
	updatedContents = append(updatedContents, []byte(lineEnding)...)

	return updatedContents
}

func detectLineEnding(contents []byte) string {
	if bytes.Contains(contents, []byte("\r\n")) {
		return "\r\n"
	}

	return "\n"
}

func marshalCreatedConfig(projectRoot string, targets []Target, profiles []Profile) ([]byte, error) {
	configuredTargets := make([]fileTarget, 0, len(targets))
	for _, target := range targets {
		targetType := target.Type
		if targetType == "" {
			if inferredType, ok := InferTargetType(target.File); ok {
				targetType = inferredType
			}
		}

		configuredTarget := fileTarget{
			Name:     target.Name,
			File:     configTargetPath(projectRoot, target.File),
			Type:     string(targetType),
			JSONPath: target.JSONPath,
			Key:      target.Key,
			YAMLPath: target.YAMLPath,
		}

		configuredTargets = append(configuredTargets, configuredTarget)
	}

	configuredProfiles := make([]fileProfile, 0, len(profiles))
	for _, profile := range profiles {
		configuredProfile := fileProfile{
			Name:      profile.Name,
			Protected: profile.Protected,
		}

		configuredProfile.Values = make([]fileProfileValue, 0, len(profile.Values))
		for _, value := range profile.Values {
			configuredValue := fileProfileValue{Target: value.Target}
			if value.Value != nil {
				literalValue := *value.Value
				configuredValue.Value = &literalValue
			}
			if value.ValueFromEnv != nil {
				environmentVariableName := *value.ValueFromEnv
				configuredValue.ValueFromEnv = &environmentVariableName
			}

			configuredProfile.Values = append(configuredProfile.Values, configuredValue)
		}

		configuredProfiles = append(configuredProfiles, configuredProfile)
	}

	configFile := struct {
		Version  int           `yaml:"version"`
		Targets  []fileTarget  `yaml:"targets"`
		Profiles []fileProfile `yaml:"profiles"`
	}{
		Version:  namedTargetVersion,
		Targets:  configuredTargets,
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
