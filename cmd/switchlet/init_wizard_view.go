package main

import (
	"fmt"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
)

// View renders the current init-wizard state.
func (model initWizardModel) View() string {
	if model.isTerminalTooSmall() {
		return fmt.Sprintf(
			"Switchlet init\n\nTerminal too small.\nMinimum size: %dx%d\nCurrent size: %dx%d\n\nResize the terminal to continue.\nq Cancel\nCtrl+C cancels immediately.\n",
			initWizardMinimumTerminalWidth,
			initWizardMinimumTerminalHeight,
			model.width,
			model.height,
		)
	}

	switch model.step {
	case initWizardStepFileSelect:
		return model.fileSelectionView()
	case initWizardStepFileFilter:
		return model.fileFilterView()
	case initWizardStepManualFile:
		return model.textInputView(1, "Enter target JSON file path", []string{
			"Enter a path relative to the project root or an absolute path.",
			"Switchlet will inspect the file and keep only existing string-valued JSON targets.",
		}, "Target file", textInputHelpLine("Validate file"))
	case initWizardStepPathBrowse:
		return model.pathBrowseView()
	case initWizardStepPathSearch:
		return model.pathSearchView()
	case initWizardStepManualPath:
		return model.textInputView(2, "Enter target JSON path", []string{
			fmt.Sprintf("Selected file: %s", model.selectedFile.displayPath),
			"Enter the existing string-valued JSON path Switchlet should manage.",
		}, "Target JSON path", textInputHelpLine("Validate path"))
	case initWizardStepProfileName:
		return model.textInputView(3, "Add profiles", []string{
			fmt.Sprintf("Target file: %s", model.selectedFile.displayPath),
			fmt.Sprintf("Target JSON path: %s", model.selectedJSONPath),
			fmt.Sprintf("Profiles added: %d", len(model.profiles)),
		}, "Profile name", textInputHelpLine("Continue"))
	case initWizardStepProfileSource:
		return model.profileChoiceView(
			3,
			"Choose profile source",
			[]string{
				fmt.Sprintf("Profile: %s", model.draftProfile.Name),
				"Choose whether this profile stores a literal value or an environment variable name.",
			},
			[]string{"Use a literal value", "Use an environment variable"},
			"Enter Select  ↑/↓ or j/k Move  q Cancel",
		)
	case initWizardStepProfileValue:
		label := "Literal value"
		if model.draftProfile.UseEnvironment {
			label = "Environment variable name"
		}

		return model.textInputView(3, "Enter profile value", []string{
			fmt.Sprintf("Profile: %s", model.draftProfile.Name),
			fmt.Sprintf("Source: %s", profileSourceSummary(model.draftProfile.UseEnvironment)),
		}, label, textInputHelpLine("Continue"))
	case initWizardStepProfileProtected:
		return model.profileChoiceView(
			3,
			"Protected profile confirmation",
			[]string{
				fmt.Sprintf("Profile: %s", model.draftProfile.Name),
				"Protected profiles require confirmation before Switchlet applies them later.",
			},
			[]string{"Do not require confirmation", "Require confirmation"},
			"Enter Select  ↑/↓ or j/k Move  q Cancel",
		)
	case initWizardStepProfileSummary:
		return model.profileSummaryView()
	case initWizardStepReview:
		return model.reviewView()
	default:
		return "Switchlet init\n\nUnsupported wizard state.\n"
	}
}

func (model initWizardModel) fileSelectionView() string {
	matchingCandidates := model.filteredFileCandidates(model.fileFilter)
	model.clampCursor(len(matchingCandidates))

	var builder strings.Builder
	model.writeStepHeader(&builder, 1, "Choose target JSON file", []string{
		"Pick the JSON file Switchlet should update.",
		"Use the highlighted list, narrow the results when needed, or enter a file path manually.",
	}...)

	if model.fileFilter != "" {
		builder.WriteString("Active filter: ")
		builder.WriteString(model.fileFilter)
		builder.WriteString("\n\n")
	}

	if len(model.fileCandidates) == 0 {
		builder.WriteString("No target JSON files with selectable string values were discovered under the current directory.\n")
		builder.WriteString("Press m to enter a file path manually.\n")
	} else {
		model.writeCandidateList(&builder, matchingCandidates, model.cursor, targetFileChoiceWindowSize)
		builder.WriteString("\n")
		if model.fileFilter != "" {
			builder.WriteString(fmt.Sprintf("Showing %d matching file(s) out of %d discovered.\n", len(matchingCandidates), len(model.fileCandidates)))
		} else {
			builder.WriteString(fmt.Sprintf("Showing %d discovered file(s).\n", len(matchingCandidates)))
		}
	}

	model.writeErrorAndHelp(&builder, "Enter Select  ↑/↓ or j/k Move  f or / Filter  m Manual path  q Cancel")

	return builder.String()
}

