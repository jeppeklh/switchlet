package tui

import "github.com/jeppeklh/switchlet/internal/app"

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
			Badges: profileBadges(item),
		})
	}

	return rows
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

	maxVisibleRows := model.height - 12
	if maxVisibleRows < 3 {
		return 3
	}

	return maxVisibleRows
}

func selectedProfileTitle(profile app.ProfileItem) string {
	badges := RenderBadges(profileBadges(profile))
	if badges == "" {
		return profile.Name
	}

	return profile.Name + " " + badges
}

func profileBadges(profile app.ProfileItem) []Badge {
	badges := make([]Badge, 0, 5)
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
	switch profile.Source {
	case app.ProfileSourceEnvironment:
		badges = append(badges, Badge{Label: "env"})
	case app.ProfileSourceLiteral:
		badges = append(badges, Badge{Label: "literal"})
	case app.ProfileSourceMixed:
		badges = append(badges, Badge{Label: "mixed"})
	}

	return badges
}
