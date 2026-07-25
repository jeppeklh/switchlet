package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

func writeListText(output io.Writer, profiles []app.ProfileItem) error {
	for _, profileItem := range profiles {
		line := profileItem.Name
		indicators := profileIndicators(profileItem)
		if len(indicators) > 0 {
			line += " [" + strings.Join(indicators, ", ") + "]"
		}

		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
		if profileItem.UnavailableReason != "" {
			if _, err := fmt.Fprintf(output, "  reason: %s\n", profileItem.UnavailableReason); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeInspectText(output io.Writer, profileItem app.ProfileItem) error {
	if _, err := fmt.Fprintf(output, "Profile: %s\n", profileItem.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Availability: %s\n", availabilityLabel(profileItem)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Source: %s\n", sourceLabel(profileItem.Source)); err != nil {
		return err
	}
	if profileItem.EnvironmentVariableName != "" {
		if _, err := fmt.Fprintf(output, "Environment variable: %s\n", profileItem.EnvironmentVariableName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(output, "Protection: %s\n\n", protectionLabel(profileItem)); err != nil {
		return err
	}
	if profileItem.TargetCount > 0 {
		if _, err := fmt.Fprintf(output, "Changes: %s\n\n", changeCountLabel(profileItem.TargetCount, profileItem.TotalTargets)); err != nil {
			return err
		}
	}
	if len(profileItem.Values) <= 1 {
		if _, err := fmt.Fprintf(output, "Masked value:\n%s\n", maskedValueLabel(profileItem)); err != nil {
			return err
		}
		if profileItem.UnavailableReason != "" {
			if _, err := fmt.Fprintf(output, "\nResolution error:\n%s\n", profileItem.UnavailableReason); err != nil {
				return err
			}
		}
	}
	if len(profileItem.Values) > 0 {
		if _, err := fmt.Fprintln(output, "\nPlanned targets:"); err != nil {
			return err
		}
		for _, valueItem := range profileItem.Values {
			if err := writeProfileValueText(output, valueItem); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeApplyText(output io.Writer, result app.Result) error {
	if result.DryRun {
		if isSingleTargetResult(result) {
			if _, err := fmt.Fprintf(output, "Dry run successful: %s\n\nTarget file:\n%s\n\n%s:\n%s\n\nNo changes were written.\n", result.ProfileName, singleResultTargetFile(result), singleResultSelectorLabel(result), singleResultSelector(result)); err != nil {
				return err
			}

			return nil
		}

		if _, err := fmt.Fprintf(output, "Dry run successful: %s\n\nPlanned targets: %d\n", result.ProfileName, len(result.Changes)); err != nil {
			return err
		}
		if err := writePlannedChangesText(output, result.Changes); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "\nNo changes were written."); err != nil {
			return err
		}

		return nil
	}

	if isSingleTargetResult(result) {
		if _, err := fmt.Fprintf(output, "Applied profile: %s\n\nTarget file:\n%s\n\nUpdated target:\n%s\n", result.ProfileName, singleResultTargetFile(result), singleResultSelector(result)); err != nil {
			return err
		}

		return nil
	}

	if _, err := fmt.Fprintf(output, "Applied profile: %s\n\nUpdated targets: %d\n", result.ProfileName, len(result.Changes)); err != nil {
		return err
	}
	if err := writePlannedChangesText(output, result.Changes); err != nil {
		return err
	}

	return nil
}

func writeListJSON(output io.Writer, profiles []app.ProfileItem) error {
	encodedProfiles := make([]profileJSON, 0, len(profiles))
	for _, profileItem := range profiles {
		encodedProfiles = append(encodedProfiles, profileJSONFromItem(profileItem))
	}

	return writeJSON(output, struct {
		Profiles []profileJSON `json:"profiles"`
	}{Profiles: encodedProfiles})
}

func writeInspectJSON(output io.Writer, profileItem app.ProfileItem) error {
	return writeJSON(output, struct {
		Profile profileJSON `json:"profile"`
	}{Profile: profileJSONFromItem(profileItem)})
}

func writeApplyJSON(output io.Writer, result app.Result) error {
	return writeJSON(output, struct {
		Result applyResultJSON `json:"result"`
	}{Result: applyResultJSON{
		ProfileName: result.ProfileName,
		Status:      applyResultStatus(result),
		TargetPath:  result.TargetPath,
		TargetFile:  result.TargetFile,
		TargetCount: len(result.Changes),
		Changes:     plannedChangeJSONFromChanges(result.Changes),
		Protected:   result.Protected,
		DryRun:      result.DryRun,
	}})
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

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

func writeProfileValueText(output io.Writer, valueItem app.ProfileValueItem) error {
	if _, err := fmt.Fprintf(output, "- %s%s\n", targetNameLabel(valueItem.TargetName), targetTypeBadge(string(valueItem.TargetType))); err != nil {
		return err
	}
	if valueItem.TargetFile != "" {
		if _, err := fmt.Fprintf(output, "  file: %s\n", valueItem.TargetFile); err != nil {
			return err
		}
	}
	if valueItem.Selector != "" {
		if _, err := fmt.Fprintf(output, "  %s: %s\n", selectorFieldName(valueItem.SelectorName), valueItem.Selector); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(output, "  status: %s\n", availabilityStatus(valueItem.Available)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  source: %s\n", sourceLabel(valueItem.Source)); err != nil {
		return err
	}
	if valueItem.EnvironmentVariableName != "" {
		if _, err := fmt.Fprintf(output, "  environment variable: %s\n", valueItem.EnvironmentVariableName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(output, "  masked value: %s\n", valueMaskedValueLabel(valueItem)); err != nil {
		return err
	}
	if valueItem.UnavailableReason != "" {
		if _, err := fmt.Fprintf(output, "  resolution error: %s\n", valueItem.UnavailableReason); err != nil {
			return err
		}
	}

	return nil
}

func writePlannedChangesText(output io.Writer, changes []app.PlannedChange) error {
	for _, change := range changes {
		if _, err := fmt.Fprintf(output, "%s%s -> %s\n", targetNameLabel(change.TargetName), targetTypeBadge(string(change.TargetType)), change.TargetFile); err != nil {
			return err
		}
		if change.Selector != "" {
			if _, err := fmt.Fprintf(output, "  %s: %s\n", selectorFieldName(change.SelectorName), change.Selector); err != nil {
				return err
			}
		}
	}

	return nil
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

func singleResultSelectorLabel(result app.Result) string {
	if len(result.Changes) == 1 && result.Changes[0].SelectorName == "key" {
		return "Target key"
	}

	return "Target JSON path"
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

type profileJSON struct {
	Name                    string             `json:"name"`
	Status                  string             `json:"status"`
	Protected               bool               `json:"protected"`
	Available               bool               `json:"available"`
	Source                  app.ProfileSource  `json:"source"`
	EnvironmentVariableName string             `json:"environmentVariableName"`
	MaskedValue             string             `json:"maskedValue"`
	UnavailableReason       string             `json:"unavailableReason"`
	TargetCount             int                `json:"targetCount"`
	TotalTargets            int                `json:"totalTargets"`
	Partial                 bool               `json:"partial"`
	Values                  []profileValueJSON `json:"values"`
}

type applyResultJSON struct {
	ProfileName string              `json:"profileName"`
	Status      string              `json:"status"`
	TargetPath  string              `json:"targetPath"`
	TargetFile  string              `json:"targetFile"`
	TargetCount int                 `json:"targetCount"`
	Changes     []plannedChangeJSON `json:"changes"`
	Protected   bool                `json:"protected"`
	DryRun      bool                `json:"dryRun"`
}

type profileValueJSON struct {
	TargetName              string            `json:"targetName"`
	TargetFile              string            `json:"targetFile"`
	TargetType              string            `json:"targetType"`
	SelectorName            string            `json:"selectorName"`
	Selector                string            `json:"selector"`
	Status                  string            `json:"status"`
	Available               bool              `json:"available"`
	Source                  app.ProfileSource `json:"source"`
	EnvironmentVariableName string            `json:"environmentVariableName"`
	MaskedValue             string            `json:"maskedValue"`
	UnavailableReason       string            `json:"unavailableReason"`
}

type plannedChangeJSON struct {
	TargetName   string `json:"targetName"`
	TargetFile   string `json:"targetFile"`
	TargetType   string `json:"targetType"`
	SelectorName string `json:"selectorName"`
	Selector     string `json:"selector"`
}

func profileJSONFromItem(profileItem app.ProfileItem) profileJSON {
	return profileJSON{
		Name:                    profileItem.Name,
		Status:                  availabilityStatus(profileItem.Available),
		Protected:               profileItem.Protected,
		Available:               profileItem.Available,
		Source:                  profileItem.Source,
		EnvironmentVariableName: profileItem.EnvironmentVariableName,
		MaskedValue:             profileItem.MaskedValue,
		UnavailableReason:       profileItem.UnavailableReason,
		TargetCount:             profileItem.TargetCount,
		TotalTargets:            profileItem.TotalTargets,
		Partial:                 profileItem.Partial,
		Values:                  profileValueJSONFromItems(profileItem.Values),
	}
}

func profileValueJSONFromItems(values []app.ProfileValueItem) []profileValueJSON {
	encodedValues := make([]profileValueJSON, 0, len(values))
	for _, valueItem := range values {
		reason := valueItem.UnavailableReason
		encodedValues = append(encodedValues, profileValueJSON{
			TargetName:              valueItem.TargetName,
			TargetFile:              valueItem.TargetFile,
			TargetType:              string(valueItem.TargetType),
			SelectorName:            valueItem.SelectorName,
			Selector:                valueItem.Selector,
			Status:                  availabilityStatus(valueItem.Available),
			Available:               valueItem.Available,
			Source:                  valueItem.Source,
			EnvironmentVariableName: valueItem.EnvironmentVariableName,
			MaskedValue:             valueItem.MaskedValue,
			UnavailableReason:       reason,
		})
	}

	return encodedValues
}

func plannedChangeJSONFromChanges(changes []app.PlannedChange) []plannedChangeJSON {
	encodedChanges := make([]plannedChangeJSON, 0, len(changes))
	for _, change := range changes {
		encodedChanges = append(encodedChanges, plannedChangeJSON{
			TargetName:   change.TargetName,
			TargetFile:   change.TargetFile,
			TargetType:   string(change.TargetType),
			SelectorName: change.SelectorName,
			Selector:     change.Selector,
		})
	}

	return encodedChanges
}
