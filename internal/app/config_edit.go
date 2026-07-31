package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

var (
	// ErrManagedValueNotFound indicates that a requested managed value does not exist.
	ErrManagedValueNotFound = errors.New("managed value not found")
	// ErrConfigEditNoChanges indicates that a save was requested for an unchanged draft.
	ErrConfigEditNoChanges = errors.New("no pending configuration changes")
)

// ConfigEditDependencies lets tests replace config-edit persistence and validation effects.
type ConfigEditDependencies struct {
	LoadSnapshot       func(string) (config.ConfigSnapshot, error)
	ValidateTargets    func([]config.Target) error
	PrepareReplacement func(config.ConfigSnapshot, []config.Target, []config.Profile) (config.PreparedReplacement, error)
}

// ConfigEditDocument represents an editable .switchlet.yaml draft.
type ConfigEditDocument struct {
	ProjectRoot            string
	ConfigPath             string
	OriginalVersion        int
	ConvertsToVersionThree bool
	Targets                []config.Target
	Profiles               []config.Profile

	originalTargets  []config.Target
	originalProfiles []config.Profile
	snapshot         config.ConfigSnapshot
}

// ConfigEditChangeKind identifies one pending configuration-edit change.
type ConfigEditChangeKind string

const (
	// ConfigEditChangeCompatibilityConversion warns that a compatibility config will be saved as Version 3.
	ConfigEditChangeCompatibilityConversion ConfigEditChangeKind = "compatibility_conversion"
	// ConfigEditChangeFormattingNormalization warns about normalized .switchlet.yaml output.
	ConfigEditChangeFormattingNormalization ConfigEditChangeKind = "formatting_normalization"
	// ConfigEditChangeProfileAdded reports an added profile.
	ConfigEditChangeProfileAdded ConfigEditChangeKind = "profile_added"
	// ConfigEditChangeProfileUpdated reports an edited profile.
	ConfigEditChangeProfileUpdated ConfigEditChangeKind = "profile_updated"
	// ConfigEditChangeProfileRenamed reports a profile rename inferred from the draft.
	ConfigEditChangeProfileRenamed ConfigEditChangeKind = "profile_renamed"
	// ConfigEditChangeProfileRemoved reports a removed profile.
	ConfigEditChangeProfileRemoved ConfigEditChangeKind = "profile_removed"
	// ConfigEditChangeManagedValueAdded reports an added managed value.
	ConfigEditChangeManagedValueAdded ConfigEditChangeKind = "managed_value_added"
	// ConfigEditChangeManagedValueUpdated reports an edited managed value location.
	ConfigEditChangeManagedValueUpdated ConfigEditChangeKind = "managed_value_updated"
	// ConfigEditChangeManagedValueRenamed reports a managed value rename and reference update.
	ConfigEditChangeManagedValueRenamed ConfigEditChangeKind = "managed_value_renamed"
	// ConfigEditChangeManagedValueRemoved reports a removed managed value and affected profile values.
	ConfigEditChangeManagedValueRemoved ConfigEditChangeKind = "managed_value_removed"
)

// ConfigEditChange describes one pending draft change without exposing profile values.
type ConfigEditChange struct {
	Kind    ConfigEditChangeKind
	Summary string
	Detail  []string
	Warning bool
}

// ConfigEditRenameManagedValueResult describes profile references updated by a rename.
type ConfigEditRenameManagedValueResult struct {
	UpdatedProfiles []string
}

// ConfigEditRemoveManagedValueResult describes profile values affected by managed-value removal.
type ConfigEditRemoveManagedValueResult struct {
	AffectedProfiles []string
	InvalidProfiles  []string
}

// ConfigEditProfileDraft is the app-owned editable form of one profile.
// It can include literal values because active profile value entry is an
// explicit config-editing surface; overview and review summaries still hide
// literal values by default.
type ConfigEditProfileDraft struct {
	Name      string
	Protected bool
	Values    []ConfigEditProfileValueDraft
}

// ConfigEditProfileValueDraft is one editable managed-value entry for a profile.
type ConfigEditProfileValueDraft struct {
	TargetDescriptor
	Included                bool
	Source                  ProfileSource
	LiteralValue            string
	EnvironmentVariableName string
}

