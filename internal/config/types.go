package config

// Config is a validated Switchlet configuration loaded from .switchlet.yaml.
type Config struct {
	Version int
	// Target is the compatibility single target used by Version 1 and Version 2 callers.
	// Target-aware code should consume Targets instead.
	Target   Target
	Targets  []Target
	Profiles []Profile
}

// TargetType identifies the concrete target implementation used for a target.
type TargetType string

const (
	// TargetTypeJSON identifies a JSON target selected by JSONPath.
	TargetTypeJSON TargetType = "json"
	// TargetTypeDotenv identifies a dotenv target selected by Key.
	TargetTypeDotenv TargetType = "dotenv"
	// TargetTypeYAML identifies a YAML target selected by YAMLPath.
	TargetTypeYAML TargetType = "yaml"
	// TargetTypeTOML identifies a TOML target selected by TOMLPath.
	TargetTypeTOML TargetType = "toml"
)

// Target defines one named configuration value Switchlet may modify.
type Target struct {
	Name     string
	File     string
	Type     TargetType
	JSONPath string
	Key      string
	YAMLPath string
	TOMLPath string
}

// Profile defines one available configuration profile.
type Profile struct {
	Name         string
	Values       []ProfileValue
	Value        *string
	ValueFromEnv *string
	Protected    bool
}

// ProfileValue defines one target-specific value inside a profile.
type ProfileValue struct {
	Target       string
	Value        *string
	ValueFromEnv *string
}
