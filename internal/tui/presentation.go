package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	Height   int
}

// RenderShell renders a compact application shell with titled content regions.
func RenderShell(shell Shell) string {
	width := normalizedWidth(shell.Width)
	styles := defaultStyles()

	var builder strings.Builder
	writeShellHeader(&builder, shell, width, styles)

	if len(shell.Panels) > 0 {
		builder.WriteString("\n")
		writeShellPanels(&builder, shell.Panels, width, styles)
	}

	if len(shell.Actions) == 0 {
		return builder.String()
	}

	actionBlock := renderShellActions(shell.Actions, width)
	if shell.Height > 0 {
		return joinShellContentAndActions(builder.String(), actionBlock, shell.Height)
	}

	return builder.String() + actionBlock
}

func renderShellActions(actions []Action, width int) string {
	return Separator(width) + "\n" + fitLine(RenderCommandBar(actions), width) + "\n"
}

func joinShellContentAndActions(content string, actionBlock string, height int) string {
	padding := height - renderedLineCount(content) - renderedLineCount(actionBlock)
	if padding <= 0 {
		return content + actionBlock
	}

	return content + strings.Repeat("\n", padding) + actionBlock
}

func renderedLineCount(text string) int {
	if text == "" {
		return 0
	}

	lineCount := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		lineCount++
	}

	return lineCount
}

// RenderHeader renders only the title block used by partial wizard screens.
func RenderHeader(title string, subtitle string) string {
	styles := defaultStyles()
	if subtitle == "" {
		return styles.title.Render(title) + "\n\n"
	}

	return styles.title.Render(title) + "\n" + styles.subtitle.Render(subtitle) + "\n\n"
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
	styles := defaultStyles()
	line := rowStyle(row.State, styles).Render(rowMarker(row.State) + row.Label)
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

	styles := defaultStyles()
	renderedBadges := make([]string, 0, len(badges))
	for _, badge := range badges {
		if badge.Label == "" {
			continue
		}
		renderedBadges = append(renderedBadges, styles.badge.Render("["+badge.Label+"]"))
	}

	return strings.Join(renderedBadges, " ")
}

// RenderCommandBar renders grouped keyboard actions.
func RenderCommandBar(actions []Action) string {
	styles := defaultStyles()
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
			parts = append(parts, styles.commandKey.Render(action.Key))
			continue
		}

		parts = append(parts, styles.commandKey.Render(action.Key)+" "+action.Label)
	}

	return styles.commandBar.Render(strings.Join(parts, "  "))
}

// RenderInput renders a text input with an explicit cursor marker.
func RenderInput(label string, value string, cursor int) string {
	return defaultStyles().input.Render(rawInputLine(label, value, cursor))
}

func rawInputLine(label string, value string, cursor int) string {
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
	line := rawInputLine(label, value, cursor)
	width = normalizedWidth(width)
	styles := defaultStyles()
	if lipgloss.Width(line) <= width {
		return styles.input.Render(line)
	}

	prefix := label + ": "
	ellipsisWidth := lipgloss.Width(textEllipsis)
	availableInputWidth := width - lipgloss.Width(prefix)
	if availableInputWidth <= ellipsisWidth {
		return styles.input.Render(fitLine(line, width))
	}

	valueRunes := []rune(value)
	cursor = clampRuneIndex(cursor, len(valueRunes))
	markedRunes := make([]rune, 0, len(valueRunes)+1)
	markedRunes = append(markedRunes, valueRunes[:cursor]...)
	markedRunes = append(markedRunes, '_')
	markedRunes = append(markedRunes, valueRunes[cursor:]...)

	cursorMarkerIndex := cursor
	if cursorMarkerIndex < availableInputWidth-ellipsisWidth {
		return styles.input.Render(prefix + string(markedRunes[:availableInputWidth-ellipsisWidth]) + textEllipsis)
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

	return styles.input.Render(prefix + textEllipsis + string(markedRunes[start:end]))
}

// PrimaryPanelWidth returns the line width for the dominant panel in RenderShell.
func PrimaryPanelWidth(shellWidth int, panelCount int) int {
	width := normalizedWidth(shellWidth)
	panelWidth := width
	if panelCount == 2 && width >= splitShellWidth {
		panelWidth = width * 55 / 100
	}

	return panelContentWidth(panelWidth, defaultStyles())
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
	border := lipgloss.NormalBorder()
	return defaultStyles().muted.Render(strings.Repeat(border.Top, normalizedWidth(width)))
}

func writeShellHeader(builder *strings.Builder, shell Shell, width int, styles styleSet) {
	leftLines := []string{styles.title.Render(shell.Title)}
	if shell.Subtitle != "" {
		leftLines = append(leftLines, styles.subtitle.Render(shell.Subtitle))
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
			right = styles.muted.Render(shell.Metadata[index])
		}

		builder.WriteString(joinHeaderLine(left, right, width))
		builder.WriteString("\n")
	}
}

