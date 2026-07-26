package main

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

var initWizardStepLabels = []string{"File", "Value", "Name", "Profiles", "Review"}

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
			Height:  model.height,
		})
	}

	switch model.step {
	case initWizardStepFileSelect:
		return model.fileSelectionView()
	case initWizardStepFileFilter:
		return model.fileFilterView()
	case initWizardStepManualFile:
		return model.textInputView(1, "Enter configuration file path", []string{
			"Task",
			"Enter a relative or absolute JSON or dotenv file path.",
			"Switchlet inspects it before continuing.",
		}, "Configuration file", "Validate file")
	case initWizardStepTypeSelect:
		return model.profileChoiceView(
			1,
			"Choose file type",
			[]string{
				ui.RenderKeyValue("Selected file", model.selectedFile.displayPath),
				"",
				"Task",
				"Choose the file type because the format cannot be inferred safely.",
			},
			[]string{"JSON", "dotenv"},
		)
	case initWizardStepPathBrowse:
		return model.pathBrowseView()
	case initWizardStepPathSearch:
		return model.pathSearchView()
	case initWizardStepManualPath:
		return model.textInputView(2, "Enter JSON value path", []string{
			ui.RenderKeyValue("Selected file", model.selectedFile.displayPath),
			ui.RenderKeyValue("Detected format", targetTypeDisplayName(model.selectedFile.targetType)),
			"",
			"Task",
			"Enter one existing string-valued JSON path.",
			"Switchlet does not create missing values.",
		}, "JSON value path", "Validate path")
	case initWizardStepDotenvKeySelect:
		return model.dotenvKeySelectView()
	case initWizardStepManualDotenvKey:
		return model.textInputView(2, "Enter dotenv value key", []string{
			ui.RenderKeyValue("Selected file", model.selectedFile.displayPath),
			ui.RenderKeyValue("Detected format", targetTypeDisplayName(model.selectedFile.targetType)),
			"",
			"Task",
			"Enter one existing dotenv key that appears once.",
			"Switchlet does not create missing keys.",
		}, "Dotenv value key", "Validate key")
	case initWizardStepManagedValueName:
		return model.managedValueNameView()
	case initWizardStepManagedValueCheckpoint:
		return model.managedValueCheckpointView()
	case initWizardStepProfileName:
		return model.profileNameView()
	case initWizardStepProfileTargetInclude:
		return model.profileTargetIncludeView()
	case initWizardStepProfileValueSource:
		return model.profileValueSourceView()
	case initWizardStepProfileValue:
		label := "Literal value"
		if model.draftProfile.UseEnvironment {
			label = "Environment variable name"
		}

		return model.profileValueInputView("Set "+model.currentDraftTarget().Name+" for "+model.draftProfile.Name, []string{
			ui.RenderKeyValue("Profile", model.draftProfile.Name),
			ui.RenderKeyValue("Managed value", model.currentDraftTarget().Name),
			ui.RenderKeyValue("Source", profileSourceSummary(model.draftProfile.UseEnvironment)),
			ui.RenderKeyValue("Protection", profileProtectionSummary(model.draftProfile.Protected)),
			"",
			"Task",
			profileValueGuidance(model.draftProfile.UseEnvironment),
		}, label, "Save")
	case initWizardStepProfileSummary:
		return model.profileSummaryView()
	case initWizardStepReview:
		return model.reviewView()
	default:
		return ui.RenderShell(ui.Shell{
			Title:    "Switchlet init",
			Subtitle: "Unsupported wizard state.",
			Width:    model.width,
			Height:   model.height,
		})
	}
}

func (model initWizardModel) managedValueNameView() string {
	return model.initWizardShell(3, "Name this managed value", []ui.Panel{
		{Title: "Name", Lines: []string{model.inputLine("Name")}, Focused: true},
		{Title: "Context", Lines: model.withErrorLines(model.managedValueNameGuidanceLines())},
	}, textInputActions("Save"))
}