func (model initWizardModel) fileFilterView() string {
	matchingCandidates := model.filteredFileCandidates(model.inputValue)
	model.clampCursor(len(matchingCandidates))

	var builder strings.Builder
	model.writeStepHeader(&builder, 1, "Filter target JSON files", []string{
		"Type part of a file name or path to narrow the discovered results.",
		"Press Enter to select the highlighted file.",
	}...)
	model.writeInputLine(&builder, "Filter", model.inputValue)
	builder.WriteString("\n")
	if len(model.fileCandidates) == 0 {
		builder.WriteString("No discovered files are available. Press Esc and use manual file entry instead.\n")
	} else {
		model.writeCandidateList(&builder, matchingCandidates, model.cursor, targetFileChoiceWindowSize)
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Showing %d matching file(s) out of %d discovered.\n", len(matchingCandidates), len(model.fileCandidates)))
	}

	model.writeErrorAndHelp(&builder, searchableTextInputHelpLine("Select"))

	return builder.String()
}

func (model initWizardModel) pathBrowseView() string {
	var builder strings.Builder
	model.writeStepHeader(&builder, 2, "Choose target JSON path", []string{
		fmt.Sprintf("Selected file: %s", model.selectedFile.displayPath),
		"Choose the existing string-valued JSON path Switchlet should manage.",
	}...)

	if len(model.browseAncestors) == 0 {
		builder.WriteString("Current location: root\n\n")
	} else {
		builder.WriteString("Current location: ")
		builder.WriteString(model.browseAncestors[len(model.browseAncestors)-1].path)
		builder.WriteString("\n\n")
	}

	choices := make([]string, 0, len(model.browseNodes)+2)
	for _, node := range model.browseNodes {
		choices = append(choices, targetNodeChoiceLabel(node))
	}
	if len(model.browseAncestors) > 0 {
		choices = append(choices, goBackChoiceLabel)
	}
	choices = append(choices, searchJSONPathsChoiceLabel, manualJSONPathChoiceLabel)
	model.writeStringChoices(&builder, choices, model.cursor, jsonPathChoiceWindowSize)

	model.writeErrorAndHelp(&builder, "Enter Open/Select  ↑/↓ or j/k Move  s or / Search  m Manual path  Esc Back  q Cancel")

	return builder.String()
}

func (model initWizardModel) pathSearchView() string {
	matchingPaths := model.filteredSelectableJSONPaths(model.inputValue)
	model.clampCursor(len(matchingPaths))

	var builder strings.Builder
	model.writeStepHeader(&builder, 2, "Search selectable JSON paths", []string{
		fmt.Sprintf("Selected file: %s", model.selectedFile.displayPath),
		"Type part of a path or leaf name, then press Enter to choose the highlighted result.",
	}...)
	model.writeInputLine(&builder, "Search", model.inputValue)
	builder.WriteString("\n")
	model.writeStringChoices(&builder, matchingPaths, model.cursor, jsonPathChoiceWindowSize)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Showing %d matching path(s).\n", len(matchingPaths)))

	model.writeErrorAndHelp(&builder, searchableTextInputHelpLine("Select"))

	return builder.String()
}

func (model initWizardModel) profileSummaryView() string {
	var builder strings.Builder
	model.writeStepHeader(&builder, 3, "Profile summary", []string{
		fmt.Sprintf("Target file: %s", model.selectedFile.displayPath),
		fmt.Sprintf("Target JSON path: %s", model.selectedJSONPath),
	}...)

	builder.WriteString("Configured profiles:\n")
	for _, profile := range model.profiles {
		description := "literal"
		if profile.ValueFromEnv != nil {
			description = fmt.Sprintf("env %s", *profile.ValueFromEnv)
		}
		if profile.Protected {
			description += ", protected"
		}

		builder.WriteString("  - ")
		builder.WriteString(profile.Name)
		builder.WriteString(" (")
		builder.WriteString(description)
		builder.WriteString(")\n")
	}
	builder.WriteString("\n")

	choices := []string{"Review and create configuration", "Add another profile", "Remove last profile", "Back to target JSON path"}
	model.writeStringChoices(&builder, choices, model.cursor, len(choices))

	model.writeErrorAndHelp(&builder, "Enter Select  ↑/↓ or j/k Move  Esc Back  q Cancel")

	return builder.String()
}

