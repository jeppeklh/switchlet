package app

import (
	"fmt"
	"sort"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
	"github.com/jeppeklh/switchlet/internal/profile"
)

// CompareStatus compares all configured profiles against current managed target values.
func (application Application) CompareStatus() (StatusComparison, error) {
	currentValues, err := application.readCurrentTargetValues(application.targets)
	if err != nil {
		return StatusComparison{}, fmt.Errorf("compare current status: %w", err)
	}

	comparison := StatusComparison{
		Status:      StatusComparisonUnmatched,
		TargetCount: len(application.targets),
		Complete:    true,
	}

	profileComparisons := make([]statusProfileComparison, 0, len(application.profiles))
	for _, configuredProfile := range application.profiles {
		profileComparison, err := application.compareProfileWithCurrentValues(configuredProfile, currentValues)
		if err != nil {
			return StatusComparison{}, fmt.Errorf("compare profile %q: %w", configuredProfile.Name, err)
		}

		profileComparisons = append(profileComparisons, profileComparison)
		if profileComparison.isExactCompleteMatch() {
			comparison.Matches = append(comparison.Matches, ProfileMatch{
				ProfileName: profileComparison.profileName,
				Protected:   profileComparison.protected,
			})
		}
		if profileComparison.isPartialMatch() {
			comparison.PartialMatches = append(comparison.PartialMatches, PartialProfileMatch{
				ProfileName:     profileComparison.profileName,
				Protected:       profileComparison.protected,
				MatchedTargets:  profileComparison.matchedTargets,
				IncludedTargets: profileComparison.includedTargets,
				OmittedTargets:  profileComparison.omittedTargets,
				TargetCount:     profileComparison.targetCount,
			})
		}
		if profileComparison.hasUnavailableValues() {
			comparison.UnavailableProfiles = append(comparison.UnavailableProfiles, UnavailableProfile{
				ProfileName: profileComparison.profileName,
				Protected:   profileComparison.protected,
				Reason:      profileComparison.unavailableReason,
				Values:      profileComparison.unavailableValues,
			})
		}
	}

	switch len(comparison.Matches) {
	case 0:
		comparison.ClosestProfiles = closestProfileMatches(profileComparisons)
	case 1:
		comparison.Status = StatusComparisonMatched
		comparison.CurrentProfile = comparison.Matches[0].ProfileName
		comparison.MatchedTargets = targetDescriptors(application.targets)
	default:
		comparison.Status = StatusComparisonAmbiguous
		comparison.MatchedTargets = targetDescriptors(application.targets)
	}

	return comparison, nil
}

// CurrentProfileNames returns profiles whose included targets match current target values.
func (comparison StatusComparison) CurrentProfileNames() []string {
	names := make([]string, 0, len(comparison.Matches)+len(comparison.PartialMatches))
	seen := make(map[string]struct{}, cap(names))

	appendName := func(name string) {
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}

		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, match := range comparison.Matches {
		appendName(match.ProfileName)
	}
	for _, match := range comparison.PartialMatches {
		appendName(match.ProfileName)
	}

	return names
}

// DiffProfileByName compares one selected profile against current managed target values.
func (application Application) DiffProfileByName(profileName string) (ProfileDiff, error) {
	configuredProfile, err := application.profileByName(profileName)
	if err != nil {
		return ProfileDiff{}, err
	}

	return application.diffConfiguredProfile(configuredProfile)
}

func (application Application) diffConfiguredProfile(configuredProfile config.Profile) (ProfileDiff, error) {
	resolvedProfile := profile.ResolveProfile(configuredProfile)
	targetsByName := application.targetsByName()
	includedTargets := make(map[string]struct{}, len(resolvedProfile.Values))
	includedValues := make([]profileDiffValue, 0, len(resolvedProfile.Values))
	targetsToRead := make([]config.Target, 0, len(resolvedProfile.Values))
	diff := ProfileDiff{
		ProfileName: resolvedProfile.Name,
		Protected:   resolvedProfile.Protected,
		Complete:    true,
	}

	for _, resolvedValue := range resolvedProfile.Values {
		target, ok := application.targetForValue(resolvedValue.Target, targetsByName)
		if !ok {
			return ProfileDiff{}, fmt.Errorf("target %q is not configured", resolvedValue.Target)
		}
		if _, exists := includedTargets[target.Name]; exists {
			return ProfileDiff{}, fmt.Errorf("profile %q includes target %q more than once", resolvedProfile.Name, target.Name)
		}
		includedTargets[target.Name] = struct{}{}

		includedValues = append(includedValues, profileDiffValue{
			resolvedValue: resolvedValue,
			target:        target,
			descriptor:    targetDescriptor(target),
		})
		targetsToRead = append(targetsToRead, target)
	}

	currentValues, err := editor.ReadTargetValues(targetsToRead)
	if err != nil {
		return ProfileDiff{}, fmt.Errorf("read current target value for profile %q: %w", resolvedProfile.Name, err)
	}

	for _, includedValue := range includedValues {
		resolvedValue := includedValue.resolvedValue
		target := includedValue.target
		descriptor := includedValue.descriptor
		if resolvedValue.ResolutionError != nil {
			diff.Complete = false
			diff.Unavailable = append(diff.Unavailable, unavailableValue(descriptor, resolvedValue))
			continue
		}

		currentValue, ok := currentValues[target.Name]
		if !ok {
			return ProfileDiff{}, fmt.Errorf("current value for target %q was not read", target.Name)
		}
		if resolvedValue.Value == currentValue {
			diff.AlreadyMatches = append(diff.AlreadyMatches, descriptor)
			continue
		}

		diff.WouldUpdate = append(diff.WouldUpdate, descriptor)
	}

	diff.OmittedTargets = application.omittedTargetDescriptors(includedTargets)
	return diff, nil
}