// ConfigEditPreparedSave is a validated config replacement that has not yet been committed.
type ConfigEditPreparedSave struct {
	ConfigPath string
	Changes    []ConfigEditChange

	replacement config.PreparedReplacement
}

// Commit writes the prepared .switchlet.yaml replacement.
func (save ConfigEditPreparedSave) Commit() error {
	return save.replacement.Commit()
}

// ConfigEditWorkflow coordinates pure config edit operations and save preparation.
type ConfigEditWorkflow struct {
	dependencies ConfigEditDependencies
}

// NewConfigEditWorkflow creates a config edit workflow using supplied dependencies and defaults.
func NewConfigEditWorkflow(dependencies ConfigEditDependencies) ConfigEditWorkflow {
	return ConfigEditWorkflow{dependencies: dependencies}
}

// DefaultConfigEditWorkflow creates the production config edit workflow.
func DefaultConfigEditWorkflow() ConfigEditWorkflow {
	return NewConfigEditWorkflow(ConfigEditDependencies{})
}

// LoadDocument loads an existing configuration as an editable Version 3 draft.
func (workflow ConfigEditWorkflow) LoadDocument(configPath string) (ConfigEditDocument, error) {
	snapshot, err := workflow.loadSnapshot(configPath)
	if err != nil {
		return ConfigEditDocument{}, err
	}

	document := ConfigEditDocument{
		ProjectRoot:            snapshot.ProjectRoot,
		ConfigPath:             snapshot.ConfigPath,
		OriginalVersion:        snapshot.OriginalVersion,
		ConvertsToVersionThree: snapshot.OriginalVersion != 3,
		Targets:                copyConfigEditTargets(snapshot.Config.Targets),
		Profiles:               copyConfigEditProfiles(snapshot.Config.Profiles),
		snapshot:               snapshot,
	}
	document.originalTargets = copyConfigEditTargets(document.Targets)
	document.originalProfiles = copyConfigEditProfiles(document.Profiles)

	return document, nil
}

// NewProfileDraft creates a profile draft for the document's configured managed values.
func (workflow ConfigEditWorkflow) NewProfileDraft(document ConfigEditDocument) ConfigEditProfileDraft {
	return newProfileDraft(document.Targets)
}

// ProfileDraft returns an editable profile draft for an existing profile.
func (workflow ConfigEditWorkflow) ProfileDraft(document ConfigEditDocument, profileName string) (ConfigEditProfileDraft, error) {
	profileIndex := profileIndexByName(document.Profiles, profileName)
	if profileIndex < 0 {
		return ConfigEditProfileDraft{}, fmt.Errorf("profile %q was not found: %w", profileName, ErrProfileNotFound)
	}

	return profileDraftFromProfile(document.Targets, document.Profiles[profileIndex]), nil
}

// AddProfileDraft appends an app-owned profile draft after schema validation.
func (workflow ConfigEditWorkflow) AddProfileDraft(document ConfigEditDocument, draft ConfigEditProfileDraft) (ConfigEditDocument, error) {
	profile, err := profileFromDraft(draft)
	if err != nil {
		return ConfigEditDocument{}, err
	}

	return workflow.AddProfile(document, profile)
}

// UpdateProfileDraft replaces an existing profile with an app-owned draft after schema validation.
func (workflow ConfigEditWorkflow) UpdateProfileDraft(document ConfigEditDocument, existingName string, draft ConfigEditProfileDraft) (ConfigEditDocument, error) {
	profile, err := profileFromDraft(draft)
	if err != nil {
		return ConfigEditDocument{}, err
	}

	return workflow.UpdateProfile(document, existingName, profile)
}

// AddProfile appends a profile to the draft after schema validation.
func (workflow ConfigEditWorkflow) AddProfile(document ConfigEditDocument, profile config.Profile) (ConfigEditDocument, error) {
	draft := document.clone()
	draft.Profiles = append(draft.Profiles, copyConfigEditProfile(profile))

	return workflow.normalizeDraft(draft, fmt.Sprintf("add profile %q", profile.Name))
}

