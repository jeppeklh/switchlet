package app

import (
	"errors"
	"fmt"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
	"github.com/jeppeklh/switchlet/internal/profile"
)

// Application coordinates profile resolution and target-file editing.
type Application struct {
	targets          []config.Target
	profiles         []config.Profile
	readTargetValues func([]config.Target) (map[string]string, error)
}

const defaultTargetName = "default"

// New creates an application service configured with one target and its profiles.
func New(target config.Target, profiles []config.Profile) Application {
	return NewWithTargets([]config.Target{target}, profiles)
}

// NewWithTargets creates an application service configured with one or more targets and profiles.
func NewWithTargets(targets []config.Target, profiles []config.Profile) Application {
	configuredTargets := normalizeTargets(targets)
	configuredProfiles := copyProfiles(profiles)

	return Application{
		targets:          configuredTargets,
		profiles:         configuredProfiles,
		readTargetValues: editor.ReadTargetValues,
	}
}

// TargetFile returns the configured target file path.
func (application Application) TargetFile() string {
	if len(application.targets) != 1 {
		return ""
	}

	return application.targets[0].File
}

// TargetPath returns the configured target JSON path.
func (application Application) TargetPath() string {
	if len(application.targets) != 1 {
		return ""
	}

	_, selector := targetSelector(application.targets[0])
	return selector
}

// Profiles returns the configured profiles with resolved availability and display status.
func (application Application) Profiles() []ProfileItem {
	items := make([]ProfileItem, 0, len(application.profiles))
	targetsByName := application.targetsByName()
	for _, configuredProfile := range application.profiles {
		items = append(items, application.profileItem(configuredProfile, targetsByName))
	}

	return items
}

// InspectProfileByName returns one resolved profile in a display-safe form.
func (application Application) InspectProfileByName(profileName string) (ProfileItem, error) {
	configuredProfile, err := application.profileByName(profileName)
	if err != nil {
		return ProfileItem{}, err
	}

	return application.profileItem(configuredProfile, application.targetsByName()), nil
}

// ValidateStartup verifies that the configured target is valid before the UI starts.
func (application Application) ValidateStartup() error {
	if err := editor.ValidateTargets(application.targets); err != nil {
		return fmt.Errorf("validate configured targets: %w", err)
	}

	return nil
}

// HealthChecks returns value-safe project health checks that require application
// coordination across target validation, profile availability, and current state.
func (application Application) HealthChecks() []HealthCheck {
	checks := make([]HealthCheck, 0, 3)

	startupErr := application.ValidateStartup()
	if startupErr != nil {
		checks = append(checks, failedHealthCheck("startup_target_validation", "configured target validation failed", startupErr))
	} else {
		checks = append(checks, HealthCheck{
			Name:    "startup_target_validation",
			Status:  HealthCheckOK,
			Message: fmt.Sprintf("validated %d configured target(s)", len(application.targets)),
			Targets: targetDescriptors(application.targets),
		})
	}

	profileCheck := application.profileAvailabilityHealthCheck()
	checks = append(checks, profileCheck)

	if startupErr != nil {
		checks = append(checks, HealthCheck{
			Name:    "current_state_comparison",
			Status:  HealthCheckSkipped,
			Message: "skipped because startup target validation failed",
		})
		return checks
	}

	status, err := application.CompareStatus()
	if err != nil {
		checks = append(checks, failedHealthCheck("current_state_comparison", "current-state comparison failed", err))
		return checks
	}

	checks = append(checks, HealthCheck{
		Name:    "current_state_comparison",
		Status:  HealthCheckOK,
		Message: currentStateHealthMessage(status),
	})

	return checks
}

// ApplyProfileByName resolves and applies one configured profile owned by the application.
func (application Application) ApplyProfileByName(profileName string) (Result, error) {
	return application.ApplyProfileByNameWithOptions(profileName, ApplyOptions{AllowProtected: true})
}

