package tui

import (
	"fmt"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	minimumTerminalWidth  = 80
	minimumTerminalHeight = 24
)

// View renders the current terminal state.
func (model Model) View() string {
	if model.isTerminalTooSmall() {
		return model.tooSmallTerminalView()
	}

	switch model.state {
	case inspectState:
		return model.inspectionView()
	case confirmState:
		return model.confirmationView()
	case errorState:
		return model.errorView()
	case successState:
		return model.successView()
	default:
		return model.listView()
	}
}

func (model Model) listView() string {
	profileLines := []string{"No profiles available."}
	if len(model.profiles) > 0 {
		profileLines = RenderListRows(model.profileRows(RowSelected))
	}

	return RenderShell(Shell{
		Title: "Switchlet",
		Panels: []Panel{
			{Title: model.profilePanelTitle(), Lines: profileLines, Focused: true},
			{Title: "Selected profile", Lines: model.selectionSummaryLines()},
		},
		Actions: model.listActions(),
		Width:   model.width,
		Height:  model.height,
	})
}

func (model Model) listActions() []Action {
	if model.isApplying() {
		return []Action{{Key: "Ctrl+C", Label: "Exit immediately"}}
	}

	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return []Action{{Key: "q", Label: "Quit"}}
	}

	actions := []Action{
		{Key: "↑/↓ or j/k", Label: "Move", Priority: ActionPrioritySecondary},
	}
	if model.profileListPositionContext() != "" {
		actions = append(actions,
			Action{Key: "PgUp/PgDn", Label: "Page", Priority: ActionPrioritySecondary},
			Action{Key: "Home/End", Label: "Jump", Priority: ActionPrioritySecondary},
		)
	}
	actions = append(actions,
		Action{Key: "Enter", Label: enterActionLabel(selectedProfile), Priority: ActionPriorityPrimary},
		Action{Key: "i", Label: "Inspect", Priority: ActionPrioritySecondary},
		Action{Key: "q", Label: "Quit", Priority: ActionPriorityCritical},
	)

	return actions
}

func (model Model) selectionSummaryLines() []string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return []string{
			RenderKeyValue("State", "No profile selected"),
			RenderKeyValue("Enter", "Nothing to apply."),
		}
	}

	lines := []string{
		RenderKeyValue("Profile", selectedProfileTitle(selectedProfile)),
		RenderKeyValue("State", model.profileStateLabel(selectedProfile)),
	}
	if selectedProfile.TargetCount > 0 {
		lines = append(lines, RenderKeyValue("Changes", changeCountLabel(selectedProfile.TargetCount, selectedProfile.TotalTargets)))
	}
	lines = append(lines, RenderKeyValue("Enter", model.actionDescription(selectedProfile)))
	if !selectedProfile.Available && selectedProfile.UnavailableReason != "" {
		lines = append(lines, RenderKeyValue("Reason", selectedProfile.UnavailableReason))
	}

	if shouldShowTargetCount(selectedProfile) {
		lines = append(lines, "", "Affected targets")
		lines = append(lines, targetNamePreviewLines(selectedProfile.Values, 4)...)
	} else {
		lines = appendSingleTargetContextLines(lines, selectedProfile, model)
	}

	return lines
}

func appendSingleTargetContextLines(lines []string, profile app.ProfileItem, model Model) []string {
	targetFile := model.application.TargetFile()
	selectorName := "jsonPath"
	selector := model.application.TargetPath()

	if valueItem, ok := singleProfileValue(profile); ok {
		if valueItem.TargetFile != "" {
			targetFile = valueItem.TargetFile
		}
		if valueItem.SelectorName != "" {
			selectorName = valueItem.SelectorName
		}
		if valueItem.Selector != "" {
			selector = valueItem.Selector
		}
	}

	if targetFile == "" && selector == "" {
		return lines
	}

	lines = append(lines, "", "Target")
	if targetFile != "" {
		lines = append(lines, RenderKeyValue("Target file", model.compactTargetFileValue(targetFile, "Target file")))
	}
	if selector != "" {
		lines = append(lines, RenderKeyValue(targetSelectorDisplayLabel(selectorName), selector))
	}

	return lines
}

