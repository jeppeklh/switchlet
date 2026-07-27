package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

const (
	manualTargetFileChoiceLabel  = "Enter configuration file manually"
	manualJSONPathChoiceLabel    = "Enter JSON value path manually"
	manualYAMLPathChoiceLabel    = "Enter YAML value path manually"
	manualDotenvKeyChoiceLabel   = "Enter dotenv value key manually"
	chooseDifferentFileLabel     = "Back to file selection"
	goBackChoiceLabel            = "Back up one level"
	filterTargetFilesChoiceLabel = "Filter configuration files"
	clearTargetFileFilterLabel   = "Clear filter"
	targetFileChoiceWindowSize   = 12
	searchJSONPathsChoiceLabel   = "Search JSON values"
	refineJSONPathSearchLabel    = "Refine path search"
	clearJSONPathSearchLabel     = "Clear path search"
	browseJSONPathsChoiceLabel   = "Browse JSON path hierarchy"
	jsonPathChoiceWindowSize     = 12
	searchYAMLPathsChoiceLabel   = "Search YAML values"
	browseYAMLPathsChoiceLabel   = "Browse YAML path hierarchy"
	filterDotenvKeysChoiceLabel  = "Filter dotenv keys"
	dotenvKeyChoiceWindowSize    = 12
)

type targetFileSelection struct {
	path        string
	displayPath string
	targetType  config.TargetType
	nodes       []targetSelectorNode
	dotenvKeys  []string
}

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

type targetPathSearchResult struct {
	selector            string
	returnToHierarchy   bool
	chooseDifferentFile bool
}

func promptTarget(prompter initPrompter, workingDirectory string, dependencies initDependencies) (config.Target, error) {
	targets, err := promptTargets(prompter, workingDirectory, dependencies)
	if err != nil {
		return config.Target{}, err
	}

	return targets[0], nil
}

func promptTargets(prompter initPrompter, workingDirectory string, dependencies initDependencies) ([]config.Target, error) {
	targets := make([]config.Target, 0, 1)
	seenNames := make(map[string]struct{})

	for {
		target, err := promptNamedTarget(prompter, workingDirectory, seenNames, dependencies)
		if err != nil {
			return nil, err
		}

		targets = append(targets, target)
		seenNames[target.Name] = struct{}{}

		addAnother, err := prompter.promptYesNo(formatYesNoPrompt("Add another managed value?", false), false)
		if err != nil {
			return nil, err
		}
		if !addAnother {
			return targets, nil
		}

		if err := writeInitStep(prompter.writer, 1, "Choose configuration file",
			"Pick the next JSON, YAML, or dotenv file containing a value Switchlet should manage.",
			"You can also enter a file path manually.",
		); err != nil {
			return nil, err
		}
	}
}

func promptNamedTarget(prompter initPrompter, workingDirectory string, seenNames map[string]struct{}, dependencies initDependencies) (config.Target, error) {
	for {
		selectedFile, err := promptTargetFile(prompter, workingDirectory, dependencies)
		if err != nil {
			return config.Target{}, err
		}

		if err := writeInitStep(prompter.writer, 2, targetSelectorStepTitle(selectedFile.targetType),
			fmt.Sprintf("Selected file: %s", selectedFile.displayPath),
			fmt.Sprintf("Detected format: %s", targetTypeDisplayName(selectedFile.targetType)),
			targetSelectorStepGuidance(selectedFile.targetType),
		); err != nil {
			return config.Target{}, err
		}

		selector, chooseDifferentFile, err := promptTargetSelector(prompter, selectedFile, dependencies)
		if err != nil {
			return config.Target{}, err
		}
		if chooseDifferentFile {
			continue
		}

		if err := writeInitStep(prompter.writer, 3, "Name this managed value",
			fmt.Sprintf("Selected file: %s", selectedFile.displayPath),
			fmt.Sprintf("Selected value: %s", selector),
			"Profiles refer to this short name.",
		); err != nil {
			return config.Target{}, err
		}

		name, err := promptTargetName(prompter, seenNames)
		if err != nil {
			return config.Target{}, err
		}

		target := config.Target{
			Name: name,
			File: selectedFile.path,
			Type: selectedFile.targetType,
		}
		switch selectedFile.targetType {
		case config.TargetTypeDotenv:
			target.Key = selector
		case config.TargetTypeYAML:
			target.YAMLPath = selector
		default:
			target.JSONPath = selector
		}

		return target, nil
	}
}

func targetSelectorStepTitle(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeDotenv:
		return "Choose dotenv value"
	case config.TargetTypeYAML:
		return "Choose YAML value"
	default:
		return "Choose JSON value"
	}
}

func targetSelectorStepGuidance(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeDotenv:
		return "Choose an existing dotenv key that appears once. Switchlet does not create missing keys."
	case config.TargetTypeYAML:
		return "Choose an existing string-valued YAML path. Switchlet does not create missing values. Browse the mapping hierarchy, search when the file has many selectable values, or enter a path manually."
	default:
		return "Choose an existing string-valued JSON path. Switchlet does not create missing values. Browse the hierarchy, search when the file has many selectable values, or enter a path manually."
	}
}

