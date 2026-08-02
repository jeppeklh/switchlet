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
	case statusLoadingState, statusReadyState:
		return model.statusComparisonView()
	case diffLoadingState, diffReadyState:
		return model.diffComparisonView()
	case comparisonErrorState:
		return model.comparisonErrorView()
	default:
		return model.listView()
	}
}

func (model Model) listView() string {
	return RenderShell(Shell{
		Headerless: true,
		Panels: []Panel{
			{Title: model.profilePanelTitle(), Lines: model.profileListLines(RowSelected), Focused: true, FillHeight: model.shouldFillWorkspacePanels()},
			{Title: "Profile contents", Lines: model.profileContentsLines(), FillHeight: model.shouldFillWorkspacePanels()},
		},
		Actions:     model.listActions(),
		CommandLine: model.profileSearchCommandLine(),
		Width:       model.width,
		Height:      model.height,
	})
}

func (model Model) shouldFillWorkspacePanels() bool {
	return model.height > 0 && normalizedWidth(model.width) >= splitShellWidth
}

func (model Model) listActions() []Action {
	if model.isApplying() {
		return []Action{{Key: "q", Label: "Quit", Priority: ActionPriorityCritical}}
	}
	if model.state == searchState {
		return []Action{
			{Key: "Enter", Label: "Apply filter", Priority: ActionPriorityPrimary},
			{Key: "Left/Right", Label: "Move", Priority: ActionPrioritySecondary},
			{Key: "Home/End", Label: "Jump", Priority: ActionPrioritySecondary},
			{Key: "Bksp/Del", Label: "Edit", Priority: ActionPrioritySecondary},
			{Key: "Esc", Label: "Back", Priority: ActionPriorityPrimary},
		}
	}

	selectedProfile, ok := model.selectedProfile()
	if !ok {
		actions := make([]Action, 0, 4)
		if len(model.profiles) > 0 {
			actions = append(actions, Action{Key: "/", Label: "Search", Priority: ActionPrioritySecondary})
		}
		if model.profileFilter != "" {
			actions = append(actions, Action{Key: "Esc", Label: "Clear filter", Priority: ActionPriorityPrimary})
		}
		return append(actions, Action{Key: "c", Label: "Config", Priority: ActionPriorityNormal}, Action{Key: "q", Label: "Quit", Priority: ActionPriorityCritical})
	}

	actions := []Action{
		{Key: "↑/↓ or j/k", Label: "Move", Priority: ActionPrioritySecondary},
	}
	if len(model.profiles) > 0 {
		actions = append(actions, Action{Key: "/", Label: "Search", Priority: ActionPrioritySecondary})
	}
	if model.profileFilter != "" {
		actions = append(actions,
			Action{Key: "n/N", Label: "Matches", Priority: ActionPrioritySecondary},
			Action{Key: "Esc", Label: "Clear filter", Priority: ActionPriorityPrimary},
		)
	}
	if model.profileListPositionContext() != "" {
		actions = append(actions,
			Action{Key: "PgUp/PgDn", Label: "Page", Priority: ActionPrioritySecondary},
			Action{Key: "Home/End", Label: "Jump", Priority: ActionPrioritySecondary},
		)
	}
	actions = append(actions,
		Action{Key: "Enter", Label: enterActionLabel(selectedProfile), Priority: ActionPriorityPrimary},
		Action{Key: "Space", Label: stayActionLabel(selectedProfile), Priority: ActionPriorityPrimary},
		Action{Key: "i", Label: "Inspect", Priority: ActionPriorityNormal},
		Action{Key: "c", Label: "Config", Priority: ActionPriorityNormal},
		Action{Key: "s", Label: "Status", Priority: ActionPrioritySecondary},
		Action{Key: "d", Label: "Diff", Priority: ActionPrioritySecondary},
		model.valueRevealAction(),
		Action{Key: "q", Label: "Quit", Priority: ActionPriorityCritical},
	)

	return actions
}