func (application Application) compareProfileWithCurrentValues(configuredProfile config.Profile, currentValues map[string]string) (statusProfileComparison, error) {
	resolvedProfile := profile.ResolveProfile(configuredProfile)
	targetsByName := application.targetsByName()
	comparison := statusProfileComparison{
		profileName:       resolvedProfile.Name,
		protected:         resolvedProfile.Protected,
		targetCount:       len(application.targets),
		unavailableReason: profileResolutionReason(resolvedProfile),
	}
	includedTargets := make(map[string]struct{}, len(resolvedProfile.Values))

	for _, resolvedValue := range resolvedProfile.Values {
		target, ok := application.targetForValue(resolvedValue.Target, targetsByName)
		if !ok {
			return statusProfileComparison{}, fmt.Errorf("target %q is not configured", resolvedValue.Target)
		}
		if _, exists := includedTargets[target.Name]; exists {
			return statusProfileComparison{}, fmt.Errorf("profile %q includes target %q more than once", resolvedProfile.Name, target.Name)
		}
		includedTargets[target.Name] = struct{}{}

		if resolvedValue.ResolutionError != nil {
			comparison.unavailableTargets++
			comparison.unavailableValues = append(comparison.unavailableValues, unavailableValue(targetDescriptor(target), resolvedValue))
			continue
		}

		currentValue, ok := currentValues[target.Name]
		if !ok {
			return statusProfileComparison{}, fmt.Errorf("current value for target %q was not read", target.Name)
		}
		if resolvedValue.Value == currentValue {
			comparison.matchedTargets++
		}
	}

	comparison.includedTargets = len(includedTargets)
	comparison.omittedTargets = comparison.targetCount - comparison.includedTargets
	return comparison, nil
}

func (application Application) readCurrentTargetValues(targets []config.Target) (map[string]string, error) {
	return editor.ReadTargetValues(targets)
}

type profileDiffValue struct {
	resolvedValue profile.ResolvedValue
	target        config.Target
	descriptor    TargetDescriptor
}

func (application Application) omittedTargetDescriptors(includedTargets map[string]struct{}) []TargetDescriptor {
	omittedTargets := make([]TargetDescriptor, 0)
	for _, target := range application.targets {
		if _, included := includedTargets[target.Name]; included {
			continue
		}

		omittedTargets = append(omittedTargets, targetDescriptor(target))
	}

	return omittedTargets
}

func targetDescriptors(targets []config.Target) []TargetDescriptor {
	descriptors := make([]TargetDescriptor, 0, len(targets))
	for _, target := range targets {
		descriptors = append(descriptors, targetDescriptor(target))
	}

	return descriptors
}

func unavailableValue(descriptor TargetDescriptor, resolvedValue profile.ResolvedValue) UnavailableValue {
	reason := ""
	if resolvedValue.ResolutionError != nil {
		reason = resolvedValue.ResolutionError.Error()
	}

	return UnavailableValue{
		TargetDescriptor:        descriptor,
		EnvironmentVariableName: resolvedValue.EnvironmentVariableName,
		Reason:                  reason,
	}
}

func profileResolutionReason(resolvedProfile profile.ResolvedProfile) string {
	if resolvedProfile.ResolutionError == nil {
		return ""
	}

	return resolvedProfile.ResolutionError.Error()
}

type statusProfileComparison struct {
	profileName        string
	protected          bool
	matchedTargets     int
	includedTargets    int
	unavailableTargets int
	omittedTargets     int
	targetCount        int
	unavailableReason  string
	unavailableValues  []UnavailableValue
}

func (comparison statusProfileComparison) isExactCompleteMatch() bool {
	return comparison.includedTargets == comparison.targetCount && comparison.allIncludedTargetsMatch()
}

func (comparison statusProfileComparison) isPartialMatch() bool {
	return comparison.includedTargets > 0 && comparison.omittedTargets > 0 && comparison.allIncludedTargetsMatch()
}

func (comparison statusProfileComparison) allIncludedTargetsMatch() bool {
	return comparison.unavailableTargets == 0 && comparison.matchedTargets == comparison.includedTargets
}

func (comparison statusProfileComparison) hasUnavailableValues() bool {
	return comparison.unavailableReason != "" || len(comparison.unavailableValues) > 0
}

func (comparison statusProfileComparison) comparableTargets() int {
	return comparison.includedTargets - comparison.unavailableTargets
}

func closestProfileMatches(comparisons []statusProfileComparison) []ClosestProfileMatch {
	candidates := make([]statusProfileComparison, 0, len(comparisons))
	for _, comparison := range comparisons {
		if comparison.isPartialMatch() || comparison.comparableTargets() == 0 {
			continue
		}

		candidates = append(candidates, comparison)
	}

	sort.SliceStable(candidates, func(leftIndex int, rightIndex int) bool {
		left := candidates[leftIndex]
		right := candidates[rightIndex]
		if left.matchedTargets != right.matchedTargets {
			return left.matchedTargets > right.matchedTargets
		}
		return left.comparableTargets() > right.comparableTargets()
	})

	matches := make([]ClosestProfileMatch, 0, len(candidates))
	for _, candidate := range candidates {
		matches = append(matches, ClosestProfileMatch{
			ProfileName:        candidate.profileName,
			Protected:          candidate.protected,
			MatchedTargets:     candidate.matchedTargets,
			IncludedTargets:    candidate.includedTargets,
			UnavailableTargets: candidate.unavailableTargets,
			TargetCount:        candidate.targetCount,
		})
	}

	return matches
}
