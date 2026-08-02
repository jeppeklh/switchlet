package main

import (
	"fmt"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
)

const (
	manualJSONPathChoiceLabel   = "Enter JSON value path manually"
	manualYAMLPathChoiceLabel   = "Enter YAML value path manually"
	manualTOMLPathChoiceLabel   = "Enter TOML value path manually"
	manualDotenvKeyChoiceLabel  = "Enter dotenv value key manually"
	goBackChoiceLabel           = "Back up one level"
	searchJSONPathsChoiceLabel  = "Search JSON values"
	refineJSONPathSearchLabel   = "Refine path search"
	clearJSONPathSearchLabel    = "Clear path search"
	browseJSONPathsChoiceLabel  = "Browse JSON path hierarchy"
	jsonPathChoiceWindowSize    = 12
	searchYAMLPathsChoiceLabel  = "Search YAML values"
	browseYAMLPathsChoiceLabel  = "Browse YAML path hierarchy"
	searchTOMLPathsChoiceLabel  = "Search TOML values"
	browseTOMLPathsChoiceLabel  = "Browse TOML path hierarchy"
	filterDotenvKeysChoiceLabel = "Filter dotenv keys"
	dotenvKeyChoiceWindowSize   = 12
)

type targetBrowseLevel struct {
	path  string
	nodes []targetSelectorNode
}

type targetPathSearchResult struct {
	selector            string
	returnToHierarchy   bool
	chooseDifferentFile bool
}

func promptTargetSelector(prompter initPrompter, selectedFile targetFileSelection, dependencies initDependencies) (string, bool, error) {
	switch selectedFile.targetType {
	case config.TargetTypeJSON:
		return promptTargetStructuredPath(prompter, selectedFile, dependencies)
	case config.TargetTypeYAML:
		return promptTargetStructuredPath(prompter, selectedFile, dependencies)
	case config.TargetTypeTOML:
		return promptTargetStructuredPath(prompter, selectedFile, dependencies)
	case config.TargetTypeDotenv:
		return promptTargetDotenvKey(prompter, selectedFile, dependencies)
	default:
		return "", false, fmt.Errorf("target type %q is not supported", selectedFile.targetType)
	}
}

