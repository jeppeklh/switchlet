package editor

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
)

// TargetChange describes one already-resolved value to write to a configured target.
type TargetChange struct {
	Target config.Target
	Value  string
}

// ManagedPreview contains side-effect-free preview data for managed target changes.
type ManagedPreview struct {
	Files []ManagedPreviewFile
}

// ManagedPreviewFile contains preview hunks for one prepared target file.
type ManagedPreviewFile struct {
	TargetFile string
	TargetType config.TargetType
	Hunks      []ManagedPreviewHunk
}

// ManagedPreviewHunk describes one configured target location in a managed preview.
type ManagedPreviewHunk struct {
	Target        config.Target
	SelectorName  string
	Selector      string
	OriginalValue string
	ProposedValue string
}

// PreparedChanges contains target-file updates that have all been validated
// and serialized but not yet written.
type PreparedChanges struct {
	fileChanges []preparedFileChange
}

type preparedFileChange struct {
	targetFile  string
	contents    []byte
	permissions fs.FileMode
}

type targetChangeGroup struct {
	targetFile string
	targetType config.TargetType
	changes    []TargetChange
}

// TargetError describes a failure for one configured target while preserving
// the underlying reason for callers that need user-facing diagnostics.
type TargetError struct {
	Target config.Target
	Err    error
}

func (err TargetError) Error() string {
	selectorName, selector := targetSelectorSummary(err.Target)
	targetType := string(err.Target.Type)
	if targetType == "" {
		targetType = "unknown"
	}
	return fmt.Sprintf("target %q in file %q (type %q, %s %q): %v", err.Target.Name, err.Target.File, targetType, selectorName, selector, err.Err)
}

func (err TargetError) Unwrap() error {
	return err.Err
}

// ValidateTarget verifies that one configured target points at an existing
// editable value.
func ValidateTarget(target config.Target) error {
	switch target.Type {
	case config.TargetTypeJSON:
		if err := ValidateStringTarget(target.File, target.JSONPath); err != nil {
			return targetError(target, err)
		}
		return nil
	case config.TargetTypeDotenv:
		if err := ValidateDotenvTarget(target.File, target.Key); err != nil {
			return targetError(target, err)
		}
		return nil
	case config.TargetTypeYAML:
		if err := ValidateYAMLTarget(target.File, target.YAMLPath); err != nil {
			return targetError(target, err)
		}
		return nil
	case config.TargetTypeTOML:
		if err := ValidateTOMLTarget(target.File, target.TOMLPath); err != nil {
			return targetError(target, err)
		}
		return nil
	default:
		return targetError(target, fmt.Errorf("target type %q is not supported", target.Type))
	}
}

// ValidateTargets verifies that every configured target points at an existing
// editable value.
func ValidateTargets(targets []config.Target) error {
	for _, target := range targets {
		if err := ValidateTarget(target); err != nil {
			return err
		}
	}

	return nil
}

// ReadTargetValue reads the current string value for one configured target
// without preparing or writing file changes.
func ReadTargetValue(target config.Target) (string, error) {
	normalizedTarget, _, err := normalizeTarget(target)
	if err != nil {
		return "", err
	}

	contents, _, err := readTargetFile(normalizedTarget.File)
	if err != nil {
		return "", targetError(normalizedTarget, err)
	}

	value, err := readTargetValueFromContents(contents, normalizedTarget)
	if err != nil {
		return "", targetError(normalizedTarget, err)
	}

	return value, nil
}

// PreviewTargetChanges validates and serializes target changes without writing
// any target files.
func PreviewTargetChanges(changes []TargetChange) error {
	_, err := PrepareTargetChanges(changes)
	return err
}

// PreviewManagedTargetChanges validates and prepares changes, then returns
// managed target-level preview data without writing target files.
func PreviewManagedTargetChanges(changes []TargetChange) (ManagedPreview, error) {
	if len(changes) == 0 {
		return ManagedPreview{}, fmt.Errorf("at least one target change must be requested")
	}

	groups, err := groupTargetChanges(changes)
	if err != nil {
		return ManagedPreview{}, err
	}

	files := make([]ManagedPreviewFile, 0, len(groups))
	for _, group := range groups {
		filePreview, err := prepareManagedPreviewFile(group)
		if err != nil {
			return ManagedPreview{}, err
		}

		files = append(files, filePreview)
	}

	return ManagedPreview{Files: files}, nil
}

// ApplyTargetChanges prepares every affected target file before writing any
// file, then writes each prepared file safely.
func ApplyTargetChanges(changes []TargetChange) error {
	preparedChanges, err := PrepareTargetChanges(changes)
	if err != nil {
		return err
	}

	return preparedChanges.Write()
}

