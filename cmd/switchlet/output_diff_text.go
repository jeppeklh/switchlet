package main

import (
	"fmt"
	"io"

	"github.com/jeppeklh/switchlet/internal/app"
)

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

func styledDiffProtectionLabel(styles commandOutputStyles, diff app.ProfileDiff) string {
	if diff.Protected {
		return styles.warning.Render("protected")
	}

	return styles.muted.Render("not protected")
}
