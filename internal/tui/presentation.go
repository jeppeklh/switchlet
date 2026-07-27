package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	defaultShellWidth        = 80
	splitShellWidth          = 100
	panelGapWidth            = 3
	textEllipsis             = "..."
	unboundedPanelBodyHeight = -1
	unboundedShellHeight     = -1
)

// Badge describes compact metadata attached to a row or status line.
type Badge struct {
	Label string
}

// Action describes one command-bar action.
type Action struct {
	Key      string
	Label    string
	Priority ActionPriority
}

// ActionPriority controls which command-bar actions survive width pressure.
type ActionPriority int

const (
	// ActionPriorityDefault lets the renderer infer priority from key and label.
	ActionPriorityDefault ActionPriority = iota
	// ActionPrioritySecondary is for movement and auxiliary hints.
	ActionPrioritySecondary
	// ActionPriorityNormal is for useful but non-critical actions.
	ActionPriorityNormal
	// ActionPriorityPrimary is for forward progress and return actions.
	ActionPriorityPrimary
	// ActionPriorityCritical is for cancellation and exit actions.
	ActionPriorityCritical
)

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
	Width   int
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
	actionLines := shellActionLines(shell.Actions, width)
	contentHeight := unboundedShellHeight
	if shell.Height > 0 {
		contentHeight = shell.Height - len(actionLines)
		if contentHeight < 0 {
			contentHeight = 0
		}
	}

	contentLines := shellContentLines(shell, width, styles, contentHeight)

	if len(actionLines) == 0 {
		return renderLines(contentLines)
	}

	if shell.Height > 0 {
		return joinShellContentAndActions(contentLines, actionLines, shell.Height)
	}

	return renderLines(append(contentLines, actionLines...))
}

func renderShellActions(actions []Action, width int) string {
	return renderLines(shellActionLines(actions, width))
}

func shellActionLines(actions []Action, width int) []string {
	if len(actions) == 0 {
		return nil
	}

	return []string{Separator(width), fitLine(renderCommandBarWithinWidth(actions, width), width)}
}

