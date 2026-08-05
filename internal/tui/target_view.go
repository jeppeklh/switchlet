package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeppeklh/switchlet/internal/app"
)

const hiddenValuePlaceholder = "****"

func shouldShowTargetCount(profile app.ProfileItem) bool {
	return profile.TargetCount > 1
}

func targetCountLabel(targetCount int) string {
	if targetCount == 1 {
		return "1 target"
	}

	return fmt.Sprintf("%d targets", targetCount)
}

func changeCountLabel(targetCount int, totalTargets int) string {
	_ = totalTargets
	return targetCountLabel(targetCount)
}

func profileValueDetailLines(values []app.ProfileValueItem, projectRoot string, maxLineWidth int) []string {
	if len(values) == 0 {
		return []string{"No planned target changes."}
	}

	groups := groupProfileValuesByFile(values)
	lines := make([]string, 0, len(values)*6+len(groups))
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, targetFileLabel(group.targetFile, projectRoot, maxLineWidth))
		for _, valueItem := range group.values {
			lines = append(lines, profileValueDetailFieldLines(valueItem, maxLineWidth)...)
		}
	}

	return lines
}

func profileValueTargetLines(values []app.ProfileValueItem, projectRoot string, maxLineWidth int) []string {
	if len(values) == 0 {
		return []string{"No affected targets."}
	}

	groups := groupProfileValuesByFile(values)
	lines := make([]string, 0, len(values)+len(groups))
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, targetFileLabel(group.targetFile, projectRoot, maxLineWidth))
		for _, valueItem := range group.values {
			lines = append(lines, profileValueTargetFieldLines(valueItem, maxLineWidth)...)
		}
	}

	return lines
}

func appendProfileContentsFileGroups(lines []string, groups []app.ProfileContentsFileGroup, projectRoot string, maxLineWidth int) []string {
	if len(groups) == 0 {
		return append(lines, "", "No included targets.")
	}

	lines = append(lines, "", RenderSectionHeading("Targets"))
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, targetFileLabel(group.TargetFile, projectRoot, maxLineWidth))
		for targetIndex, target := range group.Targets {
			if targetIndex > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, profileContentsTargetLines(target, maxLineWidth)...)
		}
	}

	return lines
}

func profileContentsTargetLines(target app.ProfileContentsTarget, maxLineWidth int) []string {
	fields := []DetailField{{Label: "Target", Value: targetNameLabel(target.TargetName)}}
	if target.Selector != "" {
		fields = append(fields, DetailField{Label: "Selector", Value: target.Selector})
	}
	if !target.Available {
		fields = appendUnavailableProfileContentsFields(fields, target)
		return renderIndentedDetailFields("  ", fields, maxLineWidth)
	}

	fields = append(fields, DetailField{Label: "Value", Value: profileContentsValueLabel(target, maxLineWidth)})
	return renderIndentedDetailFields("  ", fields, maxLineWidth)
}

func appendUnavailableProfileContentsFields(fields []DetailField, target app.ProfileContentsTarget) []DetailField {
	fields = append(fields, DetailField{Label: "Availability", Value: "Unavailable"})
	if target.EnvironmentVariableName != "" {
		fields = append(fields, DetailField{Label: "Environment variable", Value: target.EnvironmentVariableName})
	}
	if target.UnavailableReason != "" {
		fields = append(fields, DetailField{Label: "Reason", Value: target.UnavailableReason})
	}

	return fields
}

func profileContentsValueLabel(target app.ProfileContentsTarget, maxLineWidth int) string {
	if !target.ValueVisible {
		return hiddenValuePlaceholder
	}
	if target.Value == "" {
		return "<empty>"
	}

	return fitLine(target.Value, maxLineWidth-4)
}