func targetSelectorLabel(target config.Target) (string, string) {
	switch target.Type {
	case config.TargetTypeDotenv:
		return "Key", target.Key
	case config.TargetTypeYAML:
		return "YAML path", target.YAMLPath
	default:
		return "JSON path", target.JSONPath
	}
}

func targetTypeDisplayName(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeJSON:
		return "JSON"
	case config.TargetTypeYAML:
		return "YAML"
	case config.TargetTypeDotenv:
		return "dotenv"
	default:
		return string(targetType)
	}
}

func promptTargetFile(prompter initPrompter, workingDirectory string, dependencies initDependencies) (targetFileSelection, error) {
discoveryLoop:
	for {
		candidates, err := dependencies.discoverTargetFileCandidates(workingDirectory)
		if err != nil {
			return targetFileSelection{}, err
		}

		if len(candidates) == 0 {
			if _, err := fmt.Fprintln(prompter.writer, "No supported configuration files with existing JSON or YAML string values or unambiguous dotenv keys were discovered under the current directory."); err != nil {
				return targetFileSelection{}, err
			}

			return promptManualTargetFile(prompter, workingDirectory, dependencies)
		}

		filterValue := ""
		for {
			matchingCandidates := filterTargetFileCandidates(candidates, filterValue)
			if len(matchingCandidates) == 0 {
				if _, err := fmt.Fprintf(prompter.writer, "No discovered configuration files match %q.\n", filterValue); err != nil {
					return targetFileSelection{}, err
				}

				filterValue, err = promptTargetFileFilter(prompter)
				if err != nil {
					return targetFileSelection{}, err
				}
				continue
			}

			visibleCandidates := matchingCandidates
			truncated := false
			if len(visibleCandidates) > targetFileChoiceWindowSize {
				visibleCandidates = visibleCandidates[:targetFileChoiceWindowSize]
				truncated = true
			}

			choices := make([]string, 0, len(visibleCandidates)+3)
			for _, candidate := range visibleCandidates {
				choices = append(choices, candidate.RelativePath)
			}

			showFilterAction := len(candidates) > targetFileChoiceWindowSize || filterValue != ""
			if showFilterAction {
				choices = append(choices, filterTargetFilesChoiceLabel)
			}
			if filterValue != "" {
				choices = append(choices, clearTargetFileFilterLabel)
			}
			choices = append(choices, manualTargetFileChoiceLabel)

			choiceIndex, err := prompter.promptChoiceIndex(targetFileSelectionPrompt(filterValue, len(visibleCandidates), len(matchingCandidates), len(candidates), truncated), choices)
			if err != nil {
				return targetFileSelection{}, err
			}

			if choiceIndex < len(visibleCandidates) {
				candidate := visibleCandidates[choiceIndex]
				selectedFile, err := inspectTargetFileCandidate(candidate, dependencies)
				if err != nil {
					if err := writePromptError(prompter, err); err != nil {
						return targetFileSelection{}, err
					}
					continue discoveryLoop
				}

				return selectedFile, nil
			}

			nextActionIndex := len(visibleCandidates)
			if showFilterAction {
				if choiceIndex == nextActionIndex {
					filterValue, err = promptTargetFileFilter(prompter)
					if err != nil {
						return targetFileSelection{}, err
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
				return promptManualTargetFile(prompter, workingDirectory, dependencies)
			}
		}
	}
}

func promptTargetFileFilter(prompter initPrompter) (string, error) {
	return prompter.promptLine("Filter files by name or path (blank clears the filter): ")
}

func filterTargetFileCandidates(candidates []editor.TargetFileCandidate, filterValue string) []editor.TargetFileCandidate {
	normalizedFilter := normalizeTargetFileFilter(filterValue)
	if normalizedFilter == "" {
		return candidates
	}

	basenameMatches := make([]editor.TargetFileCandidate, 0)
	pathMatches := make([]editor.TargetFileCandidate, 0)
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

func targetFileSelectionPrompt(filterValue string, visibleCount int, matchingCount int, totalCount int, truncated bool) string {
	if filterValue != "" {
		if truncated {
			return fmt.Sprintf("Select configuration file matching %q (showing %d of %d matches):", filterValue, visibleCount, matchingCount)
		}

		return fmt.Sprintf("Select configuration file matching %q:", filterValue)
	}

	if truncated {
		return fmt.Sprintf("Select configuration file (showing %d of %d discovered files):", visibleCount, totalCount)
	}

	return "Select configuration file:"
}

func promptManualTargetFile(prompter initPrompter, workingDirectory string, dependencies initDependencies) (targetFileSelection, error) {
	for {
		targetPath, err := prompter.promptNonEmptyLine("Configuration file path: ")
		if err != nil {
			return targetFileSelection{}, err
		}

		resolvedTargetPath := app.ResolveInitTargetPath(workingDirectory, targetPath)
		targetType, ok := config.InferTargetType(resolvedTargetPath)
		if !ok {
			targetType, err = promptExplicitTargetType(prompter, resolvedTargetPath)
			if err != nil {
				return targetFileSelection{}, err
			}
		}

		selectedFile, err := inspectTargetFile(resolvedTargetPath, app.DisplayInitTargetPath(workingDirectory, resolvedTargetPath), targetType, dependencies)
		if err != nil {
			if err := writePromptError(prompter, err); err != nil {
				return targetFileSelection{}, err
			}
			continue
		}

		return selectedFile, nil
	}
}

func promptExplicitTargetType(prompter initPrompter, targetPath string) (config.TargetType, error) {
	choice, err := prompter.promptChoice(fmt.Sprintf("File type cannot be inferred from %s. Choose file type:", targetPath), []string{"JSON", "YAML", "dotenv"})
	if err != nil {
		return "", err
	}
	if choice == "YAML" {
		return config.TargetTypeYAML, nil
	}
	if choice == "dotenv" {
		return config.TargetTypeDotenv, nil
	}

	return config.TargetTypeJSON, nil
}

func inspectTargetFileCandidate(candidate editor.TargetFileCandidate, dependencies initDependencies) (targetFileSelection, error) {
	targetType := candidate.Type
	if targetType == "" {
		inferredType, ok := config.InferTargetType(candidate.Path)
		if !ok {
			return targetFileSelection{}, fmt.Errorf("target type cannot be inferred from file %q", candidate.Path)
		}
		targetType = inferredType
	}

	return inspectTargetFile(candidate.Path, candidate.RelativePath, targetType, dependencies)
}

func inspectTargetFile(targetPath string, displayPath string, targetType config.TargetType, dependencies initDependencies) (targetFileSelection, error) {
	selection := targetFileSelection{
		path:        targetPath,
		displayPath: displayPath,
		targetType:  targetType,
	}

	switch targetType {
	case config.TargetTypeJSON:
		nodes, err := dependencies.inspectStringTargets(targetPath)
		if err != nil {
			return targetFileSelection{}, err
		}
		selection.nodes = jsonTargetSelectorNodes(nodes)
	case config.TargetTypeDotenv:
		keys, err := dependencies.inspectDotenvKeys(targetPath)
		if err != nil {
			return targetFileSelection{}, err
		}
		selection.dotenvKeys = keys
	case config.TargetTypeYAML:
		nodes, err := dependencies.inspectYAMLStringTargets(targetPath)
		if err != nil {
			return targetFileSelection{}, err
		}
		selection.nodes = yamlTargetSelectorNodes(nodes)
	default:
		return targetFileSelection{}, fmt.Errorf("target type %q is not supported", targetType)
	}

	return selection, nil
}

func promptTargetSelector(prompter initPrompter, selectedFile targetFileSelection, dependencies initDependencies) (string, bool, error) {
	switch selectedFile.targetType {
	case config.TargetTypeJSON:
		return promptTargetStructuredPath(prompter, selectedFile, dependencies)
	case config.TargetTypeYAML:
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

	matchingKeys := make([]string, 0)
	for _, key := range keys {
		if strings.Contains(strings.ToLower(key), normalizedFilter) {
			matchingKeys = append(matchingKeys, key)
		}
	}

	return matchingKeys
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

func promptTargetName(prompter initPrompter, seenNames map[string]struct{}) (string, error) {
	for {
		name, err := prompter.promptNonEmptyLine("Managed value name: ")
		if err != nil {
			return "", err
		}

		if _, exists := seenNames[name]; exists {
			if _, err := fmt.Fprintf(prompter.writer, "Error: managed value name %q is already configured.\n", name); err != nil {
				return "", err
			}
			continue
		}

		return name, nil
	}
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

func jsonTargetSelectorNodes(nodes []editor.StringTargetNode) []targetSelectorNode {
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

func yamlTargetSelectorNodes(nodes []editor.YAMLStringTargetNode) []targetSelectorNode {
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

func searchStructuredPathsChoiceLabel(targetType config.TargetType) string {
	if targetType == config.TargetTypeYAML {
		return searchYAMLPathsChoiceLabel
	}

	return searchJSONPathsChoiceLabel
}

func manualStructuredPathChoiceLabel(targetType config.TargetType) string {
	if targetType == config.TargetTypeYAML {
		return manualYAMLPathChoiceLabel
	}

	return manualJSONPathChoiceLabel
}

func browseStructuredPathsChoiceLabel(targetType config.TargetType) string {
	if targetType == config.TargetTypeYAML {
		return browseYAMLPathsChoiceLabel
	}

	return browseJSONPathsChoiceLabel
}

func manualStructuredPathInputPrompt(targetType config.TargetType) string {
	if targetType == config.TargetTypeYAML {
		return "YAML value path: "
	}

	return "JSON value path: "
}

func validateStructuredTargetSelector(dependencies initDependencies, targetType config.TargetType, targetPath string, selector string) error {
	if targetType == config.TargetTypeYAML {
		return dependencies.validateYAMLTarget(targetPath, selector)
	}

	return dependencies.validateStringTarget(targetPath, selector)
}

func writePromptError(prompter initPrompter, err error) error {
	_, writeErr := fmt.Fprintf(prompter.writer, "Error: %v\n", err)
	return writeErr
}
