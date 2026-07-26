package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type applyCompletedMsg struct {
	requestID int
	result    app.Result
	err       error
}

// Update handles user input and application results.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case applyCompletedMsg:
		return model.handleApplyCompleted(message)
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case tea.KeyMsg:
		if matchesKey(message, keyCtrlC) {
			return model, tea.Quit
		}
		if model.isTerminalTooSmall() {
			return model.handleTooSmallTerminalKey(message)
		}
		if model.isApplying() {
			return model, nil
		}

		switch model.state {
		case listState:
			return model.handleListKey(message)
		case inspectState:
			return model.handleInspectKey(message)
		case confirmState:
			return model.handleConfirmKey(message)
		case errorState:
			return model.handleErrorKey(message)
		case successState:
			return model, tea.Quit
		default:
			return model, nil
		}
	default:
		return model, nil
	}
}

func (model Model) handleTooSmallTerminalKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(message, keyQuit):
		switch model.state {
		case inspectState, confirmState:
			model.state = listState
			return model, nil
		case errorState, listState, successState:
			return model, tea.Quit
		default:
			return model, nil
		}
	case matchesKey(message, keyEscape):
		switch model.state {
		case inspectState, confirmState:
			model.state = listState
		}

		return model, nil
	default:
		return model, nil
	}
}

func (model Model) handleApplyCompleted(message applyCompletedMsg) (tea.Model, tea.Cmd) {
	if message.requestID != model.applyRequestID || !model.isApplying() {
		return model, nil
	}

	model.applyingProfile = ""
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
	if matchesKey(message, keyQuit) {
		return model, tea.Quit
	}

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
	case matchesKey(message, keyPageUp):
		model.cursor -= model.profilePageStep()
		if model.cursor < 0 {
			model.cursor = 0
		}
		return model, nil
	case matchesKey(message, keyPageDn):
		model.cursor += model.profilePageStep()
		if model.cursor >= len(model.profiles) {
			model.cursor = len(model.profiles) - 1
		}
		return model, nil
	case matchesKey(message, keyHome):
		model.cursor = 0
		return model, nil
	case matchesKey(message, keyEnd):
		model.cursor = len(model.profiles) - 1
		return model, nil
	case matchesKey(message, keyInspect):
		model.refreshProfiles()

		if _, ok := model.selectedProfile(); !ok {
			return model, nil
		}

		model.state = inspectState
		return model, nil
	case matchesKey(message, keyEnter):
		model.refreshProfiles()
		return model.applyOrConfirmSelectedProfile()
	default:
		return model, nil
	}
}

func (model Model) handleInspectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(message, keyQuit, keyInspect, keyEscape):
		model.state = listState
		return model, nil
	case matchesKey(message, keyEnter):
		model.refreshProfiles()
		return model.applyOrConfirmSelectedProfile()
	default:
		return model, nil
	}
}

func (model Model) handleConfirmKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(message, keyQuit, keyCancel, keyEscape):
		model.state = listState
		return model, nil
	case matchesKey(message, keyConfirm, keyEnter):
		model.refreshProfiles()

		selectedProfile, ok := model.selectedProfile()
		if !ok {
			return model, nil
		}
		if !selectedProfile.Available {
			model.state = errorState
			model.errorMessage = selectedProfile.UnavailableReason
			return model, nil
		}

		return model.startApplyingSelectedProfile(selectedProfile)
	default:
		return model, nil
	}
}

func (model Model) handleErrorKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matchesKey(message, keyQuit) {
		return model, tea.Quit
	}

	model.state = listState
	model.errorMessage = ""
	model.refreshProfiles()
	return model, nil
}

func (model Model) applyOrConfirmSelectedProfile() (tea.Model, tea.Cmd) {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return model, nil
	}

	if !selectedProfile.Available {
		model.state = errorState
		model.errorMessage = selectedProfile.UnavailableReason
		return model, nil
	}
	if selectedProfile.Protected {
		model.state = confirmState
		return model, nil
	}

	return model.startApplyingSelectedProfile(selectedProfile)
}

func (model Model) startApplyingSelectedProfile(selectedProfile app.ProfileItem) (tea.Model, tea.Cmd) {
	model.applyingProfile = selectedProfile.Name
	model.applyRequestID++
	model.errorMessage = ""
	model.successResult = nil
	model.state = listState

	return model, applySelectedProfile(model.application, selectedProfile.Name, model.applyRequestID)
}

func (model Model) isApplying() bool {
	return model.applyingProfile != ""
}

func applySelectedProfile(application app.Application, profileName string, requestID int) tea.Cmd {
	return func() tea.Msg {
		result, err := application.ApplyProfileByName(profileName)
		return applyCompletedMsg{requestID: requestID, result: result, err: err}
	}
}