// PrepareTargetChanges validates and serializes the final contents for every
// affected target file. No filesystem writes occur during preparation.
func PrepareTargetChanges(changes []TargetChange) (PreparedChanges, error) {
	if len(changes) == 0 {
		return PreparedChanges{}, fmt.Errorf("at least one target change must be requested")
	}

	groups, err := groupTargetChanges(changes)
	if err != nil {
		return PreparedChanges{}, err
	}

	fileChanges := make([]preparedFileChange, 0, len(groups))
	for _, group := range groups {
		fileChange, err := prepareTargetFileChange(group)
		if err != nil {
			return PreparedChanges{}, err
		}

		fileChanges = append(fileChanges, fileChange)
	}

	return PreparedChanges{fileChanges: fileChanges}, nil
}

// Write persists every prepared file update. Callers should treat a write
// failure after an earlier successful replacement as an uncertain partial state.
func (preparedChanges PreparedChanges) Write() error {
	for index, fileChange := range preparedChanges.fileChanges {
		if err := writeFileAtomically(fileChange.targetFile, fileChange.contents, fileChange.permissions); err != nil {
			if index > 0 {
				return fmt.Errorf("write prepared target file %q after %d file(s) were already replaced; target files may now be partially updated: %w", fileChange.targetFile, index, err)
			}

			return fmt.Errorf("write prepared target file %q: %w", fileChange.targetFile, err)
		}
	}

	return nil
}

func groupTargetChanges(changes []TargetChange) ([]targetChangeGroup, error) {
	groupIndexesByFile := make(map[string]int)
	groups := make([]targetChangeGroup, 0)
	seenLocations := make(map[string]string, len(changes))

	for _, change := range changes {
		normalizedChange, selector, err := normalizeTargetChange(change)
		if err != nil {
			return nil, err
		}

		locationKey := targetLocationKey(normalizedChange.Target, selector)
		if existingTargetName, exists := seenLocations[locationKey]; exists {
			return nil, targetError(normalizedChange.Target, fmt.Errorf("duplicates target location used by target %q", existingTargetName))
		}
		seenLocations[locationKey] = normalizedChange.Target.Name

		existingGroupIndex, exists := groupIndexesByFile[normalizedChange.Target.File]
		if exists {
			existingGroup := &groups[existingGroupIndex]
			if existingGroup.targetType != normalizedChange.Target.Type {
				return nil, targetError(normalizedChange.Target, fmt.Errorf("target file already has %q changes queued", existingGroup.targetType))
			}

			existingGroup.changes = append(existingGroup.changes, normalizedChange)
			continue
		}

		groups = append(groups, targetChangeGroup{
			targetFile: normalizedChange.Target.File,
			targetType: normalizedChange.Target.Type,
			changes:    []TargetChange{normalizedChange},
		})
		groupIndexesByFile[normalizedChange.Target.File] = len(groups) - 1
	}

	return groups, nil
}

func normalizeTargetChange(change TargetChange) (TargetChange, string, error) {
	normalizedTarget, selector, err := normalizeTarget(change.Target)
	if err != nil {
		return TargetChange{}, "", err
	}

	change.Target = normalizedTarget
	return change, selector, nil
}

func normalizeTarget(target config.Target) (config.Target, string, error) {
	if target.File == "" {
		return config.Target{}, "", targetError(target, fmt.Errorf("target file must be set"))
	}

	selector, err := validateTargetSelector(target)
	if err != nil {
		return config.Target{}, "", targetError(target, err)
	}

	target.File = filepath.Clean(target.File)
	return target, selector, nil
}

func validateTargetSelector(target config.Target) (string, error) {
	switch target.Type {
	case config.TargetTypeJSON:
		if target.JSONPath == "" {
			return "", fmt.Errorf("jsonPath must be set")
		}
		pathSegments, err := config.ParseJSONPath(target.JSONPath)
		if err != nil {
			return "", fmt.Errorf("invalid JSON path %q: %w", target.JSONPath, err)
		}
		return strings.Join(pathSegments, "."), nil
	case config.TargetTypeDotenv:
		if target.Key == "" {
			return "", fmt.Errorf("key must be set")
		}
		if err := validateDotenvKey(target.Key); err != nil {
			return "", fmt.Errorf("dotenv key is invalid: %w", err)
		}
		return target.Key, nil
	case config.TargetTypeYAML:
		if target.YAMLPath == "" {
			return "", fmt.Errorf("yamlPath must be set")
		}
		pathSegments, err := config.ParseYAMLPath(target.YAMLPath)
		if err != nil {
			return "", fmt.Errorf("invalid YAML path %q: %w", target.YAMLPath, err)
		}
		return strings.Join(pathSegments, "."), nil
	case config.TargetTypeTOML:
		if target.TOMLPath == "" {
			return "", fmt.Errorf("tomlPath must be set")
		}
		pathSegments, err := config.ParseTOMLPath(target.TOMLPath)
		if err != nil {
			return "", fmt.Errorf("invalid TOML path %q: %w", target.TOMLPath, err)
		}
		return strings.Join(pathSegments, "."), nil
	default:
		return "", fmt.Errorf("target type %q is not supported", target.Type)
	}
}

