package editor

import (
	"errors"
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
	targetFile          string
	originalContents    []byte
	updatedContents     []byte
	originalPermissions fs.FileMode
}

// PreflightError describes a failed target-file write preflight before any
// prepared target files were replaced.
type PreflightError struct {
	TargetFile string
	Err        error
}

func (err PreflightError) Error() string {
	return fmt.Sprintf("preflight target file %q: %v", err.TargetFile, err.Err)
}

func (err PreflightError) Unwrap() error {
	return err.Err
}

// RecoveryError describes a failed multi-file apply after Switchlet attempted
// to restore already replaced target files from same-apply captured contents.
type RecoveryError struct {
	FailedFile      string
	ReplacedFiles   []string
	RestoredFiles   []string
	UnrestoredFiles []string
	Err             error
	RestoreErr      error
}

func (err RecoveryError) Error() string {
	replacedCount := len(err.ReplacedFiles)
	if len(err.UnrestoredFiles) == 0 {
		return fmt.Sprintf("write prepared target file %q failed after %d file(s) were already replaced; restored prior replacements: %v", err.FailedFile, replacedCount, err.Err)
	}

	return fmt.Sprintf("write prepared target file %q failed after %d file(s) were already replaced; restoration failed for %d file(s); target files may now be partially updated: %v", err.FailedFile, replacedCount, len(err.UnrestoredFiles), err.Err)
}

func (err RecoveryError) Unwrap() error {
	return errors.Join(err.Err, err.RestoreErr)
}

type targetChangeGroup struct {
	targetFile string
	targetType config.TargetType
	changes    []TargetChange
}

type targetReadGroup struct {
	targetFile string
	targetType config.TargetType
	targets    []config.Target
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
	currentValues, err := ReadTargetValues([]config.Target{target})
	if err != nil {
		return "", err
	}

	for _, value := range currentValues {
		return value, nil
	}

	return "", fmt.Errorf("current value for target %q was not read", target.Name)
}

// ReadTargetValues reads current string values for configured targets, grouping
// reads by target file without preparing or writing file changes.
func ReadTargetValues(targets []config.Target) (map[string]string, error) {
	currentValues := make(map[string]string, len(targets))
	if len(targets) == 0 {
		return currentValues, nil
	}

	groups, err := groupTargetReads(targets)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		contents, _, err := readTargetFile(group.targetFile)
		if err != nil {
			return nil, targetError(group.targets[0], err)
		}

		groupValues, err := readTargetValuesFromContents(contents, group.targetType, group.targets)
		if err != nil {
			return nil, err
		}
		for targetName, value := range groupValues {
			currentValues[targetName] = value
		}
	}

	return currentValues, nil
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

// Write persists every prepared file update, restoring already replaced files
// on a best-effort basis if a later replacement fails.
func (preparedChanges PreparedChanges) Write() error {
	if err := preparedChanges.preflight(); err != nil {
		return err
	}

	replacedFiles := make([]preparedFileChange, 0, len(preparedChanges.fileChanges))
	for index, fileChange := range preparedChanges.fileChanges {
		if err := writeFileAtomically(fileChange.targetFile, fileChange.updatedContents, fileChange.originalPermissions); err != nil {
			if index > 0 {
				return restoreReplacedFiles(fileChange.targetFile, err, replacedFiles)
			}

			return fmt.Errorf("write prepared target file %q: %w", fileChange.targetFile, err)
		}

		replacedFiles = append(replacedFiles, fileChange)
	}

	return nil
}

func (preparedChanges PreparedChanges) preflight() error {
	for _, fileChange := range preparedChanges.fileChanges {
		if err := preflightAtomicWrite(fileChange.targetFile, fileChange.originalPermissions); err != nil {
			return PreflightError{TargetFile: fileChange.targetFile, Err: err}
		}
	}

	return nil
}