func promptTargetDotenvKey(prompter initPrompter, selectedFile targetFileSelection, dependencies initDependencies) (string, bool, error) {
	filterValue := ""
	for {
		matchingKeys := filterDotenvKeys(selectedFile.dotenvKeys, filterValue)
		if len(matchingKeys) == 0 {
			if _, err := fmt.Fprintf(prompter.writer, "No unambiguous dotenv keys in %s match %q.\n", selectedFile.displayPath, filterValue); err != nil {
				return "", false, err
			}
			var err error
			filterValue, err = prompter.promptLine("Filter dotenv keys by name (blank clears the filter): ")
			if err != nil {
				return "", false, err
			}
			continue
		}

		visibleKeys := matchingKeys
		truncated := false
		if len(visibleKeys) > dotenvKeyChoiceWindowSize {
			visibleKeys = visibleKeys[:dotenvKeyChoiceWindowSize]
			truncated = true
		}

		choices := make([]string, 0, len(visibleKeys)+4)
		choices = append(choices, visibleKeys...)
		showFilterAction := len(selectedFile.dotenvKeys) > dotenvKeyChoiceWindowSize || filterValue != ""
		if showFilterAction {
			choices = append(choices, filterDotenvKeysChoiceLabel)
		}
		if filterValue != "" {
			choices = append(choices, clearTargetFileFilterLabel)
		}
		choices = append(choices, manualDotenvKeyChoiceLabel, chooseDifferentFileLabel)

		choiceIndex, err := prompter.promptChoiceIndex(targetDotenvKeyPrompt(selectedFile.displayPath, filterValue, len(visibleKeys), len(matchingKeys), truncated), choices)
		if err != nil {
			return "", false, err
		}

		if choiceIndex < len(visibleKeys) {
			return visibleKeys[choiceIndex], false, nil
		}

		nextActionIndex := len(visibleKeys)
		if showFilterAction {
			if choiceIndex == nextActionIndex {
				filterValue, err = prompter.promptLine("Filter dotenv keys by name (blank clears the filter): ")
				if err != nil {
					return "", false, err
				}
				continue
			}
			nextActionIndex++
		}

		if filterValue != "" {
			if choiceIndex == nextActionIndex {
				filterValue = ""
				continue
			}
			nextActionIndex++
		}

		if choiceIndex == nextActionIndex {
			key, err := prompter.promptNonEmptyLine("Dotenv value key: ")
			if err != nil {
				return "", false, err
			}
			if err := dependencies.validateDotenvTarget(selectedFile.path, key); err != nil {
				if err := writePromptError(prompter, err); err != nil {
					return "", false, err
				}
				continue
			}

			return key, false, nil
		}

		return "", true, nil
	}
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

func targetDotenvKeyPrompt(displayPath string, filterValue string, visibleCount int, matchingCount int, truncated bool) string {
	if filterValue != "" {
		if truncated {
			return fmt.Sprintf("Select dotenv value key matching %q in %s (showing %d of %d matches):", filterValue, displayPath, visibleCount, matchingCount)
		}

		return fmt.Sprintf("Select dotenv value key matching %q in %s:", filterValue, displayPath)
	}

	if truncated {
		return fmt.Sprintf("Select dotenv value key in %s (showing %d of %d unique keys):", displayPath, visibleCount, matchingCount)
	}

	return fmt.Sprintf("Select dotenv value key in %s:", displayPath)
}

func promptTargetJSONPath(prompter initPrompter, selectedFile targetFileSelection, dependencies initDependencies) (string, bool, error) {
	selectedFile.targetType = config.TargetTypeJSON
	return promptTargetStructuredPath(prompter, selectedFile, dependencies)
}

func promptTargetStructuredPath(prompter initPrompter, selectedFile targetFileSelection, dependencies initDependencies) (string, bool, error) {
	currentNodes := selectedFile.nodes
	ancestors := make([]targetBrowseLevel, 0)
	selectablePaths := flattenSelectableTargetPaths(selectedFile.nodes)
	showSearchAction := len(selectablePaths) > jsonPathChoiceWindowSize
	formatName := targetTypeDisplayName(selectedFile.targetType)

	for {
		prompt := fmt.Sprintf("Choose %s value in %s:", formatName, selectedFile.displayPath)
		if len(ancestors) > 0 {
			prompt = fmt.Sprintf("Choose %s value under %s in %s:", formatName, ancestors[len(ancestors)-1].path, selectedFile.displayPath)
		}

		choices := make([]string, 0, len(currentNodes)+4)
		for _, node := range currentNodes {
			choices = append(choices, targetNodeChoiceLabel(node))
		}
		if len(ancestors) > 0 {
			choices = append(choices, goBackChoiceLabel)
		}
		if showSearchAction {
			choices = append(choices, searchStructuredPathsChoiceLabel(selectedFile.targetType))
		}
		choices = append(choices, manualStructuredPathChoiceLabel(selectedFile.targetType), chooseDifferentFileLabel)

		choiceIndex, err := prompter.promptChoiceIndex(prompt, choices)
		if err != nil {
			return "", false, err
		}

		if choiceIndex < len(currentNodes) {
			selectedNode := currentNodes[choiceIndex]
			if selectedNode.selectable {
				return selectedNode.selector, false, nil
			}

			ancestors = append(ancestors, targetBrowseLevel{
				path:  selectedNode.selector,
				nodes: currentNodes,
			})
			currentNodes = selectedNode.children
			continue
		}

		nextActionIndex := len(currentNodes)
		if len(ancestors) > 0 {
			if choiceIndex == nextActionIndex {
				previousLevel := ancestors[len(ancestors)-1]
				ancestors = ancestors[:len(ancestors)-1]
				currentNodes = previousLevel.nodes
				continue
			}
			nextActionIndex++
		}

		if showSearchAction {
			if choiceIndex == nextActionIndex {
				searchResult, err := promptTargetStructuredPathSearch(prompter, selectedFile, selectablePaths, dependencies)
				if err != nil {
					return "", false, err
				}
				if searchResult.chooseDifferentFile {
					return "", true, nil
				}
				if searchResult.returnToHierarchy {
					continue
				}

				return searchResult.selector, false, nil
			}
			nextActionIndex++
		}

		if choiceIndex == nextActionIndex {
			selector, err := prompter.promptNonEmptyLine(manualStructuredPathInputPrompt(selectedFile.targetType))
			if err != nil {
				return "", false, err
			}

			if err := validateStructuredTargetSelector(dependencies, selectedFile.targetType, selectedFile.path, selector); err != nil {
				if err := writePromptError(prompter, err); err != nil {
					return "", false, err
				}
				continue
			}

			return selector, false, nil
		}

		return "", true, nil
	}
}

func promptTargetStructuredPathSearch(prompter initPrompter, selectedFile targetFileSelection, selectablePaths []string, dependencies initDependencies) (targetPathSearchResult, error) {
	filterValue := ""
	formatName := targetTypeDisplayName(selectedFile.targetType)

	for {
		if filterValue == "" {
			nextFilterValue, err := prompter.promptLine(fmt.Sprintf("Search %s values by name or path (blank returns to browsing): ", formatName))
			if err != nil {
				return targetPathSearchResult{}, err
			}
			if strings.TrimSpace(nextFilterValue) == "" {
				return targetPathSearchResult{returnToHierarchy: true}, nil
			}

			filterValue = nextFilterValue
		}

		matchingPaths := filterSelectableTargetPaths(selectablePaths, filterValue)
		if len(matchingPaths) == 0 {
			if _, err := fmt.Fprintf(prompter.writer, "No selectable %s paths in %s match %q.\n", formatName, selectedFile.displayPath, filterValue); err != nil {
				return targetPathSearchResult{}, err
			}
			filterValue = ""
			continue
		}

		visiblePaths := matchingPaths
		truncated := false
		if len(visiblePaths) > jsonPathChoiceWindowSize {
			visiblePaths = visiblePaths[:jsonPathChoiceWindowSize]
			truncated = true
		}

		choices := make([]string, 0, len(visiblePaths)+5)
		choices = append(choices, visiblePaths...)
		choices = append(choices, refineJSONPathSearchLabel, clearJSONPathSearchLabel, manualStructuredPathChoiceLabel(selectedFile.targetType), browseStructuredPathsChoiceLabel(selectedFile.targetType), chooseDifferentFileLabel)

		choiceIndex, err := prompter.promptChoiceIndex(targetStructuredPathSearchPrompt(formatName, selectedFile.displayPath, filterValue, len(visiblePaths), len(matchingPaths), truncated), choices)
		if err != nil {
			return targetPathSearchResult{}, err
		}

		if choiceIndex < len(visiblePaths) {
			return targetPathSearchResult{selector: visiblePaths[choiceIndex]}, nil
		}

		nextActionIndex := len(visiblePaths)
		if choiceIndex == nextActionIndex {
			filterValue = ""
			continue
		}
		nextActionIndex++

		if choiceIndex == nextActionIndex {
			filterValue = ""
			continue
		}
		nextActionIndex++

		if choiceIndex == nextActionIndex {
			selector, err := prompter.promptNonEmptyLine(manualStructuredPathInputPrompt(selectedFile.targetType))
			if err != nil {
				return targetPathSearchResult{}, err
			}

			if err := validateStructuredTargetSelector(dependencies, selectedFile.targetType, selectedFile.path, selector); err != nil {
				if err := writePromptError(prompter, err); err != nil {
					return targetPathSearchResult{}, err
				}
				continue
			}

			return targetPathSearchResult{selector: selector}, nil
		}
		nextActionIndex++

		if choiceIndex == nextActionIndex {
			return targetPathSearchResult{returnToHierarchy: true}, nil
		}

		return targetPathSearchResult{chooseDifferentFile: true}, nil
	}
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

func targetStructuredPathSearchPrompt(formatName string, displayPath string, filterValue string, visibleCount int, matchingCount int, truncated bool) string {
	if truncated {
		return fmt.Sprintf("Select %s value matching %q in %s (showing %d of %d matches):", formatName, filterValue, displayPath, visibleCount, matchingCount)
	}

	return fmt.Sprintf("Select %s value matching %q in %s:", formatName, filterValue, displayPath)
}

func targetNodeChoiceLabel(node targetSelectorNode) string {
	if node.selectable {
		return node.name
	}

	return node.name + "/"
}

func searchStructuredPathsChoiceLabel(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeYAML:
		return searchYAMLPathsChoiceLabel
	case config.TargetTypeTOML:
		return searchTOMLPathsChoiceLabel
	}

	return searchJSONPathsChoiceLabel
}

func manualStructuredPathChoiceLabel(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeYAML:
		return manualYAMLPathChoiceLabel
	case config.TargetTypeTOML:
		return manualTOMLPathChoiceLabel
	}

	return manualJSONPathChoiceLabel
}

func browseStructuredPathsChoiceLabel(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeYAML:
		return browseYAMLPathsChoiceLabel
	case config.TargetTypeTOML:
		return browseTOMLPathsChoiceLabel
	}

	return browseJSONPathsChoiceLabel
}

func manualStructuredPathInputPrompt(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeYAML:
		return "YAML value path: "
	case config.TargetTypeTOML:
		return "TOML value path: "
	}

	return "JSON value path: "
}

func validateStructuredTargetSelector(dependencies initDependencies, targetType config.TargetType, targetPath string, selector string) error {
	switch targetType {
	case config.TargetTypeYAML:
		return dependencies.validateYAMLTarget(targetPath, selector)
	case config.TargetTypeTOML:
		return dependencies.validateTOMLTarget(targetPath, selector)
	}

	return dependencies.validateStringTarget(targetPath, selector)
}
