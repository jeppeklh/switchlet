package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeppeklh/switchlet/internal/app"
)

func shouldShowTargetCount(profile app.ProfileItem) bool {
	return profile.TotalTargets > 1 || profile.TargetCount > 1 || profile.Partial
}

func targetCountLabel(targetCount int) string {
	if targetCount == 1 {
		return "1 target"
	}

	return fmt.Sprintf("%d targets", targetCount)
}

func changeCountLabel(targetCount int, totalTargets int) string {
	if totalTargets > 0 && targetCount != totalTargets {
		return fmt.Sprintf("%d of %d targets", targetCount, totalTargets)
	}

	return targetCountLabel(targetCount)
}

func targetNamePreviewLines(values []app.ProfileValueItem, limit int) []string {
	if len(values) == 0 {
		return []string{"No affected targets."}
	}

	if limit <= 0 || limit > len(values) {
		limit = len(values)
	}

	lines := make([]string, 0, limit+1)
	for index := 0; index < limit; index++ {
		valueItem := values[index]
		line := targetNameLabel(valueItem.TargetName)
		if !valueItem.Available {
			line += " [unavailable]"
		}
		lines = append(lines, line)
	}
	if remaining := len(values) - limit; remaining > 0 {
		lines = append(lines, fmt.Sprintf("+ %d more", remaining))
	}

	return lines
}

func profileValueDetailLines(values []app.ProfileValueItem, maxLineWidth int) []string {
	if len(values) == 0 {
		return []string{"No planned target changes."}
	}

	groups := groupProfileValuesByFile(values)
	lines := make([]string, 0, len(values)*5+len(groups))
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, targetFileLabel(group.targetFile, maxLineWidth))
		for _, valueItem := range group.values {
			lines = append(lines, "  "+profileValueTargetSummary(valueItem))
			lines = append(lines, "  "+RenderKeyValue("Source", sourceLabel(valueItem.Source)))
			if valueItem.EnvironmentVariableName != "" {
				lines = append(lines, "  "+RenderKeyValue("Environment variable", valueItem.EnvironmentVariableName))
			}
			if valueItem.UnavailableReason != "" {
				lines = append(lines, "  "+RenderKeyValue("Resolution error", valueItem.UnavailableReason))
			} else {
				lines = append(lines, "  "+RenderKeyValue("Value", profileValueMaskedValueLabel(valueItem)))
			}
		}
	}

	return lines
}

func profileValueTargetLines(values []app.ProfileValueItem, maxLineWidth int) []string {
	if len(values) == 0 {
		return []string{"No affected targets."}
	}

	groups := groupProfileValuesByFile(values)
	lines := make([]string, 0, len(values)+len(groups))
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, targetFileLabel(group.targetFile, maxLineWidth))
		for _, valueItem := range group.values {
			lines = append(lines, "  "+profileValueTargetSummary(valueItem))
		}
	}

	return lines
}

func resultChangeLines(changes []app.PlannedChange, maxLineWidth int) []string {
	if len(changes) == 0 {
		return []string{"No target changes."}
	}

	lines := make([]string, 0, len(changes)*3)
	for index, change := range changes {
		if index > 0 {
			lines = append(lines, "")
		}
		prefix := "updated "
		lines = append(lines, prefix+targetFileForResultLine(change.TargetFile, valueWidthAfterPrefix(maxLineWidth, prefix)))
		lines = append(lines, "  "+targetNameLabel(change.TargetName)+targetTypeBadge(string(change.TargetType)))
		if change.Selector != "" {
			lines = append(lines, "  "+change.Selector)
		}
	}

	return lines
}

func finalResultChangeLines(changes []app.PlannedChange) []string {
	if len(changes) == 0 {
		return []string{"No target changes."}
	}

	lines := make([]string, 0, len(changes)*4)
	for index, change := range changes {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "updated "+targetFileForResultLine(change.TargetFile, 0))
		lines = append(lines, "  "+targetNameLabel(change.TargetName)+targetTypeBadge(string(change.TargetType)))
		if change.Selector != "" {
			lines = append(lines, "  "+change.Selector)
		}
	}

	return lines
}