func restoreReplacedFiles(failedFile string, writeErr error, replacedFiles []preparedFileChange) error {
	restoredFiles := make([]string, 0, len(replacedFiles))
	unrestoredFiles := make([]string, 0)
	restoreErrs := make([]error, 0)

	for index := len(replacedFiles) - 1; index >= 0; index-- {
		fileChange := replacedFiles[index]
		if err := writeFileAtomically(fileChange.targetFile, fileChange.originalContents, fileChange.originalPermissions); err != nil {
			unrestoredFiles = append(unrestoredFiles, fileChange.targetFile)
			restoreErrs = append(restoreErrs, fmt.Errorf("restore target file %q: %w", fileChange.targetFile, err))
			continue
		}

		restoredFiles = append(restoredFiles, fileChange.targetFile)
	}

	replacedFilePaths := make([]string, 0, len(replacedFiles))
	for _, fileChange := range replacedFiles {
		replacedFilePaths = append(replacedFilePaths, fileChange.targetFile)
	}

	return RecoveryError{
		FailedFile:      failedFile,
		ReplacedFiles:   replacedFilePaths,
		RestoredFiles:   restoredFiles,
		UnrestoredFiles: unrestoredFiles,
		Err:             writeErr,
		RestoreErr:      errors.Join(restoreErrs...),
	}
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

func groupTargetReads(targets []config.Target) ([]targetReadGroup, error) {
	groupIndexesByFile := make(map[string]int)
	groups := make([]targetReadGroup, 0)
	seenLocations := make(map[string]string, len(targets))

	for _, target := range targets {
		normalizedTarget, selector, err := normalizeTarget(target)
		if err != nil {
			return nil, err
		}

		locationKey := targetLocationKey(normalizedTarget, selector)
		if existingTargetName, exists := seenLocations[locationKey]; exists {
			return nil, targetError(normalizedTarget, fmt.Errorf("duplicates target location used by target %q", existingTargetName))
		}
		seenLocations[locationKey] = normalizedTarget.Name

		existingGroupIndex, exists := groupIndexesByFile[normalizedTarget.File]
		if exists {
			existingGroup := &groups[existingGroupIndex]
			if existingGroup.targetType != normalizedTarget.Type {
				return nil, targetError(normalizedTarget, fmt.Errorf("target file already has %q reads queued", existingGroup.targetType))
			}

			existingGroup.targets = append(existingGroup.targets, normalizedTarget)
			continue
		}

		groups = append(groups, targetReadGroup{
			targetFile: normalizedTarget.File,
			targetType: normalizedTarget.Type,
			targets:    []config.Target{normalizedTarget},
		})
		groupIndexesByFile[normalizedTarget.File] = len(groups) - 1
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
		return selectorSegmentsKey(pathSegments), nil
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
		return selectorSegmentsKey(pathSegments), nil
	case config.TargetTypeTOML:
		if target.TOMLPath == "" {
			return "", fmt.Errorf("tomlPath must be set")
		}
		pathSegments, err := config.ParseTOMLPath(target.TOMLPath)
		if err != nil {
			return "", fmt.Errorf("invalid TOML path %q: %w", target.TOMLPath, err)
		}
		return selectorSegmentsKey(pathSegments), nil
	default:
		return "", fmt.Errorf("target type %q is not supported", target.Type)
	}
}

func targetLocationKey(target config.Target, selector string) string {
	return string(target.Type) + "\x00" + target.File + "\x00" + selector
}

func selectorSegmentsKey(pathSegments []string) string {
	var key strings.Builder
	for _, segment := range pathSegments {
		key.WriteString(fmt.Sprintf("%d:%s", len(segment), segment))
	}

	return key.String()
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
		targetFile:          group.targetFile,
		originalContents:    append([]byte(nil), contents...),
		updatedContents:     updatedContents,
		originalPermissions: targetInfo.Mode().Perm(),
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
	currentValues, err := readTargetValuesFromContents(contents, group.targetType, targetChangeTargets(group.changes))
	if err != nil {
		return nil, err
	}

	hunks := make([]ManagedPreviewHunk, 0, len(group.changes))
	for _, change := range group.changes {
		originalValue, ok := currentValues[change.Target.Name]
		if !ok {
			return nil, targetError(change.Target, fmt.Errorf("current value was not read"))
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

func targetChangeTargets(changes []TargetChange) []config.Target {
	targets := make([]config.Target, 0, len(changes))
	for _, change := range changes {
		targets = append(targets, change.Target)
	}

	return targets
}

func readTargetValuesFromContents(contents []byte, targetType config.TargetType, targets []config.Target) (map[string]string, error) {
	switch targetType {
	case config.TargetTypeJSON:
		return readJSONTargetValues(contents, targets)
	case config.TargetTypeDotenv:
		return readDotenvTargetValues(contents, targets)
	case config.TargetTypeYAML:
		return readYAMLTargetValues(contents, targets)
	case config.TargetTypeTOML:
		return readTOMLTargetValues(contents, targets)
	default:
		return nil, fmt.Errorf("target type %q is not supported", targetType)
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