// UpdateProfile replaces one profile in the draft after schema validation.
func (workflow ConfigEditWorkflow) UpdateProfile(document ConfigEditDocument, existingName string, updatedProfile config.Profile) (ConfigEditDocument, error) {
	draft := document.clone()
	profileIndex := profileIndexByName(draft.Profiles, existingName)
	if profileIndex < 0 {
		return ConfigEditDocument{}, fmt.Errorf("profile %q was not found: %w", existingName, ErrProfileNotFound)
	}

	draft.Profiles[profileIndex] = copyConfigEditProfile(updatedProfile)

	return workflow.normalizeDraft(draft, fmt.Sprintf("update profile %q", existingName))
}

// RemoveProfile removes one profile from the draft. The resulting draft may be
// temporarily invalid until another profile is added.
func (workflow ConfigEditWorkflow) RemoveProfile(document ConfigEditDocument, profileName string) (ConfigEditDocument, error) {
	draft := document.clone()
	profileIndex := profileIndexByName(draft.Profiles, profileName)
	if profileIndex < 0 {
		return ConfigEditDocument{}, fmt.Errorf("profile %q was not found: %w", profileName, ErrProfileNotFound)
	}

	draft.Profiles = append(draft.Profiles[:profileIndex], draft.Profiles[profileIndex+1:]...)

	return workflow.normalizeIfValid(draft), nil
}

// AddManagedValue appends one managed value target to the draft after schema validation.
func (workflow ConfigEditWorkflow) AddManagedValue(document ConfigEditDocument, target config.Target) (ConfigEditDocument, error) {
	draft := document.clone()
	draft.Targets = append(draft.Targets, target)

	return workflow.normalizeDraft(draft, fmt.Sprintf("add managed value %q", target.Name))
}

// RenameManagedValue renames a managed value and updates every profile reference in the draft.
func (workflow ConfigEditWorkflow) RenameManagedValue(document ConfigEditDocument, existingName string, newName string) (ConfigEditDocument, ConfigEditRenameManagedValueResult, error) {
	draft := document.clone()
	targetIndex := targetIndexByName(draft.Targets, existingName)
	if targetIndex < 0 {
		return ConfigEditDocument{}, ConfigEditRenameManagedValueResult{}, fmt.Errorf("managed value %q was not found: %w", existingName, ErrManagedValueNotFound)
	}

	draft.Targets[targetIndex].Name = newName
	result := ConfigEditRenameManagedValueResult{}
	for profileIndex := range draft.Profiles {
		updatedProfile := false
		for valueIndex := range draft.Profiles[profileIndex].Values {
			if draft.Profiles[profileIndex].Values[valueIndex].Target != existingName {
				continue
			}

			draft.Profiles[profileIndex].Values[valueIndex].Target = newName
			updatedProfile = true
		}
		if updatedProfile {
			result.UpdatedProfiles = append(result.UpdatedProfiles, draft.Profiles[profileIndex].Name)
		}
	}

	normalizedDraft, err := workflow.normalizeDraft(draft, fmt.Sprintf("rename managed value %q", existingName))
	if err != nil {
		return ConfigEditDocument{}, ConfigEditRenameManagedValueResult{}, err
	}

	return normalizedDraft, result, nil
}

// UpdateManagedValueLocation updates one managed value's file, type, and selector while preserving its name.
func (workflow ConfigEditWorkflow) UpdateManagedValueLocation(document ConfigEditDocument, targetName string, updatedTarget config.Target) (ConfigEditDocument, error) {
	draft := document.clone()
	targetIndex := targetIndexByName(draft.Targets, targetName)
	if targetIndex < 0 {
		return ConfigEditDocument{}, fmt.Errorf("managed value %q was not found: %w", targetName, ErrManagedValueNotFound)
	}

	updatedTarget.Name = targetName
	draft.Targets[targetIndex] = updatedTarget

	return workflow.normalizeDraft(draft, fmt.Sprintf("update managed value %q", targetName))
}

