package tui

// RecoverableError contains user-facing error copy for interactive TUI states.
type RecoverableError struct {
	Problem  string
	Context  []string
	Reason   string
	Recovery string
	Cause    error
}

// IsZero reports whether the error has no user-facing content.
func (err RecoverableError) IsZero() bool {
	return err.Problem == "" && len(err.Context) == 0 && err.Reason == "" && err.Recovery == ""
}

// RecoverableErrorLines renders one structured recoverable error for a panel.
func RecoverableErrorLines(err RecoverableError, maxLineWidth int) []string {
	if err.IsZero() {
		err.Problem = "Action could not continue."
		err.Reason = "Unknown error."
		err.Recovery = "Return to the previous screen, then try again."
	}

	lines := wrapErrorText(err.Problem, maxLineWidth)
	if len(err.Context) > 0 {
		lines = append(lines, "", "Context:")
		lines = append(lines, err.Context...)
	}
	if err.Reason != "" {
		lines = append(lines, "", "Reason:")
		lines = append(lines, wrapErrorText(err.Reason, maxLineWidth)...)
	}
	if err.Recovery != "" {
		lines = append(lines, "", "Recovery:")
		lines = append(lines, wrapErrorText(err.Recovery, maxLineWidth)...)
	}

	return lines
}

func wrapErrorText(value string, maxLineWidth int) []string {
	if value == "" {
		return nil
	}

	return wrapText(value, maxLineWidth)
}
