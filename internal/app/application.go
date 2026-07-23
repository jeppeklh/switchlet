package app

import (
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

// New creates an application service.
func New() Application {
	return Application{}
}

// WithConfig returns an application service configured with one target and its profiles.
func (application Application) WithConfig(target config.Target, profiles []config.Profile) Application {
	configuredProfiles := append([]config.Profile(nil), profiles...)

	application.target = target
	application.profiles = configuredProfiles

	return application
}

// Profiles returns the configured profiles with resolved availability and display status.
func (application Application) Profiles() []ProfileItem {
	items := make([]ProfileItem, 0, len(application.profiles))
	for _, configuredProfile := range application.profiles {
		resolvedProfile := profile.ResolveProfile(configuredProfile)

		item := ProfileItem{
			Name:                    resolvedProfile.Name,
			Protected:               resolvedProfile.Protected,
			Available:               resolvedProfile.IsAvailable(),
			Source:                  resolvedProfile.Source,
			EnvironmentVariableName: resolvedProfile.EnvironmentVariableName,
			MaskedValue:             resolvedProfile.MaskedValue,
		}
		if resolvedProfile.ResolutionError != nil {
			item.UnavailableReason = resolvedProfile.ResolutionError.Error()
		}

		items = append(items, item)
	}

	return items
}

// ApplyProfileByName resolves and applies one configured profile owned by the application.
func (application Application) ApplyProfileByName(profileName string) (Result, error) {
	configuredProfile, err := application.profileByName(profileName)
	if err != nil {
		return Result{}, err
	}

	return application.ApplyProfile(application.target, configuredProfile)
}

// ApplyProfile resolves one configured profile and applies it to the configured target.
func (application Application) ApplyProfile(target config.Target, configuredProfile config.Profile) (Result, error) {
	resolvedProfile := profile.ResolveProfile(configuredProfile)
	if !resolvedProfile.IsAvailable() {
		return Result{}, fmt.Errorf("resolve profile %q: %w", configuredProfile.Name, resolvedProfile.ResolutionError)
	}
	if resolvedProfile.Value == "" {
		return Result{}, fmt.Errorf("resolved profile %q is empty", configuredProfile.Name)
	}

	if err := editor.UpdateConnectionString(target.File, target.ConnectionName, resolvedProfile.Value); err != nil {
		return Result{}, fmt.Errorf("apply profile %q to target file %q: %w", configuredProfile.Name, target.File, err)
	}

	return Result{
		ProfileName:    resolvedProfile.Name,
		Protected:      resolvedProfile.Protected,
		Source:         resolvedProfile.Source,
		TargetPath:     target.File,
		ConnectionName: target.ConnectionName,
	}, nil
}

func (application Application) profileByName(profileName string) (config.Profile, error) {
	for _, configuredProfile := range application.profiles {
		if configuredProfile.Name == profileName {
			return configuredProfile, nil
		}
	}

	return config.Profile{}, fmt.Errorf("configured profile %q was not found", profileName)
}
