package main

import (
	"fmt"
	"path/filepath"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

var initWizardStepLabels = []string{"Target", "Path", "Profiles", "Review"}

// View renders the current init-wizard state.
func (model initWizardModel) View() string {
	if model.isTerminalTooSmall() {
		return ui.RenderShell(ui.Shell{
			Title:    "Switchlet init",
			Subtitle: "Terminal too small.",
			Panels: []ui.Panel{{Title: "Resize required", Lines: []string{
				fmt.Sprintf("Minimum size: %dx%d", initWizardMinimumTerminalWidth, initWizardMinimumTerminalHeight),
				fmt.Sprintf("Current size: %dx%d", model.width, model.height),
				"Resize the terminal to continue.",
			}}},
			Actions: []ui.Action{{Key: "q", Label: "Cancel"}, {Key: "Ctrl+C", Label: "Cancel immediately"}},
			Width:   model.width,
		})
	}

	switch model.step {
	case initWizardStepFileSelect:
		return model.fileSelectionView()
	case initWizardStepFileFilter:
		return model.fileFilterView()
	case initWizardStepManualFile:
		return model.textInputView(1, "Enter target JSON file path", []string{
			"Enter a path relative to the project root or an absolute path.",
			"Switchlet inspects the file and keeps only existing string-valued JSON targets.",
		}, "Target file", "Validate file")
	case initWizardStepPathBrowse:
		return model.pathBrowseView()
	case initWizardStepPathSearch:
		return model.pathSearchView()
	case initWizardStepManualPath:
		return model.textInputView(2, "Enter target JSON path", []string{
			fmt.Sprintf("Selected file: %s", model.selectedFile.displayPath),
			"Enter the existing string-valued JSON path Switchlet should manage.",
		}, "Target JSON path", "Validate path")
	case initWizardStepProfileName:
		return model.textInputView(3, "Add profiles", []string{
			fmt.Sprintf("Target file: %s", model.selectedFile.displayPath),
			fmt.Sprintf("Target JSON path: %s", model.selectedJSONPath),
			fmt.Sprintf("Profiles added: %d", len(model.profiles)),
		}, "Profile name", "Continue")
	case initWizardStepProfileSource:
		return model.profileChoiceView(
			3,
			"Choose profile source",
			[]string{
				fmt.Sprintf("Profile: %s", model.draftProfile.Name),
				"Choose whether this profile stores a literal value or an environment variable name.",
			},
			[]string{"Use a literal value", "Use an environment variable"},
		)
	case initWizardStepProfileValue:
		label := "Literal value"
		if model.draftProfile.UseEnvironment {
			label = "Environment variable name"
		}

		return model.textInputView(3, "Enter profile value", []string{
			fmt.Sprintf("Profile: %s", model.draftProfile.Name),
			fmt.Sprintf("Source: %s", profileSourceSummary(model.draftProfile.UseEnvironment)),
		}, label, "Continue")
	case initWizardStepProfileProtected:
		return model.profileChoiceView(
			3,
			"Protected profile confirmation",
			[]string{
				fmt.Sprintf("Profile: %s", model.draftProfile.Name),
				"Protected profiles require confirmation before Switchlet applies them later.",
			},
			[]string{"Do not require confirmation", "Require confirmation"},
		)
	case initWizardStepProfileSummary:
		return model.profileSummaryView()
	case initWizardStepReview:
		return model.reviewView()
	default:
		return ui.RenderShell(ui.Shell{
			Title:    "Switchlet init",
			Subtitle: "Unsupported wizard state.",
			Width:    model.width,
		})
	}
}

func (model initWizardModel) fileSelectionView() string {
	matchingCandidates := model.filteredFileCandidates(model.fileFilter)
	model.clampCursor(len(matchingCandidates))

	workLines := make([]string, 0)
	if model.fileFilter != "" {
		workLines = append(workLines, ui.RenderKeyValue("Active filter", model.fileFilter), "")
	}
	if len(model.fileCandidates) == 0 {
		workLines = append(workLines,
			"No target JSON files with selectable string values were discovered under the current directory.",
			"Press m to enter a file path manually.",
		)
	} else {
		workLines = append(workLines, model.candidateListLines(matchingCandidates, model.cursor, targetFileChoiceWindowSize)...)
		workLines = append(workLines, "")
		if model.fileFilter != "" {
			workLines = append(workLines, fmt.Sprintf("Showing %d matching file(s) out of %d discovered.", len(matchingCandidates), len(model.fileCandidates)))
		} else {
			workLines = append(workLines, fmt.Sprintf("Showing %d discovered file(s).", len(matchingCandidates)))
		}
	}

	guidanceLines := []string{
		"Pick the JSON file Switchlet should update.",
		"Filter large lists when needed, or use manual entry for files outside discovery.",
	}
	if len(matchingCandidates) > 0 {
		guidanceLines = append(guidanceLines, "", "Selected", matchingCandidates[model.cursor].RelativePath)
	}

	return model.initWizardShell(1, "Choose target JSON file", []ui.Panel{
		{Title: "Target JSON files", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines(guidanceLines)},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "f or /", Label: "Filter"},
		{Key: "m", Label: "Manual path"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) fileFilterView() string {
	matchingCandidates := model.filteredFileCandidates(model.inputValue)
	model.clampCursor(len(matchingCandidates))

	workLines := []string{model.inputLine("Filter"), ""}
	if len(model.fileCandidates) == 0 {
		workLines = append(workLines, "No discovered files are available. Press Esc and use manual file entry instead.")
	} else {
		workLines = append(workLines, model.candidateListLines(matchingCandidates, model.cursor, targetFileChoiceWindowSize)...)
		workLines = append(workLines, "", fmt.Sprintf("Showing %d matching file(s) out of %d discovered.", len(matchingCandidates), len(model.fileCandidates)))
	}

	return model.initWizardShell(1, "Filter target JSON files", []ui.Panel{
		{Title: "Filter results", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			"Type part of a file name or path to narrow the discovered results.",
			"Enter selects the highlighted file.",
			"Esc returns to the file list with the typed filter.",
		})},
	}, searchableTextInputActions("Select"))
}

func (model initWizardModel) pathBrowseView() string {
	choices := make([]string, 0, len(model.browseNodes)+2)
	for _, node := range model.browseNodes {
		choices = append(choices, targetNodeChoiceLabel(node))
	}
	if len(model.browseAncestors) > 0 {
		choices = append(choices, goBackChoiceLabel)
	}
	choices = append(choices, searchJSONPathsChoiceLabel, manualJSONPathChoiceLabel)

	currentLocation := "root"
	if len(model.browseAncestors) > 0 {
		currentLocation = model.browseAncestors[len(model.browseAncestors)-1].path
	}

	workLines := []string{ui.RenderKeyValue("Current location", currentLocation), ""}
	workLines = append(workLines, model.choiceLines(choices, model.cursor, jsonPathChoiceWindowSize)...)

	return model.initWizardShell(2, "Choose target JSON path", []ui.Panel{
		{Title: "JSON path hierarchy", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			fmt.Sprintf("Selected file: %s", model.selectedFile.displayPath),
			"Choose the existing string-valued JSON path Switchlet should manage.",
			"Selectable values are shown without a trailing slash.",
		})},
	}, []ui.Action{
		{Key: "Enter", Label: "Open/Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "s or /", Label: "Search"},
		{Key: "m", Label: "Manual path"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) pathSearchView() string {
	matchingPaths := model.filteredSelectableJSONPaths(model.inputValue)
	model.clampCursor(len(matchingPaths))

	workLines := []string{model.inputLine("Search"), ""}
	workLines = append(workLines, model.choiceLines(matchingPaths, model.cursor, jsonPathChoiceWindowSize)...)
	workLines = append(workLines, "", fmt.Sprintf("Showing %d matching path(s).", len(matchingPaths)))

	return model.initWizardShell(2, "Search selectable JSON paths", []ui.Panel{
		{Title: "Search results", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			fmt.Sprintf("Selected file: %s", model.selectedFile.displayPath),
			"Type part of a path or leaf name.",
			"Enter chooses the highlighted existing string target.",
		})},
	}, searchableTextInputActions("Select"))
}

func (model initWizardModel) profileSummaryView() string {
	choices := []string{"Review and create configuration", "Add another profile", "Remove last profile", "Back to target JSON path"}

	return model.initWizardShell(3, "Profile summary", []ui.Panel{
		{Title: "Configured profiles", Lines: model.configuredProfileLines()},
		{Title: "Next action", Lines: model.choiceLines(choices, model.cursor, len(choices)), Focused: true},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) reviewView() string {
	choices := []string{"Create .switchlet.yaml"}
	if hasLiteralProfiles(model.profiles) {
		choices = append(choices, "Toggle .gitignore protection")
	}
	choices = append(choices, "Back to profiles")

	decisionLines := make([]string, 0)
	if hasLiteralProfiles(model.profiles) {
		status := "Enabled"
		if !model.shouldIgnoreConfig {
			status = "Disabled"
		}
		decisionLines = append(decisionLines,
			ui.RenderKeyValue(".gitignore protection", status),
			"Literal values will be stored directly in .switchlet.yaml.",
			"For more sensitive setups, prefer environment-backed profiles.",
			"",
		)
	} else {
		decisionLines = append(decisionLines, "No literal values configured.", "No ignore-file update is needed.", "")
	}
	decisionLines = append(decisionLines, model.choiceLines(choices, model.cursor, len(choices))...)

	return model.initWizardShell(4, "Review and create configuration", []ui.Panel{
		{Title: "Configuration summary", Lines: model.reviewSummaryLines()},
		{Title: "Create", Lines: model.withErrorLines(decisionLines), Focused: true},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) profileChoiceView(stepNumber int, title string, details []string, choices []string) string {
	return model.initWizardShell(stepNumber, title, []ui.Panel{
		{Title: "Choices", Lines: model.choiceLines(choices, model.cursor, len(choices)), Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines(details)},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) textInputView(stepNumber int, title string, details []string, label string, enterAction string) string {
	return model.initWizardShell(stepNumber, title, []ui.Panel{
		{Title: "Input", Lines: []string{model.inputLine(label)}, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines(details)},
	}, textInputActions(enterAction))
}

func (model initWizardModel) initWizardShell(stepNumber int, subtitle string, panels []ui.Panel, actions []ui.Action) string {
	return ui.RenderShell(ui.Shell{
		Title:    "Switchlet init",
		Subtitle: subtitle,
		Metadata: []string{
			fmt.Sprintf("Step %d of %d", stepNumber, initStepCount),
			ui.RenderStepProgress(stepNumber, initWizardStepLabels),
		},
		Panels:  panels,
		Actions: actions,
		Width:   model.width,
	})
}

func (model initWizardModel) candidateListLines(candidates []editor.TargetFileCandidate, cursor int, windowSize int) []string {
	if len(candidates) == 0 {
		return []string{"No matching files."}
	}

	start, end := windowRange(cursor, len(candidates), windowSize)
	rows := make([]ui.ListRow, 0, end-start)
	for index := start; index < end; index++ {
		state := ui.RowNormal
		if index == cursor {
			state = ui.RowSelected
		}

		rows = append(rows, ui.ListRow{Label: candidates[index].RelativePath, State: state})
	}

	lines := ui.RenderListRows(rows)
	if len(candidates) > windowSize {
		lines = append(lines, "", fmt.Sprintf("Showing %d-%d of %d files.", start+1, end, len(candidates)))
	}

	return lines
}

func (model initWizardModel) choiceLines(choices []string, cursor int, windowSize int) []string {
	if len(choices) == 0 {
		return []string{"No matching items."}
	}

	start, end := windowRange(cursor, len(choices), windowSize)
	rows := make([]ui.ListRow, 0, end-start)
	for index := start; index < end; index++ {
		state := ui.RowNormal
		if index == cursor {
			state = ui.RowSelected
		}

		rows = append(rows, ui.ListRow{Label: choices[index], State: state})
	}

	lines := ui.RenderListRows(rows)
	if len(choices) > windowSize {
		lines = append(lines, "", fmt.Sprintf("Showing %d-%d of %d items.", start+1, end, len(choices)))
	}

	return lines
}

func (model initWizardModel) configuredProfileLines() []string {
	lines := []string{
		ui.RenderKeyValue("Target file", model.selectedFile.displayPath),
		ui.RenderKeyValue("Target JSON path", model.selectedJSONPath),
		"",
		"Profiles",
	}
	lines = append(lines, model.profileRows()...)

	return lines
}

func (model initWizardModel) reviewSummaryLines() []string {
	lines := []string{
		ui.RenderKeyValue("Configuration file", filepath.Join(model.workingDirectory, ".switchlet.yaml")),
		ui.RenderKeyValue("Target file", displayTargetPath(model.workingDirectory, model.selectedFile.path)),
		ui.RenderKeyValue("Target JSON path", model.selectedJSONPath),
		"",
		"Profiles",
	}
	lines = append(lines, model.profileRows()...)

	return lines
}

func (model initWizardModel) profileRows() []string {
	if len(model.profiles) == 0 {
		return []string{"No profiles configured."}
	}

	rows := make([]ui.ListRow, 0, len(model.profiles))
	for _, profile := range model.profiles {
		rows = append(rows, ui.ListRow{
			Label:  profileReviewLabel(profile),
			State:  ui.RowNormal,
			Badges: profileReviewBadges(profile),
		})
	}

	return ui.RenderListRows(rows)
}

func (model initWizardModel) inputLine(label string) string {
	return ui.RenderInputWithinWidth(label, model.inputValue, model.inputCursor, ui.PrimaryPanelWidth(model.width, 2))
}

func (model initWizardModel) withErrorLines(lines []string) []string {
	if model.errorMessage == "" {
		return lines
	}

	return append(lines, "", "Error: "+model.errorMessage)
}

func textInputActions(enterAction string) []ui.Action {
	return []ui.Action{
		{Key: "Enter", Label: enterAction},
		{Key: "←/→", Label: "Move"},
		{Key: "Home/End", Label: "Jump"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Back"},
		{Key: "Ctrl+C", Label: "Cancel"},
	}
}

func searchableTextInputActions(enterAction string) []ui.Action {
	return []ui.Action{
		{Key: "Enter", Label: enterAction},
		{Key: "↑/↓", Label: "Move"},
		{Key: "←/→", Label: "Edit"},
		{Key: "Home/End", Label: "Jump"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Back"},
		{Key: "Ctrl+C", Label: "Cancel"},
	}
}

func profileReviewLabel(profile config.Profile) string {
	if profile.ValueFromEnv != nil {
		return profile.Name + " -> " + *profile.ValueFromEnv
	}

	return profile.Name
}

func profileReviewBadges(profile config.Profile) []ui.Badge {
	badges := make([]ui.Badge, 0, 2)
	if profile.Protected {
		badges = append(badges, ui.Badge{Label: "protected"})
	}
	if profile.ValueFromEnv != nil {
		badges = append(badges, ui.Badge{Label: "env"})
	} else {
		badges = append(badges, ui.Badge{Label: "literal"})
	}

	return badges
}

func profileSourceSummary(useEnvironment bool) string {
	if useEnvironment {
		return sourceLabel(app.ProfileSourceEnvironment)
	}

	return sourceLabel(app.ProfileSourceLiteral)
}
