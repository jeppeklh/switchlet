package main

import (
	"fmt"
	"io"
	"path/filepath"
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

func writeInspectText(output io.Writer, profileItem app.ProfileItem, projectRoot string) error {
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
			if err := writeProfileValueText(output, valueItem, projectRoot); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeApplyText(output io.Writer, result app.Result, projectRoot string) error {
	if result.DryRun {
		if _, err := fmt.Fprintf(output, "Dry run successful for profile %q\n\n%s:\n", result.ProfileName, targetListHeading("Planned target", result.Changes)); err != nil {
			return err
		}
		if err := writePlannedChangesText(output, result.Changes, "would update", projectRoot); err != nil {
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
	if err := writePlannedChangesText(output, result.Changes, "updated", projectRoot); err != nil {
		return err
	}

	return nil
}

func writeStatusText(output io.Writer, status app.StatusComparison, projectRoot string, outputOptions commandOutputOptions) error {
	styles := defaultCommandOutputStyles(outputOptions)
	if _, err := fmt.Fprintln(output, styles.title.Render("Switchlet status")); err != nil {
		return err
	}

	switch status.Status {
	case app.StatusComparisonMatched:
		if err := writeCommandDetail(output, styles, "Current profile", styles.success.Render(status.CurrentProfile)); err != nil {
			return err
		}
		if err := writeTargetDescriptorSection(output, styles, "Matched targets", status.MatchedTargets, projectRoot); err != nil {
			return err
		}
	case app.StatusComparisonAmbiguous:
		if err := writeCommandDetail(output, styles, "State", styles.warning.Render("multiple complete profiles match")); err != nil {
			return err
		}
		if err := writeProfileMatchSection(output, styles, "Matches", status.Matches); err != nil {
			return err
		}
	default:
		if err := writeCommandDetail(output, styles, "State", styles.warning.Render("no complete profile match")); err != nil {
			return err
		}
		if err := writePartialMatchSection(output, styles, status.PartialMatches); err != nil {
			return err
		}
		if err := writeClosestProfileSection(output, styles, status.ClosestProfiles); err != nil {
			return err
		}
	}

	return writeUnavailableProfileSection(output, styles, status.UnavailableProfiles, projectRoot)
}

func writeDiffText(output io.Writer, diff app.ProfileDiff, projectRoot string, outputOptions commandOutputOptions) error {
	styles := defaultCommandOutputStyles(outputOptions)
	if _, err := fmt.Fprintf(output, "%s  %s\n", styles.title.Render("Switchlet diff"), styles.heading.Render(diff.ProfileName)); err != nil {
		return err
	}
	if err := writeCommandDetail(output, styles, "Protection", styledDiffProtectionLabel(styles, diff)); err != nil {
		return err
	}

	if err := writeTargetDescriptorSection(output, styles, "Would update", diff.WouldUpdate, projectRoot); err != nil {
		return err
	}
	if err := writeTargetDescriptorSection(output, styles, "Already matches", diff.AlreadyMatches, projectRoot); err != nil {
		return err
	}
	if err := writeUnavailableValueSection(output, styles, "Unavailable", diff.Unavailable, projectRoot); err != nil {
		return err
	}
	return writeTargetDescriptorSection(output, styles, "Omitted targets", diff.OmittedTargets, projectRoot)
}

func writeProfileValueText(output io.Writer, valueItem app.ProfileValueItem, projectRoot string) error {
	if _, err := fmt.Fprintf(output, "- %s%s\n", targetNameLabel(valueItem.TargetName), targetTypeBadge(string(valueItem.TargetType))); err != nil {
		return err
	}
	if valueItem.TargetFile != "" {
		if _, err := fmt.Fprintf(output, "  file: %s\n", displayProjectPath(projectRoot, valueItem.TargetFile)); err != nil {
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

func writePlannedChangesText(output io.Writer, changes []app.PlannedChange, marker string, projectRoot string) error {
	for index, change := range changes {
		if _, err := fmt.Fprintf(output, "%s %s\n", marker, displayProjectPath(projectRoot, change.TargetFile)); err != nil {
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

func writeCommandDetail(output io.Writer, styles commandOutputStyles, label string, value string) error {
	_, err := fmt.Fprintf(output, "%s  %s\n", styles.label.Render(commandDetailLabel(label)), value)
	return err
}

func writeTargetDetail(output io.Writer, styles commandOutputStyles, label string, value string) error {
	_, err := fmt.Fprintf(output, "  %s  %s\n", styles.label.Render(commandDetailLabel(label)), value)
	return err
}

func commandDetailLabel(label string) string {
	return fmt.Sprintf("%-16s", label)
}

func writeTargetDescriptorSection(output io.Writer, styles commandOutputStyles, heading string, descriptors []app.TargetDescriptor, projectRoot string) error {
	if len(descriptors) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s\n", styledSectionHeading(styles, heading)); err != nil {
		return err
	}
	for _, descriptor := range descriptors {
		if err := writeTargetDescriptorText(output, styles, descriptor, projectRoot); err != nil {
			return err
		}
	}

	return nil
}

func writeTargetDescriptorText(output io.Writer, styles commandOutputStyles, descriptor app.TargetDescriptor, projectRoot string) error {
	if _, err := fmt.Fprintf(output, "%s %s%s\n", styles.marker.Render(">"), styles.heading.Render(targetNameLabel(descriptor.TargetName)), styledTargetTypeBadge(styles, string(descriptor.TargetType))); err != nil {
		return err
	}
	if descriptor.TargetFile != "" {
		if err := writeTargetDetail(output, styles, "file", displayProjectPath(projectRoot, descriptor.TargetFile)); err != nil {
			return err
		}
	}
	if descriptor.Selector != "" {
		if err := writeTargetDetail(output, styles, selectorFieldName(descriptor.SelectorName), descriptor.Selector); err != nil {
			return err
		}
	}

	return nil
}

func writeProfileMatchSection(output io.Writer, styles commandOutputStyles, heading string, matches []app.ProfileMatch) error {
	if len(matches) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s\n", styledSectionHeading(styles, heading)); err != nil {
		return err
	}
	for _, match := range matches {
		if _, err := fmt.Fprintf(output, "%s %s\n", styles.marker.Render(">"), styledProfileLabel(styles, match.ProfileName, match.Protected)); err != nil {
			return err
		}
	}

	return nil
}

func writePartialMatchSection(output io.Writer, styles commandOutputStyles, matches []app.PartialProfileMatch) error {
	if len(matches) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s\n", styledSectionHeading(styles, "Partial matches")); err != nil {
		return err
	}
	for _, match := range matches {
		if _, err := fmt.Fprintf(
			output,
			"%s %s  %s\n",
			styles.marker.Render(">"),
			styledProfileLabel(styles, match.ProfileName, match.Protected),
			styles.muted.Render(fmt.Sprintf("%d/%d included match; %d omitted", match.MatchedTargets, match.IncludedTargets, match.OmittedTargets)),
		); err != nil {
			return err
		}
	}

	return nil
}

func writeClosestProfileSection(output io.Writer, styles commandOutputStyles, matches []app.ClosestProfileMatch) error {
	if len(matches) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s\n", styledSectionHeading(styles, "Closest profiles")); err != nil {
		return err
	}
	for _, match := range matches {
		line := fmt.Sprintf(
			"%d/%d targets match",
			match.MatchedTargets,
			match.TargetCount,
		)
		if match.UnavailableTargets > 0 {
			line += fmt.Sprintf("; %d unavailable", match.UnavailableTargets)
		}
		if _, err := fmt.Fprintf(output, "%s %s  %s\n", styles.marker.Render(">"), styledProfileLabel(styles, match.ProfileName, match.Protected), styles.muted.Render(line)); err != nil {
			return err
		}
	}

	return nil
}

func writeUnavailableProfileSection(output io.Writer, styles commandOutputStyles, profiles []app.UnavailableProfile, projectRoot string) error {
	if len(profiles) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s\n", styledSectionHeading(styles, "Unavailable profiles")); err != nil {
		return err
	}
	for _, profile := range profiles {
		if len(profile.Values) == 0 {
			if _, err := fmt.Fprintf(output, "%s %s\n", styles.marker.Render(">"), styledProfileLabel(styles, profile.ProfileName, profile.Protected)); err != nil {
				return err
			}
			if profile.Reason != "" {
				if err := writeCommandDetail(output, styles, "reason", profile.Reason); err != nil {
					return err
				}
			}
			continue
		}

		for _, value := range profile.Values {
			if _, err := fmt.Fprintf(output, "%s %s %s %s%s\n", styles.marker.Render(">"), styledProfileLabel(styles, profile.ProfileName, profile.Protected), styles.muted.Render("/"), styles.heading.Render(targetNameLabel(value.TargetName)), styledTargetTypeBadge(styles, string(value.TargetType))); err != nil {
				return err
			}
			if err := writeUnavailableValueDetails(output, styles, value, projectRoot); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeUnavailableValueSection(output io.Writer, styles commandOutputStyles, heading string, values []app.UnavailableValue, projectRoot string) error {
	if len(values) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(output, "\n%s\n", styledSectionHeading(styles, heading)); err != nil {
		return err
	}
	for _, value := range values {
		if _, err := fmt.Fprintf(output, "%s %s%s\n", styles.marker.Render(">"), styles.heading.Render(targetNameLabel(value.TargetName)), styledTargetTypeBadge(styles, string(value.TargetType))); err != nil {
			return err
		}
		if err := writeUnavailableValueDetails(output, styles, value, projectRoot); err != nil {
			return err
		}
	}

	return nil
}

func writeUnavailableValueDetails(output io.Writer, styles commandOutputStyles, value app.UnavailableValue, projectRoot string) error {
	if value.TargetFile != "" {
		if err := writeTargetDetail(output, styles, "file", displayProjectPath(projectRoot, value.TargetFile)); err != nil {
			return err
		}
	}
	if value.Selector != "" {
		if err := writeTargetDetail(output, styles, selectorFieldName(value.SelectorName), value.Selector); err != nil {
			return err
		}
	}
	if value.EnvironmentVariableName != "" {
		if err := writeTargetDetail(output, styles, "environment", value.EnvironmentVariableName); err != nil {
			return err
		}
	}
	if value.Reason != "" {
		if err := writeTargetDetail(output, styles, "reason", value.Reason); err != nil {
			return err
		}
	}

	return nil
}

func styledSectionHeading(styles commandOutputStyles, heading string) string {
	if !styles.styled {
		return styles.heading.Render(heading)
	}

	switch heading {
	case "Matched targets", "Matches", "Already matches":
		return styles.success.Bold(true).Render(heading)
	case "Would update", "Partial matches", "Closest profiles", "Omitted targets":
		return styles.warning.Bold(true).Render(heading)
	case "Unavailable", "Unavailable profiles":
		return styles.error.Bold(true).Render(heading)
	default:
		return styles.heading.Render(heading)
	}
}

func styledTargetTypeBadge(styles commandOutputStyles, targetType string) string {
	if targetType == "" {
		return ""
	}

	return " " + styles.badge.Render("["+targetType+"]")
}

func styledProfileLabel(styles commandOutputStyles, profileName string, protected bool) string {
	label := styles.heading.Render(profileName)
	if protected {
		label += " " + styles.badge.Render("[protected]")
	}

	return label
}

func styledDiffProtectionLabel(styles commandOutputStyles, diff app.ProfileDiff) string {
	if diff.Protected {
		return styles.warning.Render("protected")
	}

	return styles.muted.Render("not protected")
}

func targetListHeading(singular string, changes []app.PlannedChange) string {
	if len(changes) == 1 {
		return singular
	}

	return singular + "s"
}

func displayProjectPath(projectRoot string, path string) string {
	if projectRoot == "" || path == "" {
		return path
	}

	relativePath, err := filepath.Rel(projectRoot, path)
	if err != nil || relativePath == "." || isParentRelativePath(relativePath) || filepath.IsAbs(relativePath) {
		return path
	}

	return filepath.ToSlash(relativePath)
}

func isParentRelativePath(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator))
}
