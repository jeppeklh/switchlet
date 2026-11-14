package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

// ConfigEditImportMapping maps one configured destination file to a source.
// An empty source leaves that file's values inherited from the base profile.
type ConfigEditImportMapping struct {
	TargetFile string
	SourceFile string
	TargetType config.TargetType
}

// ConfigEditImportValue exposes read status and location, never captured values.
type ConfigEditImportValue struct {
	TargetDescriptor
	SourceFile string
	Available  bool
	Error      string
}

// ConfigEditProfileImport retains the reviewed source snapshot. Literal values
// remain private until selected for a profile draft; saving never rereads sources.
type ConfigEditProfileImport struct {
	Values   []ConfigEditImportValue
	base     ConfigEditProfileDraft
	targets  []config.Target
	captured map[string]string
}

// ProfileImportMappings lists destination files in configured order.
func (workflow ConfigEditWorkflow) ProfileImportMappings(document ConfigEditDocument) []ConfigEditImportMapping {
	var mappings []ConfigEditImportMapping
	seen := make(map[string]bool)
	for _, target := range document.Targets {
		if !seen[target.File] {
			mappings = append(mappings, ConfigEditImportMapping{TargetFile: target.File, TargetType: target.Type})
			seen[target.File] = true
		}
	}
	return mappings
}

// PrepareProfileImport reads only configured selectors, using the destination's
// declared format and value type. Individual failures remain reviewable.
func (workflow ConfigEditWorkflow) PrepareProfileImport(document ConfigEditDocument, baseName string, mappings []ConfigEditImportMapping) (ConfigEditProfileImport, error) {
	base, err := workflow.ProfileDraft(document, baseName)
	if err != nil {
		return ConfigEditProfileImport{}, err
	}
	prepared := ConfigEditProfileImport{base: base, targets: slices.Clone(document.Targets), captured: make(map[string]string)}
	known := make(map[string]bool)
	for _, target := range document.Targets {
		known[target.File] = true
	}
	sources := make(map[string]string)
	for _, mapping := range mappings {
		if !known[mapping.TargetFile] {
			return ConfigEditProfileImport{}, fmt.Errorf("unknown destination file %q", mapping.TargetFile)
		}
		if _, exists := sources[mapping.TargetFile]; exists {
			return ConfigEditProfileImport{}, fmt.Errorf("duplicate source mapping for %q", mapping.TargetFile)
		}
		source := strings.TrimSpace(mapping.SourceFile)
		if source != "" {
			source = ResolveInitTargetPath(document.ProjectRoot, source)
		}
		sources[mapping.TargetFile] = source
	}
	var reads []config.Target
	var indexes []int
	for index, target := range document.Targets {
		source := sources[target.File]
		row := ConfigEditImportValue{TargetDescriptor: targetDescriptor(target), SourceFile: source}
		if source != "" {
			sourceType, recognized := config.InferTargetType(source)
			if recognized && sourceType != target.Type {
				row.Error = fmt.Sprintf("source format %s does not match destination format %s", sourceType, target.Type)
			} else {
				target.File = source
				reads = append(reads, target)
				indexes = append(indexes, index)
			}
		}
		prepared.Values = append(prepared.Values, row)
	}
	results := editor.ReadTargetValueResults(reads)
	for index, result := range results {
		row := &prepared.Values[indexes[index]]
		if result.Err != nil {
			row.Error = result.Err.Error()
			continue
		}
		// Apply the same literal rules as ordinary profile entry before offering
		// a value for selection (including the non-empty string requirement).
		_, err := profileFromDraft(ConfigEditProfileDraft{Name: baseName, Values: []ConfigEditProfileValueDraft{{
			TargetDescriptor: row.TargetDescriptor, Included: true, Source: ProfileSourceLiteral, LiteralValue: result.Value,
		}}})
		if err != nil {
			row.Error = err.Error()
			continue
		}
		row.Available = true
		prepared.captured[row.TargetName] = result.Value
	}
	return prepared, nil
}

// ProfileDraftFromImport overlays selected captured literals onto the base draft.
// This is a pure operation: environment references and unselected values survive
// unchanged, and stale destination/base definitions require a fresh review.
func (workflow ConfigEditWorkflow) ProfileDraftFromImport(document ConfigEditDocument, prepared ConfigEditProfileImport, name string, selected []string) (ConfigEditProfileDraft, error) {
	draft, err := workflow.DuplicateProfileDraft(document, prepared.base.Name, name)
	if err != nil {
		return ConfigEditProfileDraft{}, err
	}
	if !slices.Equal(document.Targets, prepared.targets) || draft.Protected != prepared.base.Protected || !slices.Equal(draft.Values, prepared.base.Values) {
		return ConfigEditProfileDraft{}, fmt.Errorf("base profile or destinations changed; read sources again")
	}
	if len(selected) == 0 {
		return ConfigEditProfileDraft{}, fmt.Errorf("select at least one imported value")
	}
	for _, targetName := range selected {
		captured, ok := prepared.captured[targetName]
		if !ok {
			return ConfigEditProfileDraft{}, fmt.Errorf("target %q has no successful imported value", targetName)
		}
		for index := range draft.Values {
			value := &draft.Values[index]
			if value.TargetName != targetName {
				continue
			}
			value.Included = true
			value.Source = ProfileSourceLiteral
			value.LiteralValue = captured
			value.EnvironmentVariableName = ""
			break
		}
	}
	if _, err := workflow.AddProfileDraft(document, draft); err != nil {
		return ConfigEditProfileDraft{}, err
	}
	return draft, nil
}