func (model Model) profileContentsLines() []string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		lines := model.currentProfileDetectionFeedbackLines(nil)
		return append(lines,
			RenderKeyValue("State", "No profile selected"),
			RenderKeyValue("Enter", "Nothing to apply."),
		)
	}

	contents, err := model.application.ProfileContentsByName(selectedProfile.Name, model.profileContentsPreviewOptions())
	if err != nil {
		lines := model.currentProfileDetectionFeedbackLines([]string{RenderSectionHeading(selectedProfileTitle(selectedProfile))})
		return append(lines,
			model.profileStateLabel(selectedProfile),
			"",
			"Profile contents unavailable.",
			RenderKeyValue("Reason", fitValueForLabel(err.Error(), secondaryPanelContentWidth(model.width), "Reason")),
		)
	}

	lines := []string{RenderSectionHeading(selectedProfileTitle(selectedProfile))}
	lines = model.currentProfileDetectionFeedbackLines(lines)
	if model.isApplyingSelectedProfile(selectedProfile) {
		lines = append(lines, model.profileStateLabel(selectedProfile))
	}
	if !selectedProfile.Available && selectedProfile.UnavailableReason != "" {
		lines = appendUnavailableProfileSummary(lines, selectedProfile, secondaryPanelContentWidth(model.width))
	}

	return appendProfileContentsFileGroups(lines, contents.Files, model.projectRoot, secondaryPanelContentWidth(model.width))
}

func (model Model) currentProfileDetectionFeedbackLines(lines []string) []string {
	message := model.currentProfileDetectionMessage()
	if message == "" {
		return lines
	}

	return append(lines, message)
}

func (model Model) currentProfileDetectionMessage() string {
	switch model.currentDetection {
	case currentProfileDetectionChecking:
		return "Checking current profile..."
	case currentProfileDetectionUnavailable:
		return "Current profile check unavailable."
	default:
		return ""
	}
}

func appendUnavailableProfileSummary(lines []string, profile app.ProfileItem, maxLineWidth int) []string {
	addedDetail := false
	for _, value := range profile.Values {
		if value.Available || value.EnvironmentVariableName == "" {
			continue
		}

		lines = append(lines, fitLine(fmt.Sprintf("Set %s to apply target %s.", value.EnvironmentVariableName, targetNameLabel(value.TargetName)), maxLineWidth))
		addedDetail = true
	}
	if addedDetail {
		return lines
	}

	return append(lines, fitLine(profile.UnavailableReason, maxLineWidth))
}

func (model Model) profileContentsPreviewOptions() app.PreviewOptions {
	if model.valuesVisible {
		return app.PreviewOptions{ValueVisibility: app.ValueVisibilityShown}
	}

	return app.PreviewOptions{ValueVisibility: app.ValueVisibilityHidden}
}

func appendSingleTargetContextLines(lines []string, profile app.ProfileItem, model Model) []string {
	targetFile := model.application.TargetFile()
	targetLabel := ""
	selector := model.application.TargetPath()

	if valueItem, ok := singleProfileValue(profile); ok {
		targetLabel = profileValueTargetLabel(valueItem)
		if valueItem.TargetFile != "" {
			targetFile = valueItem.TargetFile
		}
		if valueItem.Selector != "" {
			selector = valueItem.Selector
		}
	}

	if targetFile == "" && selector == "" {
		return lines
	}

	fields := make([]DetailField, 0, 3)
	if targetFile != "" {
		fields = append(fields, DetailField{Label: "File", Value: model.displayProjectPath(targetFile)})
	}
	if targetLabel != "" {
		fields = append(fields, DetailField{Label: "Managed value", Value: targetLabel})
	}
	if selector != "" {
		fields = append(fields, DetailField{Label: "Selector", Value: selector})
	}

	lines = append(lines, "", RenderSectionHeading("Target"))
	return append(lines, renderIndentedDetailFields("  ", fields, secondaryPanelContentWidth(model.width))...)
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
		Actions: []Action{{Key: "q", Label: "Quit"}},
		Width:   model.width,
		Height:  model.height,
	})
}

func (model Model) statusComparisonView() string {
	shell := model.statusComparisonShell()
	return RenderShell(model.withFocusedPanelScroll(shell, 1))
}