// RemoveManagedValue removes one managed value and every corresponding profile value from the draft.
func (workflow ConfigEditWorkflow) RemoveManagedValue(document ConfigEditDocument, targetName string) (ConfigEditDocument, ConfigEditRemoveManagedValueResult, error) {
	draft := document.clone()
	targetIndex := targetIndexByName(draft.Targets, targetName)
	if targetIndex < 0 {
		return ConfigEditDocument{}, ConfigEditRemoveManagedValueResult{}, fmt.Errorf("managed value %q was not found: %w", targetName, ErrManagedValueNotFound)
	}

	draft.Targets = append(draft.Targets[:targetIndex], draft.Targets[targetIndex+1:]...)
	result := ConfigEditRemoveManagedValueResult{}
	for profileIndex := range draft.Profiles {
		values := draft.Profiles[profileIndex].Values[:0]
		removedValue := false
		for _, value := range draft.Profiles[profileIndex].Values {
			if value.Target == targetName {
				removedValue = true
				continue
			}

			values = append(values, value)
		}
		draft.Profiles[profileIndex].Values = values
		if removedValue {
			result.AffectedProfiles = append(result.AffectedProfiles, draft.Profiles[profileIndex].Name)
			if len(values) == 0 {
				result.InvalidProfiles = append(result.InvalidProfiles, draft.Profiles[profileIndex].Name)
			}
		}
	}

	return workflow.normalizeIfValid(draft), result, nil
}

// ValidateDraft validates the draft as the Version 3 configuration that would be saved.
func (workflow ConfigEditWorkflow) ValidateDraft(document ConfigEditDocument) error {
	_, err := config.ValidateVersionThreeDraft(document.ProjectRoot, document.Targets, document.Profiles)
	return err
}

// HasChanges reports whether saving the document would modify .switchlet.yaml.
func (workflow ConfigEditWorkflow) HasChanges(document ConfigEditDocument) bool {
	return len(workflow.SummarizeChanges(document)) > 0
}

// IsSaveable reports whether the draft has changes and passes schema validation.
func (workflow ConfigEditWorkflow) IsSaveable(document ConfigEditDocument) bool {
	return workflow.HasChanges(document) && workflow.ValidateDraft(document) == nil
}

// PrepareSave validates the draft, validates configured target files, and prepares a conflict-aware replacement.
func (workflow ConfigEditWorkflow) PrepareSave(document ConfigEditDocument) (ConfigEditPreparedSave, error) {
	changes := workflow.SummarizeChanges(document)
	if len(changes) == 0 {
		return ConfigEditPreparedSave{}, ErrConfigEditNoChanges
	}

	normalizedDraft, err := workflow.normalizeDraft(document, "validate configuration draft")
	if err != nil {
		return ConfigEditPreparedSave{}, err
	}
	if err := workflow.validateTargets(normalizedDraft.Targets); err != nil {
		return ConfigEditPreparedSave{}, fmt.Errorf("validate configured targets: %w", err)
	}

	replacement, err := workflow.prepareReplacement(normalizedDraft.snapshot, normalizedDraft.Targets, normalizedDraft.Profiles)
	if err != nil {
		return ConfigEditPreparedSave{}, err
	}

	return ConfigEditPreparedSave{
		ConfigPath:  replacement.ConfigPath(),
		Changes:     changes,
		replacement: replacement,
	}, nil
}

// SummarizeChanges returns a value-safe review summary for the pending draft changes.
func (workflow ConfigEditWorkflow) SummarizeChanges(document ConfigEditDocument) []ConfigEditChange {
	modelChanges := append(workflow.summarizeManagedValueChanges(document), workflow.summarizeProfileChanges(document)...)
	if !document.ConvertsToVersionThree && len(modelChanges) == 0 {
		return nil
	}

	changes := make([]ConfigEditChange, 0, len(modelChanges)+2)
	if document.ConvertsToVersionThree {
		changes = append(changes, ConfigEditChange{
			Kind:    ConfigEditChangeCompatibilityConversion,
			Summary: fmt.Sprintf("Configuration will be saved as version 3 from version %d.", document.OriginalVersion),
			Warning: true,
		})
	}
	changes = append(changes, ConfigEditChange{
		Kind:    ConfigEditChangeFormattingNormalization,
		Summary: ".switchlet.yaml formatting may be normalized and comments may not be preserved.",
		Warning: true,
	})
	changes = append(changes, modelChanges...)

	return changes
}

func newProfileDraft(targets []config.Target) ConfigEditProfileDraft {
	draft := ConfigEditProfileDraft{
		Values: make([]ConfigEditProfileValueDraft, 0, len(targets)),
	}
	includeOnlyManagedValue := len(targets) == 1
	for _, target := range targets {
		draft.Values = append(draft.Values, ConfigEditProfileValueDraft{
			TargetDescriptor: targetDescriptor(target),
			Included:         includeOnlyManagedValue,
			Source:           ProfileSourceLiteral,
		})
	}

	return draft
}