// ApplyProfileByNameWithOptions resolves and applies one configured profile
// using the requested non-interactive behavior.
func (application Application) ApplyProfileByNameWithOptions(profileName string, options ApplyOptions) (Result, error) {
	configuredProfile, err := application.profileByName(profileName)
	if err != nil {
		return Result{}, err
	}

	return application.applyConfiguredProfile(configuredProfile, options)
}

func (application Application) applyConfiguredProfile(configuredProfile config.Profile, options ApplyOptions) (Result, error) {
	if configuredProfile.Protected && !options.AllowProtected {
		return Result{}, fmt.Errorf("profile %q is protected: %w", configuredProfile.Name, ErrProtectedProfileRequiresApproval)
	}
	if options.DryRun {
		return application.dryRunConfiguredProfile(configuredProfile)
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)
	if !resolvedProfile.IsAvailable() {
		return Result{}, fmt.Errorf("resolve profile %q: %w", configuredProfile.Name, errors.Join(ErrProfileUnavailable, resolvedProfile.ResolutionError))
	}

	targetChanges, plannedChanges, err := application.targetChangesForResolvedProfile(resolvedProfile)
	if err != nil {
		return Result{}, fmt.Errorf("build target changes for profile %q: %w", configuredProfile.Name, err)
	}

	if err := editor.ApplyTargetChanges(targetChanges); err != nil {
		return Result{}, fmt.Errorf("apply profile %q: %w", configuredProfile.Name, err)
	}
	if err := application.verifyAppliedTargetChanges(resolvedProfile.Name, targetChanges, plannedChanges); err != nil {
		return Result{}, fmt.Errorf("verify applied profile %q: %w", configuredProfile.Name, err)
	}

	result := Result{
		ProfileName: resolvedProfile.Name,
		Protected:   configuredProfile.Protected,
		Changes:     plannedChanges,
	}
	if len(plannedChanges) == 1 {
		result.TargetFile = plannedChanges[0].TargetFile
		result.TargetPath = plannedChanges[0].Selector
	}

	return result, nil
}

func (application Application) dryRunConfiguredProfile(configuredProfile config.Profile) (Result, error) {
	preview, err := application.managedPatchPreviewForConfiguredProfile(configuredProfile, PreviewOptions{ValueVisibility: ValueVisibilityHidden})
	if err != nil {
		return Result{}, fmt.Errorf("dry-run apply profile %q: %w", configuredProfile.Name, err)
	}

	plannedChanges := managedPatchPreviewPlannedChanges(preview)
	result := Result{
		ProfileName:   preview.ProfileName,
		Protected:     preview.Protected,
		DryRun:        true,
		Changes:       plannedChanges,
		DryRunPreview: &preview,
	}
	if len(plannedChanges) == 1 {
		result.TargetFile = plannedChanges[0].TargetFile
		result.TargetPath = plannedChanges[0].Selector
	}

	return result, nil
}

func managedPatchPreviewPlannedChanges(preview ManagedPatchPreview) []PlannedChange {
	changes := make([]PlannedChange, 0, preview.IncludedTargetCount)
	for _, fileGroup := range preview.Files {
		for _, hunk := range fileGroup.Hunks {
			changes = append(changes, hunk.TargetDescriptor)
		}
	}

	return changes
}

func (application Application) profileByName(profileName string) (config.Profile, error) {
	for _, configuredProfile := range application.profiles {
		if configuredProfile.Name == profileName {
			return configuredProfile, nil
		}
	}

	return config.Profile{}, fmt.Errorf("configured profile %q was not found: %w", profileName, ErrProfileNotFound)
}

func (application Application) targetChangesForResolvedProfile(resolvedProfile profile.ResolvedProfile) ([]editor.TargetChange, []PlannedChange, error) {
	targetsByName := application.targetsByName()
	targetChanges := make([]editor.TargetChange, 0, len(resolvedProfile.Values))
	plannedChanges := make([]PlannedChange, 0, len(resolvedProfile.Values))

	for _, resolvedValue := range resolvedProfile.Values {
		target, ok := application.targetForValue(resolvedValue.Target, targetsByName)
		if !ok {
			return nil, nil, fmt.Errorf("target %q is not configured", resolvedValue.Target)
		}

		targetChanges = append(targetChanges, editor.TargetChange{Target: target, Value: resolvedValue.Value})
		plannedChanges = append(plannedChanges, plannedChange(target))
	}

	return targetChanges, plannedChanges, nil
}

