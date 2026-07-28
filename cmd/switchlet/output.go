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
		if _, err := fmt.Fprintf(output, "Dry run successful for profile %q\n\n%s:\n", result.ProfileName, targetListHeading("Planned target", result.Changes)); err != nil {
			return err
		}
		if err := writePlannedChangesText(output, result.Changes, "would update"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "\nNo changes were written."); err != nil {
			return err
		}

		return nil
	}

	if _, err := fmt.Fprintf(output, "Applied profile %q\n\n%s:\n", result.ProfileName, targetListHeading("Updated target", result.Changes)); err != nil {
		return err
	}
	if err := writePlannedChangesText(output, result.Changes, "updated"); err != nil {
		return err
	}

	return nil
}

func writeStatusText(output io.Writer, status app.StatusComparison) error {
	switch status.Status {
	case app.StatusComparisonMatched:
		if _, err := fmt.Fprintf(output, "Current profile: %s\n", status.CurrentProfile); err != nil {
			return err
		}
		if err := writeTargetDescriptorSection(output, "Matched targets", status.MatchedTargets); err != nil {
			return err
		}
	case app.StatusComparisonAmbiguous:
		if _, err := fmt.Fprintln(output, "Current configuration matches multiple complete profiles."); err != nil {
			return err
		}
		if err := writeProfileMatchSection(output, "Matches", status.Matches); err != nil {
			return err
		}
	default:
		if _, err := fmt.Fprintln(output, "Current configuration does not match any complete profile."); err != nil {
			return err
		}
		if err := writePartialMatchSection(output, status.PartialMatches); err != nil {
			return err
		}
		if err := writeClosestProfileSection(output, status.ClosestProfiles); err != nil {
			return err
		}
	}

	return writeUnavailableProfileSection(output, status.UnavailableProfiles)
}

func writeDiffText(output io.Writer, diff app.ProfileDiff) error {
	if _, err := fmt.Fprintf(output, "Diff for profile %q\n", diff.ProfileName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Protection: %s\n", diffProtectionLabel(diff)); err != nil {
		return err
	}

	if err := writeTargetDescriptorSection(output, "Would update", diff.WouldUpdate); err != nil {
		return err
	}
	if err := writeTargetDescriptorSection(output, "Already matches", diff.AlreadyMatches); err != nil {
		return err
	}
	if err := writeUnavailableValueSection(output, "Unavailable", diff.Unavailable); err != nil {
		return err
	}
	return writeTargetDescriptorSection(output, "Omitted targets", diff.OmittedTargets)
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
		Changes:     targetDescriptorJSONFromDescriptors(result.Changes),
		Protected:   result.Protected,
		DryRun:      result.DryRun,
	}})
}

func writeStatusJSON(output io.Writer, status app.StatusComparison) error {
	return writeJSON(output, struct {
		Result statusResultJSON `json:"result"`
	}{Result: statusResultJSON{
		Command:             "status",
		Status:              string(status.Status),
		CurrentProfile:      status.CurrentProfile,
		Matches:             profileMatchJSONFromMatches(status.Matches),
		MatchedTargets:      targetDescriptorJSONFromDescriptors(status.MatchedTargets),
		PartialMatches:      partialProfileMatchJSONFromMatches(status.PartialMatches),
		ClosestProfiles:     closestProfileMatchJSONFromMatches(status.ClosestProfiles),
		UnavailableProfiles: unavailableProfileJSONFromProfiles(status.UnavailableProfiles),
		TargetCount:         status.TargetCount,
		Complete:            status.Complete,
	}})
}

