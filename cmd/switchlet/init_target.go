package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

const (
	manualTargetFileChoiceLabel  = "Enter file path manually"
	manualJSONPathChoiceLabel    = "Enter JSON path manually"
	chooseDifferentFileLabel     = "Back to file selection"
	goBackChoiceLabel            = "Back up one level"
	filterTargetFilesChoiceLabel = "Filter files by name or path"
	clearTargetFileFilterLabel   = "Clear filter"
	targetFileChoiceWindowSize   = 12
	searchJSONPathsChoiceLabel   = "Search selectable JSON paths"
	refineJSONPathSearchLabel    = "Refine path search"
	clearJSONPathSearchLabel     = "Clear path search"
	browseJSONPathsChoiceLabel   = "Browse JSON path hierarchy"
	jsonPathChoiceWindowSize     = 12
)

type targetFileSelection struct {
	path        string
	displayPath string
	nodes       []editor.StringTargetNode
}

type targetBrowseLevel struct {
	path  string
	nodes []editor.StringTargetNode
}

type jsonPathSearchResult struct {
	jsonPath            string
	returnToHierarchy   bool
	chooseDifferentFile bool
}

func promptTarget(prompter initPrompter, workingDirectory string, dependencies initDependencies) (config.Target, error) {
	for {
		selectedFile, err := promptTargetFile(prompter, workingDirectory, dependencies)
		if err != nil {
			return config.Target{}, err
		}

		if err := writeInitStep(prompter.writer, 2, "Choose target JSON path",
			fmt.Sprintf("Selected file: %s", selectedFile.displayPath),
			"Choose the existing string-valued JSON path Switchlet should manage.",
			"Browse the hierarchy, search when the file has many selectable paths, or enter a path manually.",
		); err != nil {
			return config.Target{}, err
		}

		jsonPath, chooseDifferentFile, err := promptTargetJSONPath(prompter, selectedFile, dependencies)
		if err != nil {
			return config.Target{}, err
		}
		if chooseDifferentFile {
			continue
		}

		return config.Target{
			File:     selectedFile.path,
			JSONPath: jsonPath,
		}, nil
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
			if _, err := fmt.Fprintln(prompter.writer, "No target JSON files with selectable string values were discovered under the current directory."); err != nil {
				return targetFileSelection{}, err
			}

			return promptManualTargetFile(prompter, workingDirectory, dependencies)
		}

		filterValue := ""
		for {
			matchingCandidates := filterTargetFileCandidates(candidates, filterValue)
			if len(matchingCandidates) == 0 {
				if _, err := fmt.Fprintf(prompter.writer, "No discovered target JSON files match %q.\n", filterValue); err != nil {
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
				nodes, err := dependencies.inspectStringTargets(candidate.Path)
				if err != nil {
					if err := writePromptError(prompter, err); err != nil {
						return targetFileSelection{}, err
					}
					continue discoveryLoop
				}

				return targetFileSelection{
					path:        candidate.Path,
					displayPath: candidate.RelativePath,
					nodes:       nodes,
				}, nil
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
			return fmt.Sprintf("Select target JSON file matching %q (showing %d of %d matches):", filterValue, visibleCount, matchingCount)
		}

		return fmt.Sprintf("Select target JSON file matching %q:", filterValue)
	}

	if truncated {
		return fmt.Sprintf("Select target JSON file (showing %d of %d discovered files):", visibleCount, totalCount)
	}

	return "Select target JSON file:"
}

func promptManualTargetFile(prompter initPrompter, workingDirectory string, dependencies initDependencies) (targetFileSelection, error) {
	for {
		targetPath, err := prompter.promptNonEmptyLine("Target JSON file path: ")
		if err != nil {
			return targetFileSelection{}, err
		}

		resolvedTargetPath := resolveTargetPath(workingDirectory, targetPath)
		nodes, err := dependencies.inspectStringTargets(resolvedTargetPath)
		if err != nil {
			if err := writePromptError(prompter, err); err != nil {
				return targetFileSelection{}, err
			}
			continue
		}

		return targetFileSelection{
			path:        resolvedTargetPath,
			displayPath: displayTargetPath(workingDirectory, resolvedTargetPath),
			nodes:       nodes,
		}, nil
	}
}

func promptTargetJSONPath(prompter initPrompter, selectedFile targetFileSelection, dependencies initDependencies) (string, bool, error) {
	currentNodes := selectedFile.nodes
	ancestors := make([]targetBrowseLevel, 0)
	selectablePaths := flattenSelectableJSONPaths(selectedFile.nodes)
	showSearchAction := len(selectablePaths) > jsonPathChoiceWindowSize

	for {
		prompt := fmt.Sprintf("Browse JSON paths in %s:", selectedFile.displayPath)
		if len(ancestors) > 0 {
			prompt = fmt.Sprintf("Browse JSON paths under %s in %s:", ancestors[len(ancestors)-1].path, selectedFile.displayPath)
		}

		choices := make([]string, 0, len(currentNodes)+4)
		for _, node := range currentNodes {
			choices = append(choices, targetNodeChoiceLabel(node))
		}
		if len(ancestors) > 0 {
			choices = append(choices, goBackChoiceLabel)
		}
		if showSearchAction {
			choices = append(choices, searchJSONPathsChoiceLabel)
		}
		choices = append(choices, manualJSONPathChoiceLabel, chooseDifferentFileLabel)

		choiceIndex, err := prompter.promptChoiceIndex(prompt, choices)
		if err != nil {
			return "", false, err
		}

		if choiceIndex < len(currentNodes) {
			selectedNode := currentNodes[choiceIndex]
			if selectedNode.Selectable {
				return selectedNode.JSONPath, false, nil
			}

			ancestors = append(ancestors, targetBrowseLevel{
				path:  selectedNode.JSONPath,
				nodes: currentNodes,
			})
			currentNodes = selectedNode.Children
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
				searchResult, err := promptTargetJSONPathSearch(prompter, selectedFile, selectablePaths, dependencies)
				if err != nil {
					return "", false, err
				}
				if searchResult.chooseDifferentFile {
					return "", true, nil
				}
				if searchResult.returnToHierarchy {
					continue
				}

				return searchResult.jsonPath, false, nil
			}
			nextActionIndex++
		}

		if choiceIndex == nextActionIndex {
			jsonPath, err := prompter.promptNonEmptyLine("Target JSON path: ")
			if err != nil {
				return "", false, err
			}

			if err := dependencies.validateStringTarget(selectedFile.path, jsonPath); err != nil {
				if err := writePromptError(prompter, err); err != nil {
					return "", false, err
				}
				continue
			}

			return jsonPath, false, nil
		}

		return "", true, nil
	}
}

