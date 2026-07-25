package tui

import (
	"fmt"

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

func profileValueDetailLines(values []app.ProfileValueItem) []string {
	if len(values) == 0 {
		return []string{"No planned target changes."}
	}

	groups := groupProfileValuesByFile(values)
	lines := make([]string, 0, len(values)*5+len(groups))
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, targetFileLabel(group.targetFile))
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

func profileValueTargetLines(values []app.ProfileValueItem) []string {
	if len(values) == 0 {
		return []string{"No affected targets."}
	}

	groups := groupProfileValuesByFile(values)
	lines := make([]string, 0, len(values)+len(groups))
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, targetFileLabel(group.targetFile))
		for _, valueItem := range group.values {
			lines = append(lines, "  "+profileValueTargetSummary(valueItem))
		}
	}

	return lines
}

func resultChangeLines(changes []app.PlannedChange) []string {
	if len(changes) == 0 {
		return []string{"No target changes."}
	}

	lines := make([]string, 0, len(changes)*2)
	for _, change := range changes {
		lines = append(lines, fmt.Sprintf("%s%s -> %s", targetNameLabel(change.TargetName), targetTypeBadge(string(change.TargetType)), change.TargetFile))
		if change.Selector != "" {
			lines = append(lines, "  "+RenderKeyValue(selectorFieldName(change.SelectorName), change.Selector))
		}
	}

	return lines
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

func targetFileLabel(targetFile string) string {
	if targetFile == "" {
		return "Target details"
	}

	return targetFile
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

func recoverableProfileContextLines(profile app.ProfileItem) []string {
	if profile.Available {
		return nil
	}

	lines := make([]string, 0, len(profile.Values)*3)
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
	return len(result.Changes) <= 1
}

func singleResultTargetFile(result app.Result) string {
	if result.TargetFile != "" {
		return result.TargetFile
	}
	if len(result.Changes) == 1 {
		return result.Changes[0].TargetFile
	}

	return ""
}

func singleResultSelector(result app.Result) string {
	if result.TargetPath != "" {
		return result.TargetPath
	}
	if len(result.Changes) == 1 {
		return result.Changes[0].Selector
	}

	return ""
}

func (model Model) isApplyingSelectedProfile(profile app.ProfileItem) bool {
	return model.applyingProfile != "" && model.applyingProfile == profile.Name
}
