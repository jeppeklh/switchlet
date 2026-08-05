package app

import (
	"errors"
	"fmt"

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
	// ErrPostApplyVerificationFailed indicates that target writes completed, but
	// Switchlet could not confirm the selected profile state afterward.
	ErrPostApplyVerificationFailed = errors.New("post-apply verification failed")
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

// PostApplyVerificationError describes failed verification after target writes
// completed. It carries only target context and value-safe reasons.
type PostApplyVerificationError struct {
	ProfileName string
	Failures    []PostApplyVerificationFailure
	Err         error
}

func (err PostApplyVerificationError) Error() string {
	base := fmt.Sprintf("post-apply verification failed for profile %q", err.ProfileName)
	if len(err.Failures) == 0 {
		if err.Err != nil {
			return fmt.Sprintf("%s: %v", base, err.Err)
		}

		return base
	}
	if len(err.Failures) > 1 {
		return fmt.Sprintf("%s: %d target(s) failed verification", base, len(err.Failures))
	}

	failure := err.Failures[0]
	reason := failure.Reason
	if reason == "" {
		reason = "verification failed"
	}
	return fmt.Sprintf("%s: target %q: %s", base, targetNameForError(failure.TargetName), reason)
}

func (err PostApplyVerificationError) Unwrap() error {
	return errors.Join(ErrPostApplyVerificationFailed, err.Err)
}

// PostApplyVerificationFailure identifies one target that could not be verified
// without exposing current or expected managed values.
type PostApplyVerificationFailure struct {
	TargetDescriptor
	Reason string
}

func targetNameForError(targetName string) string {
	if targetName == "" {
		return "target"
	}

	return targetName
}

// ValueVisibility identifies whether app-owned preview data should include
// managed values for an explicit interactive reveal surface.
type ValueVisibility string

const (
	// ValueVisibilityHidden omits raw managed values from preview data.
	ValueVisibilityHidden ValueVisibility = "hidden"
	// ValueVisibilityShown includes raw managed values in supported preview data.
	ValueVisibilityShown ValueVisibility = "shown"
)

// PreviewOptions controls app-owned profile contents and managed patch preview data.
type PreviewOptions struct {
	ValueVisibility ValueVisibility
}

// ProfileContents describes the selected profile's included managed targets
// grouped for the main picker profile-contents panel.
type ProfileContents struct {
	ProfileName        string
	Protected          bool
	Available          bool
	Source             ProfileSource
	UnavailableReason  string
	TargetCount        int
	TotalTargets       int
	OmittedTargetCount int
	Partial            bool
	Files              []ProfileContentsFileGroup
	OmittedTargets     []TargetDescriptor
}

// ProfileContentsFileGroup contains included profile targets for one target file.
type ProfileContentsFileGroup struct {
	TargetFile string
	Targets    []ProfileContentsTarget
}

// ProfileContentsTarget describes one included managed target in profile contents.
type ProfileContentsTarget struct {
	TargetDescriptor
	Source                  ProfileSource
	EnvironmentVariableName string
	Available               bool
	UnavailableReason       string
	Value                   string
	ValueVisible            bool
}

// ManagedPatchStatus identifies one managed patch preview target state.
type ManagedPatchStatus string

const (
	// ManagedPatchStatusWouldUpdate means the selected profile value differs from the current target value.
	ManagedPatchStatusWouldUpdate ManagedPatchStatus = "would_update"
	// ManagedPatchStatusAlreadyMatches means the selected profile value already matches the current target value.
	ManagedPatchStatusAlreadyMatches ManagedPatchStatus = "already_matches"
	// ManagedPatchStatusUnavailable means the selected profile value could not be resolved safely.
	ManagedPatchStatusUnavailable ManagedPatchStatus = "unavailable"
)

// ManagedPatchPreview describes selected-profile diff data for managed patch rendering.
type ManagedPatchPreview struct {
	ProfileName         string
	Protected           bool
	Complete            bool
	TargetCount         int
	IncludedTargetCount int
	OmittedTargetCount  int
	Partial             bool
	Files               []ManagedPatchFileGroup
	OmittedTargets      []TargetDescriptor
}

// ManagedPatchFileGroup contains managed patch hunks for one target file.
type ManagedPatchFileGroup struct {
	TargetFile string
	Hunks      []ManagedPatchHunk
}

// ManagedPatchHunk describes one configured target location in a managed patch preview.
type ManagedPatchHunk struct {
	TargetDescriptor
	Status                  ManagedPatchStatus
	Source                  ProfileSource
	EnvironmentVariableName string
	Available               bool
	UnavailableReason       string
	CurrentValue            string
	CurrentValueVisible     bool
	ProfileValue            string
	ProfileValueVisible     bool
}

// Result describes a successful profile application or dry run.
type Result struct {
	ProfileName   string
	TargetPath    string
	TargetFile    string
	Protected     bool
	DryRun        bool
	Changes       []PlannedChange
	DryRunPreview *ManagedPatchPreview
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

// HealthCheckStatus identifies the result of one doctor health check.
type HealthCheckStatus string

const (
	// HealthCheckOK indicates that a doctor check passed.
	HealthCheckOK HealthCheckStatus = "ok"
	// HealthCheckWarning indicates that a doctor check found a non-fatal issue.
	HealthCheckWarning HealthCheckStatus = "warning"
	// HealthCheckFailed indicates that a doctor check found a health failure.
	HealthCheckFailed HealthCheckStatus = "failed"
	// HealthCheckSkipped indicates that a doctor check could not run safely.
	HealthCheckSkipped HealthCheckStatus = "skipped"
)

// HealthCheck describes one value-safe project health check.
type HealthCheck struct {
	Name                string
	Status              HealthCheckStatus
	Message             string
	Targets             []TargetDescriptor
	Profiles            []HealthProfile
	UnavailableProfiles []UnavailableProfile
	TargetFailure       TargetFailure
	HasTargetFailure    bool
}

// HealthProfile describes a configured profile without exposing managed values.
type HealthProfile struct {
	Name         string
	Protected    bool
	Available    bool
	TargetCount  int
	TotalTargets int
	Partial      bool
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
