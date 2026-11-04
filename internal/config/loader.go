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
	ValueType      ValueType `yaml:"valueType,omitempty"`
	Name           string    `yaml:"name,omitempty"`
	File           string    `yaml:"file"`
	Type           string    `yaml:"type,omitempty"`
	JSONPath       string    `yaml:"jsonPath,omitempty"`
	Key            string    `yaml:"key,omitempty"`
	YAMLPath       string    `yaml:"yamlPath,omitempty"`
	TOMLPath       string    `yaml:"tomlPath,omitempty"`
	ConnectionName string    `yaml:"connectionName,omitempty"`
}

type fileProfile struct {
	Name         string             `yaml:"name"`
	Values       []fileProfileValue `yaml:"values,omitempty"`
	Value        *string            `yaml:"value,omitempty"`
	ValueFromEnv *string            `yaml:"valueFromEnv,omitempty"`
	Protected    bool               `yaml:"protected,omitempty"`
}

type fileProfileValue struct {
	ValueType    ValueType `yaml:"-"`
	rawValue     *yaml.Node
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
	var root yaml.Node
	if err := yaml.Unmarshal(contents, &root); err != nil {
		return Config{}, fmt.Errorf("parse configuration file %q: %w", resolvedConfigPath, err)
	}

	var parsed fileConfig
	if err := root.Decode(&parsed); err != nil {
		return Config{}, fmt.Errorf("parse configuration file %q: %w", resolvedConfigPath, err)
	}

	loadedConfig, err := validateConfig(resolvedConfigPath, parsed)
	if err != nil {
		return Config{}, fmt.Errorf("validate configuration file %q: %w", resolvedConfigPath, validationErrorWithLocation(resolvedConfigPath, &root, err))
	}

	return loadedConfig, nil
}

func fingerprintConfigContents(contents []byte) ConfigFingerprint {
	return ConfigFingerprint{hash: sha256.Sum256(contents)}
}

func (value *fileProfileValue) UnmarshalYAML(node *yaml.Node) error {
	type plain fileProfileValue
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return fmt.Errorf("profile value must be a scalar or environment reference")
	}
	*value = fileProfileValue(decoded)
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == "value" {
			value.rawValue = node.Content[index+1]
		}
	}
	return nil
}

// bindProfileValueTypes attaches target-owned type metadata before normalization.
// Only Version 4 interprets literal YAML tags; older versions keep string decoding.
func bindProfileValueTypes(version int, profiles []fileProfile, targets []Target) error {
	kinds := make(map[string]ValueType, len(targets))
	for _, target := range targets {
		kinds[target.Name] = target.ValueType
	}
	for profileIndex := range profiles {
		configuredProfile := &profiles[profileIndex]
		for valueIndex := range configuredProfile.Values {
			value := &configuredProfile.Values[valueIndex]
			value.ValueType = kinds[value.Target]
			if version < typedTargetVersion || value.rawValue == nil {
				continue
			}
			kind := EffectiveValueType(value.ValueType)
			if value.rawValue.Kind != yaml.ScalarNode || !literalTagMatches(kind, value.rawValue.Tag) {
				return fmt.Errorf("profile %q target %q literal must have type %s", configuredProfile.Name, value.Target, kind)
			}
		}
	}
	return nil
}
func literalTagMatches(kind ValueType, tag string) bool {
	switch kind {
	case ValueTypeString:
		return tag == "!!str"
	case ValueTypeBoolean:
		return tag == "!!bool"
	case ValueTypeInteger:
		return tag == "!!int"
	case ValueTypeNumber:
		return tag == "!!int" || tag == "!!float"
	}
	return false
}