func (model initWizardModel) reviewView() string {
	var builder strings.Builder
	model.writeStepHeader(&builder, 4, "Review and create configuration", []string{
		"Review the target and profiles below before creating .switchlet.yaml.",
	}...)

	if err := printInitSummary(&builder, model.workingDirectory, config.Target{File: model.selectedFile.path, JSONPath: model.selectedJSONPath}, model.profiles); err != nil {
		return fmt.Sprintf("Switchlet init\n\nError: %v\n", err)
	}

	builder.WriteString("\n")
	if hasLiteralProfiles(model.profiles) {
		status := "Enabled"
		if !model.shouldIgnoreConfig {
			status = "Disabled"
		}
		builder.WriteString(".gitignore protection: ")
		builder.WriteString(status)
		builder.WriteString("\n")
		builder.WriteString("Literal values will be stored directly in .switchlet.yaml. For more sensitive setups, prefer environment-backed profiles.\n\n")
	}

	choices := []string{"Create .switchlet.yaml"}
	if hasLiteralProfiles(model.profiles) {
		choices = append(choices, "Toggle .gitignore protection")
	}
	choices = append(choices, "Back to profiles")
	model.writeStringChoices(&builder, choices, model.cursor, len(choices))

	model.writeErrorAndHelp(&builder, "Enter Select  ↑/↓ or j/k Move  Esc Back  q Cancel")

	return builder.String()
}

func (model initWizardModel) profileChoiceView(stepNumber int, title string, details []string, choices []string, helpLine string) string {
	var builder strings.Builder
	model.writeStepHeader(&builder, stepNumber, title, details...)
	model.writeStringChoices(&builder, choices, model.cursor, len(choices))
	model.writeErrorAndHelp(&builder, helpLine)
	return builder.String()
}

func (model initWizardModel) textInputView(stepNumber int, title string, details []string, label string, helpLine string) string {
	var builder strings.Builder
	model.writeStepHeader(&builder, stepNumber, title, details...)
	model.writeInputLine(&builder, label, model.inputValue)
	model.writeErrorAndHelp(&builder, helpLine)
	return builder.String()
}

func (model initWizardModel) writeStepHeader(builder *strings.Builder, stepNumber int, title string, details ...string) {
	builder.WriteString("Switchlet init\n\n")
	builder.WriteString(fmt.Sprintf("Step %d of %d\n", stepNumber, initStepCount))
	builder.WriteString(title)
	builder.WriteString("\n")
	builder.WriteString(strings.Repeat("-", len(title)))
	builder.WriteString("\n\n")
	for _, detail := range details {
		builder.WriteString(detail)
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
}

func (model initWizardModel) writeCandidateList(builder *strings.Builder, candidates []editor.TargetFileCandidate, cursor int, windowSize int) {
	if len(candidates) == 0 {
		builder.WriteString("No matching files.\n")
		return
	}

	start, end := windowRange(cursor, len(candidates), windowSize)
	for index := start; index < end; index++ {
		prefix := "  "
		if index == cursor {
			prefix = "> "
		}

		builder.WriteString(prefix)
		builder.WriteString(candidates[index].RelativePath)
		builder.WriteString("\n")
	}

	if len(candidates) > windowSize {
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Showing %d-%d of %d files.\n", start+1, end, len(candidates)))
	}
}

func (model initWizardModel) writeStringChoices(builder *strings.Builder, choices []string, cursor int, windowSize int) {
	if len(choices) == 0 {
		builder.WriteString("No matching items.\n")
		return
	}

	start, end := windowRange(cursor, len(choices), windowSize)
	for index := start; index < end; index++ {
		prefix := "  "
		if index == cursor {
			prefix = "> "
		}

		builder.WriteString(prefix)
		builder.WriteString(choices[index])
		builder.WriteString("\n")
	}

	if len(choices) > windowSize {
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Showing %d-%d of %d items.\n", start+1, end, len(choices)))
	}
}

func (model initWizardModel) writeErrorAndHelp(builder *strings.Builder, helpLine string) {
	builder.WriteString("\n")
	if model.errorMessage != "" {
		builder.WriteString("Error: ")
		builder.WriteString(model.errorMessage)
		builder.WriteString("\n\n")
	}
	builder.WriteString("----------------------------------------\n")
	builder.WriteString(helpLine)
	builder.WriteString("\n")
}

func (model initWizardModel) writeInputLine(builder *strings.Builder, label string, value string) {
	runes := []rune(value)
	cursor := model.inputCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	builder.WriteString(label)
	builder.WriteString(": ")
	builder.WriteString(string(runes[:cursor]))
	builder.WriteString("_")
	builder.WriteString(string(runes[cursor:]))
	builder.WriteString("\n")
}

func textInputHelpLine(enterAction string) string {
	return fmt.Sprintf("Enter %s  ←/→ Move  Home/End Jump  Backspace/Delete Edit  Esc Back  Ctrl+C Cancel", enterAction)
}

func searchableTextInputHelpLine(enterAction string) string {
	return fmt.Sprintf("Enter %s  ↑/↓ Move  ←/→ Edit  Home/End Jump  Backspace/Delete Edit  Esc Back  Ctrl+C Cancel", enterAction)
}

func profileSourceSummary(useEnvironment bool) string {
	if useEnvironment {
		return sourceLabel(app.ProfileSourceEnvironment)
	}

	return sourceLabel(app.ProfileSourceLiteral)
}