func resultChangeLines(changes []app.PlannedChange, projectRoot string, maxLineWidth int) []string {
	if len(changes) == 0 {
		return []string{"No target changes."}
	}

	lines := make([]string, 0, len(changes)*3)
	for index, change := range changes {
		if index > 0 {
			lines = append(lines, "")
		}
		prefix := "updated "
		lines = append(lines, prefix+targetFileForResultLine(change.TargetFile, projectRoot, valueWidthAfterPrefix(maxLineWidth, prefix)))
		lines = append(lines, "  "+targetNameLabel(change.TargetName)+targetTypeBadge(string(change.TargetType)))
		if selectorLine := plannedChangeSelectorLine(change); selectorLine != "" {
			lines = append(lines, "  "+selectorLine)
		}
	}

	return lines
}

func targetDescriptorDetailLines(descriptors []app.TargetDescriptor, projectRoot string, maxLineWidth int) []string {
	lines := make([]string, 0, len(descriptors)*4)
	for index, descriptor := range descriptors {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, renderIndentedDetailFields("  ", targetDescriptorFields(descriptor, true, projectRoot), maxLineWidth)...)
	}

	return lines
}

func unavailableValueDetailLines(value app.UnavailableValue, projectRoot string, maxLineWidth int) []string {
	fields := targetDescriptorFields(value.TargetDescriptor, true, projectRoot)
	if value.EnvironmentVariableName != "" {
		fields = append(fields, DetailField{Label: "Environment variable", Value: value.EnvironmentVariableName})
	}
	if value.Reason != "" {
		fields = append(fields, DetailField{Label: "Reason", Value: value.Reason})
	}

	return renderIndentedDetailFields("  ", fields, maxLineWidth)
}

func appendManagedPatchFileGroups(lines []string, groups []app.ManagedPatchFileGroup, valuesVisible bool, projectRoot string, maxLineWidth int) []string {
	if len(groups) == 0 {
		return append(lines, "", "No included managed targets.")
	}

	for groupIndex, group := range groups {
		lines = append(lines, "")
		if groupIndex == 0 && len(groups) > 1 {
			lines = append(lines, "Affected files")
		}
		lines = append(lines, targetFileLabel(group.TargetFile, projectRoot, maxLineWidth))
		for _, hunk := range group.Hunks {
			lines = append(lines, managedPatchHunkLines(hunk, valuesVisible, projectRoot, maxLineWidth)...)
		}
	}

	return lines
}

func managedPatchHunkLines(hunk app.ManagedPatchHunk, valuesVisible bool, projectRoot string, maxLineWidth int) []string {
	fields := targetDescriptorFields(hunk.TargetDescriptor, false, projectRoot)
	if hunk.EnvironmentVariableName != "" {
		fields = append(fields, DetailField{Label: "Environment variable", Value: hunk.EnvironmentVariableName})
	}
	lines := renderIndentedDetailFields("  ", fields, maxLineWidth)

	switch hunk.Status {
	case app.ManagedPatchStatusWouldUpdate:
		return append(lines, managedPatchChangedValueLines(hunk, valuesVisible, maxLineWidth)...)
	case app.ManagedPatchStatusAlreadyMatches:
		return append(lines, managedPatchUnchangedValueLine(hunk, valuesVisible, maxLineWidth))
	case app.ManagedPatchStatusUnavailable:
		lines = append(lines, "  "+RenderKeyValue("State", managedPatchStatusLabel(hunk.Status)))
		if hunk.UnavailableReason != "" {
			lines = append(lines, "  "+RenderKeyValue("Reason", fitValueForLabel(hunk.UnavailableReason, maxLineWidth-2, "Reason")))
		}
	}

	return lines
}

func managedPatchChangedValueLines(hunk app.ManagedPatchHunk, valuesVisible bool, maxLineWidth int) []string {
	styles := defaultStyles()
	return []string{
		styles.error.Render(managedPatchValueLine("  - current  ", managedPatchCurrentValueLabel(hunk, valuesVisible), maxLineWidth)),
		styles.success.Render(managedPatchValueLine("  + profile  ", managedPatchProfileValueLabel(hunk, valuesVisible), maxLineWidth)),
	}
}

func managedPatchUnchangedValueLine(hunk app.ManagedPatchHunk, valuesVisible bool, maxLineWidth int) string {
	return defaultStyles().muted.Render(managedPatchValueLine("  = value    ", managedPatchCurrentValueLabel(hunk, valuesVisible), maxLineWidth))
}