func joinShellContentAndActions(contentLines []string, actionLines []string, height int) string {
	if height <= 0 {
		return renderLines(append(contentLines, actionLines...))
	}

	if len(actionLines) >= height {
		return renderLines(lastLines(actionLines, height))
	}

	contentBudget := height - len(actionLines)
	if len(contentLines) > contentBudget {
		contentLines = contentLines[:contentBudget]
	}

	lines := make([]string, 0, height)
	lines = append(lines, contentLines...)
	for len(lines)+len(actionLines) < height {
		lines = append(lines, "")
	}
	lines = append(lines, actionLines...)

	return renderLines(lines)
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

func renderCommandBarWithinWidth(actions []Action, width int) string {
	actions = commandBarActionsForWidth(actions, width)
	return RenderCommandBar(actions)
}

func commandBarActionsForWidth(actions []Action, width int) []Action {
	selectedActions := compactActions(actions)
	if width <= 0 || lipgloss.Width(RenderCommandBar(selectedActions)) <= width {
		return selectedActions
	}

	for len(selectedActions) > 1 && lipgloss.Width(RenderCommandBar(selectedActions)) > width {
		removeIndex := lowestPriorityActionIndex(selectedActions)
		selectedActions = append(selectedActions[:removeIndex], selectedActions[removeIndex+1:]...)
	}

	return selectedActions
}

func compactActions(actions []Action) []Action {
	selectedActions := make([]Action, 0, len(actions))
	for _, action := range actions {
		if action.Key == "" && action.Label == "" {
			continue
		}
		selectedActions = append(selectedActions, action)
	}

	return selectedActions
}

func lowestPriorityActionIndex(actions []Action) int {
	removeIndex := 0
	lowestPriority := resolvedActionPriority(actions[0])
	for index := 1; index < len(actions); index++ {
		priority := resolvedActionPriority(actions[index])
		if priority <= lowestPriority {
			lowestPriority = priority
			removeIndex = index
		}
	}

	return removeIndex
}

func resolvedActionPriority(action Action) ActionPriority {
	if action.Priority != ActionPriorityDefault {
		return action.Priority
	}

	actionText := strings.ToLower(action.Key + " " + action.Label)
	switch {
	case strings.Contains(actionText, "ctrl+c") || strings.Contains(actionText, "quit") || strings.Contains(actionText, "cancel"):
		return ActionPriorityCritical
	case strings.Contains(actionText, "apply") || strings.Contains(actionText, "confirm") || strings.Contains(actionText, "continue") || strings.Contains(actionText, "select") || strings.Contains(actionText, "save") || strings.Contains(actionText, "create") || strings.Contains(actionText, "return") || strings.Contains(actionText, "back") || strings.Contains(actionText, "source") || strings.Contains(actionText, "profiles") || strings.Contains(actionText, "managed values"):
		return ActionPriorityPrimary
	default:
		return ActionPriorityNormal
	}
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

	return panelTextWidth(panelWidth, defaultStyles().panel)
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

func shellContentLines(shell Shell, width int, styles styleSet, heightBudget int) []string {
	lines := shellHeaderLines(shell, width, styles)
	if heightBudget >= 0 && len(lines) >= heightBudget {
		return lines[:heightBudget]
	}

	if len(shell.Panels) == 0 {
		return lines
	}

	lines = append(lines, "")
	if heightBudget >= 0 && len(lines) >= heightBudget {
		return lines[:heightBudget]
	}

	panelHeightBudget := unboundedShellHeight
	if heightBudget >= 0 {
		panelHeightBudget = heightBudget - len(lines)
	}
	lines = append(lines, renderShellPanels(shell.Panels, width, styles, panelHeightBudget)...)
	if heightBudget >= 0 && len(lines) > heightBudget {
		return lines[:heightBudget]
	}

	return lines
}

func shellHeaderLines(shell Shell, width int, styles styleSet) []string {
	leftLines := []string{styles.title.Render(shell.Title)}
	if shell.Subtitle != "" {
		leftLines = append(leftLines, styles.subtitle.Render(shell.Subtitle))
	}

	lineCount := len(leftLines)
	if len(shell.Metadata) > lineCount {
		lineCount = len(shell.Metadata)
	}

	lines := make([]string, 0, lineCount)
	for index := 0; index < lineCount; index++ {
		left := ""
		if index < len(leftLines) {
			left = leftLines[index]
		}
		right := ""
		if index < len(shell.Metadata) {
			right = styles.muted.Render(shell.Metadata[index])
		}

		lines = append(lines, joinHeaderLine(left, right, width))
	}

	return lines
}

func renderShellPanels(panels []Panel, width int, styles styleSet, heightBudget int) []string {
	if heightBudget == 0 {
		return nil
	}
	if shouldUseSplitLayout(panels, width) {
		return renderSplitPanels(panels[0], panels[1], width, styles, heightBudget)
	}

	return renderStackedPanels(panels, width, styles, heightBudget)
}

func shouldUseSplitLayout(panels []Panel, width int) bool {
	return len(panels) == 2 && width >= splitShellWidth
}

func renderStackedPanels(panels []Panel, width int, styles styleSet, heightBudget int) []string {
	if heightBudget < 0 {
		return renderUnboundedStackedPanels(panels, width, styles)
	}

	separatorBudget := len(panels) - 1
	if separatorBudget > heightBudget {
		separatorBudget = heightBudget
	}
	panelBudgets := allocateStackedPanelHeights(panels, width, styles, heightBudget-separatorBudget)

	lines := make([]string, 0, heightBudget)
	for index, panel := range panels {
		if len(lines) >= heightBudget {
			break
		}

		panelLines := renderPanelWithinHeight(panel, width, styles, panelBudgets[index])
		lines = append(lines, panelLines...)
		if len(lines) > heightBudget {
			return lines[:heightBudget]
		}
		if index != len(panels)-1 && len(lines) < heightBudget {
			lines = append(lines, "")
		}
	}

	return lines
}

func renderUnboundedStackedPanels(panels []Panel, width int, styles styleSet) []string {
	lines := make([]string, 0)
	for index, panel := range panels {
		lines = append(lines, renderAlignedPanel(panel, width, styles, unboundedPanelBodyHeight)...)
		if index != len(panels)-1 {
			lines = append(lines, "")
		}
	}

	return lines
}

func renderSplitPanels(leftPanel Panel, rightPanel Panel, width int, styles styleSet, heightBudget int) []string {
	if heightBudget == 0 {
		return nil
	}

	gap := strings.Repeat(" ", panelGapWidth)
	leftWidth := width * 55 / 100
	rightWidth := width - leftWidth - panelGapWidth
	leftLines := renderPanelWithinHeight(leftPanel, leftWidth, styles, heightBudget)
	rightLines := renderPanelWithinHeight(rightPanel, rightWidth, styles, heightBudget)
	lineCount := len(leftLines)
	if len(rightLines) > lineCount {
		lineCount = len(rightLines)
	}
	if heightBudget >= 0 && lineCount > heightBudget {
		lineCount = heightBudget
	}

	lines := make([]string, 0, lineCount)
	for index := 0; index < lineCount; index++ {
		leftLine := ""
		if index < len(leftLines) {
			leftLine = leftLines[index]
		}
		rightLine := ""
		if index < len(rightLines) {
			rightLine = rightLines[index]
		}

		lines = append(lines, padLine(leftLine, leftWidth)+gap+fitLine(rightLine, rightWidth))
	}

	return lines
}

func renderPanelWithinHeight(panel Panel, width int, styles styleSet, heightBudget int) []string {
	if heightBudget < 0 {
		return renderAlignedPanel(panel, width, styles, unboundedPanelBodyHeight)
	}
	if heightBudget == 0 {
		return nil
	}

	bodyHeight := panelBodyHeightBudget(panel, styles, heightBudget)
	panelLines := renderAlignedPanel(panel, width, styles, bodyHeight)
	if len(panelLines) > heightBudget {
		return panelLines[:heightBudget]
	}

	return panelLines
}

func renderAlignedPanel(panel Panel, width int, styles styleSet, bodyHeight int) []string {
	panelWidth := panelRenderWidth(panel, width)
	panelLines := renderPanel(panel, panelWidth, styles, bodyHeight)
	if panelWidth >= width {
		return panelLines
	}

	leftPadding := strings.Repeat(" ", (width-panelWidth)/2)
	for index, line := range panelLines {
		panelLines[index] = fitLine(leftPadding+line, width)
	}

	return panelLines
}

func panelRenderWidth(panel Panel, shellWidth int) int {
	if panel.Width <= 0 || panel.Width >= shellWidth {
		return shellWidth
	}

	return panel.Width
}

func renderPanel(panel Panel, width int, styles styleSet, bodyHeight int) []string {
	style, titleStyle, title := panelStyles(panel, styles)
	blockWidth := panelBlockWidth(width, styles)
	textWidth := panelTextWidth(width, style)
	bodyLines := panel.Lines
	if bodyHeight >= 0 {
		bodyLines = clippedPanelBodyLines(bodyLines, bodyHeight)
	}

	lines := make([]string, 0, len(bodyLines)+2)
	if title != "" {
		lines = append(lines, titleStyle.Render(fitLine(title, textWidth)))
	}

	for _, line := range bodyLines {
		lines = append(lines, fitLine(line, textWidth))
	}

	panelBlock := style.Width(blockWidth).Render(strings.Join(lines, "\n"))
	panelLines := strings.Split(panelBlock, "\n")
	for index, line := range panelLines {
		panelLines[index] = fitLine(line, width)
	}

	return panelLines
}

func allocateStackedPanelHeights(panels []Panel, width int, styles styleSet, heightBudget int) []int {
	budgets := make([]int, len(panels))
	if heightBudget <= 0 || len(panels) == 0 {
		return budgets
	}

	naturalHeights := make([]int, len(panels))
	totalNaturalHeight := 0
	for index, panel := range panels {
		naturalHeights[index] = len(renderPanel(panel, width, styles, unboundedPanelBodyHeight))
		totalNaturalHeight += naturalHeights[index]
	}
	if totalNaturalHeight <= heightBudget {
		return naturalHeights
	}

	remainingHeight := heightBudget
	for index, panel := range panels {
		minimumHeight := minimumPanelHeight(panel, styles)
		if minimumHeight > naturalHeights[index] {
			minimumHeight = naturalHeights[index]
		}
		if minimumHeight > remainingHeight {
			minimumHeight = remainingHeight
		}

		budgets[index] = minimumHeight
		remainingHeight -= minimumHeight
		if remainingHeight == 0 {
			return budgets
		}
	}

	for remainingHeight > 0 {
		allocated := false
		for index := range budgets {
			if budgets[index] >= naturalHeights[index] {
				continue
			}

			budgets[index]++
			remainingHeight--
			allocated = true
			if remainingHeight == 0 {
				break
			}
		}
		if !allocated {
			break
		}
	}

	return budgets
}

func minimumPanelHeight(panel Panel, styles styleSet) int {
	style, _, title := panelStyles(panel, styles)
	minimumHeight := style.GetVerticalFrameSize()
	if title != "" {
		minimumHeight++
	}
	if minimumHeight < 1 {
		return 1
	}

	return minimumHeight
}

func panelBodyHeightBudget(panel Panel, styles styleSet, heightBudget int) int {
	style, _, title := panelStyles(panel, styles)
	bodyHeight := heightBudget - style.GetVerticalFrameSize()
	if title != "" {
		bodyHeight--
	}
	if bodyHeight < 0 {
		return 0
	}

	return bodyHeight
}

func panelStyles(panel Panel, styles styleSet) (lipgloss.Style, lipgloss.Style, string) {
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

	return style, titleStyle, title
}

func clippedPanelBodyLines(lines []string, bodyHeight int) []string {
	if bodyHeight < 0 || len(lines) <= bodyHeight {
		return lines
	}
	if bodyHeight == 0 {
		return nil
	}
	if bodyHeight == 1 {
		return []string{panelOverflowLine(0, len(lines))}
	}

	visibleLineCount := bodyHeight - 1
	start := 0
	if selectedIndex := selectedBodyLineIndex(lines); selectedIndex >= visibleLineCount {
		start = selectedIndex - visibleLineCount/2
		if start+visibleLineCount > len(lines) {
			start = len(lines) - visibleLineCount
		}
	}
	if start < 0 {
		start = 0
	}

	end := start + visibleLineCount
	visibleLines := append([]string(nil), lines[start:end]...)
	visibleLines = append(visibleLines, panelOverflowLine(start, len(lines)-end))

	return visibleLines
}

func selectedBodyLineIndex(lines []string) int {
	for index, line := range lines {
		if strings.Contains(line, "> ") {
			return index
		}
	}

	return -1
}

func panelOverflowLine(hiddenBefore int, hiddenAfter int) string {
	switch {
	case hiddenBefore > 0 && hiddenAfter > 0:
		return fmt.Sprintf("... %d earlier, %d more", hiddenBefore, hiddenAfter)
	case hiddenBefore > 0:
		return fmt.Sprintf("... %d earlier", hiddenBefore)
	default:
		return fmt.Sprintf("... %d more", hiddenAfter)
	}
}

func lastLines(lines []string, count int) []string {
	if count >= len(lines) {
		return lines
	}
	if count <= 0 {
		return nil
	}

	return lines[len(lines)-count:]
}

func renderLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n") + "\n"
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

	return ansi.Truncate(line, width, textEllipsis)
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

func panelBlockWidth(panelWidth int, styles styleSet) int {
	contentWidth := panelWidth - styles.panel.GetHorizontalFrameSize()
	if contentWidth < 1 {
		return 1
	}

	return contentWidth
}

func panelTextWidth(panelWidth int, style lipgloss.Style) int {
	textWidth := panelWidth - style.GetHorizontalFrameSize() - style.GetHorizontalPadding()
	if textWidth < 1 {
		return 1
	}

	return textWidth
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
