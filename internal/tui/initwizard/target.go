package initwizard

import (
	"path/filepath"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	manualJSONPathChoiceLabel  = "Enter JSON value path manually"
	manualDotenvKeyChoiceLabel = "Enter dotenv value key manually"
	targetFileChoiceWindowSize = 12
	searchJSONPathsChoiceLabel = "Search JSON values"
	jsonPathChoiceWindowSize   = 12
	dotenvKeyChoiceWindowSize  = 12
)

type targetBrowseLevel struct {
	path  string
	nodes []app.InitStringTargetNode
}

func filterTargetFileCandidates(candidates []app.InitTargetFileCandidate, filterValue string) []app.InitTargetFileCandidate {
	normalizedFilter := normalizeTargetFileFilter(filterValue)
	if normalizedFilter == "" {
		return candidates
	}

	basenameMatches := make([]app.InitTargetFileCandidate, 0)
	pathMatches := make([]app.InitTargetFileCandidate, 0)
	for _, candidate := range candidates {
		relativePath := strings.ToLower(filepath.ToSlash(candidate.RelativePath))
		basename := strings.ToLower(filepath.Base(candidate.RelativePath))

		switch {
		case strings.Contains(basename, normalizedFilter):
			basenameMatches = append(basenameMatches, candidate)
		case strings.Contains(relativePath, normalizedFilter):
			pathMatches = append(pathMatches, candidate)
		}
	}

	return append(basenameMatches, pathMatches...)
}

func normalizeTargetFileFilter(filterValue string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(filterValue), "\\", "/"))
}

func filterDotenvKeys(keys []string, filterValue string) []string {
	normalizedFilter := normalizeTargetFileFilter(filterValue)
	if normalizedFilter == "" {
		return keys
	}

	matchingKeys := make([]string, 0)
	for _, key := range keys {
		if strings.Contains(strings.ToLower(key), normalizedFilter) {
			matchingKeys = append(matchingKeys, key)
		}
	}

	return matchingKeys
}

func flattenSelectableJSONPaths(nodes []app.InitStringTargetNode) []string {
	paths := make([]string, 0)
	for _, node := range nodes {
		if node.Selectable {
			paths = append(paths, node.JSONPath)
		}
		if len(node.Children) > 0 {
			paths = append(paths, flattenSelectableJSONPaths(node.Children)...)
		}
	}

	return paths
}

func filterSelectableJSONPaths(selectablePaths []string, filterValue string) []string {
	normalizedFilter := normalizeTargetFileFilter(filterValue)
	if normalizedFilter == "" {
		return selectablePaths
	}

	leafMatches := make([]string, 0)
	pathMatches := make([]string, 0)
	for _, selectablePath := range selectablePaths {
		normalizedPath := strings.ToLower(selectablePath)
		leafName := normalizedPath
		if lastSeparatorIndex := strings.LastIndex(normalizedPath, "."); lastSeparatorIndex >= 0 {
			leafName = normalizedPath[lastSeparatorIndex+1:]
		}

		switch {
		case strings.Contains(leafName, normalizedFilter):
			leafMatches = append(leafMatches, selectablePath)
		case strings.Contains(normalizedPath, normalizedFilter):
			pathMatches = append(pathMatches, selectablePath)
		}
	}

	return append(leafMatches, pathMatches...)
}

func targetNodeChoiceLabel(node app.InitStringTargetNode) string {
	if node.Selectable {
		return node.Name
	}

	return node.Name + "/"
}
