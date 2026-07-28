package tui

import (
	"fmt"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	shellActionLineCount      = 2
	shellHeaderLineCount      = 2
	shellHeaderPanelGap       = 1
	stackedPanelGapLineCount  = 1
	minimumVisibleProfileRows = 3
)

func (model Model) profilePanel(selectedState RowState, focused bool) Panel {
	return Panel{
		Title:   model.profilePanelTitle(),
		Lines:   RenderListRows(model.profileRows(selectedState)),
		Focused: focused,
	}
}

func (model Model) profileRows(selectedState RowState) []ListRow {
	start, end := model.visibleProfileRange()
	rows := make([]ListRow, 0, end-start)
	for index := start; index < end; index++ {
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
			Badges: profileListBadges(item, index == model.cursor),
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
	profileCount := len(model.profiles)
	if profileCount == 0 || start == 0 && end == profileCount {
		return ""
	}

	return fmt.Sprintf("Showing %d-%d of %d profiles", start+1, end, profileCount)
}

func (model Model) visibleProfileRange() (int, int) {
	profileCount := len(model.profiles)
	if profileCount == 0 {
		return 0, 0
	}

	maxVisibleRows := model.maxVisibleProfileRows()
	if maxVisibleRows >= profileCount {
		return 0, profileCount
	}

	start := model.cursor - maxVisibleRows/2
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
		return len(model.profiles)
	}

	panelHeightBudget := model.height - shellActionLineCount - shellHeaderLineCount - shellHeaderPanelGap
	if normalizedWidth(model.width) < splitShellWidth {
		panelHeightBudget = (panelHeightBudget - stackedPanelGapLineCount) / 2
	}

	maxVisibleRows := panelHeightBudget - minimumPanelHeight(Panel{Title: "Profiles"}, defaultStyles())
	if maxVisibleRows < minimumVisibleProfileRows {
		return minimumVisibleProfileRows
	}

	return maxVisibleRows
}

func (model Model) profilePageStep() int {
	step := model.maxVisibleProfileRows()
	if step < 1 {
		return 1
	}

	return step
}

func selectedProfileTitle(profile app.ProfileItem) string {
	badges := RenderBadges(profileBadges(profile))
	if badges == "" {
		return profile.Name
	}

	return profile.Name + " " + badges
}

func profileBadges(profile app.ProfileItem) []Badge {
	badges := make([]Badge, 0, 4)
	if shouldShowTargetCount(profile) {
		badges = append(badges, Badge{Label: targetCountLabel(profile.TargetCount)})
	}
	if profile.Partial {
		badges = append(badges, Badge{Label: "partial"})
	}
	if profile.Protected {
		badges = append(badges, Badge{Label: "protected"})
	}
	if !profile.Available {
		badges = append(badges, Badge{Label: "unavailable"})
	}

	return badges
}

func profileListBadges(profile app.ProfileItem, selected bool) []Badge {
	badges := make([]Badge, 0, 5)
	if selected {
		badges = append(badges, Badge{Label: "selected"})
	}

	return append(badges, profileBadges(profile)...)
}
