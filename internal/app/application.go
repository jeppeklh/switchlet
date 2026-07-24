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
	target   config.Target
	profiles []config.Profile
}

// New creates an application service configured with one target and its profiles.
func New(target config.Target, profiles []config.Profile) Application {
	configuredProfiles := append([]config.Profile(nil), profiles...)

	return Application{
		target:   target,
		profiles: configuredProfiles,
	}
}

// Profiles returns the configured profiles with resolved availability and display status.
func (application Application) Profiles() []ProfileItem {
	items := make([]ProfileItem, 0, len(application.profiles))
	for _, configuredProfile := range application.profiles {
		items = append(items, profileItem(configuredProfile))
	}

	return items
}

// InspectProfileByName returns one resolved profile in a display-safe form.
func (application Application) InspectProfileByName(profileName string) (ProfileItem, error) {
	configuredProfile, err := application.profileByName(profileName)
	if err != nil {
		return ProfileItem{}, err
	}

	return profileItem(configuredProfile), nil
}

// ValidateStartup verifies that the configured target is valid before the UI starts.
func (application Application) ValidateStartup() error {
	if err := editor.ValidateStringTarget(application.target.File, application.target.JSONPath); err != nil {
		return fmt.Errorf("validate configured target: %w", err)
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

	return application.applyConfiguredProfile(application.target, configuredProfile, options)
}

func (application Application) applyConfiguredProfile(target config.Target, configuredProfile config.Profile, options ApplyOptions) (Result, error) {
	if configuredProfile.Protected && !options.AllowProtected {
		return Result{}, fmt.Errorf("profile %q is protected: %w", configuredProfile.Name, ErrProtectedProfileRequiresApproval)
	}

	resolvedProfile := profile.ResolveProfile(configuredProfile)
	if !resolvedProfile.IsAvailable() {
		return Result{}, fmt.Errorf("resolve profile %q: %w", configuredProfile.Name, errors.Join(ErrProfileUnavailable, resolvedProfile.ResolutionError))
	}
	if resolvedProfile.Value == "" {
		return Result{}, fmt.Errorf("resolved profile %q is empty", configuredProfile.Name)
	}

	if options.DryRun {
		if err := editor.PreviewStringValueUpdate(target.File, target.JSONPath, resolvedProfile.Value); err != nil {
			return Result{}, fmt.Errorf("dry-run apply profile %q to target file %q: %w", configuredProfile.Name, target.File, err)
		}
	} else {
		if err := editor.UpdateStringValue(target.File, target.JSONPath, resolvedProfile.Value); err != nil {
			return Result{}, fmt.Errorf("apply profile %q to target file %q: %w", configuredProfile.Name, target.File, err)
		}
	}

	return Result{
		ProfileName: resolvedProfile.Name,
		TargetPath:  target.JSONPath,
		TargetFile:  target.File,
		Protected:   configuredProfile.Protected,
		DryRun:      options.DryRun,
	}, nil
}

func (application Application) profileByName(profileName string) (config.Profile, error) {
	for _, configuredProfile := range application.profiles {
		if configuredProfile.Name == profileName {
			return configuredProfile, nil
		}
	}

	return config.Profile{}, fmt.Errorf("configured profile %q was not found: %w", profileName, ErrProfileNotFound)
}

func profileItem(configuredProfile config.Profile) ProfileItem {
	resolvedProfile := profile.ResolveProfile(configuredProfile)

	item := ProfileItem{
		Name:                    resolvedProfile.Name,
		Protected:               resolvedProfile.Protected,
		Available:               resolvedProfile.IsAvailable(),
		Source:                  profileSource(resolvedProfile.Source),
		EnvironmentVariableName: resolvedProfile.EnvironmentVariableName,
		MaskedValue:             resolvedProfile.MaskedValue,
	}
	if resolvedProfile.ResolutionError != nil {
		item.UnavailableReason = resolvedProfile.ResolutionError.Error()
	}

	return item
}

func profileSource(source profile.ValueSource) ProfileSource {
	switch source {
	case profile.ValueSourceEnvironment:
		return ProfileSourceEnvironment
	case profile.ValueSourceLiteral:
		return ProfileSourceLiteral
	default:
		return ""
	}
}