func (model Model) statusComparisonShell() Shell {
	lines := []string{"Checking current managed values..."}
	if model.state == statusReadyState && model.statusComparison != nil {
		lines = statusComparisonLines(*model.statusComparison, model.projectRoot, secondaryPanelContentWidth(model.width))
	}

	return Shell{
		Headerless: true,
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Status", Lines: lines, Focused: true, FillHeight: model.shouldFillWorkspacePanels()},
		},
		Actions: comparisonActions(comparisonRequestStatus, model.valuesVisible),
		Width:   model.width,
		Height:  model.height,
	}
}

func (model Model) diffComparisonView() string {
	shell := model.diffComparisonShell()
	return RenderShell(model.withFocusedPanelScroll(shell, 0))
}

func (model Model) diffComparisonShell() Shell {
	profileName := model.comparisonProfileName
	if model.state == diffReadyState && model.diffPreview != nil && model.diffPreview.ProfileName != "" {
		profileName = model.diffPreview.ProfileName
	}
	if profileName == "" {
		if selectedProfile, ok := model.selectedProfile(); ok {
			profileName = selectedProfile.Name
		}
	}

	panelWidth := fullPanelContentWidth(model.width)
	lines := diffComparisonLoadingLines(profileName)
	if model.state == diffReadyState && model.diffPreview != nil {
		lines = managedPatchPreviewLines(*model.diffPreview, model.valuesVisible, model.projectRoot, panelWidth)
	}

	return Shell{
		Headerless: true,
		Panels:     []Panel{{Title: "Managed patch", Lines: lines, Focused: true, FillHeight: model.shouldFillWorkspacePanels()}},
		Actions:    comparisonActions(comparisonRequestDiff, model.valuesVisible),
		Width:      model.width,
		Height:     model.height,
	}
}

func (model Model) comparisonErrorView() string {
	shell := model.comparisonErrorShell()
	return RenderShell(model.withFocusedPanelScroll(shell, 1))
}

func (model Model) comparisonErrorShell() Shell {
	return Shell{
		Headerless: true,
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Error", Lines: RecoverableErrorLines(model.comparisonError, secondaryPanelContentWidth(model.width)), Focused: true, FillHeight: model.shouldFillWorkspacePanels()},
		},
		Actions: comparisonActions(comparisonRequestNone, model.valuesVisible),
		Width:   model.width,
		Height:  model.height,
	}
}

func diffComparisonLoadingLines(profileName string) []string {
	lines := make([]string, 0, 2)
	if profileName != "" {
		lines = append(lines, RenderKeyValue("Profile", profileName))
	}

	return append(lines, "Comparing selected profile...")
}

func comparisonActions(kind comparisonRequestKind, valuesVisible bool) []Action {
	actions := []Action{
		{Key: "r", Label: "Refresh", Priority: ActionPrioritySecondary},
	}
	switch kind {
	case comparisonRequestStatus:
		actions = append(actions, Action{Key: "s", Label: "Return", Priority: ActionPriorityPrimary})
	case comparisonRequestDiff:
		actions = append(actions, valueRevealAction(valuesVisible))
		actions = append(actions, Action{Key: "d", Label: "Return", Priority: ActionPriorityPrimary})
	}

	return append(actions,
		Action{Key: "Esc", Label: "Return", Priority: ActionPriorityPrimary},
		Action{Key: "q", Label: "Quit", Priority: ActionPriorityCritical},
	)
}