func targetFileForResultLine(targetFile string, maxLineWidth int) string {
	if targetFile == "" {
		return "target file"
	}
	if maxLineWidth <= 0 {
		return normalizedDisplayPath(targetFile)
	}

	return compactPathForDisplay(targetFile, maxLineWidth)
}

type profileValueGroup struct {
	targetFile string
	values     []app.ProfileValueItem
}

func groupProfileValuesByFile(values []app.ProfileValueItem) []profileValueGroup {
	groups := make([]profileValueGroup, 0)
	for _, valueItem := range values {
		groupIndex := -1
		for index, group := range groups {
			if group.targetFile == valueItem.TargetFile {
				groupIndex = index
				break
			}
		}
		if groupIndex == -1 {
			groups = append(groups, profileValueGroup{targetFile: valueItem.TargetFile})
			groupIndex = len(groups) - 1
		}

		groups[groupIndex].values = append(groups[groupIndex].values, valueItem)
	}

	return groups
}

func targetFileLabel(targetFile string, maxLineWidth int) string {
	if targetFile == "" {
		return "Target details"
	}

	return compactPathForDisplay(targetFile, maxLineWidth)
}

func profileValueTargetSummary(valueItem app.ProfileValueItem) string {
	targetLabel := targetNameLabel(valueItem.TargetName) + targetTypeBadge(string(valueItem.TargetType))
	if valueItem.Selector == "" {
		return targetLabel
	}

	return fmt.Sprintf("%s -> %s", targetLabel, valueItem.Selector)
}

func profileValueMaskedValueLabel(valueItem app.ProfileValueItem) string {
	if !valueItem.Available {
		return "Unavailable"
	}
	if valueItem.MaskedValue == "" {
		return "<empty>"
	}

	return valueItem.MaskedValue
}

func recoverableProfileContextLines(profile app.ProfileItem, maxLineWidth int) []string {
	lines := []string{RenderKeyValue("Profile", selectedProfileTitle(profile))}
	if len(profile.Values) > 0 {
		lines = append(lines, "", "Affected targets")
		lines = append(lines, profileValueTargetLines(profile.Values, maxLineWidth)...)
	}
	if profile.Available {
		return lines
	}

	lines = append(lines, "", "Unavailable values")
	if profile.EnvironmentVariableName != "" {
		lines = append(lines, RenderKeyValue("Environment variable", profile.EnvironmentVariableName))
	}
	for _, valueItem := range profile.Values {
		if valueItem.Available {
			continue
		}

		lines = append(lines, RenderKeyValue("Unavailable target", targetNameLabel(valueItem.TargetName)))
		if valueItem.EnvironmentVariableName != "" {
			lines = append(lines, RenderKeyValue("Environment variable", valueItem.EnvironmentVariableName))
		}
		if valueItem.UnavailableReason != "" {
			lines = append(lines, RenderKeyValue("Target reason", valueItem.UnavailableReason))
		}
	}

	return lines
}

func (model Model) unavailableProfileError(profile app.ProfileItem) RecoverableError {
	unavailableValues := unavailableProfileValues(profile)
	context := []string{RenderKeyValue("Profile", selectedProfileTitle(profile))}
	for _, valueItem := range unavailableValues {
		context = append(context, "", RenderKeyValue("Affected target", profileValueTargetSummary(valueItem)))
		if valueItem.TargetFile != "" {
			context = append(context, RenderKeyValue("File", model.compactTargetFileValue(valueItem.TargetFile, "File")))
		}
		if valueItem.EnvironmentVariableName != "" {
			context = append(context, RenderKeyValue("Environment variable", valueItem.EnvironmentVariableName))
		}
	}

	reason := profile.UnavailableReason
	if len(unavailableValues) == 1 && unavailableValues[0].UnavailableReason != "" {
		reason = unavailableValues[0].UnavailableReason
	}
	if reason == "" {
		reason = "One or more profile values are unavailable."
	}

	recovery := "Fix the selected profile or choose another profile. Press any key to return."
	if profileHasEnvironmentUnavailableValue(unavailableValues) {
		recovery = "Set the environment variable or choose another profile. Press any key to return."
	}

	return RecoverableError{
		Problem:  fmt.Sprintf("Profile %q is unavailable.", profile.Name),
		Context:  context,
		Reason:   reason,
		Recovery: recovery,
	}
}

