package tui

import (
	"fmt"
	"strings"
)

const (
	defaultShellWidth = 80
	splitShellWidth   = 100
	panelGapWidth     = 3
	textEllipsis      = "..."
)

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
	Title   string
	Lines   []string
	Focused bool
}

// Shell describes the common application surface used by Switchlet screens.
type Shell struct {
	Title    string
	Subtitle string
	Metadata []string
	Panels   []Panel
	Actions  []Action
	Width    int
}

// RenderShell renders a compact application shell with titled content regions.
func RenderShell(shell Shell) string {
	width := normalizedWidth(shell.Width)

	var builder strings.Builder
	writeShellHeader(&builder, shell, width)

	if len(shell.Panels) > 0 {
		builder.WriteString(Separator(width))
		builder.WriteString("\n")
		writeShellPanels(&builder, shell.Panels, width)
	}

	if len(shell.Actions) > 0 {
		builder.WriteString(Separator(width))
		builder.WriteString("\n")
		builder.WriteString(fitLine(RenderCommandBar(shell.Actions), width))
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

// RenderInputWithinWidth renders a text input while keeping the cursor visible.
func RenderInputWithinWidth(label string, value string, cursor int, width int) string {
	line := RenderInput(label, value, cursor)
	width = normalizedWidth(width)
	if len([]rune(line)) <= width {
		return line
	}

	prefix := label + ": "
	ellipsisWidth := len([]rune(textEllipsis))
	availableInputWidth := width - len([]rune(prefix))
	if availableInputWidth <= ellipsisWidth {
		return fitLine(line, width)
	}

	valueRunes := []rune(value)
	cursor = clampRuneIndex(cursor, len(valueRunes))
	markedRunes := make([]rune, 0, len(valueRunes)+1)
	markedRunes = append(markedRunes, valueRunes[:cursor]...)
	markedRunes = append(markedRunes, '_')
	markedRunes = append(markedRunes, valueRunes[cursor:]...)

	cursorMarkerIndex := cursor
	if cursorMarkerIndex < availableInputWidth-ellipsisWidth {
		return prefix + string(markedRunes[:availableInputWidth-ellipsisWidth]) + textEllipsis
	}

	windowWidth := availableInputWidth - ellipsisWidth
	start := cursorMarkerIndex - windowWidth + 1
	if start < 0 {
		start = 0
	}
	end := start + windowWidth
	if end > len(markedRunes) {
		end = len(markedRunes)
		start = end - windowWidth
		if start < 0 {
			start = 0
		}
	}

	return prefix + textEllipsis + string(markedRunes[start:end])
}

// PrimaryPanelWidth returns the line width for the dominant panel in RenderShell.
func PrimaryPanelWidth(shellWidth int, panelCount int) int {
	width := normalizedWidth(shellWidth)
	if panelCount == 2 && width >= splitShellWidth {
		return width * 55 / 100
	}

	return width
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
	return strings.Repeat("-", normalizedWidth(width))
}

func writeShellHeader(builder *strings.Builder, shell Shell, width int) {
	leftLines := []string{shell.Title}
	if shell.Subtitle != "" {
		leftLines = append(leftLines, shell.Subtitle)
	}

	lineCount := len(leftLines)
	if len(shell.Metadata) > lineCount {
		lineCount = len(shell.Metadata)
	}

	for index := 0; index < lineCount; index++ {
		left := ""
		if index < len(leftLines) {
			left = leftLines[index]
		}
		right := ""
		if index < len(shell.Metadata) {
			right = shell.Metadata[index]
		}

		builder.WriteString(joinHeaderLine(left, right, width))
		builder.WriteString("\n")
	}
}

func writeShellPanels(builder *strings.Builder, panels []Panel, width int) {
	if shouldUseSplitLayout(panels, width) {
		writeSplitPanels(builder, panels[0], panels[1], width)
		return
	}

	writeStackedPanels(builder, panels, width)
}

func shouldUseSplitLayout(panels []Panel, width int) bool {
	return len(panels) == 2 && width >= splitShellWidth
}

func writeStackedPanels(builder *strings.Builder, panels []Panel, width int) {
	for index, panel := range panels {
		for _, line := range renderPanel(panel, width) {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
		if index != len(panels)-1 {
			builder.WriteString("\n")
		}
	}
}

func writeSplitPanels(builder *strings.Builder, leftPanel Panel, rightPanel Panel, width int) {
	gap := strings.Repeat(" ", panelGapWidth)
	leftWidth := width * 55 / 100
	rightWidth := width - leftWidth - panelGapWidth
	leftLines := renderPanel(leftPanel, leftWidth)
	rightLines := renderPanel(rightPanel, rightWidth)
	lineCount := len(leftLines)
	if len(rightLines) > lineCount {
		lineCount = len(rightLines)
	}

	for index := 0; index < lineCount; index++ {
		leftLine := ""
		if index < len(leftLines) {
			leftLine = leftLines[index]
		}
		rightLine := ""
		if index < len(rightLines) {
			rightLine = rightLines[index]
		}

		builder.WriteString(padLine(leftLine, leftWidth))
		builder.WriteString(gap)
		builder.WriteString(fitLine(rightLine, rightWidth))
		builder.WriteString("\n")
	}
}

func renderPanel(panel Panel, width int) []string {
	lines := make([]string, 0, len(panel.Lines)+2)
	if panel.Title != "" {
		title := panel.Title
		if panel.Focused {
			title = "* " + title
		}
		lines = append(lines, fitLine(title, width))
		lines = append(lines, strings.Repeat("-", limitedRuneCount(title, width)))
	}

	for _, line := range panel.Lines {
		lines = append(lines, fitLine(line, width))
	}

	return lines
}

func joinHeaderLine(left string, right string, width int) string {
	if right == "" {
		return fitLine(left, width)
	}

	rightRunes := []rune(right)
	if len(rightRunes) > width/2 {
		right = fitLine(right, width/2)
		rightRunes = []rune(right)
	}

	availableLeftWidth := width - len(rightRunes) - 1
	if availableLeftWidth <= 0 {
		return fitLine(right, width)
	}

	left = fitLine(left, availableLeftWidth)
	spaceCount := width - len([]rune(left)) - len(rightRunes)
	if spaceCount < 1 {
		spaceCount = 1
	}

	return left + strings.Repeat(" ", spaceCount) + right
}

func padLine(line string, width int) string {
	line = fitLine(line, width)
	padding := width - len([]rune(line))
	if padding <= 0 {
		return line
	}

	return line + strings.Repeat(" ", padding)
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(line)
	if len(runes) <= width {
		return line
	}
	if width <= len([]rune(textEllipsis)) {
		return string(runes[:width])
	}

	return string(runes[:width-len([]rune(textEllipsis))]) + textEllipsis
}

func limitedRuneCount(value string, limit int) int {
	runeCount := len([]rune(value))
	if runeCount > limit {
		return limit
	}

	return runeCount
}

func normalizedWidth(width int) int {
	if width <= 0 {
		return defaultShellWidth
	}

	return width
}

func clampRuneIndex(index int, runeCount int) int {
	if index < 0 {
		return 0
	}
	if index > runeCount {
		return runeCount
	}

	return index
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
