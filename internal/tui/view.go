package tui

import (
	"fmt"
	"strings"
)

// View renders the current terminal state.
func (model Model) View() string {
	switch model.state {
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
	builder.WriteString("↑/↓ or j/k Move  Enter Apply  q Quit\n")

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
		return fmt.Sprintf("Status: %q is protected", selectedProfile.Name)
	}

	return fmt.Sprintf("Status: Ready to apply %q", selectedProfile.Name)
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
		"Applied profile: %s\n\nUpdated:\nConnectionStrings.%s\n",
		model.successResult.ProfileName,
		model.successResult.ConnectionName,
	)
}