func (model Model) targetFailureError(profileName string, failure app.TargetFailure, cause error) RecoverableError {
	context := []string{RenderKeyValue("Profile", profileName)}
	context = append(context, RenderKeyValue("Target", targetNameLabel(failure.TargetName)+targetTypeBadge(string(failure.TargetType))))
	if failure.TargetFile != "" {
		context = append(context, RenderKeyValue("File", model.compactTargetFileValue(failure.TargetFile, "File")))
	}
	if failure.Selector != "" {
		context = append(context, RenderKeyValue("Selector", failure.Selector))
	}

	reason := failure.Reason
	if reason == "" && cause != nil {
		reason = cause.Error()
	}

	return RecoverableError{
		Problem:  fmt.Sprintf("Could not prepare target %q.", targetNameLabel(failure.TargetName)),
		Context:  context,
		Reason:   reason,
		Recovery: "Inspect this profile, fix the target, then try again. Press any key to return.",
		Cause:    cause,
	}
}

func (model Model) genericRecoverableError(cause error) RecoverableError {
	reason := "Unknown error."
	if cause != nil {
		reason = cause.Error()
	}

	context := []string(nil)
	if selectedProfile, ok := model.selectedProfile(); ok {
		context = recoverableProfileContextLines(selectedProfile, secondaryPanelContentWidth(model.width))
	}

	return RecoverableError{
		Problem:  "Action could not continue.",
		Context:  context,
		Reason:   reason,
		Recovery: "Fix the selected profile or target, then try again. Press any key to return.",
		Cause:    cause,
	}
}

func unavailableProfileValues(profile app.ProfileItem) []app.ProfileValueItem {
	unavailableValues := make([]app.ProfileValueItem, 0)
	for _, valueItem := range profile.Values {
		if !valueItem.Available {
			unavailableValues = append(unavailableValues, valueItem)
		}
	}

	return unavailableValues
}

func profileHasEnvironmentUnavailableValue(values []app.ProfileValueItem) bool {
	for _, valueItem := range values {
		if valueItem.EnvironmentVariableName != "" {
			return true
		}
	}

	return false
}

func targetNameLabel(targetName string) string {
	if targetName == "" {
		return "target"
	}

	return targetName
}

func targetTypeBadge(targetType string) string {
	if targetType == "" {
		return ""
	}

	return " [" + targetType + "]"
}

func targetSelectorDisplayLabel(selectorName string) string {
	if selectorName == "key" {
		return "Target key"
	}

	if selectorName == "jsonPath" || selectorName == "" {
		return "Target JSON path"
	}

	return "Target selector"
}

func selectorFieldName(selectorName string) string {
	if selectorName == "" {
		return "selector"
	}

	return selectorName
}

func isSingleTargetResult(result app.Result) bool {
	return len(resultPlannedChanges(result)) <= 1
}

func singleResultTargetFile(result app.Result) string {
	if result.TargetFile != "" {
		return result.TargetFile
	}
	changes := resultPlannedChanges(result)
	if len(changes) == 1 {
		return changes[0].TargetFile
	}

	return ""
}

func singleResultSelector(result app.Result) string {
	if result.TargetPath != "" {
		return result.TargetPath
	}
	changes := resultPlannedChanges(result)
	if len(changes) == 1 {
		return changes[0].Selector
	}

	return ""
}

