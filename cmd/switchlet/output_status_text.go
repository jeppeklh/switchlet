package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

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

func writeStatusExpectationText(output io.Writer, expectation statusExpectationResult, outputOptions commandOutputOptions) error {
	styles := defaultCommandOutputStyles(outputOptions)
	if _, err := fmt.Fprintf(output, "\n%s\n", styledSectionHeading(styles, "Expectation")); err != nil {
		return err
	}
	if err := writeCommandDetail(output, styles, "expected", expectation.ExpectedProfile); err != nil {
		return err
	}
	result := styles.success.Render("matched")
	if !expectation.Matched {
		result = styles.error.Render("not matched")
	}
	if err := writeCommandDetail(output, styles, "result", result); err != nil {
		return err
	}
	if expectation.Message != "" {
		if err := writeCommandDetail(output, styles, "reason", expectation.Message); err != nil {
			return err
		}
	}
	if len(expectation.ObservedProfiles) > 0 {
		return writeCommandDetail(output, styles, "observed", strings.Join(expectation.ObservedProfiles, ", "))
	}

	return nil
}

func writeStatusShortText(output io.Writer, status app.StatusComparison) error {
	switch status.Status {
	case app.StatusComparisonMatched:
		_, err := fmt.Fprintf(output, "Current profile: %s\n", status.CurrentProfile)
		return err
	case app.StatusComparisonAmbiguous:
		names := profileMatchNames(status.Matches)
		if len(names) == 0 {
			_, err := fmt.Fprintln(output, "Current profile: ambiguous")
			return err
		}

		_, err := fmt.Fprintf(output, "Current profile: ambiguous (%s)\n", strings.Join(names, ", "))
		return err
	default:
		_, err := fmt.Fprintln(output, "Current profile: none")
		return err
	}
}

func profileMatchNames(matches []app.ProfileMatch) []string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.ProfileName == "" {
			continue
		}

		names = append(names, match.ProfileName)
	}

	return names
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
