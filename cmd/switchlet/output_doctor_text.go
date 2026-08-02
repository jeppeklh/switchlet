package main

import (
	"fmt"
	"io"

	"github.com/jeppeklh/switchlet/internal/app"
)

func writeDoctorText(output io.Writer, report doctorReport, outputOptions commandOutputOptions) error {
	styles := defaultCommandOutputStyles(outputOptions)
	if _, err := fmt.Fprintln(output, styles.title.Render("Switchlet doctor")); err != nil {
		return err
	}
	if err := writeCommandDetail(output, styles, "health", styledDoctorStatus(styles, report)); err != nil {
		return err
	}
	if report.ConfigPath != "" {
		if err := writeCommandDetail(output, styles, "config", displayProjectPath(report.ProjectRoot, report.ConfigPath)); err != nil {
			return err
		}
	}

	for _, check := range report.Checks {
		if err := writeDoctorCheckText(output, styles, check, report.ProjectRoot); err != nil {
			return err
		}
	}

	return nil
}

func writeDoctorCheckText(output io.Writer, styles commandOutputStyles, check app.HealthCheck, projectRoot string) error {
	if _, err := fmt.Fprintf(output, "\n%s %s\n", styledDoctorCheckStatus(styles, check.Status), styles.heading.Render(check.Name)); err != nil {
		return err
	}
	if check.Message != "" {
		if err := writeCommandDetail(output, styles, "message", check.Message); err != nil {
			return err
		}
	}
	if check.HasTargetFailure {
		if err := writeDoctorTargetFailureText(output, styles, check.TargetFailure, projectRoot); err != nil {
			return err
		}
	}
	if len(check.Targets) > 0 {
		if err := writeTargetDescriptorSection(output, styles, "Targets", check.Targets, projectRoot); err != nil {
			return err
		}
	}
	if len(check.UnavailableProfiles) > 0 {
		return writeUnavailableProfileSection(output, styles, check.UnavailableProfiles, projectRoot)
	}

	return nil
}

func writeDoctorTargetFailureText(output io.Writer, styles commandOutputStyles, failure app.TargetFailure, projectRoot string) error {
	if failure.TargetName != "" {
		if err := writeCommandDetail(output, styles, "target", targetNameLabel(failure.TargetName)+targetTypeBadge(string(failure.TargetType))); err != nil {
			return err
		}
	}
	if failure.TargetFile != "" {
		if err := writeCommandDetail(output, styles, "file", displayProjectPath(projectRoot, failure.TargetFile)); err != nil {
			return err
		}
	}
	if failure.Selector != "" {
		if err := writeCommandDetail(output, styles, selectorFieldName(failure.SelectorName), failure.Selector); err != nil {
			return err
		}
	}
	if failure.Reason != "" {
		if err := writeCommandDetail(output, styles, "reason", failure.Reason); err != nil {
			return err
		}
	}

	return nil
}

func styledDoctorStatus(styles commandOutputStyles, report doctorReport) string {
	status := doctorReportStatus(report)
	switch status {
	case "failed":
		return styles.error.Render(status)
	case "warning":
		return styles.warning.Render(status)
	default:
		return styles.success.Render(status)
	}
}

func styledDoctorCheckStatus(styles commandOutputStyles, status app.HealthCheckStatus) string {
	label := "[" + string(status) + "]"
	switch status {
	case app.HealthCheckOK:
		return styles.success.Render(label)
	case app.HealthCheckWarning:
		return styles.warning.Render(label)
	case app.HealthCheckFailed:
		return styles.error.Render(label)
	default:
		return styles.muted.Render(label)
	}
}