func profileDraftFromProfile(targets []config.Target, profile config.Profile) ConfigEditProfileDraft {
	draft := newProfileDraft(targets)
	draft.Name = profile.Name
	draft.Protected = profile.Protected
	valuesByTarget := make(map[string]config.ProfileValue, len(profile.Values))
	for _, value := range profile.Values {
		valuesByTarget[value.Target] = value
	}

	for index := range draft.Values {
		profileValue, exists := valuesByTarget[draft.Values[index].TargetName]
		if !exists {
			continue
		}

		draft.Values[index].Included = true
		if profileValue.ValueFromEnv != nil {
			draft.Values[index].Source = ProfileSourceEnvironment
			draft.Values[index].EnvironmentVariableName = *profileValue.ValueFromEnv
			continue
		}

		draft.Values[index].Source = ProfileSourceLiteral
		if profileValue.Value != nil {
			draft.Values[index].LiteralValue = *profileValue.Value
		}
	}

	return draft
}

func profileFromDraft(draft ConfigEditProfileDraft) (config.Profile, error) {
	profileName := strings.TrimSpace(draft.Name)
	if profileName == "" {
		return config.Profile{}, fmt.Errorf("profile name must be set")
	}

	profile := config.Profile{
		Name:      profileName,
		Protected: draft.Protected,
		Values:    make([]config.ProfileValue, 0, len(draft.Values)),
	}
	for _, draftValue := range draft.Values {
		if !draftValue.Included {
			continue
		}

		profileValue := config.ProfileValue{Target: draftValue.TargetName}
		switch draftValue.Source {
		case ProfileSourceEnvironment:
			environmentVariableName := strings.TrimSpace(draftValue.EnvironmentVariableName)
			if environmentVariableName == "" {
				return config.Profile{}, fmt.Errorf("profile %q value for managed value %q environment variable must be set", profileName, draftValue.TargetName)
			}
			profileValue.ValueFromEnv = &environmentVariableName
		default:
			if strings.TrimSpace(draftValue.LiteralValue) == "" {
				return config.Profile{}, fmt.Errorf("profile %q value for managed value %q literal value must be set", profileName, draftValue.TargetName)
			}
			literalValue := draftValue.LiteralValue
			profileValue.Value = &literalValue
		}

		profile.Values = append(profile.Values, profileValue)
	}
	if len(profile.Values) == 0 {
		return config.Profile{}, fmt.Errorf("profile %q must include at least one managed value", profileName)
	}

	return profile, nil
}

func (workflow ConfigEditWorkflow) normalizeDraft(document ConfigEditDocument, action string) (ConfigEditDocument, error) {
	normalizedConfig, err := config.ValidateVersionThreeDraft(document.ProjectRoot, document.Targets, document.Profiles)
	if err != nil {
		return ConfigEditDocument{}, fmt.Errorf("%s: %w", action, err)
	}

	document.Targets = copyConfigEditTargets(normalizedConfig.Targets)
	document.Profiles = copyConfigEditProfiles(normalizedConfig.Profiles)

	return document, nil
}

func (workflow ConfigEditWorkflow) normalizeIfValid(document ConfigEditDocument) ConfigEditDocument {
	normalizedDocument, err := workflow.normalizeDraft(document, "normalize draft")
	if err != nil {
		return document
	}

	return normalizedDocument
}

func (workflow ConfigEditWorkflow) loadSnapshot(configPath string) (config.ConfigSnapshot, error) {
	if workflow.dependencies.LoadSnapshot != nil {
		return workflow.dependencies.LoadSnapshot(configPath)
	}

	return config.LoadSnapshot(configPath)
}

func (workflow ConfigEditWorkflow) validateTargets(targets []config.Target) error {
	if workflow.dependencies.ValidateTargets != nil {
		return workflow.dependencies.ValidateTargets(targets)
	}

	return editor.ValidateTargets(targets)
}

