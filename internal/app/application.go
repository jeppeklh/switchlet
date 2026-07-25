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
	targets  []config.Target
	profiles []config.Profile
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
		targets:  configuredTargets,
		profiles: configuredProfiles,
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

	resolvedProfile := profile.ResolveProfile(configuredProfile)
	if !resolvedProfile.IsAvailable() {
		return Result{}, fmt.Errorf("resolve profile %q: %w", configuredProfile.Name, errors.Join(ErrProfileUnavailable, resolvedProfile.ResolutionError))
	}

	targetChanges, plannedChanges, err := application.targetChangesForResolvedProfile(resolvedProfile)
	if err != nil {
		return Result{}, fmt.Errorf("build target changes for profile %q: %w", configuredProfile.Name, err)
	}

	if options.DryRun {
		if err := editor.PreviewTargetChanges(targetChanges); err != nil {
			return Result{}, fmt.Errorf("dry-run apply profile %q: %w", configuredProfile.Name, err)
		}
	} else {
		if err := editor.ApplyTargetChanges(targetChanges); err != nil {
			return Result{}, fmt.Errorf("apply profile %q: %w", configuredProfile.Name, err)
		}
	}

	result := Result{
		ProfileName: resolvedProfile.Name,
		Protected:   configuredProfile.Protected,
		DryRun:      options.DryRun,
		Changes:     plannedChanges,
	}
	if len(plannedChanges) == 1 {
		result.TargetFile = plannedChanges[0].TargetFile
		result.TargetPath = plannedChanges[0].Selector
	}

	return result, nil
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

func (application Application) profileValueItems(resolvedValues []profile.ResolvedValue, targetsByName map[string]config.Target) []ProfileValueItem {
	items := make([]ProfileValueItem, 0, len(resolvedValues))
	for _, resolvedValue := range resolvedValues {
		item := ProfileValueItem{
			TargetName:              resolvedValue.Target,
			Source:                  profileSource(resolvedValue.Source),
			EnvironmentVariableName: resolvedValue.EnvironmentVariableName,
			MaskedValue:             resolvedValue.MaskedValue,
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
			if configuredTarget.Key != "" {
				configuredTarget.Type = config.TargetTypeDotenv
			} else {
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
	selectorName, selector := targetSelector(target)
	return PlannedChange{
		TargetName:   target.Name,
		TargetFile:   target.File,
		TargetType:   target.Type,
		SelectorName: selectorName,
		Selector:     selector,
	}
}

func targetSelector(target config.Target) (string, string) {
	if target.Type == config.TargetTypeDotenv {
		return "key", target.Key
	}

	return "jsonPath", target.JSONPath
}