func writeShellPanels(builder *strings.Builder, panels []Panel, width int, styles styleSet) {
	if shouldUseSplitLayout(panels, width) {
		writeSplitPanels(builder, panels[0], panels[1], width, styles)
		return
	}

	writeStackedPanels(builder, panels, width, styles)
}

func shouldUseSplitLayout(panels []Panel, width int) bool {
	return len(panels) == 2 && width >= splitShellWidth
}

func writeStackedPanels(builder *strings.Builder, panels []Panel, width int, styles styleSet) {
	for index, panel := range panels {
		for _, line := range renderPanel(panel, width, styles) {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
		if index != len(panels)-1 {
			builder.WriteString("\n")
		}
	}
}

func writeSplitPanels(builder *strings.Builder, leftPanel Panel, rightPanel Panel, width int, styles styleSet) {
	gap := strings.Repeat(" ", panelGapWidth)
	leftWidth := width * 55 / 100
	rightWidth := width - leftWidth - panelGapWidth
	leftLines := renderPanel(leftPanel, leftWidth, styles)
	rightLines := renderPanel(rightPanel, rightWidth, styles)
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

func renderPanel(panel Panel, width int, styles styleSet) []string {
	style := styles.panel
	titleStyle := styles.panelTitle
	title := panel.Title
	if panel.Focused {
		style = styles.focusedPanel
		titleStyle = styles.focusedTitle
		if title != "" {
			title = "* " + title
		}
	}

	contentWidth := panelContentWidth(width, styles)
	lines := make([]string, 0, len(panel.Lines)+2)
	if title != "" {
		lines = append(lines, titleStyle.Render(fitLine(title, contentWidth)))
	}

	for _, line := range panel.Lines {
		lines = append(lines, fitLine(line, contentWidth))
	}

	panelBlock := style.Width(contentWidth).Render(strings.Join(lines, "\n"))
	panelLines := strings.Split(panelBlock, "\n")
	for index, line := range panelLines {
		panelLines[index] = fitLine(line, width)
	}

	return panelLines
}

func joinHeaderLine(left string, right string, width int) string {
	if right == "" {
		return fitLine(left, width)
	}

	if lipgloss.Width(right) > width/2 {
		right = fitLine(right, width/2)
	}

	availableLeftWidth := width - lipgloss.Width(right) - 1
	if availableLeftWidth <= 0 {
		return fitLine(right, width)
	}

	left = fitLine(left, availableLeftWidth)
	spaceCount := width - lipgloss.Width(left) - lipgloss.Width(right)
	if spaceCount < 1 {
		spaceCount = 1
	}

	return left + strings.Repeat(" ", spaceCount) + right
}

func padLine(line string, width int) string {
	line = fitLine(line, width)
	padding := width - lipgloss.Width(line)
	if padding <= 0 {
		return line
	}

	return line + strings.Repeat(" ", padding)
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}

	if lipgloss.Width(line) <= width {
		return line
	}
	ellipsisWidth := lipgloss.Width(textEllipsis)
	if width <= ellipsisWidth {
		return lipgloss.NewStyle().MaxWidth(width).Render(line)
	}

	return lipgloss.NewStyle().MaxWidth(width-ellipsisWidth).Render(line) + textEllipsis
}

func wrapText(text string, width int) []string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	lines := make([]string, 0, lipgloss.Width(text)/width+1)
	currentLine := ""
	for _, word := range words {
		if currentLine == "" {
			if lipgloss.Width(word) > width {
				lines = append(lines, splitLongText(word, width)...)
				continue
			}

			currentLine = word
			continue
		}

		candidate := currentLine + " " + word
		if lipgloss.Width(candidate) <= width {
			currentLine = candidate
			continue
		}

		lines = append(lines, currentLine)
		currentLine = ""
		if lipgloss.Width(word) > width {
			lines = append(lines, splitLongText(word, width)...)
			continue
		}

		currentLine = word
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

func splitLongText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	lines := make([]string, 0)
	currentRunes := make([]rune, 0, width)
	for _, value := range text {
		candidate := string(append(currentRunes, value))
		if len(currentRunes) > 0 && lipgloss.Width(candidate) > width {
			lines = append(lines, string(currentRunes))
			currentRunes = currentRunes[:0]
		}

		currentRunes = append(currentRunes, value)
	}
	if len(currentRunes) > 0 {
		lines = append(lines, string(currentRunes))
	}

	return lines
}

func normalizedWidth(width int) int {
	if width <= 0 {
		return defaultShellWidth
	}

	return width
}

func panelContentWidth(panelWidth int, styles styleSet) int {
	contentWidth := panelWidth - styles.panel.GetHorizontalFrameSize()
	if contentWidth < 1 {
		return 1
	}

	return contentWidth
}

func rowStyle(state RowState, styles styleSet) lipgloss.Style {
	switch state {
	case RowSelected:
		return styles.selectedRow
	case RowInactiveSelected:
		return styles.inactiveRow
	case RowDisabled:
		return styles.disabledRow
	default:
		return lipgloss.NewStyle()
	}
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
