package app

import (
	"errors"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
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

// TargetDescriptor identifies one configured target without exposing its value.
type TargetDescriptor struct {
	TargetName   string
	TargetFile   string
	TargetType   config.TargetType
	SelectorName string
	Selector     string
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
type PlannedChange = TargetDescriptor

// StatusComparisonStatus identifies the top-level current-status result.
type StatusComparisonStatus string

const (
	// StatusComparisonMatched indicates that exactly one complete profile matches.
	StatusComparisonMatched StatusComparisonStatus = "matched"
	// StatusComparisonAmbiguous indicates that multiple complete profiles match.
	StatusComparisonAmbiguous StatusComparisonStatus = "ambiguous"
	// StatusComparisonUnmatched indicates that no complete profile matches.
	StatusComparisonUnmatched StatusComparisonStatus = "unmatched"
)

// StatusComparison describes how current target values compare with configured profiles.
type StatusComparison struct {
	Status              StatusComparisonStatus
	CurrentProfile      string
	Matches             []ProfileMatch
	MatchedTargets      []TargetDescriptor
	PartialMatches      []PartialProfileMatch
	ClosestProfiles     []ClosestProfileMatch
	UnavailableProfiles []UnavailableProfile
	TargetCount         int
	Complete            bool
}

// ProfileMatch describes one complete profile that exactly matches current targets.
type ProfileMatch struct {
	ProfileName string
	Protected   bool
}

// PartialProfileMatch describes a partial profile whose included targets match current values.
type PartialProfileMatch struct {
	ProfileName     string
	Protected       bool
	MatchedTargets  int
	IncludedTargets int
	OmittedTargets  int
	TargetCount     int
}

// ClosestProfileMatch describes a profile's safe match counts against current values.
type ClosestProfileMatch struct {
	ProfileName        string
	Protected          bool
	MatchedTargets     int
	IncludedTargets    int
	UnavailableTargets int
	TargetCount        int
}

// UnavailableProfile describes a profile that could not be fully compared.
type UnavailableProfile struct {
	ProfileName string
	Protected   bool
	Reason      string
	Values      []UnavailableValue
}

// UnavailableValue describes one unresolved profile value without exposing target values.
type UnavailableValue struct {
	TargetDescriptor
	EnvironmentVariableName string
	Reason                  string
}

// ProfileDiff compares one selected profile with current target values.
type ProfileDiff struct {
	ProfileName    string
	Protected      bool
	Complete       bool
	WouldUpdate    []TargetDescriptor
	AlreadyMatches []TargetDescriptor
	Unavailable    []UnavailableValue
	OmittedTargets []TargetDescriptor
}

// TargetFailure describes a target-specific application failure in a
// presentation-neutral form.
type TargetFailure struct {
	TargetName   string
	TargetFile   string
	TargetType   config.TargetType
	SelectorName string
	Selector     string
	Reason       string
}

// TargetFailureFromError extracts target-specific failure context from an error.
func TargetFailureFromError(err error) (TargetFailure, bool) {
	var targetErr editor.TargetError
	if !errors.As(err, &targetErr) {
		return TargetFailure{}, false
	}

	selectorName, selector := targetSelector(targetErr.Target)
	failure := TargetFailure{
		TargetName:   targetErr.Target.Name,
		TargetFile:   targetErr.Target.File,
		TargetType:   targetErr.Target.Type,
		SelectorName: selectorName,
		Selector:     selector,
	}
	if targetErr.Err != nil {
		failure.Reason = targetErr.Err.Error()
	}

	return failure, true
}
