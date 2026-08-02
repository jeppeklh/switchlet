package initwizard

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	manualJSONPathChoiceLabel  = "Enter JSON value path manually"
	manualYAMLPathChoiceLabel  = "Enter YAML value path manually"
	manualTOMLPathChoiceLabel  = "Enter TOML value path manually"
	manualDotenvKeyChoiceLabel = "Enter dotenv value key manually"
	targetFileChoiceWindowSize = 12
	searchJSONPathsChoiceLabel = "Search JSON values"
	searchYAMLPathsChoiceLabel = "Search YAML values"
	searchTOMLPathsChoiceLabel = "Search TOML values"
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
	filterTerms := searchFilterTerms(normalizedFilter)

	exactBasenameMatches := make([]app.InitTargetFileCandidate, 0)
	exactPathMatches := make([]app.InitTargetFileCandidate, 0)
	termBasenameMatches := make([]app.InitTargetFileCandidate, 0)
	termPathMatches := make([]app.InitTargetFileCandidate, 0)
	for _, candidate := range candidates {
		relativePath := strings.ToLower(filepath.ToSlash(candidate.RelativePath))
		basename := strings.ToLower(filepath.Base(candidate.RelativePath))

		switch {
		case strings.Contains(basename, normalizedFilter):
			exactBasenameMatches = append(exactBasenameMatches, candidate)
		case strings.Contains(relativePath, normalizedFilter):
			exactPathMatches = append(exactPathMatches, candidate)
		case searchTermsMatch(basename, filterTerms):
			termBasenameMatches = append(termBasenameMatches, candidate)
		case searchTermsMatch(relativePath, filterTerms):
			termPathMatches = append(termPathMatches, candidate)
		}
	}

	matches := append(exactBasenameMatches, exactPathMatches...)
	matches = append(matches, termBasenameMatches...)
	return append(matches, termPathMatches...)
}

func normalizeTargetFileFilter(filterValue string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(filterValue), "\\", "/"))
}

func searchFilterTerms(normalizedFilter string) []string {
	return strings.FieldsFunc(normalizedFilter, func(value rune) bool {
		return unicode.IsSpace(value) || value == '/' || value == '\\'
	})
}

func searchTermsMatch(value string, terms []string) bool {
	if len(terms) == 0 {
		return false
	}

	for _, term := range terms {
		if !strings.Contains(value, term) {
			return false
		}
	}

	return true
}

func filterDotenvKeys(keys []string, filterValue string) []string {
	normalizedFilter := normalizeTargetFileFilter(filterValue)
	if normalizedFilter == "" {
		return keys
	}
	filterTerms := searchFilterTerms(normalizedFilter)

	exactMatches := make([]string, 0)
	termMatches := make([]string, 0)
	for _, key := range keys {
		normalizedKey := strings.ToLower(key)
		switch {
		case strings.Contains(normalizedKey, normalizedFilter):
			exactMatches = append(exactMatches, key)
		case searchTermsMatch(normalizedKey, filterTerms):
			termMatches = append(termMatches, key)
		}
	}

	return append(exactMatches, termMatches...)
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
	filterTerms := searchFilterTerms(normalizedFilter)

	exactLeafMatches := make([]string, 0)
	exactPathMatches := make([]string, 0)
	termLeafMatches := make([]string, 0)
	termPathMatches := make([]string, 0)
	for _, selectablePath := range selectablePaths {
		normalizedPath := normalizeTargetFileFilter(selectablePath)
		leafName := normalizedPath
		if lastSeparatorIndex := strings.LastIndex(normalizedPath, "."); lastSeparatorIndex >= 0 {
			leafName = normalizedPath[lastSeparatorIndex+1:]
		}

		switch {
		case strings.Contains(leafName, normalizedFilter):
			exactLeafMatches = append(exactLeafMatches, selectablePath)
		case strings.Contains(normalizedPath, normalizedFilter):
			exactPathMatches = append(exactPathMatches, selectablePath)
		case searchTermsMatch(leafName, filterTerms):
			termLeafMatches = append(termLeafMatches, selectablePath)
		case searchTermsMatch(normalizedPath, filterTerms):
			termPathMatches = append(termPathMatches, selectablePath)
		}
	}

	matches := append(exactLeafMatches, exactPathMatches...)
	matches = append(matches, termLeafMatches...)
	return append(matches, termPathMatches...)
}

func targetNodeChoiceLabel(node targetSelectorNode) string {
	if node.selectable {
		return node.name
	}

	return node.name + "/"
}

func targetSelectorNodesForSelection(selection app.InitTargetFileSelection) []targetSelectorNode {
	switch selection.TargetType {
	case app.InitTargetTypeYAML:
		return yamlTargetSelectorNodes(selection.YAMLNodes)
	case app.InitTargetTypeTOML:
		return tomlTargetSelectorNodes(selection.TOMLNodes)
	default:
		return jsonTargetSelectorNodes(selection.Nodes)
	}
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

func tomlTargetSelectorNodes(nodes []app.InitTOMLStringTargetNode) []targetSelectorNode {
	convertedNodes := make([]targetSelectorNode, 0, len(nodes))
	for _, node := range nodes {
		convertedNodes = append(convertedNodes, targetSelectorNode{
			name:       node.Name,
			selector:   node.TOMLPath,
			selectable: node.Selectable,
			children:   tomlTargetSelectorNodes(node.Children),
		})
	}

	return convertedNodes
}

func structuredValueFormatName(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeYAML:
		return "YAML"
	case app.InitTargetTypeTOML:
		return "TOML"
	default:
		return "JSON"
	}
}

func structuredPathSummaryLabel(targetType app.InitTargetType) string {
	return structuredValueFormatName(targetType) + " path"
}

func structuredPathInputLabel(targetType app.InitTargetType) string {
	return structuredValueFormatName(targetType) + " value path"
}

func manualStructuredPathChoiceLabel(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeYAML:
		return manualYAMLPathChoiceLabel
	case app.InitTargetTypeTOML:
		return manualTOMLPathChoiceLabel
	default:
		return manualJSONPathChoiceLabel
	}
}

func manualStructuredPathActionLabel(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeYAML:
		return "Manual yamlPath"
	case app.InitTargetTypeTOML:
		return "Manual tomlPath"
	default:
		return "Manual path"
	}
}

func searchStructuredPathsChoiceLabel(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeYAML:
		return searchYAMLPathsChoiceLabel
	case app.InitTargetTypeTOML:
		return searchTOMLPathsChoiceLabel
	default:
		return searchJSONPathsChoiceLabel
	}
}

func structuredBrowseNestingGuidance(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeYAML:
		return "Rows ending in / open nested mappings."
	case app.InitTargetTypeTOML:
		return "Rows ending in / open nested tables."
	default:
		return "Rows ending in / open nested objects."
	}
}