func (model initWizardModel) profileNameView() string {
	details := []string{
		ui.RenderKeyValue("Managed values", fmt.Sprintf("%d", len(model.targets))),
		ui.RenderKeyValue("Profiles added", fmt.Sprintf("%d", len(model.profiles))),
		"",
		"Task",
		"Name the profile users will select later.",
	}

	actions := []ui.Action{
		{Key: "Enter", Label: "Continue"},
		{Key: "←/→", Label: "Move"},
		{Key: "Home/End", Label: "Jump"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Managed values"},
		{Key: "Ctrl+C", Label: "Cancel"},
	}

	return model.initWizardShell(4, "Profile name", []ui.Panel{
		{Title: "Name", Lines: []string{model.inputLine("Profile name")}, Focused: true},
		{Title: "Context", Lines: model.withErrorLines(details)},
	}, actions)
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
			"No supported configuration files were discovered.",
			"Need existing JSON strings or unique dotenv keys.",
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
		"Task",
		"Choose a supported configuration file.",
		"JSON and dotenv files are both supported.",
		"File format chooses the value step.",
		"",
		"Manual fallback",
		"Press m when the file is not listed.",
	}
	if len(matchingCandidates) > 0 {
		guidanceLines = append(guidanceLines, "", ui.RenderKeyValue("Selected file", matchingCandidates[model.cursor].RelativePath))
	}

	return model.initWizardShell(1, "Choose configuration file", []ui.Panel{
		{Title: "Configuration files", Lines: workLines, Focused: true},
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
		workLines = append(workLines, "No discovered configuration files are available. Press Esc and use manual file entry instead.")
	} else {
		workLines = append(workLines, model.candidateListLines(matchingCandidates, model.cursor, targetFileChoiceWindowSize)...)
		workLines = append(workLines, "", fmt.Sprintf("Showing %d matching file(s) out of %d discovered.", len(matchingCandidates), len(model.fileCandidates)))
	}

	return model.initWizardShell(1, "Filter configuration files", []ui.Panel{
		{Title: "Filter results", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			"Task",
			"Narrow discovered configuration files by name or path.",
			"Enter selects the highlighted file.",
			"Esc returns to the file list with this filter.",
		})},
	}, searchableTextInputActions("Select"))
}