func writeDiffJSON(output io.Writer, diff app.ProfileDiff) error {
	return writeJSON(output, struct {
		Result diffResultJSON `json:"result"`
	}{Result: diffResultJSON{
		Command:        "diff",
		ProfileName:    diff.ProfileName,
		Protected:      diff.Protected,
		Complete:       diff.Complete,
		WouldUpdate:    targetDescriptorJSONFromDescriptors(diff.WouldUpdate),
		AlreadyMatches: targetDescriptorJSONFromDescriptors(diff.AlreadyMatches),
		Unavailable:    unavailableValueJSONFromValues(diff.Unavailable),
		OmittedTargets: targetDescriptorJSONFromDescriptors(diff.OmittedTargets),
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

func diffProtectionLabel(diff app.ProfileDiff) string {
	if diff.Protected {
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

func writePlannedChangesText(output io.Writer, changes []app.PlannedChange, marker string) error {
	for index, change := range changes {
		if _, err := fmt.Fprintf(output, "%s %s\n", marker, change.TargetFile); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(output, "  %s%s\n", targetNameLabel(change.TargetName), targetTypeBadge(string(change.TargetType))); err != nil {
			return err
		}
		if change.Selector != "" {
			if _, err := fmt.Fprintf(output, "  %s\n", change.Selector); err != nil {
				return err
			}
		}
		if index < len(changes)-1 {
			if _, err := fmt.Fprintln(output); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeTargetDescriptorSection(output io.Writer, heading string, descriptors []app.TargetDescriptor) error {
	if len(descriptors) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s:\n", heading); err != nil {
		return err
	}
	for _, descriptor := range descriptors {
		if err := writeTargetDescriptorText(output, descriptor); err != nil {
			return err
		}
	}

	return nil
}

func writeTargetDescriptorText(output io.Writer, descriptor app.TargetDescriptor) error {
	if _, err := fmt.Fprintf(output, "- %s%s\n", targetNameLabel(descriptor.TargetName), targetTypeBadge(string(descriptor.TargetType))); err != nil {
		return err
	}
	if descriptor.TargetFile != "" {
		if _, err := fmt.Fprintf(output, "  file: %s\n", descriptor.TargetFile); err != nil {
			return err
		}
	}
	if descriptor.Selector != "" {
		if _, err := fmt.Fprintf(output, "  %s: %s\n", selectorFieldName(descriptor.SelectorName), descriptor.Selector); err != nil {
			return err
		}
	}

	return nil
}

func writeProfileMatchSection(output io.Writer, heading string, matches []app.ProfileMatch) error {
	if len(matches) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s:\n", heading); err != nil {
		return err
	}
	for _, match := range matches {
		if _, err := fmt.Fprintf(output, "- %s\n", profileLabel(match.ProfileName, match.Protected)); err != nil {
			return err
		}
	}

	return nil
}

func writePartialMatchSection(output io.Writer, matches []app.PartialProfileMatch) error {
	if len(matches) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(output, "\nPartial matches:"); err != nil {
		return err
	}
	for _, match := range matches {
		if _, err := fmt.Fprintf(
			output,
			"- %s: %d of %d included targets match; %d targets omitted\n",
			profileLabel(match.ProfileName, match.Protected),
			match.MatchedTargets,
			match.IncludedTargets,
			match.OmittedTargets,
		); err != nil {
			return err
		}
	}

	return nil
}

func writeClosestProfileSection(output io.Writer, matches []app.ClosestProfileMatch) error {
	if len(matches) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(output, "\nClosest profiles:"); err != nil {
		return err
	}
	for _, match := range matches {
		line := fmt.Sprintf(
			"- %s: %d of %d targets match",
			profileLabel(match.ProfileName, match.Protected),
			match.MatchedTargets,
			match.TargetCount,
		)
		if match.UnavailableTargets > 0 {
			line += fmt.Sprintf("; %d unavailable", match.UnavailableTargets)
		}
		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
	}

	return nil
}

func writeUnavailableProfileSection(output io.Writer, profiles []app.UnavailableProfile) error {
	if len(profiles) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(output, "\nUnavailable profiles:"); err != nil {
		return err
	}
	for _, profile := range profiles {
		if len(profile.Values) == 0 {
			if _, err := fmt.Fprintf(output, "- %s\n", profileLabel(profile.ProfileName, profile.Protected)); err != nil {
				return err
			}
			if profile.Reason != "" {
				if _, err := fmt.Fprintf(output, "  reason: %s\n", profile.Reason); err != nil {
					return err
				}
			}
			continue
		}

		for _, value := range profile.Values {
			if _, err := fmt.Fprintf(output, "- %s / %s%s\n", profileLabel(profile.ProfileName, profile.Protected), targetNameLabel(value.TargetName), targetTypeBadge(string(value.TargetType))); err != nil {
				return err
			}
			if err := writeUnavailableValueDetails(output, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeUnavailableValueSection(output io.Writer, heading string, values []app.UnavailableValue) error {
	if len(values) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s:\n", heading); err != nil {
		return err
	}
	for _, value := range values {
		if _, err := fmt.Fprintf(output, "- %s%s\n", targetNameLabel(value.TargetName), targetTypeBadge(string(value.TargetType))); err != nil {
			return err
		}
		if err := writeUnavailableValueDetails(output, value); err != nil {
			return err
		}
	}

	return nil
}

func writeUnavailableValueDetails(output io.Writer, value app.UnavailableValue) error {
	if value.TargetFile != "" {
		if _, err := fmt.Fprintf(output, "  file: %s\n", value.TargetFile); err != nil {
			return err
		}
	}
	if value.Selector != "" {
		if _, err := fmt.Fprintf(output, "  %s: %s\n", selectorFieldName(value.SelectorName), value.Selector); err != nil {
			return err
		}
	}
	if value.EnvironmentVariableName != "" {
		if _, err := fmt.Fprintf(output, "  environment variable: %s\n", value.EnvironmentVariableName); err != nil {
			return err
		}
	}
	if value.Reason != "" {
		if _, err := fmt.Fprintf(output, "  reason: %s\n", value.Reason); err != nil {
			return err
		}
	}

	return nil
}

func targetListHeading(singular string, changes []app.PlannedChange) string {
	if len(changes) == 1 {
		return singular
	}

	return singular + "s"
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

func profileLabel(profileName string, protected bool) string {
	if protected {
		return profileName + " [protected]"
	}

	return profileName
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
	if len(result.Changes) != 1 {
		return "Target JSON path"
	}

	switch result.Changes[0].SelectorName {
	case "key":
		return "Target key"
	case "yamlPath":
		return "Target YAML path"
	case "tomlPath":
		return "Target TOML path"
	default:
		return "Target JSON path"
	}
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
	ProfileName string                 `json:"profileName"`
	Status      string                 `json:"status"`
	TargetPath  string                 `json:"targetPath"`
	TargetFile  string                 `json:"targetFile"`
	TargetCount int                    `json:"targetCount"`
	Changes     []targetDescriptorJSON `json:"changes"`
	Protected   bool                   `json:"protected"`
	DryRun      bool                   `json:"dryRun"`
}

type statusResultJSON struct {
	Command             string                    `json:"command"`
	Status              string                    `json:"status"`
	CurrentProfile      string                    `json:"currentProfile"`
	Matches             []profileMatchJSON        `json:"matches"`
	MatchedTargets      []targetDescriptorJSON    `json:"matchedTargets"`
	PartialMatches      []partialProfileMatchJSON `json:"partialMatches"`
	ClosestProfiles     []closestProfileMatchJSON `json:"closestProfiles"`
	UnavailableProfiles []unavailableProfileJSON  `json:"unavailableProfiles"`
	TargetCount         int                       `json:"targetCount"`
	Complete            bool                      `json:"complete"`
}

type diffResultJSON struct {
	Command        string                 `json:"command"`
	ProfileName    string                 `json:"profileName"`
	Protected      bool                   `json:"protected"`
	Complete       bool                   `json:"complete"`
	WouldUpdate    []targetDescriptorJSON `json:"wouldUpdate"`
	AlreadyMatches []targetDescriptorJSON `json:"alreadyMatches"`
	Unavailable    []unavailableValueJSON `json:"unavailable"`
	OmittedTargets []targetDescriptorJSON `json:"omittedTargets"`
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

type targetDescriptorJSON struct {
	TargetName   string `json:"targetName"`
	TargetFile   string `json:"targetFile"`
	TargetType   string `json:"targetType"`
	SelectorName string `json:"selectorName"`
	Selector     string `json:"selector"`
}

type profileMatchJSON struct {
	ProfileName string `json:"profileName"`
	Protected   bool   `json:"protected"`
}

type partialProfileMatchJSON struct {
	ProfileName     string `json:"profileName"`
	Protected       bool   `json:"protected"`
	MatchedTargets  int    `json:"matchedTargets"`
	IncludedTargets int    `json:"includedTargets"`
	OmittedTargets  int    `json:"omittedTargets"`
	TargetCount     int    `json:"targetCount"`
}

type closestProfileMatchJSON struct {
	ProfileName        string `json:"profileName"`
	Protected          bool   `json:"protected"`
	MatchedTargets     int    `json:"matchedTargets"`
	IncludedTargets    int    `json:"includedTargets"`
	UnavailableTargets int    `json:"unavailableTargets"`
	TargetCount        int    `json:"targetCount"`
}

type unavailableProfileJSON struct {
	ProfileName string                 `json:"profileName"`
	Protected   bool                   `json:"protected"`
	Reason      string                 `json:"reason"`
	Values      []unavailableValueJSON `json:"values"`
}

type unavailableValueJSON struct {
	TargetName              string `json:"targetName"`
	TargetFile              string `json:"targetFile"`
	TargetType              string `json:"targetType"`
	SelectorName            string `json:"selectorName"`
	Selector                string `json:"selector"`
	EnvironmentVariableName string `json:"environmentVariable"`
	Reason                  string `json:"reason"`
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

func targetDescriptorJSONFromDescriptors(descriptors []app.TargetDescriptor) []targetDescriptorJSON {
	encodedDescriptors := make([]targetDescriptorJSON, 0, len(descriptors))
	for _, descriptor := range descriptors {
		encodedDescriptors = append(encodedDescriptors, targetDescriptorJSONFromDescriptor(descriptor))
	}

	return encodedDescriptors
}

func targetDescriptorJSONFromDescriptor(descriptor app.TargetDescriptor) targetDescriptorJSON {
	return targetDescriptorJSON{
		TargetName:   descriptor.TargetName,
		TargetFile:   descriptor.TargetFile,
		TargetType:   string(descriptor.TargetType),
		SelectorName: descriptor.SelectorName,
		Selector:     descriptor.Selector,
	}
}

func profileMatchJSONFromMatches(matches []app.ProfileMatch) []profileMatchJSON {
	encodedMatches := make([]profileMatchJSON, 0, len(matches))
	for _, match := range matches {
		encodedMatches = append(encodedMatches, profileMatchJSON{
			ProfileName: match.ProfileName,
			Protected:   match.Protected,
		})
	}

	return encodedMatches
}

func partialProfileMatchJSONFromMatches(matches []app.PartialProfileMatch) []partialProfileMatchJSON {
	encodedMatches := make([]partialProfileMatchJSON, 0, len(matches))
	for _, match := range matches {
		encodedMatches = append(encodedMatches, partialProfileMatchJSON{
			ProfileName:     match.ProfileName,
			Protected:       match.Protected,
			MatchedTargets:  match.MatchedTargets,
			IncludedTargets: match.IncludedTargets,
			OmittedTargets:  match.OmittedTargets,
			TargetCount:     match.TargetCount,
		})
	}

	return encodedMatches
}

func closestProfileMatchJSONFromMatches(matches []app.ClosestProfileMatch) []closestProfileMatchJSON {
	encodedMatches := make([]closestProfileMatchJSON, 0, len(matches))
	for _, match := range matches {
		encodedMatches = append(encodedMatches, closestProfileMatchJSON{
			ProfileName:        match.ProfileName,
			Protected:          match.Protected,
			MatchedTargets:     match.MatchedTargets,
			IncludedTargets:    match.IncludedTargets,
			UnavailableTargets: match.UnavailableTargets,
			TargetCount:        match.TargetCount,
		})
	}

	return encodedMatches
}

func unavailableProfileJSONFromProfiles(profiles []app.UnavailableProfile) []unavailableProfileJSON {
	encodedProfiles := make([]unavailableProfileJSON, 0, len(profiles))
	for _, profile := range profiles {
		encodedProfiles = append(encodedProfiles, unavailableProfileJSON{
			ProfileName: profile.ProfileName,
			Protected:   profile.Protected,
			Reason:      profile.Reason,
			Values:      unavailableValueJSONFromValues(profile.Values),
		})
	}

	return encodedProfiles
}

func unavailableValueJSONFromValues(values []app.UnavailableValue) []unavailableValueJSON {
	encodedValues := make([]unavailableValueJSON, 0, len(values))
	for _, value := range values {
		encodedValues = append(encodedValues, unavailableValueJSON{
			TargetName:              value.TargetName,
			TargetFile:              value.TargetFile,
			TargetType:              string(value.TargetType),
			SelectorName:            value.SelectorName,
			Selector:                value.Selector,
			EnvironmentVariableName: value.EnvironmentVariableName,
			Reason:                  value.Reason,
		})
	}

	return encodedValues
}
