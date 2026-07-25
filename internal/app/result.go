package app

import (
	"errors"

	"github.com/jeppeklh/switchlet/internal/config"
)

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
	// ProfileSourceMixed indicates that the profile contains literal and environment-backed values.
	ProfileSourceMixed ProfileSource = "mixed"
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
	Values                  []ProfileValueItem
	TargetCount             int
	TotalTargets            int
	Partial                 bool
}

// ProfileValueItem describes one target value included by a profile.
type ProfileValueItem struct {
	TargetName              string
	TargetFile              string
	TargetType              config.TargetType
	SelectorName            string
	Selector                string
	Source                  ProfileSource
	EnvironmentVariableName string
	MaskedValue             string
	Available               bool
	UnavailableReason       string
}

// Result describes a successful profile application or dry run.
type Result struct {
	ProfileName string
	TargetPath  string
	TargetFile  string
	Protected   bool
	DryRun      bool
	Changes     []PlannedChange
}

// PlannedChange describes one target location included in a successful plan.
type PlannedChange struct {
	TargetName   string
	TargetFile   string
	TargetType   config.TargetType
	SelectorName string
	Selector     string
}