func singleProfileValue(profile app.ProfileItem) (app.ProfileValueItem, bool) {
	if len(profile.Values) != 1 {
		return app.ProfileValueItem{}, false
	}

	return profile.Values[0], true
}

func (model Model) isTerminalTooSmall() bool {
	if model.width == 0 || model.height == 0 {
		return false
	}

	return model.width < minimumTerminalWidth || model.height < minimumTerminalHeight
}

func (model Model) tooSmallTerminalView() string {
	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Terminal too small.",
		Panels: []Panel{{Title: "Resize required", Lines: []string{
			fmt.Sprintf("Minimum size: %dx%d", minimumTerminalWidth, minimumTerminalHeight),
			fmt.Sprintf("Current size: %dx%d", model.width, model.height),
			"Resize the terminal to continue.",
		}}},
		Actions: []Action{{Key: "q", Label: "Quit"}, {Key: "Ctrl+C", Label: "Exit immediately"}},
		Width:   model.width,
		Height:  model.height,
	})
}

func (model Model) inspectionView() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return model.listView()
	}

	profileLines := []string{
		RenderKeyValue("Profile", selectedProfile.Name),
		RenderKeyValue("State", availabilityLabel(selectedProfile)),
		RenderKeyValue("Source", sourceLabel(selectedProfile.Source)),
	}
	if selectedProfile.TargetCount > 0 {
		profileLines = append(profileLines, RenderKeyValue("Changes", changeCountLabel(selectedProfile.TargetCount, selectedProfile.TotalTargets)))
	}
	if selectedProfile.EnvironmentVariableName != "" {
		profileLines = append(profileLines, RenderKeyValue("Environment variable", selectedProfile.EnvironmentVariableName))
	}
	profileLines = append(profileLines, RenderKeyValue("Protection", protectionLabel(selectedProfile)))
	if !selectedProfile.Available && selectedProfile.UnavailableReason != "" {
		profileLines = append(profileLines, RenderKeyValue("Reason", selectedProfile.UnavailableReason))
	}

	if shouldShowTargetCount(selectedProfile) {
		profileLines = append(profileLines, "", "Planned changes")
		profileLines = append(profileLines, profileValueDetailLines(selectedProfile.Values, secondaryPanelContentWidth(model.width))...)
	} else {
		profileLines = appendSingleTargetContextLines(profileLines, selectedProfile, model)

		valueLines := []string{"", "Value preview", RenderKeyValue("Masked value", maskedValueLabel(selectedProfile))}
		if selectedProfile.UnavailableReason != "" {
			valueLines = append(valueLines, "", "Resolution error:", selectedProfile.UnavailableReason)
		}
		profileLines = append(profileLines, valueLines...)
	}

	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Inspect Profile",
		Metadata: model.targetMetadata(),
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Profile detail", Lines: profileLines, Focused: true},
		},
		Actions: []Action{{Key: "Enter", Label: enterActionLabel(selectedProfile)}, {Key: "i/Esc/q", Label: "Return"}},
		Width:   model.width,
		Height:  model.height,
	})
}

func (model Model) confirmationView() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return model.listView()
	}

	lines := []string{
		RenderKeyValue("Profile", selectedProfile.Name),
		RenderKeyValue("Protection", "Required"),
	}
	if shouldShowTargetCount(selectedProfile) {
		lines = append(lines, RenderKeyValue("Changes", changeCountLabel(selectedProfile.TargetCount, selectedProfile.TotalTargets)))
		lines = append(lines,
			"",
			"This will update configured targets only.",
			"Resolved values are intentionally hidden.",
			"",
			"Affected targets",
		)
		lines = append(lines, profileValueTargetLines(selectedProfile.Values, secondaryPanelContentWidth(model.width))...)
		lines = append(lines, "", "Press Enter or y to confirm.")
	} else {
		lines = appendSingleTargetContextLines(lines, selectedProfile, model)
		lines = append(lines,
			"",
			"This will update only the configured target value.",
			"The resolved value is intentionally hidden.",
			"Press Enter or y to confirm.",
		)
	}

	return RenderShell(Shell{
		Title:    "Apply protected profile?",
		Metadata: model.targetMetadata(),
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Confirmation", Lines: lines, Focused: true},
		},
		Actions: []Action{{Key: "Enter/y", Label: "Confirm"}, {Key: "n/Esc/q", Label: "Cancel"}},
		Width:   model.width,
		Height:  model.height,
	})
}

