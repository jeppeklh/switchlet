package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type applyCompletedMsg struct {
	result app.Result
	err    error
}

// Update handles user input and application results.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case applyCompletedMsg:
		return model.handleApplyCompleted(message)
	case tea.KeyMsg:
		if matchesKey(message, keyQuit, keyCtrlC) {
			return model, tea.Quit
		}

		switch model.state {
		case errorState:
			model.state = listState
			model.errorMessage = ""
			model.refreshProfiles()
			return model, nil
		case successState:
			return model, tea.Quit
		default:
			return model.handleListKey(message)
		}
	default:
		return model, nil
	}
}

func (model Model) handleApplyCompleted(message applyCompletedMsg) (tea.Model, tea.Cmd) {
	if message.err != nil {
		model.state = errorState
		model.errorMessage = message.err.Error()
		model.successResult = nil
		return model, nil
	}

	model.state = successState
	model.errorMessage = ""
	model.successResult = &message.result
	return model, tea.Quit
}

func (model Model) handleListKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(model.profiles) == 0 {
		return model, nil
	}

	switch {
	case matchesKey(message, keyUp, keyMoveUp):
		model.cursor--
		if model.cursor < 0 {
			model.cursor = len(model.profiles) - 1
		}
		return model, nil
	case matchesKey(message, keyDown, keyMoveDn):
		model.cursor++
		if model.cursor >= len(model.profiles) {
			model.cursor = 0
		}
		return model, nil
	case matchesKey(message, keyEnter):
		model.refreshProfiles()

		selectedProfile, ok := model.selectedProfile()
		if !ok {
			return model, nil
		}

		if selectedProfile.Protected {
			model.state = errorState
			model.errorMessage = fmt.Sprintf("profile %q requires confirmation and cannot be applied yet", selectedProfile.Name)
			return model, nil
		}

		if !selectedProfile.Available {
			model.state = errorState
			model.errorMessage = selectedProfile.UnavailableReason
			return model, nil
		}

		return model, applySelectedProfile(model.application, selectedProfile.Name)
	default:
		return model, nil
	}
}

func applySelectedProfile(application app.Application, profileName string) tea.Cmd {
	return func() tea.Msg {
		result, err := application.ApplyProfileByName(profileName)
		return applyCompletedMsg{result: result, err: err}
	}
}
