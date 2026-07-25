package tui

import (
	"fmt"
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	minimumTerminalWidth  = 80
	minimumTerminalHeight = 24
)

// View renders the current terminal state.
func (model Model) View() string {
	if model.isTerminalTooSmall() {
		return model.tooSmallTerminalView()
	}

	switch model.state {
	case inspectState:
		return model.inspectionView()
	case confirmState:
		return model.confirmationView()
	case errorState:
		return model.errorView()
	case successState:
		return model.successView()
	default:
		return model.listView()
	}
}

func (model Model) listView() string {
	profileLines := []string{"No profiles available."}
	if len(model.profiles) > 0 {
		profileLines = RenderListRows(model.profileRows(RowSelected))
	}

	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Select a profile",
		Panels: []Panel{
			{Title: "Profiles", Lines: profileLines},
			{Title: "Selected", Lines: model.selectionSummaryLines()},
		},
		Actions: model.listActions(),
		Width:   model.width,
	})
}

func (model Model) statusLine() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return "Status: No profile selected"
	}
	if !selectedProfile.Available {
		return "Status: " + selectedProfile.UnavailableReason
	}

	return fmt.Sprintf("Status: Selected %q", selectedProfile.Name)
}

func (model Model) listActions() []Action {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return []Action{{Key: "q", Label: "Quit"}}
	}

	return []Action{
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Enter", Label: enterActionLabel(selectedProfile)},
		{Key: "i", Label: "Inspect"},
		{Key: "q", Label: "Quit"},
	}
}

func (model Model) profileRows(selectedState RowState) []ListRow {
	rows := make([]ListRow, 0, len(model.profiles))
	for index, item := range model.profiles {
		state := RowNormal
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

func (model Model) selectionSummaryLines() []string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return []string{"Status: No profile selected"}
	}

	lines := []string{
		RenderKeyValue("Profile", selectedProfile.Name),
		RenderKeyValue("Source", sourceLabel(selectedProfile.Source)),
		model.statusLine(),
	}
	if selectedProfile.Protected {
		lines = append(lines, RenderKeyValue("Protection", "Protected"))
	}
	if model.application.TargetFile() != "" {
		lines = append(lines, RenderKeyValue("Target file", model.application.TargetFile()))
	}
	if model.application.TargetPath() != "" {
		lines = append(lines, RenderKeyValue("Target JSON path", model.application.TargetPath()))
	}
	lines = append(lines, fmt.Sprintf("Enter %s.", strings.ToLower(enterActionLabel(selectedProfile))))

	return lines
}

func profileBadges(profile app.ProfileItem) []Badge {
	badges := make([]Badge, 0, 3)
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
	}

	return badges
}

func (model Model) isTerminalTooSmall() bool {
	if model.width == 0 || model.height == 0 {
		return false
	}

	return model.width < minimumTerminalWidth || model.height < minimumTerminalHeight
}

func (model Model) tooSmallTerminalView() string {
	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Terminal too small.",
		Panels: []Panel{{Title: "Resize required", Lines: []string{
			fmt.Sprintf("Minimum size: %dx%d", minimumTerminalWidth, minimumTerminalHeight),
			fmt.Sprintf("Current size: %dx%d", model.width, model.height),
			"Resize the terminal to continue.",
		}}},
		Actions: []Action{{Key: "q", Label: "Quit"}, {Key: "Ctrl+C", Label: "Exit immediately"}},
		Width:   model.width,
	})
}

func (model Model) inspectionView() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return model.listView()
	}

	profileLines := []string{
		RenderKeyValue("Profile", selectedProfile.Name),
		RenderKeyValue("Source", sourceLabel(selectedProfile.Source)),
	}
	if selectedProfile.EnvironmentVariableName != "" {
		profileLines = append(profileLines, RenderKeyValue("Environment variable", selectedProfile.EnvironmentVariableName))
	}
	profileLines = append(profileLines, RenderKeyValue("Protection", protectionLabel(selectedProfile)))

	valueLines := []string{"Masked value:", maskedValueLabel(selectedProfile)}
	if selectedProfile.UnavailableReason != "" {
		valueLines = append(valueLines, "", "Resolution error:", selectedProfile.UnavailableReason)
	}

	return RenderShell(Shell{
		Title:    "Inspect Profile",
		Subtitle: "Review the selected profile before applying it.",
		Panels: []Panel{
			{Title: "Profile", Lines: profileLines},
			{Title: "Value", Lines: valueLines},
		},
		Actions: []Action{{Key: "Enter", Label: enterActionLabel(selectedProfile)}, {Key: "i/Esc/q", Label: "Return"}},
		Width:   model.width,
	})
}

func (model Model) confirmationView() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return model.listView()
	}

	lines := []string{RenderKeyValue("Profile", selectedProfile.Name)}
	if model.application.TargetFile() != "" {
		lines = append(lines, RenderKeyValue("Target file", model.application.TargetFile()))
	}
	if model.application.TargetPath() != "" {
		lines = append(lines, RenderKeyValue("Target JSON path", model.application.TargetPath()))
	}
	lines = append(lines, "", "This will modify the configured target value.", "Press Enter or y to confirm.")

	return RenderShell(Shell{
		Title:   "Apply protected profile?",
		Panels:  []Panel{{Title: "Confirmation", Lines: lines}},
		Actions: []Action{{Key: "Enter/y", Label: "Confirm"}, {Key: "n/Esc/q", Label: "Cancel"}},
		Width:   model.width,
	})
}

func enterActionLabel(profile app.ProfileItem) string {
	switch {
	case !profile.Available:
		return "Show Error"
	case profile.Protected:
		return "Continue"
	default:
		return "Apply"
	}
}

func (model Model) errorView() string {
	return RenderShell(Shell{
		Title: "Error",
		Panels: []Panel{{Title: "Recoverable error", Lines: []string{
			model.errorMessage,
			"",
			"Press any key to return",
		}}},
		Actions: []Action{{Key: "q", Label: "Quit"}},
		Width:   model.width,
	})
}

func (model Model) successView() string {
	if model.successResult == nil {
		return "Applied profile successfully.\n"
	}

	return RenderShell(Shell{
		Title: "Applied profile successfully.",
		Panels: []Panel{{Title: "Result", Lines: []string{
			RenderKeyValue("Applied profile", model.successResult.ProfileName),
			"Updated target:",
			model.successResult.TargetPath,
		}}},
		Width: model.width,
	})
}

func sourceLabel(source app.ProfileSource) string {
	switch source {
	case app.ProfileSourceEnvironment:
		return "Environment variable"
	case app.ProfileSourceLiteral:
		return "Literal"
	default:
		return "Unknown"
	}
}

func protectionLabel(profile app.ProfileItem) string {
	if profile.Protected {
		return "Protected"
	}

	return "Not protected"
}

func maskedValueLabel(profile app.ProfileItem) string {
	if !profile.Available {
		return "Unavailable"
	}
	if profile.MaskedValue == "" {
		return "<empty>"
	}

	return profile.MaskedValue
}