func statusComparisonLines(status app.StatusComparison, projectRoot string, maxLineWidth int) []string {
	lines := make([]string, 0)
	switch status.Status {
	case app.StatusComparisonMatched:
		lines = append(lines,
			RenderSectionHeading("Exact match"),
			"The managed files match one complete profile.",
		)
		lines = appendStatusTargetSection(lines, "Matched targets", countOfTotalLabel(len(status.MatchedTargets), status.TargetCount), status.MatchedTargets, projectRoot, maxLineWidth)
	case app.StatusComparisonAmbiguous:
		lines = append(lines,
			RenderSectionHeading("Ambiguous match"),
			"The managed files match more than one complete profile.",
		)
		lines = appendStatusProfileMatches(lines, "Matching profiles", status.Matches)
		lines = appendStatusTargetSection(lines, "Matched targets", countOfTotalLabel(len(status.MatchedTargets), status.TargetCount), status.MatchedTargets, projectRoot, maxLineWidth)
	default:
		lines = append(lines,
			RenderSectionHeading("No exact match"),
			"No complete profile matches the managed files.",
			"",
			RenderKeyValue("Configured targets", fmt.Sprintf("%d", status.TargetCount)),
		)
		lines = appendStatusPartialMatches(lines, status.PartialMatches)
		lines = appendStatusClosestProfiles(lines, status.ClosestProfiles)
		if len(status.PartialMatches) == 0 && len(status.ClosestProfiles) == 0 && len(status.UnavailableProfiles) == 0 {
			lines = append(lines, "", "No partial, closest, or unavailable profile details.")
		}
	}

	lines = appendStatusUnavailableProfiles(lines, status.UnavailableProfiles, projectRoot, maxLineWidth)
	return lines
}

func managedPatchPreviewLines(preview app.ManagedPatchPreview, valuesVisible bool, projectRoot string, maxLineWidth int) []string {
	lines := []string{
		profileComparisonLabel(preview.ProfileName, preview.Protected),
		RenderSectionHeading(managedPatchStateLabel(preview)),
	}
	if preview.Complete && preview.IncludedTargetCount > 0 && managedPatchWouldUpdateCount(preview) == 0 {
		lines = append(lines, "All included targets already match this profile.")
	}
	if preview.OmittedTargetCount > 0 {
		lines = append(lines, RenderKeyValue("Included targets", countOfTotalLabel(preview.IncludedTargetCount, preview.TargetCount)))
	}

	lines = appendManagedPatchFileGroups(lines, preview.Files, valuesVisible, projectRoot, maxLineWidth)
	lines = appendManagedPatchOmittedTargets(lines, preview.OmittedTargets, projectRoot, maxLineWidth)
	return lines
}

func managedPatchStateLabel(preview app.ManagedPatchPreview) string {
	if !preview.Complete {
		return "Some values unavailable"
	}
	wouldUpdateCount := managedPatchWouldUpdateCount(preview)
	if wouldUpdateCount > 0 {
		return "Would update " + targetCountLabel(wouldUpdateCount)
	}

	if preview.IncludedTargetCount > 0 {
		return "No changes"
	}

	return "No included targets"
}

func managedPatchWouldUpdateCount(preview app.ManagedPatchPreview) int {
	count := 0
	for _, group := range preview.Files {
		for _, hunk := range group.Hunks {
			if hunk.Status == app.ManagedPatchStatusWouldUpdate {
				count++
			}
		}
	}

	return count
}

func appendStatusTargetSection(lines []string, heading string, countLabel string, descriptors []app.TargetDescriptor, projectRoot string, maxLineWidth int) []string {
	if countLabel == "" && len(descriptors) == 0 {
		return lines
	}

	lines = append(lines, "", RenderSectionHeading(heading))
	if countLabel != "" {
		lines = append(lines, countLabel)
	}
	return append(lines, targetDescriptorDetailLines(descriptors, projectRoot, maxLineWidth)...)
}

func appendStatusProfileMatches(lines []string, heading string, matches []app.ProfileMatch) []string {
	if len(matches) == 0 {
		return lines
	}

	lines = append(lines, "", RenderSectionHeading(heading))
	for _, match := range matches {
		lines = append(lines, "  "+profileComparisonLabel(match.ProfileName, match.Protected))
	}

	return lines
}

func appendStatusPartialMatches(lines []string, matches []app.PartialProfileMatch) []string {
	if len(matches) == 0 {
		return lines
	}

	lines = append(lines, "", RenderSectionHeading("Partial matches"))
	for _, match := range matches {
		summary := fmt.Sprintf("%d/%d included match; %d omitted", match.MatchedTargets, match.IncludedTargets, match.OmittedTargets)
		lines = append(lines, "  "+profileComparisonLabel(match.ProfileName, match.Protected)+" - "+summary)
	}

	return lines
}

