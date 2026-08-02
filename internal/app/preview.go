package app

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
	"github.com/jeppeklh/switchlet/internal/profile"
)

// ProfileContentsByName returns grouped selected-profile contents for review surfaces.
func (application Application) ProfileContentsByName(profileName string, options PreviewOptions) (ProfileContents, error) {
	configuredProfile, err := application.profileByName(profileName)
	if err != nil {
		return ProfileContents{}, err
	}

	return application.profileContentsForConfiguredProfile(configuredProfile, options)
}

// ManagedPatchPreviewByName returns selected-profile managed patch preview data.
func (application Application) ManagedPatchPreviewByName(profileName string, options PreviewOptions) (ManagedPatchPreview, error) {
	configuredProfile, err := application.profileByName(profileName)
	if err != nil {
		return ManagedPatchPreview{}, err
	}

	return application.managedPatchPreviewForConfiguredProfile(configuredProfile, options)
}

func (application Application) profileContentsForConfiguredProfile(configuredProfile config.Profile, options PreviewOptions) (ProfileContents, error) {
	resolvedProfile := profile.ResolveProfile(configuredProfile)
	targetsByName := application.targetsByName()
	valuesByTarget, includedTargets, err := application.resolvedValuesByTarget(resolvedProfile, targetsByName)
	if err != nil {
		return ProfileContents{}, err
	}

	contents := ProfileContents{
		ProfileName:        resolvedProfile.Name,
		Protected:          resolvedProfile.Protected,
		Available:          resolvedProfile.IsAvailable(),
		Source:             profileSource(resolvedProfile.Source),
		TargetCount:        len(includedTargets),
		TotalTargets:       len(application.targets),
		OmittedTargetCount: len(application.targets) - len(includedTargets),
		Partial:            len(application.targets) > 0 && len(includedTargets) < len(application.targets),
	}
	if resolvedProfile.ResolutionError != nil {
		contents.UnavailableReason = resolvedProfile.ResolutionError.Error()
	}

	groupIndexesByFile := make(map[string]int)
	for _, target := range application.targets {
		resolvedValue, included := valuesByTarget[target.Name]
		if !included {
			contents.OmittedTargets = append(contents.OmittedTargets, targetDescriptor(target))
			continue
		}

		targetContents := profileContentsTarget(targetDescriptor(target), resolvedValue, options)
		contents.Files = appendProfileContentsTarget(contents.Files, groupIndexesByFile, targetContents)
	}

	return contents, nil
}

func (application Application) managedPatchPreviewForConfiguredProfile(configuredProfile config.Profile, options PreviewOptions) (ManagedPatchPreview, error) {
	resolvedProfile := profile.ResolveProfile(configuredProfile)
	targetsByName := application.targetsByName()
	valuesByTarget, includedTargets, err := application.resolvedValuesByTarget(resolvedProfile, targetsByName)
	if err != nil {
		return ManagedPatchPreview{}, err
	}

	preview := ManagedPatchPreview{
		ProfileName:         resolvedProfile.Name,
		Protected:           resolvedProfile.Protected,
		Complete:            true,
		TargetCount:         len(application.targets),
		IncludedTargetCount: len(includedTargets),
		OmittedTargetCount:  len(application.targets) - len(includedTargets),
		Partial:             len(application.targets) > 0 && len(includedTargets) < len(application.targets),
	}

	availableChanges := make([]editor.TargetChange, 0, len(includedTargets))
	unavailableTargets := make([]config.Target, 0)
	unavailableHunks := make(map[string]ManagedPatchHunk)
	for _, target := range application.targets {
		resolvedValue, included := valuesByTarget[target.Name]
		if !included {
			continue
		}

		if resolvedValue.ResolutionError != nil {
			preview.Complete = false
			unavailableTargets = append(unavailableTargets, target)
			unavailableHunks[target.Name] = unavailableManagedPatchHunk(targetDescriptor(target), resolvedValue)
			continue
		}

		availableChanges = append(availableChanges, editor.TargetChange{
			Target: target,
			Value:  resolvedValue.Value,
		})
	}

	if len(unavailableTargets) > 0 {
		if _, err := editor.ReadTargetValues(unavailableTargets); err != nil {
			return ManagedPatchPreview{}, fmt.Errorf("read current target value for profile %q: %w", resolvedProfile.Name, err)
		}
	}

	availableHunks := make(map[string]ManagedPatchHunk)
	if len(availableChanges) > 0 {
		editorPreview, err := editor.PreviewManagedTargetChanges(availableChanges)
		if err != nil {
			return ManagedPatchPreview{}, fmt.Errorf("preview managed patch for profile %q: %w", resolvedProfile.Name, err)
		}

		availableHunks = managedPatchHunksByTargetName(editorPreview, valuesByTarget, options)
	}

	groupIndexesByFile := make(map[string]int)
	for _, target := range application.targets {
		_, included := valuesByTarget[target.Name]
		if !included {
			preview.OmittedTargets = append(preview.OmittedTargets, targetDescriptor(target))
			continue
		}

		hunk, ok := unavailableHunks[target.Name]
		if !ok {
			hunk, ok = availableHunks[target.Name]
		}
		if !ok {
			return ManagedPatchPreview{}, fmt.Errorf("managed patch preview for profile %q did not include target %q", resolvedProfile.Name, target.Name)
		}
		preview.Files = appendManagedPatchHunk(preview.Files, groupIndexesByFile, hunk)
	}

	return preview, nil
}

