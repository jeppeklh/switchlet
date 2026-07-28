package main

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/app"
)

func availabilityLabel(profileItem app.ProfileItem) string {
	if profileItem.Available {
		return "Available"
	}

	return "Unavailable"
}

func sourceLabel(source app.ProfileSource) string {
	switch source {
	case app.ProfileSourceEnvironment:
		return "Environment variable"
	case app.ProfileSourceLiteral:
		return "Literal"
	case app.ProfileSourceMixed:
		return "Mixed"
	default:
		return "Unknown"
	}
}

func protectionLabel(profileItem app.ProfileItem) string {
	if profileItem.Protected {
		return "Protected"
	}

	return "Not protected"
}

func maskedValueLabel(profileItem app.ProfileItem) string {
	if !profileItem.Available {
		return "Unavailable"
	}
	if profileItem.MaskedValue == "" {
		return "<empty>"
	}

	return profileItem.MaskedValue
}

func profileIndicators(profileItem app.ProfileItem) []string {
	indicators := make([]string, 0, 4)
	if shouldShowTargetCount(profileItem) {
		indicators = append(indicators, targetCountLabel(profileItem.TargetCount))
	}
	if profileItem.Partial {
		indicators = append(indicators, "partial")
	}
	if profileItem.Protected {
		indicators = append(indicators, "protected")
	}
	if !profileItem.Available {
		indicators = append(indicators, "unavailable")
	}

	return indicators
}

func shouldShowTargetCount(profileItem app.ProfileItem) bool {
	return profileItem.TotalTargets > 1 || profileItem.TargetCount > 1 || profileItem.Partial
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

func valueMaskedValueLabel(valueItem app.ProfileValueItem) string {
	if !valueItem.Available {
		return "Unavailable"
	}
	if valueItem.MaskedValue == "" {
		return "<empty>"
	}

	return valueItem.MaskedValue
}

func applyResultStatus(result app.Result) string {
	if result.DryRun {
		return "dry_run"
	}

	return "applied"
}

func availabilityStatus(available bool) string {
	if available {
		return "available"
	}

	return "unavailable"
}
