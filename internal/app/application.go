package app

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
	"github.com/jeppeklh/switchlet/internal/profile"
)

// Application coordinates profile resolution and target-file editing.
type Application struct{}

// New creates an application service.
func New() Application {
	return Application{}
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
