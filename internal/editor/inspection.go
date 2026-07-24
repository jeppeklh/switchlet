package editor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var skippedDiscoveryDirectoryNames = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	"bin":          {},
	"obj":          {},
}

// DiscoverTargetFileCandidates returns the JSON files under projectRoot that
// contain at least one existing string-valued JSON path Switchlet can manage.
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
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		nodes, err := inspectStringTargetsContents(contents)
		if err != nil || len(nodes) == 0 {
			return nil
		}

		relativePath, err := filepath.Rel(resolvedProjectRoot, path)
		if err != nil {
			relativePath = path
		}

		candidates = append(candidates, TargetFileCandidate{
			Path:         filepath.Clean(path),
			RelativePath: filepath.Clean(relativePath),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover target files under %q: %w", resolvedProjectRoot, err)
	}

	sort.Slice(candidates, func(leftIndex int, rightIndex int) bool {
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

func shouldSkipDiscoveryDirectory(entry fs.DirEntry) bool {
	if strings.HasPrefix(entry.Name(), ".") {
		return true
	}

	_, shouldSkip := skippedDiscoveryDirectoryNames[strings.ToLower(entry.Name())]
	return shouldSkip
}

func pathDepth(path string) int {
	return strings.Count(path, string(filepath.Separator))
}