func enterActionLabel(profile app.ProfileItem) string {
	switch {
	case !profile.Available:
		return "Show Error"
	case profile.Protected:
		return "Continue"
	default:
		return "Apply"
	}
}

func (model Model) errorView() string {
	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Recoverable error",
		Metadata: model.targetMetadata(),
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Error", Lines: RecoverableErrorLines(model.recoverableError, secondaryPanelContentWidth(model.width)), Focused: true},
		},
		Actions: []Action{{Key: "Any key", Label: "Return"}, {Key: "q", Label: "Quit"}},
		Width:   model.width,
		Height:  model.height,
	})
}

func (model Model) successView() string {
	if model.successResult == nil {
		return "Applied profile successfully.\n"
	}

	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Profile applied.",
		Panels:   []Panel{{Title: "Result", Lines: successLines(model.successResult, fullPanelContentWidth(model.width)), Focused: true}},
		Width:    model.width,
		Height:   model.height,
	})
}

func successLines(result *app.Result, maxLineWidth int) []string {
	changes := resultPlannedChanges(*result)
	lines := []string{
		RenderKeyValue("Applied profile", result.ProfileName),
		"",
		targetListHeading("Updated target", changes) + ":",
	}
	lines = append(lines, resultChangeLines(changes, maxLineWidth)...)
	lines = append(lines, "", "Switchlet will now exit.")

	return lines
}

// FinalMessage returns the concise summary shown after the full-screen UI exits.
func (model Model) FinalMessage() string {
	if model.state != successState || model.successResult == nil {
		return ""
	}

	var builder strings.Builder
	changes := resultPlannedChanges(*model.successResult)
	fmt.Fprintf(&builder, "Applied profile %q\n\n%s:\n", model.successResult.ProfileName, targetListHeading("Updated target", changes))
	for _, line := range finalResultChangeLines(changes) {
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String()
}

func targetListHeading(singular string, changes []app.PlannedChange) string {
	if len(changes) == 1 {
		return singular
	}

	return singular + "s"
}

func (model Model) targetMetadata() []string {
	if selectedProfile, ok := model.selectedProfile(); ok && selectedProfile.TotalTargets > 1 {
		return []string{
			fmt.Sprintf("%d configured targets", selectedProfile.TotalTargets),
			"Selected: " + changeCountLabel(selectedProfile.TargetCount, selectedProfile.TotalTargets),
		}
	}

	metadata := make([]string, 0, 2)
	if model.application.TargetFile() != "" {
		metadata = append(metadata, compactPathForDisplay(model.application.TargetFile(), headerMetadataWidth(model.width)))
	}
	if model.application.TargetPath() != "" {
		metadata = append(metadata, model.application.TargetPath())
	}

	return metadata
}

func sourceLabel(source app.ProfileSource) string {
	switch source {
	case app.ProfileSourceEnvironment:
		return "Environment variable"
	case app.ProfileSourceLiteral:
		return "Literal value"
	case app.ProfileSourceMixed:
		return "Mixed values"
	default:
		return "Unknown"
	}
}

func protectionLabel(profile app.ProfileItem) string {
	if profile.Protected {
		return "Required"
	}

	return "Not required"
}

func availabilityLabel(profile app.ProfileItem) string {
	if !profile.Available {
		return "Unavailable"
	}

	return "Ready to apply"
}

func (model Model) profileStateLabel(profile app.ProfileItem) string {
	if model.isApplyingSelectedProfile(profile) {
		return "Applying"
	}

	return availabilityLabel(profile)
}

func (model Model) actionDescription(profile app.ProfileItem) string {
	if model.isApplyingSelectedProfile(profile) {
		return "Applying now."
	}

	switch {
	case !profile.Available:
		return "Show recovery details."
	case profile.Protected:
		return "Open confirmation."
	default:
		return "Apply this profile."
	}
}

func maskedValueLabel(profile app.ProfileItem) string {
	if !profile.Available {
		return "Unavailable"
	}
	if profile.MaskedValue == "" {
		return "<empty>"
	}

	return profile.MaskedValue
}
