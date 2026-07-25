package tui

import (
	"fmt"
	"strings"
)

const defaultShellWidth = 40

// Badge describes compact metadata attached to a row or status line.
type Badge struct {
	Label string
}

// Action describes one command-bar action.
type Action struct {
	Key   string
	Label string
}

// RowState controls the non-color marker used for list rows.
type RowState int

const (
	RowNormal RowState = iota
	RowSelected
	RowInactiveSelected
	RowDisabled
)

// ListRow describes a selectable row rendered by the shared TUI list style.
type ListRow struct {
	Label  string
	State  RowState
	Badges []Badge
}

// Panel is one titled content region inside a terminal shell.
type Panel struct {
	Title string
	Lines []string
}

// Shell describes the common application surface used by Switchlet screens.
type Shell struct {
	Title    string
	Subtitle string
	Panels   []Panel
	Actions  []Action
	Width    int
}

// RenderShell renders a compact application shell with titled content regions.
func RenderShell(shell Shell) string {
	var builder strings.Builder

	builder.WriteString(shell.Title)
	builder.WriteString("\n")
	if shell.Subtitle != "" {
		builder.WriteString(shell.Subtitle)
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	for index, panel := range shell.Panels {
		writePanel(&builder, panel)
		if index != len(shell.Panels)-1 {
			builder.WriteString("\n")
		}
	}

	if len(shell.Actions) > 0 {
		builder.WriteString("\n")
		builder.WriteString(Separator(shell.Width))
		builder.WriteString("\n")
		builder.WriteString(RenderCommandBar(shell.Actions))
		builder.WriteString("\n")
	}

	return builder.String()
}

// RenderHeader renders only the title block used by partial wizard screens.
func RenderHeader(title string, subtitle string) string {
	if subtitle == "" {
		return title + "\n\n"
	}

	return title + "\n" + subtitle + "\n\n"
}

// RenderStepProgress renders compact progress for wizard-style flows.
func RenderStepProgress(current int, labels []string) string {
	parts := make([]string, 0, len(labels))
	for index, label := range labels {
		stepNumber := index + 1
		if stepNumber == current {
			parts = append(parts, fmt.Sprintf("[%d %s]", stepNumber, label))
			continue
		}

		parts = append(parts, fmt.Sprintf("%d %s", stepNumber, label))
	}

	return strings.Join(parts, "  ")
}

// RenderListRows renders rows with non-color markers for selection and state.
func RenderListRows(rows []ListRow) []string {
	renderedRows := make([]string, 0, len(rows))
	for _, row := range rows {
		renderedRows = append(renderedRows, RenderListRow(row))
	}

	return renderedRows
}

// RenderListRow renders one list row.
func RenderListRow(row ListRow) string {
	line := rowMarker(row.State) + row.Label
	if badgeText := RenderBadges(row.Badges); badgeText != "" {
		line += " " + badgeText
	}

	return line
}

// RenderBadges renders compact metadata badges.
func RenderBadges(badges []Badge) string {
	if len(badges) == 0 {
		return ""
	}

	renderedBadges := make([]string, 0, len(badges))
	for _, badge := range badges {
		if badge.Label == "" {
			continue
		}
		renderedBadges = append(renderedBadges, "["+badge.Label+"]")
	}

	return strings.Join(renderedBadges, " ")
}

// RenderCommandBar renders grouped keyboard actions.
func RenderCommandBar(actions []Action) string {
	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		if action.Key == "" && action.Label == "" {
			continue
		}
		if action.Key == "" {
			parts = append(parts, action.Label)
			continue
		}
		if action.Label == "" {
			parts = append(parts, action.Key)
			continue
		}

		parts = append(parts, action.Key+" "+action.Label)
	}

	return strings.Join(parts, "  ")
}

// RenderInput renders a text input with an explicit cursor marker.
func RenderInput(label string, value string, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	return label + ": " + string(runes[:cursor]) + "_" + string(runes[cursor:])
}

// RenderKeyValue renders one metadata line.
func RenderKeyValue(label string, value string) string {
	if value == "" {
		return label + ":"
	}

	return label + ": " + value
}

// Separator renders the shared command-bar separator.
func Separator(width int) string {
	if width <= 0 {
		width = defaultShellWidth
	}
	if width < defaultShellWidth {
		width = defaultShellWidth
	}

	return strings.Repeat("-", width)
}

func writePanel(builder *strings.Builder, panel Panel) {
	if panel.Title != "" {
		builder.WriteString(panel.Title)
		builder.WriteString("\n")
		builder.WriteString(strings.Repeat("-", len([]rune(panel.Title))))
		builder.WriteString("\n")
	}

	for _, line := range panel.Lines {
		builder.WriteString(line)
		builder.WriteString("\n")
	}
}

func rowMarker(state RowState) string {
	switch state {
	case RowSelected:
		return "> "
	case RowInactiveSelected:
		return "~ "
	case RowDisabled:
		return "x "
	default:
		return "  "
	}
}