func appendStatusClosestProfiles(lines []string, matches []app.ClosestProfileMatch) []string {
	if len(matches) == 0 {
		return lines
	}

	lines = append(lines, "", RenderSectionHeading("Closest profiles"))
	for _, match := range matches {
		summary := fmt.Sprintf("%d/%d targets match", match.MatchedTargets, match.TargetCount)
		if match.UnavailableTargets > 0 {
			summary += fmt.Sprintf("; %d unavailable", match.UnavailableTargets)
		}
		lines = append(lines, "  "+profileComparisonLabel(match.ProfileName, match.Protected)+" - "+summary)
	}

	return lines
}

func appendStatusUnavailableProfiles(lines []string, profiles []app.UnavailableProfile, projectRoot string, maxLineWidth int) []string {
	if len(profiles) == 0 {
		return lines
	}

	lines = append(lines, "", RenderSectionHeading("Unavailable profiles"))
	for _, profile := range profiles {
		if len(profile.Values) == 0 {
			lines = append(lines, "  "+profileComparisonLabel(profile.ProfileName, profile.Protected))
			if profile.Reason != "" {
				lines = append(lines, "  "+RenderKeyValue("Reason", fitValueForLabel(profile.Reason, maxLineWidth-2, "Reason")))
			}
			continue
		}

		for _, value := range profile.Values {
			lines = append(lines, "  "+profileComparisonLabel(profile.ProfileName, profile.Protected))
			lines = append(lines, unavailableValueDetailLines(value, projectRoot, maxLineWidth)...)
		}
	}

	return lines
}

func profileComparisonLabel(profileName string, protected bool) string {
	if profileName == "" {
		profileName = "profile"
	}
	if !protected {
		return profileName
	}

	return profileName + " " + RenderBadges([]Badge{{Label: "protected"}})
}

func countOfTotalLabel(count int, total int) string {
	if total > 0 {
		return fmt.Sprintf("%d of %d", count, total)
	}

	return fmt.Sprintf("%d", count)
}

func (model Model) inspectionView() string {
	shell, ok := model.inspectionShell()
	if !ok {
		return model.listView()
	}

	return RenderShell(model.withFocusedPanelScroll(shell, 1))
}

func (model Model) inspectionShell() (Shell, bool) {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return Shell{}, false
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
		profileLines = append(profileLines, profileValueDetailLines(selectedProfile.Values, model.projectRoot, secondaryPanelContentWidth(model.width))...)
	} else {
		profileLines = appendSingleTargetContextLines(profileLines, selectedProfile, model)

		valueLines := []string{"", "Value preview", RenderKeyValue("Masked value", maskedValueLabel(selectedProfile))}
		if selectedProfile.UnavailableReason != "" {
			valueLines = append(valueLines, "", "Resolution error:", selectedProfile.UnavailableReason)
		}
		profileLines = append(profileLines, valueLines...)
	}

	return Shell{
		Headerless: true,
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Profile detail", Lines: profileLines, Focused: true, FillHeight: model.shouldFillWorkspacePanels()},
		},
		Actions: []Action{{Key: "Enter", Label: enterActionLabel(selectedProfile)}, {Key: "Space", Label: stayActionLabel(selectedProfile), Priority: ActionPriorityPrimary}, {Key: "i", Label: "Return", Priority: ActionPriorityPrimary}, {Key: "Esc", Label: "Return", Priority: ActionPriorityPrimary}, model.valueRevealAction(), {Key: "q", Label: "Quit", Priority: ActionPriorityCritical}},
		Width:   model.width,
		Height:  model.height,
	}, true
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
		lines = append(lines, profileValueTargetLines(selectedProfile.Values, model.projectRoot, secondaryPanelContentWidth(model.width))...)
		lines = append(lines, "", model.confirmationCompletionLine(), "Press Enter or y to confirm.")
	} else {
		lines = appendSingleTargetContextLines(lines, selectedProfile, model)
		lines = append(lines,
			"",
			"This will update only the configured target value.",
			"The resolved value is intentionally hidden.",
			model.confirmationCompletionLine(),
			"Press Enter or y to confirm.",
		)
	}

	return RenderShell(Shell{
		Headerless: true,
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Confirmation", Lines: lines, Focused: true, FillHeight: model.shouldFillWorkspacePanels()},
		},
		Actions: []Action{{Key: "Enter/y", Label: "Confirm", Priority: ActionPriorityPrimary}, {Key: "n/Esc", Label: "Cancel", Priority: ActionPriorityPrimary}, {Key: "q", Label: "Quit", Priority: ActionPriorityCritical}},
		Width:   model.width,
		Height:  model.height,
	})
}