func (application Application) verifyAppliedTargetChanges(profileName string, targetChanges []editor.TargetChange, plannedChanges []PlannedChange) error {
	currentValues, err := application.readCurrentTargetValues(targetsFromChanges(targetChanges))
	if err != nil {
		return PostApplyVerificationError{
			ProfileName: profileName,
			Failures:    verificationReadFailures(err),
			Err:         err,
		}
	}

	failures := make([]PostApplyVerificationFailure, 0)
	for index, targetChange := range targetChanges {
		currentValue, ok := currentValues[targetChange.Target.Name]
		if !ok {
			failures = append(failures, PostApplyVerificationFailure{
				TargetDescriptor: plannedChanges[index],
				Reason:           fmt.Sprintf("current value for target %q was not read", targetChange.Target.Name),
			})
			continue
		}
		if currentValue == targetChange.Value {
			continue
		}

		failures = append(failures, PostApplyVerificationFailure{
			TargetDescriptor: plannedChanges[index],
			Reason:           "current value does not match selected profile value",
		})
	}
	if len(failures) == 0 {
		return nil
	}

	return PostApplyVerificationError{ProfileName: profileName, Failures: failures}
}

func targetsFromChanges(changes []editor.TargetChange) []config.Target {
	targets := make([]config.Target, 0, len(changes))
	for _, change := range changes {
		targets = append(targets, change.Target)
	}

	return targets
}

func verificationReadFailures(err error) []PostApplyVerificationFailure {
	if targetFailure, ok := TargetFailureFromError(err); ok {
		return []PostApplyVerificationFailure{{
			TargetDescriptor: TargetDescriptor{
				TargetName:   targetFailure.TargetName,
				TargetFile:   targetFailure.TargetFile,
				TargetType:   targetFailure.TargetType,
				SelectorName: targetFailure.SelectorName,
				Selector:     targetFailure.Selector,
			},
			Reason: targetFailure.Reason,
		}}
	}

	reason := "post-apply verification read failed"
	if err != nil {
		reason = err.Error()
	}
	return []PostApplyVerificationFailure{{Reason: reason}}
}

func (application Application) profileItem(configuredProfile config.Profile, targetsByName map[string]config.Target) ProfileItem {
	resolvedProfile := profile.ResolveProfile(configuredProfile)
	valueItems := application.profileValueItems(resolvedProfile.Values, targetsByName)

	item := ProfileItem{
		Name:         resolvedProfile.Name,
		Protected:    resolvedProfile.Protected,
		Available:    resolvedProfile.IsAvailable(),
		Source:       profileSource(resolvedProfile.Source),
		Values:       valueItems,
		TargetCount:  len(valueItems),
		TotalTargets: len(application.targets),
		Partial:      len(application.targets) > 0 && len(valueItems) < len(application.targets),
	}
	if len(valueItems) == 1 {
		item.EnvironmentVariableName = valueItems[0].EnvironmentVariableName
		item.MaskedValue = valueItems[0].MaskedValue
	}
	if resolvedProfile.ResolutionError != nil {
		item.UnavailableReason = resolvedProfile.ResolutionError.Error()
	}

	return item
}