func (workflow ConfigEditWorkflow) prepareReplacement(snapshot config.ConfigSnapshot, targets []config.Target, profiles []config.Profile) (config.PreparedReplacement, error) {
	if workflow.dependencies.PrepareReplacement != nil {
		return workflow.dependencies.PrepareReplacement(snapshot, targets, profiles)
	}

	return config.PrepareReplacementFromSnapshot(snapshot, targets, profiles)
}

func (workflow ConfigEditWorkflow) summarizeManagedValueChanges(document ConfigEditDocument) []ConfigEditChange {
	originalByName := targetsByName(document.originalTargets)
	draftByName := targetsByName(document.Targets)
	removedTargets := targetsMissingFrom(document.originalTargets, draftByName)
	addedTargets := targetsMissingFrom(document.Targets, originalByName)
	usedAddedTargets := make(map[int]struct{}, len(addedTargets))
	changes := make([]ConfigEditChange, 0)

	for _, originalTarget := range document.originalTargets {
		draftTarget, exists := draftByName[originalTarget.Name]
		if exists && !configEditTargetsEqual(originalTarget, draftTarget) {
			changes = append(changes, ConfigEditChange{
				Kind:    ConfigEditChangeManagedValueUpdated,
				Summary: fmt.Sprintf("Edited managed value %q.", originalTarget.Name),
				Detail:  targetChangeDetail(originalTarget, draftTarget),
			})
		}
	}

	for _, removedTarget := range removedTargets {
		matchedAddedIndex := -1
		for addedIndex, addedTarget := range addedTargets {
			if _, used := usedAddedTargets[addedIndex]; used {
				continue
			}
			if configEditTargetLocationEqual(removedTarget, addedTarget) {
				matchedAddedIndex = addedIndex
				break
			}
		}
		if matchedAddedIndex < 0 {
			changes = append(changes, ConfigEditChange{
				Kind:    ConfigEditChangeManagedValueRemoved,
				Summary: fmt.Sprintf("Removed managed value %q.", removedTarget.Name),
				Detail:  removedManagedValueDetail(document.originalProfiles, removedTarget.Name),
			})
			continue
		}

		addedTarget := addedTargets[matchedAddedIndex]
		usedAddedTargets[matchedAddedIndex] = struct{}{}
		changes = append(changes, ConfigEditChange{
			Kind:    ConfigEditChangeManagedValueRenamed,
			Summary: fmt.Sprintf("Renamed managed value %q to %q.", removedTarget.Name, addedTarget.Name),
			Detail:  renamedManagedValueDetail(document.Profiles, addedTarget.Name),
		})
	}

	for addedIndex, addedTarget := range addedTargets {
		if _, used := usedAddedTargets[addedIndex]; used {
			continue
		}

		changes = append(changes, ConfigEditChange{
			Kind:    ConfigEditChangeManagedValueAdded,
			Summary: fmt.Sprintf("Added managed value %q.", addedTarget.Name),
			Detail:  targetDetail(addedTarget),
		})
	}

	return changes
}

func (workflow ConfigEditWorkflow) summarizeProfileChanges(document ConfigEditDocument) []ConfigEditChange {
	originalByName := profilesByName(document.originalProfiles)
	draftByName := profilesByName(document.Profiles)
	removedProfiles := profilesMissingFrom(document.originalProfiles, draftByName)
	addedProfiles := profilesMissingFrom(document.Profiles, originalByName)
	usedAddedProfiles := make(map[int]struct{}, len(addedProfiles))
	changes := make([]ConfigEditChange, 0)

	for _, originalProfile := range document.originalProfiles {
		draftProfile, exists := draftByName[originalProfile.Name]
		if exists && !configEditProfilesEqual(originalProfile, draftProfile) {
			changes = append(changes, ConfigEditChange{
				Kind:    ConfigEditChangeProfileUpdated,
				Summary: fmt.Sprintf("Edited profile %q.", originalProfile.Name),
				Detail:  profileChangeDetail(originalProfile, draftProfile),
			})
		}
	}

	for _, removedProfile := range removedProfiles {
		matchedAddedIndex := -1
		for addedIndex, addedProfile := range addedProfiles {
			if _, used := usedAddedProfiles[addedIndex]; used {
				continue
			}
			if configEditProfileContentsEqual(removedProfile, addedProfile) {
				matchedAddedIndex = addedIndex
				break
			}
		}
		if matchedAddedIndex < 0 {
			changes = append(changes, ConfigEditChange{
				Kind:    ConfigEditChangeProfileRemoved,
				Summary: fmt.Sprintf("Removed profile %q.", removedProfile.Name),
			})
			continue
		}

		addedProfile := addedProfiles[matchedAddedIndex]
		usedAddedProfiles[matchedAddedIndex] = struct{}{}
		changes = append(changes, ConfigEditChange{
			Kind:    ConfigEditChangeProfileRenamed,
			Summary: fmt.Sprintf("Renamed profile %q to %q.", removedProfile.Name, addedProfile.Name),
		})
	}

	for addedIndex, addedProfile := range addedProfiles {
		if _, used := usedAddedProfiles[addedIndex]; used {
			continue
		}

		changes = append(changes, ConfigEditChange{
			Kind:    ConfigEditChangeProfileAdded,
			Summary: fmt.Sprintf("Added profile %q.", addedProfile.Name),
			Detail:  []string{fmt.Sprintf("Managed values: %d", len(addedProfile.Values))},
		})
	}

	return changes
}

