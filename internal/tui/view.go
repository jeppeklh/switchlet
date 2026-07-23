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
	var builder strings.Builder

	builder.WriteString("Switchlet\n\n")
	builder.WriteString("Select a profile\n\n")

	if len(model.profiles) == 0 {
		builder.WriteString("No profiles available.\n")
	} else {
		for index, item := range model.profiles {
			cursor := "  "
			if index == model.cursor {
				cursor = "> "
			}

			builder.WriteString(cursor)
			builder.WriteString(item.Name)

			indicators := make([]string, 0, 2)
			if item.Protected {
				indicators = append(indicators, "[protected]")
			}
			if !item.Available {
				indicators = append(indicators, "[unavailable]")
			}

			if len(indicators) > 0 {
				builder.WriteString(" ")
				builder.WriteString(strings.Join(indicators, " "))
			}

			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n")
	builder.WriteString(model.statusLine())
	builder.WriteString("\n\n")
	builder.WriteString("----------------------------------------\n")
	builder.WriteString("↑/↓ or j/k Move  Enter Apply  i Inspect  q Quit\n")

	return builder.String()
}

func (model Model) statusLine() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return "Status: No profile selected"
	}
	if !selectedProfile.Available {
		return "Status: " + selectedProfile.UnavailableReason
	}
	if selectedProfile.Protected {
		return fmt.Sprintf("Status: %q requires confirmation", selectedProfile.Name)
	}

	return fmt.Sprintf("Status: Ready to apply %q", selectedProfile.Name)
}

func (model Model) isTerminalTooSmall() bool {
	if model.width == 0 || model.height == 0 {
		return false
	}

	return model.width < minimumTerminalWidth || model.height < minimumTerminalHeight
}

func (model Model) tooSmallTerminalView() string {
	return fmt.Sprintf(
		"Switchlet\n\nTerminal too small.\nMinimum size: %dx%d\nCurrent size: %dx%d\n\nResize the terminal to continue.\nCtrl+C exits immediately.\n",
		minimumTerminalWidth,
		minimumTerminalHeight,
		model.width,
		model.height,
	)
}

func (model Model) inspectionView() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return model.listView()
	}

	var builder strings.Builder

	builder.WriteString("Inspect Profile\n\n")
	builder.WriteString("Profile: ")
	builder.WriteString(selectedProfile.Name)
	builder.WriteString("\n")
	builder.WriteString("Source: ")
	builder.WriteString(sourceLabel(selectedProfile.Source))
	builder.WriteString("\n")
	if selectedProfile.EnvironmentVariableName != "" {
		builder.WriteString("Environment variable: ")
		builder.WriteString(selectedProfile.EnvironmentVariableName)
		builder.WriteString("\n")
	}
	builder.WriteString("Protection: ")
	builder.WriteString(protectionLabel(selectedProfile))
	builder.WriteString("\n\n")
	builder.WriteString("Masked connection string:\n")
	builder.WriteString(maskedValueLabel(selectedProfile))
	builder.WriteString("\n")
	if selectedProfile.UnavailableReason != "" {
		builder.WriteString("\nResolution error:\n")
		builder.WriteString(selectedProfile.UnavailableReason)
		builder.WriteString("\n")
	}

	builder.WriteString("\n----------------------------------------\n")
	builder.WriteString("Enter Apply  i/Esc/q Return\n")

	return builder.String()
}

func (model Model) confirmationView() string {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return model.listView()
	}

	var builder strings.Builder

	builder.WriteString("Apply protected profile?\n\n")
	builder.WriteString("Profile: ")
	builder.WriteString(selectedProfile.Name)
	builder.WriteString("\n\n")
	builder.WriteString("This will modify the configured connection string.\n\n")
	builder.WriteString("----------------------------------------\n")
	builder.WriteString("y Confirm  n/Esc/q Cancel\n")

	return builder.String()
}

func (model Model) errorView() string {
	return fmt.Sprintf(
		"Error\n\n%s\n\nPress any key to return\nq Quit\n",
		model.errorMessage,
	)
}

func (model Model) successView() string {
	if model.successResult == nil {
		return "Applied profile successfully.\n"
	}

	return fmt.Sprintf(
		"Applied profile: %s\n\nUpdated:\n%s\n",
		model.successResult.ProfileName,
		model.successResult.TargetPath,
	)
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
