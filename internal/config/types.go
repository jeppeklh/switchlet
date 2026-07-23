package config

// Config is a validated Switchlet configuration loaded from .switchlet.yaml.
type Config struct {
	Version  int
	Target   Target
	Profiles []Profile
}

// Target defines the resolved file and connection string Switchlet operates on.
type Target struct {
	File           string
	ConnectionName string
}

// Profile defines one available configuration profile.
type Profile struct {
	Name         string
	Value        *string
	ValueFromEnv *string
	Protected    bool
}
