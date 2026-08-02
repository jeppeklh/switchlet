package tui

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	shellActionLineCount      = 2
	stackedPanelGapLineCount  = 1
	minimumVisibleProfileRows = 3
)

func (model Model) profilePanel(selectedState RowState, focused bool) Panel {
	return Panel{
		Title:      model.profilePanelTitle(),
		Lines:      model.profileListLines(selectedState),
		Focused:    focused,
		FillHeight: model.shouldFillWorkspacePanels(),
	}
}

func (model Model) profileListLines(selectedState RowState) []string {
	lines := make([]string, 0)
	filteredIndices := model.filteredProfileIndices()
	switch {
	case len(model.profiles) == 0:
		lines = append(lines, "No profiles available.")
	case len(filteredIndices) == 0:
		lines = append(lines, "No profiles match this filter.")
	default:
		lines = append(lines, RenderListRows(model.profileRows(selectedState))...)
	}

	return lines
}

func (model Model) profileRows(selectedState RowState) []ListRow {
	filteredIndices := model.filteredProfileIndices()
	start, end := model.visibleProfileRange()
	rows := make([]ListRow, 0, end-start)
	for position := start; position < end; position++ {
		index := filteredIndices[position]
		item := model.profiles[index]
		state := RowNormal
		if !item.Available {
			state = RowDisabled
		}
		if index == model.cursor {
			state = selectedState
		}

		rows = append(rows, ListRow{
			Label:  item.Name,
			State:  state,
			Badges: model.profileBadges(item),
		})
	}

	return rows
}

func (model Model) profilePanelTitle() string {
	positionContext := model.profileListPositionContext()
	if positionContext == "" {
		return "Profiles"
	}

	return "Profiles - " + positionContext
}

func (model Model) profileListPositionContext() string {
	start, end := model.visibleProfileRange()
	profileCount := len(model.filteredProfileIndices())
	if profileCount == 0 || start == 0 && end == profileCount {
		return ""
	}

	return fmt.Sprintf("Showing %d-%d of %d profiles", start+1, end, profileCount)
}

func (model Model) visibleProfileRange() (int, int) {
	filteredIndices := model.filteredProfileIndices()
	profileCount := len(filteredIndices)
	if profileCount == 0 {
		return 0, 0
	}

	maxVisibleRows := model.maxVisibleProfileRows()
	if maxVisibleRows >= profileCount {
		return 0, profileCount
	}

	cursorPosition := model.profileCursorPosition(filteredIndices)
	start := cursorPosition - maxVisibleRows/2
	if start < 0 {
		start = 0
	}
	if start+maxVisibleRows > profileCount {
		start = profileCount - maxVisibleRows
	}

	return start, start + maxVisibleRows
}

func (model Model) maxVisibleProfileRows() int {
	if model.height <= 0 {
		return len(model.filteredProfileIndices())
	}

	panelHeightBudget := model.height - shellActionLineCount
	if normalizedWidth(model.width) < splitShellWidth {
		panelHeightBudget = (panelHeightBudget - stackedPanelGapLineCount) / 2
	}

	maxVisibleRows := panelHeightBudget - minimumPanelHeight(Panel{Title: "Profiles"}, defaultStyles()) - model.profileListPrefixLineCount()
	if maxVisibleRows < minimumVisibleProfileRows {
		return minimumVisibleProfileRows
	}

	return maxVisibleRows
}

func (model Model) profileListPrefixLineCount() int {
	return 0
}

func (model Model) profileSearchCommandLine() string {
	if model.state != searchState {
		return ""
	}

	return CommandInputWithinWidth("/", model.searchInput, model.searchCursor, model.width)
}

func (model Model) profilePageStep() int {
	step := model.maxVisibleProfileRows()
	if step < 1 {
		return 1
	}

	return step
}

func selectedProfileTitle(profile app.ProfileItem) string {
	return profile.Name
}

func (model Model) profileBadges(profile app.ProfileItem) []Badge {
	badges := make([]Badge, 0, 1)
	if model.profileIsCurrent(profile.Name) {
		badges = append(badges, Badge{Label: "current"})
	}

	return badges
}

func (model Model) profileIsCurrent(profileName string) bool {
	if model.currentProfiles == nil {
		return false
	}

	_, current := model.currentProfiles[profileName]
	return current
}
