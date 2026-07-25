package profile

import "errors"

// ValueSource identifies how a profile value is produced.
type ValueSource string

const (
	// ValueSourceLiteral indicates that the profile value comes directly from configuration.
	ValueSourceLiteral ValueSource = "literal"
	// ValueSourceEnvironment indicates that the profile value comes from an environment variable.
	ValueSourceEnvironment ValueSource = "environment"
	// ValueSourceMixed indicates that a profile contains both literal and environment-backed values.
	ValueSourceMixed ValueSource = "mixed"
)

var (
	// ErrEnvironmentVariableNotSet indicates that a configured environment variable does not exist.
	ErrEnvironmentVariableNotSet = errors.New("environment variable is not set")
	// ErrEnvironmentVariableEmpty indicates that a configured environment variable exists but has no value.
	ErrEnvironmentVariableEmpty = errors.New("environment variable is empty")
	// ErrProfileValueEmpty indicates that a configured profile resolved to an empty value.
	ErrProfileValueEmpty = errors.New("profile value is empty")
)

// ResolvedProfile contains the display-safe and application-safe result of resolving one configured profile.
type ResolvedProfile struct {
	Name                    string
	Protected               bool
	Source                  ValueSource
	EnvironmentVariableName string
	Value                   string
	MaskedValue             string
	Values                  []ResolvedValue
	ResolutionError         error
}

// ResolvedValue contains the resolved value for one target entry in a profile.
type ResolvedValue struct {
	Target                  string
	Source                  ValueSource
	EnvironmentVariableName string
	Value                   string
	MaskedValue             string
	ResolutionError         error
}

// IsAvailable reports whether the profile resolved to a usable value.
func (profile ResolvedProfile) IsAvailable() bool {
	return profile.ResolutionError == nil
}
