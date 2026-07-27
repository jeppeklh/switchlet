package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	Version  *int          `yaml:"version"`
	Target   fileTarget    `yaml:"target"`
	Targets  []fileTarget  `yaml:"targets"`
	Profiles []fileProfile `yaml:"profiles"`
}

type fileTarget struct {
	Name           string `yaml:"name,omitempty"`
	File           string `yaml:"file"`
	Type           string `yaml:"type,omitempty"`
	JSONPath       string `yaml:"jsonPath,omitempty"`
	Key            string `yaml:"key,omitempty"`
	YAMLPath       string `yaml:"yamlPath,omitempty"`
	TOMLPath       string `yaml:"tomlPath,omitempty"`
	ConnectionName string `yaml:"connectionName,omitempty"`
}

type fileProfile struct {
	Name         string             `yaml:"name"`
	Values       []fileProfileValue `yaml:"values,omitempty"`
	Value        *string            `yaml:"value,omitempty"`
	ValueFromEnv *string            `yaml:"valueFromEnv,omitempty"`
	Protected    bool               `yaml:"protected,omitempty"`
}

type fileProfileValue struct {
	Target       string  `yaml:"target"`
	Value        *string `yaml:"value,omitempty"`
	ValueFromEnv *string `yaml:"valueFromEnv,omitempty"`
}

// Load reads, validates, and resolves a Switchlet configuration file.
func Load(configPath string) (Config, error) {
	if strings.TrimSpace(configPath) == "" {
		return Config{}, fmt.Errorf("configuration path must be set")
	}

	resolvedConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("resolve configuration path %q: %w", configPath, err)
	}

	contents, err := os.ReadFile(resolvedConfigPath)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration file %q: %w", resolvedConfigPath, err)
	}

	return loadConfigContents(resolvedConfigPath, contents)
}

func loadConfigContents(resolvedConfigPath string, contents []byte) (Config, error) {
	var parsed fileConfig
	if err := yaml.Unmarshal(contents, &parsed); err != nil {
		return Config{}, fmt.Errorf("parse configuration file %q: %w", resolvedConfigPath, err)
	}

	loadedConfig, err := validateConfig(resolvedConfigPath, parsed)
	if err != nil {
		return Config{}, fmt.Errorf("validate configuration file %q: %w", resolvedConfigPath, err)
	}

	return loadedConfig, nil
}