func managedPatchValueLine(prefix string, value string, maxLineWidth int) string {
	return prefix + fitLine(value, valueWidthAfterPrefix(maxLineWidth, prefix))
}

func managedPatchCurrentValueLabel(hunk app.ManagedPatchHunk, valuesVisible bool) string {
	if !valuesVisible || !hunk.CurrentValueVisible {
		return hiddenValuePlaceholder
	}
	if hunk.CurrentValue == "" {
		return "<empty>"
	}

	return hunk.CurrentValue
}

func managedPatchProfileValueLabel(hunk app.ManagedPatchHunk, valuesVisible bool) string {
	if !valuesVisible || !hunk.ProfileValueVisible {
		return hiddenValuePlaceholder
	}
	if hunk.ProfileValue == "" {
		return "<empty>"
	}

	return hunk.ProfileValue
}

func managedPatchStatusLabel(status app.ManagedPatchStatus) string {
	switch status {
	case app.ManagedPatchStatusWouldUpdate:
		return "would update"
	case app.ManagedPatchStatusAlreadyMatches:
		return "already matches"
	case app.ManagedPatchStatusUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

func appendManagedPatchOmittedTargets(lines []string, descriptors []app.TargetDescriptor, projectRoot string, maxLineWidth int) []string {
	if len(descriptors) == 0 {
		return lines
	}

	lines = append(lines,
		"",
		"Omitted targets",
		"  Unchanged by this partial profile.",
	)
	return append(lines, targetDescriptorDetailLines(descriptors, projectRoot, maxLineWidth)...)
}

func finalResultChangeLines(changes []app.PlannedChange, projectRoot string) []string {
	if len(changes) == 0 {
		return []string{"No target changes."}
	}

	lines := make([]string, 0, len(changes)*4)
	for index, change := range changes {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "updated "+targetFileForResultLine(change.TargetFile, projectRoot, 0))
		lines = append(lines, "  "+targetNameLabel(change.TargetName)+targetTypeBadge(string(change.TargetType)))
		if selectorLine := plannedChangeSelectorLine(change); selectorLine != "" {
			lines = append(lines, "  "+selectorLine)
		}
	}

	return lines
}

func targetFileForResultLine(targetFile string, projectRoot string, maxLineWidth int) string {
	if targetFile == "" {
		return "target file"
	}
	displayPath := displayProjectPath(projectRoot, targetFile)
	if maxLineWidth <= 0 {
		return normalizedDisplayPath(displayPath)
	}

	return compactPathForDisplay(displayPath, maxLineWidth)
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

func targetFileLabel(targetFile string, projectRoot string, maxLineWidth int) string {
	if targetFile == "" {
		return "Target details"
	}

	return compactPathForDisplay(displayProjectPath(projectRoot, targetFile), maxLineWidth)
}

func profileValueTargetSummary(valueItem app.ProfileValueItem) string {
	targetLabel := profileValueTargetLabel(valueItem)
	if valueItem.Selector == "" {
		return targetLabel
	}

	return fmt.Sprintf("%s -> %s", targetLabel, selectorSummary(valueItem.SelectorName, valueItem.Selector))
}

func profileValueTargetLabel(valueItem app.ProfileValueItem) string {
	return targetNameLabel(valueItem.TargetName) + targetTypeBadge(string(valueItem.TargetType))
}

func profileValueTargetFieldLines(valueItem app.ProfileValueItem, maxLineWidth int) []string {
	fields := []DetailField{{Label: "Managed value", Value: profileValueTargetLabel(valueItem)}}
	if valueItem.Selector != "" {
		fields = append(fields, DetailField{Label: "Selector", Value: valueItem.Selector})
	}

	return renderIndentedDetailFields("  ", fields, maxLineWidth)
}

func profileValueDetailFieldLines(valueItem app.ProfileValueItem, maxLineWidth int) []string {
	fields := []DetailField{
		{Label: "Managed value", Value: profileValueTargetLabel(valueItem)},
	}
	if valueItem.Selector != "" {
		fields = append(fields, DetailField{Label: "Selector", Value: valueItem.Selector})
	}
	fields = append(fields, DetailField{Label: "Source", Value: sourceLabel(valueItem.Source)})
	if valueItem.EnvironmentVariableName != "" {
		fields = append(fields, DetailField{Label: "Environment variable", Value: valueItem.EnvironmentVariableName})
	}
	if valueItem.UnavailableReason != "" {
		fields = append(fields, DetailField{Label: "Resolution error", Value: valueItem.UnavailableReason})
	} else {
		fields = append(fields, DetailField{Label: "Value", Value: profileValueMaskedValueLabel(valueItem)})
	}

	return renderIndentedDetailFields("  ", fields, maxLineWidth)
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

func recoverableProfileContextLines(profile app.ProfileItem, projectRoot string, maxLineWidth int) []string {
	lines := []string{RenderKeyValue("Profile", selectedProfileTitle(profile))}
	if len(profile.Values) > 0 {
		lines = append(lines, "", "Affected targets")
		lines = append(lines, profileValueTargetLines(profile.Values, projectRoot, maxLineWidth)...)
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

		lines = append(lines, RenderKeyValue("Managed value", profileValueTargetLabel(valueItem)))
		if valueItem.TargetFile != "" {
			lines = append(lines, RenderKeyValue("File", compactPathForDisplay(displayProjectPath(projectRoot, valueItem.TargetFile), valueWidthForLabel(maxLineWidth, "File"))))
		}
		if valueItem.Selector != "" {
			lines = append(lines, RenderKeyValue("Selector", fitValueForLabel(valueItem.Selector, maxLineWidth, "Selector")))
		}
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
		context = append(context, "", RenderKeyValue("Managed value", profileValueTargetLabel(valueItem)))
		if valueItem.TargetFile != "" {
			context = append(context, RenderKeyValue("File", model.compactTargetFileValue(valueItem.TargetFile, "File")))
		}
		if valueItem.Selector != "" {
			context = append(context, RenderKeyValue("Selector", fitValueForLabel(valueItem.Selector, secondaryPanelContentWidth(model.width), "Selector")))
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
	context = append(context, RenderKeyValue("Managed value", targetNameLabel(failure.TargetName)+targetTypeBadge(string(failure.TargetType))))
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

func (model Model) postApplyVerificationError(profileName string, verificationErr app.PostApplyVerificationError, cause error) RecoverableError {
	context := []string{RenderKeyValue("Profile", profileName)}
	for _, failure := range verificationErr.Failures {
		context = append(context, "", RenderKeyValue("Managed value", targetNameLabel(failure.TargetName)+targetTypeBadge(string(failure.TargetType))))
		if failure.TargetFile != "" {
			context = append(context, RenderKeyValue("File", model.compactTargetFileValue(failure.TargetFile, "File")))
		}
		if failure.Selector != "" {
			context = append(context, RenderKeyValue("Selector", fitValueForLabel(failure.Selector, secondaryPanelContentWidth(model.width), "Selector")))
		}
		if failure.Reason != "" {
			context = append(context, RenderKeyValue("Reason", failure.Reason))
		}
	}

	reason := "One or more managed targets did not match the selected profile after writing."
	if len(verificationErr.Failures) == 1 && verificationErr.Failures[0].Reason != "" {
		reason = verificationErr.Failures[0].Reason
	}

	return RecoverableError{
		Problem:  "Writes completed, but Switchlet could not confirm the final managed state.",
		Context:  context,
		Reason:   reason,
		Recovery: "Return to the profile list, then use status or diff to review current managed values. Press any key to return.",
		Cause:    cause,
	}
}

func (model Model) comparisonFailureError(kind comparisonRequestKind, profileName string, cause error) RecoverableError {
	problem := "Could not compare current status."
	context := []string{RenderKeyValue("Action", "Current status")}
	if kind == comparisonRequestDiff {
		problem = fmt.Sprintf("Could not compare profile %q.", profileName)
		context = []string{RenderKeyValue("Action", "Selected-profile diff"), RenderKeyValue("Profile", profileName)}
	}

	if targetFailure, ok := app.TargetFailureFromError(cause); ok {
		context = append(context, RenderKeyValue("Managed value", targetNameLabel(targetFailure.TargetName)+targetTypeBadge(string(targetFailure.TargetType))))
		if targetFailure.TargetFile != "" {
			context = append(context, RenderKeyValue("File", model.compactTargetFileValue(targetFailure.TargetFile, "File")))
		}
		if targetFailure.Selector != "" {
			context = append(context, RenderKeyValue("Selector", targetFailure.Selector))
		}
	}

	reason := "Unknown error."
	if cause != nil {
		reason = cause.Error()
	}

	return RecoverableError{
		Problem:  problem,
		Context:  context,
		Reason:   reason,
		Recovery: "Fix the target or profile, then return and try again.",
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
		context = recoverableProfileContextLines(selectedProfile, model.projectRoot, secondaryPanelContentWidth(model.width))
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

func selectorFieldName(selectorName string) string {
	if selectorName == "" {
		return "selector"
	}

	return selectorName
}

func selectorSummary(selectorName string, selector string) string {
	if selectorName == "yamlPath" || selectorName == "tomlPath" {
		return selectorFieldName(selectorName) + ": " + selector
	}

	return selector
}

func plannedChangeSelectorLine(change app.PlannedChange) string {
	if change.Selector == "" {
		return ""
	}

	return selectorSummary(change.SelectorName, change.Selector)
}

func targetDescriptorFields(descriptor app.TargetDescriptor, includeFile bool, projectRoot string) []DetailField {
	fields := make([]DetailField, 0, 3)
	if includeFile && descriptor.TargetFile != "" {
		fields = append(fields, DetailField{Label: "File", Value: displayProjectPath(projectRoot, descriptor.TargetFile)})
	}
	fields = append(fields, DetailField{Label: "Managed value", Value: targetNameLabel(descriptor.TargetName) + targetTypeBadge(string(descriptor.TargetType))})
	if descriptor.Selector != "" {
		fields = append(fields, DetailField{Label: "Selector", Value: descriptor.Selector})
	}

	return fields
}

func renderIndentedDetailFields(indent string, fields []DetailField, maxLineWidth int) []string {
	return RenderIndentedFieldRows(indent, fitDetailFields(fields, indent, maxLineWidth))
}

func fitDetailFields(fields []DetailField, indent string, maxLineWidth int) []DetailField {
	if maxLineWidth <= 0 || len(fields) == 0 {
		return fields
	}

	labelWidth := maxDetailFieldLabelWidth(fields)
	valueWidth := maxLineWidth - lipgloss.Width(indent) - labelWidth - 2
	if valueWidth < lipgloss.Width(textEllipsis)+1 {
		valueWidth = maxLineWidth - lipgloss.Width(indent)
	}
	if valueWidth <= 0 {
		return fields
	}

	fittedFields := make([]DetailField, 0, len(fields))
	for _, field := range fields {
		if field.Label == "File" {
			field.Value = compactPathForDisplay(field.Value, valueWidth)
		} else {
			field.Value = fitLine(field.Value, valueWidth)
		}
		fittedFields = append(fittedFields, field)
	}

	return fittedFields
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
	return compactPathForDisplay(model.displayProjectPath(targetFile), valueWidthForLabel(secondaryPanelContentWidth(model.width), label))
}

func (model Model) displayProjectPath(targetPath string) string {
	return displayProjectPath(model.projectRoot, targetPath)
}

func displayProjectPath(projectRoot string, targetPath string) string {
	if projectRoot == "" || targetPath == "" {
		return targetPath
	}

	relativePath, err := filepath.Rel(projectRoot, targetPath)
	if err != nil || relativePath == "." || isParentRelativePath(relativePath) || filepath.IsAbs(relativePath) {
		return targetPath
	}

	return filepath.ToSlash(relativePath)
}

func isParentRelativePath(targetPath string) bool {
	return targetPath == ".." || strings.HasPrefix(targetPath, ".."+string(filepath.Separator))
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

func fitValueForLabel(value string, maxLineWidth int, label string) string {
	return fitLine(value, valueWidthForLabel(maxLineWidth, label))
}