func (document ConfigEditDocument) clone() ConfigEditDocument {
	clone := document
	clone.Targets = copyConfigEditTargets(document.Targets)
	clone.Profiles = copyConfigEditProfiles(document.Profiles)
	clone.originalTargets = copyConfigEditTargets(document.originalTargets)
	clone.originalProfiles = copyConfigEditProfiles(document.originalProfiles)

	return clone
}

func copyConfigEditTargets(targets []config.Target) []config.Target {
	copiedTargets := make([]config.Target, len(targets))
	copy(copiedTargets, targets)

	return copiedTargets
}

func copyConfigEditProfiles(profiles []config.Profile) []config.Profile {
	copiedProfiles := make([]config.Profile, 0, len(profiles))
	for _, profile := range profiles {
		copiedProfiles = append(copiedProfiles, copyConfigEditProfile(profile))
	}

	return copiedProfiles
}

func copyConfigEditProfile(profile config.Profile) config.Profile {
	profileCopy := config.Profile{
		Name:      profile.Name,
		Protected: profile.Protected,
		Values:    make([]config.ProfileValue, 0, len(profile.Values)),
	}
	for _, value := range profile.Values {
		profileCopy.Values = append(profileCopy.Values, copyConfigEditProfileValue(value))
	}

	return profileCopy
}

func copyConfigEditProfileValue(value config.ProfileValue) config.ProfileValue {
	valueCopy := config.ProfileValue{Target: value.Target}
	if value.Value != nil {
		literalValue := *value.Value
		valueCopy.Value = &literalValue
	}
	if value.ValueFromEnv != nil {
		environmentVariableName := *value.ValueFromEnv
		valueCopy.ValueFromEnv = &environmentVariableName
	}

	return valueCopy
}

func targetIndexByName(targets []config.Target, targetName string) int {
	for index, target := range targets {
		if target.Name == targetName {
			return index
		}
	}

	return -1
}

func profileIndexByName(profiles []config.Profile, profileName string) int {
	for index, profile := range profiles {
		if profile.Name == profileName {
			return index
		}
	}

	return -1
}

func targetsByName(targets []config.Target) map[string]config.Target {
	byName := make(map[string]config.Target, len(targets))
	for _, target := range targets {
		byName[target.Name] = target
	}

	return byName
}

func profilesByName(profiles []config.Profile) map[string]config.Profile {
	byName := make(map[string]config.Profile, len(profiles))
	for _, profile := range profiles {
		byName[profile.Name] = profile
	}

	return byName
}

func targetsMissingFrom(targets []config.Target, comparison map[string]config.Target) []config.Target {
	missing := make([]config.Target, 0)
	for _, target := range targets {
		if _, exists := comparison[target.Name]; !exists {
			missing = append(missing, target)
		}
	}

	return missing
}

func profilesMissingFrom(profiles []config.Profile, comparison map[string]config.Profile) []config.Profile {
	missing := make([]config.Profile, 0)
	for _, profile := range profiles {
		if _, exists := comparison[profile.Name]; !exists {
			missing = append(missing, profile)
		}
	}

	return missing
}

