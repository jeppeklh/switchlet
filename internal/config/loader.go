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
	Profiles []fileProfile `yaml:"profiles"`
}

type fileTarget struct {
	File           string `yaml:"file"`
	JSONPath       string `yaml:"jsonPath,omitempty"`
	ConnectionName string `yaml:"connectionName,omitempty"`
}

type fileProfile struct {
	Name         string  `yaml:"name"`
	Value        *string `yaml:"value,omitempty"`
	ValueFromEnv *string `yaml:"valueFromEnv,omitempty"`
	Protected    bool    `yaml:"protected,omitempty"`
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
