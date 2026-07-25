package tui

import (
	"fmt"

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
		Subtitle: "Switch a named profile safely",
		Metadata: model.targetMetadata(),
		Panels: []Panel{
			{Title: "Profiles", Lines: profileLines, Focused: true},
			{Title: "Selected", Lines: model.selectionSummaryLines()},
		},
		Actions: model.listActions(),
		Width:   model.width,
	})
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

func (model Model) selectionSummaryLines() []string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return []string{"Status: No profile selected"}
	}

	lines := []string{
		selectedProfileTitle(selectedProfile),
		RenderKeyValue("Status", selectedStatusLabel(selectedProfile)),
		RenderKeyValue("Source", sourceLabel(selectedProfile.Source)),
		RenderKeyValue("Availability", availabilityLabel(selectedProfile)),
		RenderKeyValue("Protection", protectionLabel(selectedProfile)),
	}
	if model.application.TargetFile() != "" {
		lines = append(lines, RenderKeyValue("Target file", model.application.TargetFile()))
	}
	if model.application.TargetPath() != "" {
		lines = append(lines, RenderKeyValue("Target JSON path", model.application.TargetPath()))
	}
	lines = append(lines, RenderKeyValue("Action", actionDescription(selectedProfile)))

	return lines
}

func selectedProfileTitle(profile app.ProfileItem) string {
	badges := RenderBadges(profileBadges(profile))
	if badges == "" {
		return profile.Name
	}

	return profile.Name + " " + badges
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
	profileLines = append(profileLines, RenderKeyValue("Availability", availabilityLabel(selectedProfile)))
	if model.application.TargetFile() != "" {
		profileLines = append(profileLines, RenderKeyValue("Target file", model.application.TargetFile()))
	}
	if model.application.TargetPath() != "" {
		profileLines = append(profileLines, RenderKeyValue("Target JSON path", model.application.TargetPath()))
	}

	valueLines := []string{"Masked value:", maskedValueLabel(selectedProfile)}
	if selectedProfile.UnavailableReason != "" {
		valueLines = append(valueLines, "", "Resolution error:", selectedProfile.UnavailableReason)
	}
	profileLines = append(profileLines, "")
	profileLines = append(profileLines, valueLines...)

	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Inspect Profile",
		Metadata: model.targetMetadata(),
		Panels: []Panel{
			{Title: "Profiles", Lines: RenderListRows(model.profileRows(RowInactiveSelected))},
			{Title: "Profile detail", Lines: profileLines, Focused: true},
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
	lines = append(lines, "", "This will modify the configured target value.", "The resolved value is not shown here.", "Press Enter or y to confirm.")

	return RenderShell(Shell{
		Title:    "Apply protected profile?",
		Metadata: model.targetMetadata(),
		Panels: []Panel{
			{Title: "Profiles", Lines: RenderListRows(model.profileRows(RowInactiveSelected))},
			{Title: "Confirmation", Lines: lines, Focused: true},
		},
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
		Title:    "Switchlet",
		Subtitle: "Recoverable error",
		Metadata: model.targetMetadata(),
		Panels: []Panel{
			{Title: "Profiles", Lines: RenderListRows(model.profileRows(RowInactiveSelected))},
			{Title: "Error", Lines: []string{
				model.errorMessage,
				"",
				"Press any key to return",
			}, Focused: true},
		},
		Actions: []Action{{Key: "Any key", Label: "Return"}, {Key: "q", Label: "Quit"}},
		Width:   model.width,
	})
}

func (model Model) successView() string {
	if model.successResult == nil {
		return "Applied profile successfully.\n"
	}

	return RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Applied profile successfully.",
		Panels: []Panel{{Title: "Result", Lines: []string{
			RenderKeyValue("Applied profile", model.successResult.ProfileName),
			"Updated target:",
			model.successResult.TargetPath,
		}, Focused: true}},
		Width: model.width,
	})
}

// FinalMessage returns the concise summary shown after the full-screen UI exits.
func (model Model) FinalMessage() string {
	if model.state != successState || model.successResult == nil {
		return ""
	}

	return fmt.Sprintf("Applied profile: %s\n\nUpdated target:\n%s\n", model.successResult.ProfileName, model.successResult.TargetPath)
}

func (model Model) targetMetadata() []string {
	metadata := make([]string, 0, 2)
	if model.application.TargetFile() != "" {
		metadata = append(metadata, model.application.TargetFile())
	}
	if model.application.TargetPath() != "" {
		metadata = append(metadata, model.application.TargetPath())
	}

	return metadata
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
		return "Required"
	}

	return "Not required"
}

func selectedStatusLabel(profile app.ProfileItem) string {
	if !profile.Available {
		return profile.UnavailableReason
	}

	return fmt.Sprintf("Selected %q", profile.Name)
}

func availabilityLabel(profile app.ProfileItem) string {
	if !profile.Available {
		return "Unavailable"
	}

	return "Ready to apply"
}

func actionDescription(profile app.ProfileItem) string {
	switch {
	case !profile.Available:
		return "Enter shows the recoverable error."
	case profile.Protected:
		return "Enter opens confirmation."
	default:
		return "Enter applies this profile."
	}
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