func enterActionLabel(profile app.ProfileItem) string {
	switch {
	case !profile.Available:
		return "Details"
	default:
		return "Apply+Exit"
	}
}

func stayActionLabel(profile app.ProfileItem) string {
	switch {
	case !profile.Available:
		return "Details"
	default:
		return "Apply"
	}
}

func (model Model) confirmationCompletionLine() string {
	if model.confirmExits {
		return "After apply, Switchlet will exit."
	}

	return "After apply, return to the profile list."
}

func (model Model) errorView() string {
	shell := model.errorShell()
	return RenderShell(model.withFocusedPanelScroll(shell, 1))
}

func (model Model) errorShell() Shell {
	return Shell{
		Headerless: true,
		Panels: []Panel{
			model.profilePanel(RowInactiveSelected, false),
			{Title: "Error", Lines: RecoverableErrorLines(model.recoverableError, secondaryPanelContentWidth(model.width)), Focused: true, FillHeight: model.shouldFillWorkspacePanels()},
		},
		Actions: []Action{{Key: "Any key", Label: "Return"}, {Key: "q", Label: "Quit"}},
		Width:   model.width,
		Height:  model.height,
	}
}

func (model Model) withFocusedPanelScroll(shell Shell, panelIndex int) Shell {
	if panelIndex < 0 || panelIndex >= len(shell.Panels) {
		return shell
	}

	metrics := PanelScrollMetricsForShell(shell, panelIndex)
	shell.Panels[panelIndex].Scroll = &PanelScroll{Offset: metrics.ClampOffset(model.scrollOffset)}
	if metrics.CanScroll() {
		shell.Actions = append(shell.Actions, PanelScrollActions()...)
	}

	return shell
}

func (model Model) focusedPanelScrollMetrics() PanelScrollMetrics {
	shell, panelIndex, ok := model.focusedScrollablePanelShell()
	if !ok {
		return PanelScrollMetrics{}
	}

	return PanelScrollMetricsForShell(shell, panelIndex)
}

func (model Model) focusedScrollablePanelShell() (Shell, int, bool) {
	switch model.state {
	case inspectState:
		shell, ok := model.inspectionShell()
		return shell, 1, ok
	case errorState:
		return model.errorShell(), 1, true
	case statusLoadingState, statusReadyState:
		return model.statusComparisonShell(), 1, true
	case diffLoadingState, diffReadyState:
		return model.diffComparisonShell(), 0, true
	case comparisonErrorState:
		return model.comparisonErrorShell(), 1, true
	default:
		return Shell{}, 0, false
	}
}

func (model Model) successView() string {
	if model.successResult == nil {
		return "Applied profile successfully.\n"
	}

	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Profile applied.",
		Panels:   []Panel{{Title: "Result", Lines: successLines(model.successResult, model.projectRoot, fullPanelContentWidth(model.width)), Focused: true}},
		Width:    model.width,
		Height:   model.height,
	})
}

func successLines(result *app.Result, projectRoot string, maxLineWidth int) []string {
	changes := resultPlannedChanges(*result)
	lines := []string{
		RenderKeyValue("Applied profile", result.ProfileName),
		"",
		targetListHeading("Updated target", changes) + ":",
	}
	lines = append(lines, resultChangeLines(changes, projectRoot, maxLineWidth)...)
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
	for _, line := range finalResultChangeLines(changes, model.projectRoot) {
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

func (model Model) valueRevealAction() Action {
	return valueRevealAction(model.valuesVisible)
}

func valueRevealAction(valuesVisible bool) Action {
	label := "Reveal values"
	if valuesVisible {
		label = "Hide values"
	}

	return Action{Key: "v", Label: label, Priority: ActionPrioritySecondary}
}