func (application Application) resolvedValuesByTarget(resolvedProfile profile.ResolvedProfile, targetsByName map[string]config.Target) (map[string]profile.ResolvedValue, map[string]struct{}, error) {
	valuesByTarget := make(map[string]profile.ResolvedValue, len(resolvedProfile.Values))
	includedTargets := make(map[string]struct{}, len(resolvedProfile.Values))

	for _, resolvedValue := range resolvedProfile.Values {
		target, ok := application.targetForValue(resolvedValue.Target, targetsByName)
		if !ok {
			return nil, nil, fmt.Errorf("target %q is not configured", resolvedValue.Target)
		}
		if _, exists := valuesByTarget[target.Name]; exists {
			return nil, nil, fmt.Errorf("profile %q includes target %q more than once", resolvedProfile.Name, target.Name)
		}

		valuesByTarget[target.Name] = resolvedValue
		includedTargets[target.Name] = struct{}{}
	}

	return valuesByTarget, includedTargets, nil
}

func profileContentsTarget(descriptor TargetDescriptor, resolvedValue profile.ResolvedValue, options PreviewOptions) ProfileContentsTarget {
	target := ProfileContentsTarget{
		TargetDescriptor:        descriptor,
		Source:                  profileSource(resolvedValue.Source),
		EnvironmentVariableName: resolvedValue.EnvironmentVariableName,
		Available:               resolvedValue.ResolutionError == nil,
	}
	if resolvedValue.ResolutionError != nil {
		target.UnavailableReason = resolvedValue.ResolutionError.Error()
		return target
	}
	if options.valuesShown() {
		target.Value = resolvedValue.Value
		target.ValueVisible = true
	}

	return target
}

func managedPatchHunksByTargetName(editorPreview editor.ManagedPreview, valuesByTarget map[string]profile.ResolvedValue, options PreviewOptions) map[string]ManagedPatchHunk {
	hunksByTarget := make(map[string]ManagedPatchHunk)
	for _, filePreview := range editorPreview.Files {
		for _, previewHunk := range filePreview.Hunks {
			resolvedValue := valuesByTarget[previewHunk.Target.Name]
			hunksByTarget[previewHunk.Target.Name] = availableManagedPatchHunk(previewHunk, resolvedValue, options)
		}
	}

	return hunksByTarget
}

func availableManagedPatchHunk(previewHunk editor.ManagedPreviewHunk, resolvedValue profile.ResolvedValue, options PreviewOptions) ManagedPatchHunk {
	descriptor := targetDescriptor(previewHunk.Target)
	hunk := ManagedPatchHunk{
		TargetDescriptor:        descriptor,
		Source:                  profileSource(resolvedValue.Source),
		EnvironmentVariableName: resolvedValue.EnvironmentVariableName,
		Available:               true,
	}

	if previewHunk.ProposedValue == previewHunk.OriginalValue {
		hunk.Status = ManagedPatchStatusAlreadyMatches
	} else {
		hunk.Status = ManagedPatchStatusWouldUpdate
	}

	if options.valuesShown() {
		hunk.CurrentValue = previewHunk.OriginalValue
		hunk.CurrentValueVisible = true
		hunk.ProfileValue = previewHunk.ProposedValue
		hunk.ProfileValueVisible = true
	}

	return hunk
}

func unavailableManagedPatchHunk(descriptor TargetDescriptor, resolvedValue profile.ResolvedValue) ManagedPatchHunk {
	reason := ""
	if resolvedValue.ResolutionError != nil {
		reason = resolvedValue.ResolutionError.Error()
	}

	return ManagedPatchHunk{
		TargetDescriptor:        descriptor,
		Status:                  ManagedPatchStatusUnavailable,
		Source:                  profileSource(resolvedValue.Source),
		EnvironmentVariableName: resolvedValue.EnvironmentVariableName,
		Available:               false,
		UnavailableReason:       reason,
	}
}

func appendProfileContentsTarget(groups []ProfileContentsFileGroup, groupIndexesByFile map[string]int, target ProfileContentsTarget) []ProfileContentsFileGroup {
	if groupIndex, exists := groupIndexesByFile[target.TargetFile]; exists {
		groups[groupIndex].Targets = append(groups[groupIndex].Targets, target)
		return groups
	}

	groupIndexesByFile[target.TargetFile] = len(groups)
	return append(groups, ProfileContentsFileGroup{
		TargetFile: target.TargetFile,
		Targets:    []ProfileContentsTarget{target},
	})
}

func appendManagedPatchHunk(groups []ManagedPatchFileGroup, groupIndexesByFile map[string]int, hunk ManagedPatchHunk) []ManagedPatchFileGroup {
	if groupIndex, exists := groupIndexesByFile[hunk.TargetFile]; exists {
		groups[groupIndex].Hunks = append(groups[groupIndex].Hunks, hunk)
		return groups
	}

	groupIndexesByFile[hunk.TargetFile] = len(groups)
	return append(groups, ManagedPatchFileGroup{
		TargetFile: hunk.TargetFile,
		Hunks:      []ManagedPatchHunk{hunk},
	})
}

func (options PreviewOptions) valuesShown() bool {
	return options.ValueVisibility == ValueVisibilityShown
}