func configEditTargetsEqual(left config.Target, right config.Target) bool {
	return left.Name == right.Name && configEditTargetLocationEqual(left, right)
}

func configEditTargetLocationEqual(left config.Target, right config.Target) bool {
	return filepath.Clean(left.File) == filepath.Clean(right.File) && left.Type == right.Type && targetSelectorValue(left) == targetSelectorValue(right)
}

func configEditProfilesEqual(left config.Profile, right config.Profile) bool {
	return left.Name == right.Name && configEditProfileContentsEqual(left, right)
}

func configEditProfileContentsEqual(left config.Profile, right config.Profile) bool {
	if left.Protected != right.Protected || len(left.Values) != len(right.Values) {
		return false
	}
	for index := range left.Values {
		if !configEditProfileValuesEqual(left.Values[index], right.Values[index]) {
			return false
		}
	}

	return true
}

func configEditProfileValuesEqual(left config.ProfileValue, right config.ProfileValue) bool {
	return left.Target == right.Target && stringPointerEqual(left.Value, right.Value) && stringPointerEqual(left.ValueFromEnv, right.ValueFromEnv)
}

func stringPointerEqual(left *string, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}

	return *left == *right
}

func targetChangeDetail(originalTarget config.Target, draftTarget config.Target) []string {
	detail := make([]string, 0, 2)
	if filepath.Clean(originalTarget.File) != filepath.Clean(draftTarget.File) {
		detail = append(detail, fmt.Sprintf("File: %s -> %s", originalTarget.File, draftTarget.File))
	}
	if originalTarget.Type != draftTarget.Type || targetSelectorValue(originalTarget) != targetSelectorValue(draftTarget) {
		originalSelectorName, originalSelector := targetSelector(originalTarget)
		draftSelectorName, draftSelector := targetSelector(draftTarget)
		detail = append(detail, fmt.Sprintf("Selector: %s %s -> %s %s", originalSelectorName, originalSelector, draftSelectorName, draftSelector))
	}

	return detail
}

func targetDetail(target config.Target) []string {
	selectorName, selector := targetSelector(target)
	return []string{
		fmt.Sprintf("File: %s", target.File),
		fmt.Sprintf("Type: %s", target.Type),
		fmt.Sprintf("%s: %s", selectorName, selector),
	}
}

func removedManagedValueDetail(profiles []config.Profile, targetName string) []string {
	affectedProfiles := profilesReferencingTarget(profiles, targetName)
	if len(affectedProfiles) == 0 {
		return nil
	}

	return []string{fmt.Sprintf("Removed profile values from: %s", strings.Join(affectedProfiles, ", "))}
}

func renamedManagedValueDetail(profiles []config.Profile, targetName string) []string {
	affectedProfiles := profilesReferencingTarget(profiles, targetName)
	if len(affectedProfiles) == 0 {
		return nil
	}

	return []string{fmt.Sprintf("Updated profile references in: %s", strings.Join(affectedProfiles, ", "))}
}

func profileChangeDetail(originalProfile config.Profile, draftProfile config.Profile) []string {
	detail := make([]string, 0, 2)
	if originalProfile.Protected != draftProfile.Protected {
		detail = append(detail, fmt.Sprintf("Protected: %t -> %t", originalProfile.Protected, draftProfile.Protected))
	}
	if !profileValuesSameTargetsAndSources(originalProfile.Values, draftProfile.Values) {
		detail = append(detail, "Managed values or value sources changed.")
	}

	return detail
}

func profileValuesSameTargetsAndSources(left []config.ProfileValue, right []config.ProfileValue) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Target != right[index].Target || (left[index].Value != nil) != (right[index].Value != nil) || (left[index].ValueFromEnv != nil) != (right[index].ValueFromEnv != nil) {
			return false
		}
	}

	return true
}

func profilesReferencingTarget(profiles []config.Profile, targetName string) []string {
	profileNames := make([]string, 0)
	for _, profile := range profiles {
		for _, value := range profile.Values {
			if value.Target == targetName {
				profileNames = append(profileNames, profile.Name)
				break
			}
		}
	}
	sort.Strings(profileNames)

	return profileNames
}

func targetSelectorValue(target config.Target) string {
	_, selector := targetSelector(target)
	return selector
}
