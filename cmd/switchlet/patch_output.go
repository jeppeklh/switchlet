package main

import (
	"fmt"
	"io"
	"strconv"

	"github.com/jeppeklh/switchlet/internal/app"
)

func writeManagedPatchText(output io.Writer, preview app.ManagedPatchPreview, projectRoot string) error {
	if _, err := fmt.Fprintf(output, "# Switchlet managed patch: %s\n", preview.ProfileName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "# values: shown for changed managed targets"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "# protected: %t\n", preview.Protected); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "# complete: %t\n", preview.Complete); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "# targets: %d included, %d omitted, %d configured\n", preview.IncludedTargetCount, preview.OmittedTargetCount, preview.TargetCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "# read-only: true"); err != nil {
		return err
	}

	for _, fileGroup := range preview.Files {
		if err := writeManagedPatchFile(output, fileGroup, projectRoot); err != nil {
			return err
		}
	}

	return writeManagedPatchOmittedTargets(output, preview.OmittedTargets, projectRoot)
}

func writeManagedPatchFile(output io.Writer, fileGroup app.ManagedPatchFileGroup, projectRoot string) error {
	displayPath := displayProjectPath(projectRoot, fileGroup.TargetFile)
	if _, err := fmt.Fprintln(output); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "diff --switchlet %s\n", displayPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "--- %s\n", displayPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "+++ %s\n", displayPath); err != nil {
		return err
	}

	for index, hunk := range fileGroup.Hunks {
		if err := writeManagedPatchHunk(output, index+1, hunk); err != nil {
			return err
		}
	}

	return nil
}

func writeManagedPatchHunk(output io.Writer, hunkNumber int, hunk app.ManagedPatchHunk) error {
	if _, err := fmt.Fprintf(output, "@@ -%d +%d @@ %s%s", hunkNumber, hunkNumber, targetNameLabel(hunk.TargetName), targetTypeBadge(string(hunk.TargetType))); err != nil {
		return err
	}
	if hunk.Selector != "" {
		if _, err := fmt.Fprintf(output, " %s: %s", selectorFieldName(hunk.SelectorName), hunk.Selector); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(output); err != nil {
		return err
	}

	switch hunk.Status {
	case app.ManagedPatchStatusWouldUpdate:
		if _, err := fmt.Fprintf(output, "- %s\n", managedPatchValueLabel("current", hunk.CurrentValueVisible, hunk.CurrentValue)); err != nil {
			return err
		}
		_, err := fmt.Fprintf(output, "+ %s\n", managedPatchValueLabel("profile", hunk.ProfileValueVisible, hunk.ProfileValue))
		return err
	case app.ManagedPatchStatusAlreadyMatches:
		_, err := fmt.Fprintln(output, " already matches")
		return err
	case app.ManagedPatchStatusUnavailable:
		return writeManagedPatchUnavailableHunk(output, hunk)
	default:
		_, err := fmt.Fprintf(output, " status: %s\n", hunk.Status)
		return err
	}
}

func writeManagedPatchUnavailableHunk(output io.Writer, hunk app.ManagedPatchHunk) error {
	if _, err := fmt.Fprintln(output, " unavailable"); err != nil {
		return err
	}
	if hunk.EnvironmentVariableName != "" {
		if _, err := fmt.Fprintf(output, " environment: %s\n", hunk.EnvironmentVariableName); err != nil {
			return err
		}
	}
	if hunk.UnavailableReason != "" {
		if _, err := fmt.Fprintf(output, " reason: %s\n", hunk.UnavailableReason); err != nil {
			return err
		}
	}

	return nil
}

func writeManagedPatchOmittedTargets(output io.Writer, omittedTargets []app.TargetDescriptor, projectRoot string) error {
	if len(omittedTargets) == 0 {
		return nil
	}

	if _, err := fmt.Fprintln(output); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "# Omitted targets"); err != nil {
		return err
	}
	for _, target := range omittedTargets {
		if _, err := fmt.Fprintf(output, "# %s%s\n", targetNameLabel(target.TargetName), targetTypeBadge(string(target.TargetType))); err != nil {
			return err
		}
		if target.TargetFile != "" {
			if _, err := fmt.Fprintf(output, "# file: %s\n", displayProjectPath(projectRoot, target.TargetFile)); err != nil {
				return err
			}
		}
		if target.Selector != "" {
			if _, err := fmt.Fprintf(output, "# %s: %s\n", selectorFieldName(target.SelectorName), target.Selector); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(output, "# unchanged by selected profile"); err != nil {
			return err
		}
	}

	return nil
}

func managedPatchValueLabel(label string, visible bool, value string) string {
	if !visible {
		return label + ": hidden"
	}

	return label + ": " + strconv.Quote(value)
}
