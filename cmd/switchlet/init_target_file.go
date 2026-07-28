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
	filterTargetFilesChoiceLabel = "Filter configuration files"
	targetFileChoiceWindowSize   = 12
)

func promptTargetFile(prompter initPrompter, workingDirectory string, dependencies initDependencies) (targetFileSelection, error) {
discoveryLoop:
	for {
		candidates, err := dependencies.discoverTargetFileCandidates(workingDirectory)
		if err != nil {
			return targetFileSelection{}, err
		}

		if len(candidates) == 0 {
			if _, err := fmt.Fprintln(prompter.writer, "No supported configuration files with existing JSON, YAML, or TOML string values or unambiguous dotenv keys were discovered under the current directory."); err != nil {
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
	choice, err := prompter.promptChoice(fmt.Sprintf("File type cannot be inferred from %s. Choose file type:", targetPath), []string{"JSON", "YAML", "TOML", "dotenv"})
	if err != nil {
		return "", err
	}
	if choice == "YAML" {
		return config.TargetTypeYAML, nil
	}
	if choice == "TOML" {
		return config.TargetTypeTOML, nil
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
	case config.TargetTypeTOML:
		nodes, err := dependencies.inspectTOMLStringTargets(targetPath)
		if err != nil {
			return targetFileSelection{}, err
		}
		selection.nodes = tomlTargetSelectorNodes(nodes)
	default:
		return targetFileSelection{}, fmt.Errorf("target type %q is not supported", targetType)
	}

	return selection, nil
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

func tomlTargetSelectorNodes(nodes []editor.TOMLStringTargetNode) []targetSelectorNode {
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