func resultPlannedChanges(result app.Result) []app.PlannedChange {
	if len(result.Changes) > 0 {
		return result.Changes
	}
	if result.TargetFile == "" && result.TargetPath == "" {
		return nil
	}

	return []app.PlannedChange{{
		TargetFile: result.TargetFile,
		Selector:   result.TargetPath,
	}}
}

func (model Model) isApplyingSelectedProfile(profile app.ProfileItem) bool {
	return model.applyingProfile != "" && model.applyingProfile == profile.Name
}

func (model Model) compactTargetFileValue(targetFile string, label string) string {
	return compactPathForDisplay(targetFile, valueWidthForLabel(secondaryPanelContentWidth(model.width), label))
}

func compactPathForDisplay(targetPath string, maxWidth int) string {
	displayPath := normalizedDisplayPath(targetPath)
	if displayPath == "" || maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(displayPath) <= maxWidth {
		return displayPath
	}

	segments := pathSegments(displayPath)
	if len(segments) == 0 {
		return fitLine(displayPath, maxWidth)
	}

	filename := segments[len(segments)-1]
	result := compactFilename(filename, maxWidth)
	if filenameWithEllipsis := textEllipsis + "/" + filename; lipgloss.Width(filenameWithEllipsis) <= maxWidth {
		result = filenameWithEllipsis
	}

	for start := len(segments) - 2; start >= 0; start-- {
		candidate := textEllipsis + "/" + strings.Join(segments[start:], "/")
		if lipgloss.Width(candidate) > maxWidth {
			break
		}

		result = candidate
	}

	return result
}

func normalizedDisplayPath(targetPath string) string {
	trimmedPath := strings.TrimSpace(targetPath)
	if trimmedPath == "" {
		return ""
	}

	cleanedPath := filepath.ToSlash(filepath.Clean(trimmedPath))
	return strings.ReplaceAll(cleanedPath, `\`, "/")
}

func pathSegments(displayPath string) []string {
	trimmedPath := strings.Trim(displayPath, "/")
	if trimmedPath == "" {
		return nil
	}

	return strings.Split(trimmedPath, "/")
}

func compactFilename(filename string, maxWidth int) string {
	if lipgloss.Width(filename) <= maxWidth {
		return filename
	}

	ellipsisWidth := lipgloss.Width(textEllipsis)
	if maxWidth <= ellipsisWidth {
		return fitLine(filename, maxWidth)
	}

	return textEllipsis + trailingTextWithinWidth(filename, maxWidth-ellipsisWidth)
}

func trailingTextWithinWidth(value string, maxWidth int) string {
	runes := []rune(value)
	for start := len(runes) - 1; start >= 0; start-- {
		candidate := string(runes[start:])
		if lipgloss.Width(candidate) > maxWidth {
			return string(runes[start+1:])
		}
	}

	return value
}

func headerMetadataWidth(shellWidth int) int {
	return normalizedWidth(shellWidth) / 2
}

func secondaryPanelContentWidth(shellWidth int) int {
	width := normalizedWidth(shellWidth)
	panelWidth := width
	if width >= splitShellWidth {
		leftWidth := width * 55 / 100
		panelWidth = width - leftWidth - panelGapWidth
	}

	return panelTextWidth(panelWidth, defaultStyles().panel)
}

func fullPanelContentWidth(shellWidth int) int {
	return panelTextWidth(normalizedWidth(shellWidth), defaultStyles().panel)
}

func valueWidthForLabel(maxLineWidth int, label string) int {
	return valueWidthAfterPrefix(maxLineWidth, label+": ")
}

func valueWidthAfterPrefix(maxLineWidth int, prefix string) int {
	valueWidth := maxLineWidth - lipgloss.Width(prefix)
	if valueWidth < lipgloss.Width(textEllipsis)+1 {
		return maxLineWidth
	}

	return valueWidth
}
