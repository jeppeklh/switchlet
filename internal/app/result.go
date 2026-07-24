package app

import "errors"

var (
	// ErrProfileNotFound indicates that a requested profile name does not exist.
	ErrProfileNotFound = errors.New("configured profile not found")
	// ErrProfileUnavailable indicates that a configured profile could not be resolved.
	ErrProfileUnavailable = errors.New("configured profile is unavailable")
	// ErrProtectedProfileRequiresApproval indicates that a protected profile was
	// requested without explicit non-interactive approval.
	ErrProtectedProfileRequiresApproval = errors.New("protected profile requires explicit opt-in")
)

// ApplyOptions controls how a profile application request behaves.
type ApplyOptions struct {
	DryRun         bool
	AllowProtected bool
}

// ProfileSource identifies how a profile is resolved for the UI.
type ProfileSource string

const (
	// ProfileSourceLiteral indicates that the profile value comes directly from configuration.
	ProfileSourceLiteral ProfileSource = "literal"
	// ProfileSourceEnvironment indicates that the profile value comes from an environment variable.
	ProfileSourceEnvironment ProfileSource = "environment"
)

// ProfileItem describes one configured profile for TUI and CLI callers.
type ProfileItem struct {
	Name                    string
	Protected               bool
	Available               bool
	Source                  ProfileSource
	EnvironmentVariableName string
	MaskedValue             string
	UnavailableReason       string
}

// Result describes a successful profile application or dry run.
type Result struct {
	ProfileName string
	TargetPath  string
	TargetFile  string
	Protected   bool
	DryRun      bool
}
