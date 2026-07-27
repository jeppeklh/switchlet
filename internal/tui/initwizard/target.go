package initwizard

import (
	"path/filepath"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	manualJSONPathChoiceLabel  = "Enter JSON value path manually"
	manualYAMLPathChoiceLabel  = "Enter YAML value path manually"
	manualDotenvKeyChoiceLabel = "Enter dotenv value key manually"
	targetFileChoiceWindowSize = 12
	searchJSONPathsChoiceLabel = "Search JSON values"
	searchYAMLPathsChoiceLabel = "Search YAML values"
	jsonPathChoiceWindowSize   = 12
	dotenvKeyChoiceWindowSize  = 12
)

type targetSelectorNode struct {
	name       string
	selector   string
	selectable bool
	children   []targetSelectorNode
}

type targetBrowseLevel struct {
	path  string
	nodes []targetSelectorNode
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

func flattenSelectableTargetPaths(nodes []targetSelectorNode) []string {
	paths := make([]string, 0)
	for _, node := range nodes {
		if node.selectable {
			paths = append(paths, node.selector)
		}
		if len(node.children) > 0 {
			paths = append(paths, flattenSelectableTargetPaths(node.children)...)
		}
	}

	return paths
}

func filterSelectableTargetPaths(selectablePaths []string, filterValue string) []string {
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

func targetNodeChoiceLabel(node targetSelectorNode) string {
	if node.selectable {
		return node.name
	}

	return node.name + "/"
}

func targetSelectorNodesForSelection(selection app.InitTargetFileSelection) []targetSelectorNode {
	if selection.TargetType == app.InitTargetTypeYAML {
		return yamlTargetSelectorNodes(selection.YAMLNodes)
	}

	return jsonTargetSelectorNodes(selection.Nodes)
}

func jsonTargetSelectorNodes(nodes []app.InitStringTargetNode) []targetSelectorNode {
	convertedNodes := make([]targetSelectorNode, 0, len(nodes))
	for _, node := range nodes {
		convertedNodes = append(convertedNodes, targetSelectorNode{
			name:       node.Name,
			selector:   node.JSONPath,
			selectable: node.Selectable,
			children:   jsonTargetSelectorNodes(node.Children),
		})
	}

	return convertedNodes
}

func yamlTargetSelectorNodes(nodes []app.InitYAMLStringTargetNode) []targetSelectorNode {
	convertedNodes := make([]targetSelectorNode, 0, len(nodes))
	for _, node := range nodes {
		convertedNodes = append(convertedNodes, targetSelectorNode{
			name:       node.Name,
			selector:   node.YAMLPath,
			selectable: node.Selectable,
			children:   yamlTargetSelectorNodes(node.Children),
		})
	}

	return convertedNodes
}

func structuredValueFormatName(targetType app.InitTargetType) string {
	if targetType == app.InitTargetTypeYAML {
		return "YAML"
	}

	return "JSON"
}

func structuredPathSummaryLabel(targetType app.InitTargetType) string {
	return structuredValueFormatName(targetType) + " path"
}

func structuredPathInputLabel(targetType app.InitTargetType) string {
	return structuredValueFormatName(targetType) + " value path"
}

func manualStructuredPathChoiceLabel(targetType app.InitTargetType) string {
	if targetType == app.InitTargetTypeYAML {
		return manualYAMLPathChoiceLabel
	}

	return manualJSONPathChoiceLabel
}

func manualStructuredPathActionLabel(targetType app.InitTargetType) string {
	if targetType == app.InitTargetTypeYAML {
		return "Manual yamlPath"
	}

	return "Manual path"
}

func searchStructuredPathsChoiceLabel(targetType app.InitTargetType) string {
	if targetType == app.InitTargetTypeYAML {
		return searchYAMLPathsChoiceLabel
	}

	return searchJSONPathsChoiceLabel
}

func structuredBrowseNestingGuidance(targetType app.InitTargetType) string {
	if targetType == app.InitTargetTypeYAML {
		return "Rows ending in / open nested mappings."
	}

	return "Rows ending in / open nested objects."
}