func promptTargetJSONPathSearch(prompter initPrompter, selectedFile targetFileSelection, selectablePaths []string, dependencies initDependencies) (jsonPathSearchResult, error) {
	filterValue := ""

	for {
		if filterValue == "" {
			nextFilterValue, err := prompter.promptLine("Search JSON paths by name or path (blank returns to browsing): ")
			if err != nil {
				return jsonPathSearchResult{}, err
			}
			if strings.TrimSpace(nextFilterValue) == "" {
				return jsonPathSearchResult{returnToHierarchy: true}, nil
			}

			filterValue = nextFilterValue
		}

		matchingPaths := filterSelectableJSONPaths(selectablePaths, filterValue)
		if len(matchingPaths) == 0 {
			if _, err := fmt.Fprintf(prompter.writer, "No selectable JSON paths in %s match %q.\n", selectedFile.displayPath, filterValue); err != nil {
				return jsonPathSearchResult{}, err
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
		choices = append(choices, refineJSONPathSearchLabel, clearJSONPathSearchLabel, manualJSONPathChoiceLabel, browseJSONPathsChoiceLabel, chooseDifferentFileLabel)

		choiceIndex, err := prompter.promptChoiceIndex(targetJSONPathSearchPrompt(selectedFile.displayPath, filterValue, len(visiblePaths), len(matchingPaths), truncated), choices)
		if err != nil {
			return jsonPathSearchResult{}, err
		}

		if choiceIndex < len(visiblePaths) {
			return jsonPathSearchResult{jsonPath: visiblePaths[choiceIndex]}, nil
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
			jsonPath, err := prompter.promptNonEmptyLine("Target JSON path: ")
			if err != nil {
				return jsonPathSearchResult{}, err
			}

			if err := dependencies.validateStringTarget(selectedFile.path, jsonPath); err != nil {
				if err := writePromptError(prompter, err); err != nil {
					return jsonPathSearchResult{}, err
				}
				continue
			}

			return jsonPathSearchResult{jsonPath: jsonPath}, nil
		}
		nextActionIndex++

		if choiceIndex == nextActionIndex {
			return jsonPathSearchResult{returnToHierarchy: true}, nil
		}

		return jsonPathSearchResult{chooseDifferentFile: true}, nil
	}
}

func flattenSelectableJSONPaths(nodes []editor.StringTargetNode) []string {
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

func targetJSONPathSearchPrompt(displayPath string, filterValue string, visibleCount int, matchingCount int, truncated bool) string {
	if truncated {
		return fmt.Sprintf("Select target JSON path matching %q in %s (showing %d of %d matches):", filterValue, displayPath, visibleCount, matchingCount)
	}

	return fmt.Sprintf("Select target JSON path matching %q in %s:", filterValue, displayPath)
}

func targetNodeChoiceLabel(node editor.StringTargetNode) string {
	if node.Selectable {
		return node.Name
	}

	return node.Name + "/"
}

func resolveTargetPath(workingDirectory string, targetPath string) string {
	resolvedTargetPath := targetPath
	if !filepath.IsAbs(resolvedTargetPath) {
		resolvedTargetPath = filepath.Join(workingDirectory, resolvedTargetPath)
	}

	return filepath.Clean(resolvedTargetPath)
}

func displayTargetPath(workingDirectory string, targetPath string) string {
	relativePath, err := filepath.Rel(workingDirectory, targetPath)
	if err == nil {
		return relativePath
	}

	return targetPath
}

func writePromptError(prompter initPrompter, err error) error {
	_, writeErr := fmt.Fprintf(prompter.writer, "Error: %v\n", err)
	return writeErr
}