func (application Application) profileAvailabilityHealthCheck() HealthCheck {
	profileItems := application.Profiles()
	profiles := make([]HealthProfile, 0, len(profileItems))
	unavailableProfiles := make([]UnavailableProfile, 0)

	for _, profileItem := range profileItems {
		profiles = append(profiles, HealthProfile{
			Name:         profileItem.Name,
			Protected:    profileItem.Protected,
			Available:    profileItem.Available,
			TargetCount:  profileItem.TargetCount,
			TotalTargets: profileItem.TotalTargets,
			Partial:      profileItem.Partial,
		})

		if profileItem.Available {
			continue
		}

		unavailableProfiles = append(unavailableProfiles, unavailableProfileFromProfileItem(profileItem))
	}

	if len(unavailableProfiles) == 0 {
		return HealthCheck{
			Name:     "profile_availability",
			Status:   HealthCheckOK,
			Message:  fmt.Sprintf("all %d configured profile(s) are available", len(profileItems)),
			Profiles: profiles,
		}
	}

	return HealthCheck{
		Name:                "profile_availability",
		Status:              HealthCheckWarning,
		Message:             fmt.Sprintf("%d configured profile(s) are unavailable in the current environment", len(unavailableProfiles)),
		Profiles:            profiles,
		UnavailableProfiles: unavailableProfiles,
	}
}

func unavailableProfileFromProfileItem(profileItem ProfileItem) UnavailableProfile {
	unavailableValues := make([]UnavailableValue, 0)
	for _, valueItem := range profileItem.Values {
		if valueItem.Available {
			continue
		}

		unavailableValues = append(unavailableValues, UnavailableValue{
			TargetDescriptor: TargetDescriptor{
				TargetName:   valueItem.TargetName,
				TargetFile:   valueItem.TargetFile,
				TargetType:   valueItem.TargetType,
				SelectorName: valueItem.SelectorName,
				Selector:     valueItem.Selector,
			},
			EnvironmentVariableName: valueItem.EnvironmentVariableName,
			Reason:                  valueItem.UnavailableReason,
		})
	}

	return UnavailableProfile{
		ProfileName: profileItem.Name,
		Protected:   profileItem.Protected,
		Reason:      profileItem.UnavailableReason,
		Values:      unavailableValues,
	}
}

func failedHealthCheck(name string, message string, err error) HealthCheck {
	check := HealthCheck{
		Name:    name,
		Status:  HealthCheckFailed,
		Message: message,
	}
	if targetFailure, ok := TargetFailureFromError(err); ok {
		check.TargetFailure = targetFailure
		check.HasTargetFailure = true
		return check
	}
	if err != nil {
		check.Message = fmt.Sprintf("%s: %v", message, err)
	}

	return check
}

func currentStateHealthMessage(status StatusComparison) string {
	switch status.Status {
	case StatusComparisonMatched:
		return fmt.Sprintf("current managed values match profile %q", status.CurrentProfile)
	case StatusComparisonAmbiguous:
		return "current managed values match multiple complete profiles"
	default:
		return "current managed values do not exactly match a complete profile"
	}
}

func (application Application) profileValueItems(resolvedValues []profile.ResolvedValue, targetsByName map[string]config.Target) []ProfileValueItem {
	items := make([]ProfileValueItem, 0, len(resolvedValues))
	for _, resolvedValue := range resolvedValues {
		item := ProfileValueItem{
			TargetName:              resolvedValue.Target,
			Source:                  profileSource(resolvedValue.Source),
			EnvironmentVariableName: resolvedValue.EnvironmentVariableName,
			Available:               resolvedValue.ResolutionError == nil,
		}
		if target, ok := application.targetForValue(resolvedValue.Target, targetsByName); ok {
			selectorName, selector := targetSelector(target)
			item.TargetName = target.Name
			item.TargetFile = target.File
			item.TargetType = target.Type
			item.SelectorName = selectorName
			item.Selector = selector
		}
		if resolvedValue.ResolutionError == nil {
			item.MaskedValue = profile.MaskManagedValue(resolvedValue.Value, profile.ManagedValueMaskContext{
				TargetName:              item.TargetName,
				Selector:                item.Selector,
				EnvironmentVariableName: item.EnvironmentVariableName,
			})
		}
		if resolvedValue.ResolutionError != nil {
			item.UnavailableReason = resolvedValue.ResolutionError.Error()
		}

		items = append(items, item)
	}

	return items
}