func targetLocationKey(target config.Target, selector string) string {
	return string(target.Type) + "\x00" + target.File + "\x00" + selector
}

func prepareTargetFileChange(group targetChangeGroup) (preparedFileChange, error) {
	contents, targetInfo, err := readTargetFile(group.targetFile)
	if err != nil {
		return preparedFileChange{}, targetError(group.changes[0].Target, err)
	}

	updatedContents, err := prepareTargetFileContents(contents, group)
	if err != nil {
		return preparedFileChange{}, err
	}

	return preparedFileChange{
		targetFile:  group.targetFile,
		contents:    updatedContents,
		permissions: targetInfo.Mode().Perm(),
	}, nil
}

func prepareManagedPreviewFile(group targetChangeGroup) (ManagedPreviewFile, error) {
	contents, _, err := readTargetFile(group.targetFile)
	if err != nil {
		return ManagedPreviewFile{}, targetError(group.changes[0].Target, err)
	}

	hunks, err := managedPreviewHunks(contents, group)
	if err != nil {
		return ManagedPreviewFile{}, err
	}
	if _, err := prepareTargetFileContents(contents, group); err != nil {
		return ManagedPreviewFile{}, err
	}

	return ManagedPreviewFile{
		TargetFile: group.targetFile,
		TargetType: group.targetType,
		Hunks:      hunks,
	}, nil
}

func managedPreviewHunks(contents []byte, group targetChangeGroup) ([]ManagedPreviewHunk, error) {
	hunks := make([]ManagedPreviewHunk, 0, len(group.changes))
	for _, change := range group.changes {
		originalValue, err := readTargetValueFromContents(contents, change.Target)
		if err != nil {
			return nil, targetError(change.Target, err)
		}

		selectorName, selector := targetSelectorSummary(change.Target)
		hunks = append(hunks, ManagedPreviewHunk{
			Target:        change.Target,
			SelectorName:  selectorName,
			Selector:      selector,
			OriginalValue: originalValue,
			ProposedValue: change.Value,
		})
	}

	return hunks, nil
}

func readTargetValueFromContents(contents []byte, target config.Target) (string, error) {
	switch target.Type {
	case config.TargetTypeJSON:
		return readJSONStringTargetValue(contents, target.JSONPath)
	case config.TargetTypeDotenv:
		return readDotenvTargetValue(contents, target.Key)
	case config.TargetTypeYAML:
		return readYAMLStringTargetValue(contents, target.YAMLPath)
	case config.TargetTypeTOML:
		return readTOMLStringTargetValue(contents, target.TOMLPath)
	default:
		return "", fmt.Errorf("target type %q is not supported", target.Type)
	}
}

func prepareTargetFileContents(contents []byte, group targetChangeGroup) ([]byte, error) {
	var updatedContents []byte
	var err error
	switch group.targetType {
	case config.TargetTypeJSON:
		updatedContents, err = replaceJSONTargetValues(contents, group.changes)
	case config.TargetTypeDotenv:
		updatedContents, err = replaceDotenvTargetValues(contents, group.changes)
	case config.TargetTypeYAML:
		updatedContents, err = replaceYAMLTargetValues(contents, group.changes)
	case config.TargetTypeTOML:
		updatedContents, err = replaceTOMLTargetValues(contents, group.changes)
	default:
		err = fmt.Errorf("target type %q is not supported", group.targetType)
	}
	if err != nil {
		return nil, err
	}

	return updatedContents, nil
}

func targetError(target config.Target, err error) error {
	return TargetError{Target: target, Err: err}
}

func targetSelectorSummary(target config.Target) (string, string) {
	switch target.Type {
	case config.TargetTypeJSON:
		return "jsonPath", target.JSONPath
	case config.TargetTypeDotenv:
		return "key", target.Key
	case config.TargetTypeYAML:
		return "yamlPath", target.YAMLPath
	case config.TargetTypeTOML:
		return "tomlPath", target.TOMLPath
	default:
		return "selector", ""
	}
}
