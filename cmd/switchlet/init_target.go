package main

import (
	"fmt"
	"path/filepath"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

const (
	manualTargetFileChoiceLabel = "Enter file path manually"
	manualJSONPathChoiceLabel   = "Enter JSON path manually"
	chooseDifferentFileLabel    = "Choose a different file"
	goBackChoiceLabel           = "Go back"
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

func promptTarget(prompter initPrompter, workingDirectory string, dependencies initDependencies) (config.Target, error) {
	for {
		selectedFile, err := promptTargetFile(prompter, workingDirectory, dependencies)
		if err != nil {
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

		choices := make([]string, 0, len(candidates)+1)
		for _, candidate := range candidates {
			choices = append(choices, candidate.RelativePath)
		}
		choices = append(choices, manualTargetFileChoiceLabel)

		choiceIndex, err := prompter.promptChoiceIndex("Select target JSON file:", choices)
		if err != nil {
			return targetFileSelection{}, err
		}

		if choiceIndex == len(candidates) {
			return promptManualTargetFile(prompter, workingDirectory, dependencies)
		}

		candidate := candidates[choiceIndex]
		nodes, err := dependencies.inspectStringTargets(candidate.Path)
		if err != nil {
			if err := writePromptError(prompter, err); err != nil {
				return targetFileSelection{}, err
			}
			continue
		}

		return targetFileSelection{
			path:        candidate.Path,
			displayPath: candidate.RelativePath,
			nodes:       nodes,
		}, nil
	}
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

	for {
		prompt := fmt.Sprintf("Select target JSON path in %s:", selectedFile.displayPath)
		if len(ancestors) > 0 {
			prompt = fmt.Sprintf("Select target JSON path under %s in %s:", ancestors[len(ancestors)-1].path, selectedFile.displayPath)
		}

		choices := make([]string, 0, len(currentNodes)+3)
		for _, node := range currentNodes {
			choices = append(choices, targetNodeChoiceLabel(node))
		}
		if len(ancestors) > 0 {
			choices = append(choices, goBackChoiceLabel)
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
