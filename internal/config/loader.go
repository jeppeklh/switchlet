package config

import (
	"crypto/sha256"
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
	snapshot, err := LoadSnapshot(configPath)
	if err != nil {
		return Config{}, err
	}

	return snapshot.Config, nil
}

// LoadSnapshot reads and validates a Switchlet configuration file while
// preserving the file identity needed for conflict-aware editing.
func LoadSnapshot(configPath string) (ConfigSnapshot, error) {
	if strings.TrimSpace(configPath) == "" {
		return ConfigSnapshot{}, fmt.Errorf("configuration path must be set")
	}

	resolvedConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return ConfigSnapshot{}, fmt.Errorf("resolve configuration path %q: %w", configPath, err)
	}

	contents, err := os.ReadFile(resolvedConfigPath)
	if err != nil {
		return ConfigSnapshot{}, fmt.Errorf("read configuration file %q: %w", resolvedConfigPath, err)
	}

	loadedConfig, err := loadConfigContents(resolvedConfigPath, contents)
	if err != nil {
		return ConfigSnapshot{}, err
	}

	return ConfigSnapshot{
		Config:          loadedConfig,
		ConfigPath:      resolvedConfigPath,
		ProjectRoot:     filepath.Dir(resolvedConfigPath),
		OriginalVersion: loadedConfig.Version,
		fingerprint:     fingerprintConfigContents(contents),
	}, nil
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

func fingerprintConfigContents(contents []byte) ConfigFingerprint {
	return ConfigFingerprint{hash: sha256.Sum256(contents)}
}
