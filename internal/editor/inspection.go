package editor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
)

var skippedDiscoveryDirectoryNames = map[string]struct{}{
	"bin":              {},
	"bower_components": {},
	"build":            {},
	"coverage":         {},
	"dist":             {},
	"generated":        {},
	"node_modules":     {},
	"obj":              {},
	"out":              {},
	"target":           {},
	"vendor":           {},
}

// DiscoverTargetFileCandidates returns the files under projectRoot that contain
// at least one existing selector Switchlet can manage.
func DiscoverTargetFileCandidates(projectRoot string) ([]TargetFileCandidate, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, fmt.Errorf("project root must be set")
	}

	resolvedProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root %q: %w", projectRoot, err)
	}

	projectRootInfo, err := os.Stat(resolvedProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("stat project root %q: %w", resolvedProjectRoot, err)
	}
	if !projectRootInfo.IsDir() {
		return nil, fmt.Errorf("project root %q is not a directory", resolvedProjectRoot)
	}

	candidates := make([]TargetFileCandidate, 0)
	err = filepath.WalkDir(resolvedProjectRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if path != resolvedProjectRoot && shouldSkipDiscoveryDirectory(entry) {
				return filepath.SkipDir
			}

			return nil
		}

		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		targetType, ok := inferDiscoveryTargetType(path)
		if !ok {
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if !hasInspectableTargetSelectors(contents, targetType) {
			return nil
		}

		relativePath, err := filepath.Rel(resolvedProjectRoot, path)
		if err != nil {
			relativePath = path
		}

		candidates = append(candidates, TargetFileCandidate{
			Path:         filepath.Clean(path),
			RelativePath: filepath.Clean(relativePath),
			Type:         targetType,
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover target files under %q: %w", resolvedProjectRoot, err)
	}

	sort.Slice(candidates, func(leftIndex int, rightIndex int) bool {
		leftRank := discoveryCandidateRank(candidates[leftIndex])
		rightRank := discoveryCandidateRank(candidates[rightIndex])
		if leftRank != rightRank {
			return leftRank < rightRank
		}

		leftPath := candidates[leftIndex].RelativePath
		rightPath := candidates[rightIndex].RelativePath

		leftDepth := pathDepth(leftPath)
		rightDepth := pathDepth(rightPath)
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}

		return leftPath < rightPath
	})

	return candidates, nil
}

func inferDiscoveryTargetType(path string) (config.TargetType, bool) {
	if targetType, ok := config.InferTargetType(path); ok {
		return targetType, true
	}

	fileName := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(fileName, ".env") {
		return config.TargetTypeDotenv, true
	}

	return "", false
}

// InspectStringTargets returns a hierarchical view of the selectable existing
// string-valued JSON paths inside targetPath.
func InspectStringTargets(targetPath string) ([]StringTargetNode, error) {
	contents, _, err := readTargetFile(targetPath)
	if err != nil {
		return nil, err
	}

	nodes, err := inspectStringTargetsContents(contents)
	if err != nil {
		return nil, fmt.Errorf("inspect target file %q: %w", targetPath, err)
	}

	return nodes, nil
}

// InspectDotenvKeys returns the existing unambiguous dotenv keys inside
// targetPath.
func InspectDotenvKeys(targetPath string) ([]string, error) {
	contents, _, err := readTargetFile(targetPath)
	if err != nil {
		return nil, err
	}

	keys, err := inspectDotenvKeysContents(contents)
	if err != nil {
		return nil, fmt.Errorf("inspect target file %q: %w", targetPath, err)
	}

	return keys, nil
}

// InspectTOMLStringTargets returns a hierarchical view of selectable existing
// string-valued TOML paths inside targetPath.
func InspectTOMLStringTargets(targetPath string) ([]TOMLStringTargetNode, error) {
	contents, _, err := readTargetFile(targetPath)
	if err != nil {
		return nil, err
	}

	nodes, err := inspectTOMLStringTargetsContents(contents)
	if err != nil {
		return nil, fmt.Errorf("inspect target file %q: %w", targetPath, err)
	}

	return nodes, nil
}

func hasInspectableTargetSelectors(contents []byte, targetType config.TargetType) bool {
	switch targetType {
	case config.TargetTypeJSON:
		nodes, err := inspectStringTargetsContents(contents)
		return err == nil && len(nodes) > 0
	case config.TargetTypeDotenv:
		keys, err := inspectDotenvKeysContents(contents)
		return err == nil && len(keys) > 0
	case config.TargetTypeYAML:
		nodes, err := inspectYAMLStringTargetsContents(contents)
		return err == nil && len(nodes) > 0
	case config.TargetTypeTOML:
		nodes, err := inspectTOMLStringTargetsContents(contents)
		return err == nil && len(nodes) > 0
	default:
		return false
	}
}

func inspectStringTargetsContents(contents []byte) ([]StringTargetNode, error) {
	rootObject, err := parseRootObject(contents)
	if err != nil {
		return nil, err
	}

	nodes := buildStringTargetNodes(rootObject, nil)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("does not contain any existing string-valued JSON paths")
	}

	return nodes, nil
}

func inspectDotenvKeysContents(contents []byte) ([]string, error) {
	lines := splitDotenvLines(contents)
	assignments, err := parseDotenvAssignments(lines)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(assignments))
	for key, lineIndexes := range assignments {
		if len(lineIndexes) == 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return nil, fmt.Errorf("does not contain any unambiguous dotenv keys")
	}

	return keys, nil
}

func shouldSkipDiscoveryDirectory(entry fs.DirEntry) bool {
	if strings.HasPrefix(entry.Name(), ".") {
		return true
	}

	_, shouldSkip := skippedDiscoveryDirectoryNames[strings.ToLower(entry.Name())]
	return shouldSkip
}

func discoveryCandidateRank(candidate TargetFileCandidate) int {
	fileName := strings.ToLower(filepath.Base(candidate.RelativePath))
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	if isLikelyEnvironmentFile(fileName, candidate.Type) || isLikelyAppSettingsFile(fileName, candidate.Type) {
		return 0
	}
	if isCommonApplicationConfigName(baseName) {
		return 1
	}
	if isInConfigDirectory(candidate.RelativePath) {
		return 2
	}

	return 3
}

func isLikelyEnvironmentFile(fileName string, targetType config.TargetType) bool {
	if targetType != config.TargetTypeDotenv {
		return false
	}

	return fileName == ".env" || strings.HasPrefix(fileName, ".env.") || strings.HasSuffix(fileName, ".env")
}

func isLikelyAppSettingsFile(fileName string, targetType config.TargetType) bool {
	return targetType == config.TargetTypeJSON && strings.HasPrefix(fileName, "appsettings") && strings.HasSuffix(fileName, ".json")
}

func isCommonApplicationConfigName(baseName string) bool {
	commonNames := []string{"application", "config", "development", "local", "production", "settings", "staging", "test"}
	for _, name := range commonNames {
		if baseName == name || strings.HasPrefix(baseName, name+".") || strings.HasPrefix(baseName, name+"-") {
			return true
		}
	}

	return false
}

func isInConfigDirectory(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.EqualFold(segment, "config") || strings.EqualFold(segment, "configs") || strings.EqualFold(segment, "configuration") {
			return true
		}
	}

	return false
}

func pathDepth(path string) int {
	return strings.Count(path, string(filepath.Separator))
}