func profileSource(source profile.ValueSource) ProfileSource {
	switch source {
	case profile.ValueSourceEnvironment:
		return ProfileSourceEnvironment
	case profile.ValueSourceLiteral:
		return ProfileSourceLiteral
	case profile.ValueSourceMixed:
		return ProfileSourceMixed
	default:
		return ""
	}
}

func normalizeTargets(targets []config.Target) []config.Target {
	configuredTargets := make([]config.Target, 0, len(targets))
	for _, target := range targets {
		configuredTarget := target
		if configuredTarget.Name == "" {
			configuredTarget.Name = defaultTargetName
		}
		if configuredTarget.Type == "" {
			switch {
			case configuredTarget.Key != "":
				configuredTarget.Type = config.TargetTypeDotenv
			case configuredTarget.YAMLPath != "":
				configuredTarget.Type = config.TargetTypeYAML
			case configuredTarget.TOMLPath != "":
				configuredTarget.Type = config.TargetTypeTOML
			default:
				configuredTarget.Type = config.TargetTypeJSON
			}
		}

		configuredTargets = append(configuredTargets, configuredTarget)
	}

	return configuredTargets
}

func copyProfiles(profiles []config.Profile) []config.Profile {
	configuredProfiles := make([]config.Profile, 0, len(profiles))
	for _, configuredProfile := range profiles {
		configuredProfiles = append(configuredProfiles, copyProfile(configuredProfile))
	}

	return configuredProfiles
}

func copyProfile(configuredProfile config.Profile) config.Profile {
	profileCopy := configuredProfile
	profileCopy.Values = make([]config.ProfileValue, 0, len(configuredProfile.Values))
	for _, configuredValue := range configuredProfile.Values {
		profileCopy.Values = append(profileCopy.Values, copyProfileValue(configuredValue))
	}
	if configuredProfile.Value != nil {
		literalValue := *configuredProfile.Value
		profileCopy.Value = &literalValue
	}
	if configuredProfile.ValueFromEnv != nil {
		environmentVariableName := *configuredProfile.ValueFromEnv
		profileCopy.ValueFromEnv = &environmentVariableName
	}

	return profileCopy
}

func copyProfileValue(configuredValue config.ProfileValue) config.ProfileValue {
	valueCopy := configuredValue
	if configuredValue.Value != nil {
		literalValue := *configuredValue.Value
		valueCopy.Value = &literalValue
	}
	if configuredValue.ValueFromEnv != nil {
		environmentVariableName := *configuredValue.ValueFromEnv
		valueCopy.ValueFromEnv = &environmentVariableName
	}

	return valueCopy
}

func (application Application) targetsByName() map[string]config.Target {
	targetsByName := make(map[string]config.Target, len(application.targets))
	for _, target := range application.targets {
		targetsByName[target.Name] = target
	}

	return targetsByName
}

func (application Application) targetForValue(targetName string, targetsByName map[string]config.Target) (config.Target, bool) {
	if target, ok := targetsByName[targetName]; ok {
		return target, true
	}
	if targetName == "" && len(application.targets) == 1 {
		return application.targets[0], true
	}

	return config.Target{}, false
}

func plannedChange(target config.Target) PlannedChange {
	return targetDescriptor(target)
}

func targetDescriptor(target config.Target) TargetDescriptor {
	selectorName, selector := targetSelector(target)
	return TargetDescriptor{
		TargetName:   target.Name,
		TargetFile:   target.File,
		TargetType:   target.Type,
		SelectorName: selectorName,
		Selector:     selector,
	}
}

func targetSelector(target config.Target) (string, string) {
	switch target.Type {
	case config.TargetTypeDotenv:
		return "key", target.Key
	case config.TargetTypeYAML:
		return "yamlPath", target.YAMLPath
	case config.TargetTypeTOML:
		return "tomlPath", target.TOMLPath
	default:
		return "jsonPath", target.JSONPath
	}
}
