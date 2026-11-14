package app

import (
	"fmt"
	"slices"

	"github.com/jeppeklh/switchlet/internal/config"
)

// ExampleExportEntry contains only schema metadata and environment names.
type ExampleExportEntry struct {
	TargetDescriptor
	ProfileName       string
	Protected         bool
	EnvironmentName   string
	ExistingReference bool
}

// ExampleExportDraft is separate from the active config-editor document.
type ExampleExportDraft struct {
	Targets []TargetDescriptor
	Entries []ExampleExportEntry
}

// PreparedExampleExport retains sanitized output and required shared-source review.
type PreparedExampleExport struct {
	Path             string
	ReplacesExisting bool
	SharedVariables  []string
	requiredShared   []string
	prepared         config.PreparedExample
}

// Commit requires explicit approval of every newly shared environment source.
func (export PreparedExampleExport) Commit(approvedShared []string) error {
	for _, name := range export.requiredShared {
		if !slices.Contains(approvedShared, name) {
			return fmt.Errorf("confirm shared environment source %q before export", name)
		}
	}
	return export.prepared.Commit()
}

// NewExampleExportDraft describes every included profile entry without reading
// environment variables or exposing existing literals to the export interface.
func (workflow ConfigEditWorkflow) NewExampleExportDraft(document ConfigEditDocument) (ExampleExportDraft, error) {
	if err := workflow.ValidateDraft(document); err != nil {
		return ExampleExportDraft{}, err
	}
	targets := make(map[string]config.Target)
	for _, target := range document.Targets {
		targets[target.Name] = target
	}
	draft := ExampleExportDraft{Targets: targetDescriptors(document.Targets)}
	for _, profile := range document.Profiles {
		for _, value := range profile.Values {
			entry := ExampleExportEntry{ProfileName: profile.Name, Protected: profile.Protected, TargetDescriptor: targetDescriptor(targets[value.Target])}
			entry.ValueType = config.EffectiveValueType(entry.ValueType)
			if value.ValueFromEnv != nil {
				entry.EnvironmentName = *value.ValueFromEnv
				entry.ExistingReference = true
			}
			draft.Entries = append(draft.Entries, entry)
		}
	}
	return draft, nil
}

// PrepareExampleExport validates complete explicit mappings and preserves the
// original schema metadata. Fresh serialization discards all source comments.
func (workflow ConfigEditWorkflow) PrepareExampleExport(document ConfigEditDocument, draft ExampleExportDraft) (PreparedExampleExport, error) {
	expected, err := workflow.NewExampleExportDraft(document)
	if err != nil {
		return PreparedExampleExport{}, err
	}
	if len(draft.Entries) != len(expected.Entries) {
		return PreparedExampleExport{}, fmt.Errorf("export entries changed; start a fresh export")
	}
	profiles := copyConfigEditProfiles(document.Profiles)
	counts := make(map[string]int)
	changed := make(map[string]bool)
	index := 0
	for profileIndex := range profiles {
		profile := &profiles[profileIndex]
		profile.Value, profile.ValueFromEnv = nil, nil
		for valueIndex := range profile.Values {
			entry, original := draft.Entries[index], expected.Entries[index]
			index++
			if entry.ProfileName != original.ProfileName || entry.TargetName != original.TargetName || (original.ExistingReference && entry.EnvironmentName != original.EnvironmentName) {
				return PreparedExampleExport{}, fmt.Errorf("export must preserve existing profile entries and environment references")
			}
			name := entry.EnvironmentName
			if err := config.ValidateExampleEnvironmentName(name); err != nil {
				return PreparedExampleExport{}, fmt.Errorf("profile %q target %q: %w", entry.ProfileName, entry.TargetName, err)
			}
			counts[name]++
			changed[name] = changed[name] || !original.ExistingReference
			profile.Values[valueIndex].Value = nil
			profile.Values[valueIndex].ValueFromEnv = &name
		}
	}
	var shared []string
	for _, entry := range draft.Entries {
		name := entry.EnvironmentName
		if counts[name] > 1 && changed[name] && !slices.Contains(shared, name) {
			shared = append(shared, name)
		}
	}
	prepared, err := config.PrepareExample(document.ProjectRoot, document.ConfigPath, document.Targets, profiles)
	if err != nil {
		return PreparedExampleExport{}, err
	}
	return PreparedExampleExport{Path: prepared.Path(), ReplacesExisting: prepared.ReplacesExisting(), SharedVariables: slices.Clone(shared), requiredShared: shared, prepared: prepared}, nil
}

// ValidateExampleEnvironmentName validates an entered name without changing it.
func ValidateExampleEnvironmentName(name string) error {
	return config.ValidateExampleEnvironmentName(name)
}