func (model initWizardModel) pathBrowseView() string {
	choices := make([]string, 0, len(model.browseNodes)+2)
	for _, node := range model.browseNodes {
		choices = append(choices, targetNodeChoiceLabel(node))
	}
	choices = append(choices, searchJSONPathsChoiceLabel, manualJSONPathChoiceLabel)

	currentLocation := "root"
	if len(model.browseAncestors) > 0 {
		currentLocation = model.browseAncestors[len(model.browseAncestors)-1].path
	}

	workLines := []string{ui.RenderKeyValue("Current location", currentLocation), ""}
	workLines = append(workLines, model.choiceLines(choices, model.cursor, jsonPathChoiceWindowSize)...)

	return model.initWizardShell(2, "Choose value", []ui.Panel{
		{Title: "JSON strings", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			ui.RenderKeyValue("Selected file", model.selectedFile.displayPath),
			ui.RenderKeyValue("Detected format", targetTypeDisplayName(model.selectedFile.targetType)),
			"",
			"Task",
			"Only existing string values are shown.",
			"Switchlet does not create missing values.",
			"Rows ending in / open nested objects.",
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

	return model.initWizardShell(2, "Search JSON values", []ui.Panel{
		{Title: "Search results", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			ui.RenderKeyValue("Selected file", model.selectedFile.displayPath),
			ui.RenderKeyValue("Detected format", targetTypeDisplayName(model.selectedFile.targetType)),
			"",
			"Task",
			"Search existing string values by path segment or leaf name.",
			"Enter chooses the highlighted JSON value.",
		})},
	}, searchableTextInputActions("Select"))
}

func (model initWizardModel) dotenvKeySelectView() string {
	choices := append([]string(nil), model.selectedFile.dotenvKeys...)
	choices = append(choices, manualDotenvKeyChoiceLabel)

	workLines := model.choiceLines(choices, model.cursor, dotenvKeyChoiceWindowSize)
	workLines = append(workLines, "", fmt.Sprintf("Showing %d dotenv key(s).", len(model.selectedFile.dotenvKeys)))

	return model.initWizardShell(2, "Choose value", []ui.Panel{
		{Title: "Dotenv keys", Lines: workLines, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			ui.RenderKeyValue("Selected file", model.selectedFile.displayPath),
			ui.RenderKeyValue("Detected format", targetTypeDisplayName(model.selectedFile.targetType)),
			"",
			"Task",
			"Existing unique keys only.",
			"Switchlet does not create missing keys.",
		})},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "m", Label: "Manual key"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) managedValueNameGuidanceLines() []string {
	_, selector := targetSelectorLabel(config.Target{Type: model.selectedFile.targetType, JSONPath: model.selectedJSONPath, Key: model.selectedDotenvKey})
	lines := []string{
		ui.RenderKeyValue("Selected file", model.selectedFile.displayPath),
		ui.RenderKeyValue("Selected value", selector),
		"",
		"Profiles refer to this short name.",
		"Examples: database, frontendApi, redisUrl",
	}

	return lines
}

func (model initWizardModel) managedValueCheckpointView() string {
	choices := []string{"Create profiles", "Add another value", "Remove value"}
	decisionLines := []string{
		"Switchlet now manages this value.",
		"Add another, or create profiles.",
		"",
	}
	decisionLines = append(decisionLines, model.choiceLines(choices, model.cursor, len(choices))...)

	return model.initWizardShell(3, "Managed value added", []ui.Panel{
		{Title: "Next action", Lines: decisionLines, Focused: true},
		{Title: "Added value", Lines: model.lastTargetLines()},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) profileTargetIncludeView() string {
	target := model.currentDraftTarget()
	choices := profileTargetIncludeChoices(model.draftProfile.Name, target.Name)

	return model.initWizardShell(4, "Values in "+model.draftProfile.Name, []ui.Panel{
		{Title: "Scope", Lines: model.choiceLines(choices, model.cursor, len(choices)), Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines([]string{
			ui.RenderKeyValue("Profile", model.draftProfile.Name),
			ui.RenderKeyValue("Managed value", target.Name),
			ui.RenderKeyValue("Progress", fmt.Sprintf("%d of %d", model.draftProfile.TargetIndex+1, len(model.targets))),
			"",
			"Task",
			"Choose whether this profile should set this managed value.",
			"No leaves it unchanged when the profile is applied.",
		})},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func profileTargetIncludeChoices(profileName string, targetName string) []string {
	return []string{
		fmt.Sprintf("Set %s in %s? Yes", targetName, profileName),
		fmt.Sprintf("Set %s in %s? No, leave unchanged", targetName, profileName),
	}
}

func (model initWizardModel) profileValueSourceView() string {
	choices := []string{
		"Literal value",
		"Environment variable",
		"Protected: " + profileProtectionSummary(model.draftProfile.Protected),
	}

	return model.initWizardShell(4, "Choose value source", []ui.Panel{
		{Title: "Source", Lines: model.choiceLines(choices, model.cursor, len(choices)), Focused: true},
		{Title: "Context", Lines: model.withErrorLines([]string{
			ui.RenderKeyValue("Profile", model.draftProfile.Name),
			ui.RenderKeyValue("Managed value", model.currentDraftTarget().Name),
			ui.RenderKeyValue("Protection", profileProtectionSummary(model.draftProfile.Protected)),
			"",
			"Task",
			"Choose how this value is stored before entering it.",
		})},
	}, []ui.Action{
		{Key: "Enter", Label: "Select/Toggle"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) profileSummaryView() string {
	choices := []string{"Review and create", "Add another profile", "Add another managed value", "Remove last profile"}

	return model.initWizardShell(4, "Profile added", []ui.Panel{
		{Title: "Next action", Lines: model.choiceLines(choices, model.cursor, len(choices)), Focused: true},
		{Title: "Configured profiles", Lines: model.configuredProfileLines()},
	}, []ui.Action{
		{Key: "Enter", Label: "Select"},
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Esc", Label: "Back"},
		{Key: "q", Label: "Cancel"},
	})
}

func (model initWizardModel) reviewView() string {
	hasLiteralValues := hasLiteralProfiles(model.profiles)
	choices := []string{"Create .switchlet.yaml"}
	if hasLiteralValues {
		choices = append(choices, "Toggle ignore")
	}

	decisionLines := make([]string, 0)
	if hasLiteralValues {
		status := "Enabled"
		if !model.shouldIgnoreConfig {
			status = "Disabled"
		}
		decisionLines = append(decisionLines,
			ui.RenderKeyValue(".gitignore protection", status),
			"Literal values will be stored in .switchlet.yaml.",
			"Use environment-backed profiles for sensitive values.",
			"",
		)
	} else {
		decisionLines = append(decisionLines,
			"Ready to create .switchlet.yaml.",
			"All profile values use environment variables.",
			"",
		)
	}
	decisionLines = append(decisionLines, model.choiceLines(choices, model.cursor, len(choices))...)

	return model.initWizardShell(5, "Review", []ui.Panel{
		{Title: "Create", Lines: model.withErrorLines(decisionLines), Focused: true},
		{Title: "Setup summary", Lines: model.reviewSummaryLines()},
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
		{Title: "Active input", Lines: []string{model.inputLine(label)}, Focused: true},
		{Title: "Guidance", Lines: model.withErrorLines(details)},
	}, textInputActions(enterAction))
}

func (model initWizardModel) profileValueInputView(title string, details []string, label string, enterAction string) string {
	return model.initWizardShell(4, title, []ui.Panel{
		{Title: "Value", Lines: []string{model.inputLine(label)}, Focused: true},
		{Title: "Context", Lines: model.withErrorLines(details)},
	}, profileValueInputActions(enterAction))
}

func (model initWizardModel) initWizardShell(stepNumber int, subtitle string, panels []ui.Panel, actions []ui.Action) string {
	return ui.RenderShell(ui.Shell{
		Title:    "Switchlet init",
		Subtitle: subtitle,
		Metadata: wizardStepMetadata(stepNumber),
		Panels:   panels,
		Actions:  actions,
		Width:    model.width,
		Height:   model.height,
	})
}

func wizardStepMetadata(stepNumber int) []string {
	return []string{
		fmt.Sprintf("Step %d of %d", stepNumber, initStepCount),
		ui.RenderStepProgress(stepNumber, initWizardStepLabels),
	}
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

		badges := []ui.Badge(nil)
		if candidates[index].Type != "" {
			badges = []ui.Badge{{Label: string(candidates[index].Type)}}
		}

		rows = append(rows, ui.ListRow{Label: candidates[index].RelativePath, State: state, Badges: badges})
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
		ui.RenderKeyValue("Managed values", fmt.Sprintf("%d", len(model.targets))),
		"",
		"Profiles",
	}
	lines = append(lines, model.profileRows()...)

	return lines
}

func (model initWizardModel) reviewSummaryLines() []string {
	lines := []string{"Managed values"}
	lines = append(lines, model.targetRows()...)
	lines = append(lines,
		"",
		"Profiles",
	)
	lines = append(lines, model.profileRows()...)

	return lines
}

func (model initWizardModel) configuredTargetLines() []string {
	lines := []string{ui.RenderKeyValue("Managed values", fmt.Sprintf("%d", len(model.targets))), ""}
	lines = append(lines, model.targetRows()...)

	return lines
}

func (model initWizardModel) lastTargetLines() []string {
	if len(model.targets) == 0 {
		return []string{"No managed value configured."}
	}

	target := model.targets[len(model.targets)-1]
	lines := []string{ui.RenderListRow(ui.ListRow{
		Label:  target.Name,
		State:  ui.RowNormal,
		Badges: []ui.Badge{{Label: string(target.Type)}},
	})}
	lines = append(lines, ui.RenderKeyValue("File", displayTargetPath(model.workingDirectory, target.File)))
	selectorName, selector := targetSelectorLabel(target)
	lines = append(lines, ui.RenderKeyValue(selectorName, selector))

	return lines
}

func (model initWizardModel) targetRows() []string {
	if len(model.targets) == 0 {
		return []string{"No managed values configured."}
	}

	lines := make([]string, 0, len(model.targets)*3)
	for _, target := range model.targets {
		lines = append(lines, ui.RenderListRow(ui.ListRow{
			Label:  target.Name,
			State:  ui.RowNormal,
			Badges: []ui.Badge{{Label: string(target.Type)}},
		}))
		lines = append(lines, ui.RenderKeyValue("  File", displayTargetPath(model.workingDirectory, target.File)))
		selectorName, selector := targetSelectorLabel(target)
		lines = append(lines, ui.RenderKeyValue("  "+selectorName, selector))
	}

	return lines
}

func (model initWizardModel) profileRows() []string {
	if len(model.profiles) == 0 {
		return []string{"No profiles configured."}
	}

	lines := make([]string, 0, len(model.profiles)*3)
	for _, profile := range model.profiles {
		lines = append(lines, ui.RenderListRow(ui.ListRow{
			Label:  profileReviewLabel(profile),
			State:  ui.RowNormal,
			Badges: model.profileReviewBadges(profile),
		}))
		if len(model.targets) > 1 {
			lines = append(lines, ui.RenderKeyValue("  Scope", managedValueScopeLabel(len(profile.Values), len(model.targets))))
		}
		for _, value := range profile.Values {
			lines = append(lines, ui.RenderKeyValue("  "+value.Target, profileValueSourceReviewLabel(value)))
		}
	}

	return lines
}

func (model initWizardModel) inputLine(label string) string {
	return ui.RenderInputWithinWidth(label, model.inputValue, model.inputCursor, ui.PrimaryPanelWidth(model.width, 2))
}

func (model initWizardModel) withErrorLines(lines []string) []string {
	if model.errorMessage == "" {
		return lines
	}

	return append(lines, "", "Error", model.errorMessage)
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

func profileValueInputActions(enterAction string) []ui.Action {
	return []ui.Action{
		{Key: "Enter", Label: enterAction},
		{Key: "←/→", Label: "Move"},
		{Key: "Home/End", Label: "Jump"},
		{Key: "Bksp/Del", Label: "Edit"},
		{Key: "Esc", Label: "Source"},
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
	return profile.Name
}

func (model initWizardModel) profileReviewBadges(profile config.Profile) []ui.Badge {
	badges := make([]ui.Badge, 0, 3)
	if len(model.targets) > 1 && len(profile.Values) < len(model.targets) {
		badges = append(badges, ui.Badge{Label: "partial"})
	}
	if profile.Protected {
		badges = append(badges, ui.Badge{Label: "protected"})
	}
	if profileUsesMixedSources(profile) {
		badges = append(badges, ui.Badge{Label: "mixed"})
	} else if profileUsesEnvironment(profile) {
		badges = append(badges, ui.Badge{Label: "env"})
	} else {
		badges = append(badges, ui.Badge{Label: "literal"})
	}

	return badges
}

func managedValueScopeLabel(includedCount int, totalCount int) string {
	return fmt.Sprintf("%d of %d managed values", includedCount, totalCount)
}

func profileValueSourceReviewLabel(value config.ProfileValue) string {
	if value.ValueFromEnv != nil {
		return "env " + *value.ValueFromEnv
	}

	return "literal"
}

func profileUsesEnvironment(profile config.Profile) bool {
	if profile.ValueFromEnv != nil {
		return true
	}
	for _, value := range profile.Values {
		if value.ValueFromEnv != nil {
			return true
		}
	}

	return false
}

func profileUsesMixedSources(profile config.Profile) bool {
	literal := profile.Value != nil
	environment := profile.ValueFromEnv != nil
	for _, value := range profile.Values {
		if value.ValueFromEnv != nil {
			environment = true
		} else {
			literal = true
		}
	}

	return literal && environment
}

func profileSourceSummary(useEnvironment bool) string {
	if useEnvironment {
		return sourceLabel(app.ProfileSourceEnvironment)
	}

	return sourceLabel(app.ProfileSourceLiteral)
}

func profileProtectionSummary(protected bool) string {
	if protected {
		return "on"
	}

	return "off"
}

func profileValueGuidance(useEnvironment bool) string {
	if useEnvironment {
		return "Enter the environment variable name only."
	}

	return "Enter the literal value to store in .switchlet.yaml."
}

func targetTypeDisplayName(targetType config.TargetType) string {
	switch targetType {
	case config.TargetTypeJSON:
		return "JSON"
	case config.TargetTypeDotenv:
		return "dotenv"
	default:
		return string(targetType)
	}
}
