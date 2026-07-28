package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type applyCompletedMsg struct {
	requestID int
	result    app.Result
	err       error
}

type statusComparisonCompletedMsg struct {
	requestID int
	result    app.StatusComparison
}

type diffComparisonCompletedMsg struct {
	requestID int
	profile   string
	result    app.ProfileDiff
}

type comparisonFailedMsg struct {
	requestID int
	kind      comparisonRequestKind
	profile   string
	err       error
}

// Update handles user input and application results.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case applyCompletedMsg:
		return model.handleApplyCompleted(message)
	case statusComparisonCompletedMsg:
		return model.handleStatusComparisonCompleted(message)
	case diffComparisonCompletedMsg:
		return model.handleDiffComparisonCompleted(message)
	case comparisonFailedMsg:
		return model.handleComparisonFailed(message)
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
			if matchesKey(message, keyReveal) {
				return model, nil
			}
			return model, tea.Quit
		case statusLoadingState, statusReadyState:
			return model.handleStatusComparisonKey(message)
		case diffLoadingState, diffReadyState:
			return model.handleDiffComparisonKey(message)
		case comparisonErrorState:
			return model.handleComparisonErrorKey(message)
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
		return model, tea.Quit
	case matchesKey(message, keyEscape):
		switch model.state {
		case inspectState, confirmState, statusLoadingState, statusReadyState, diffLoadingState, diffReadyState, comparisonErrorState:
			model.state = listState
			model.clearComparisonState()
		}

		return model, nil
	default:
		return model, nil
	}
}

func (model Model) handleStatusComparisonCompleted(message statusComparisonCompletedMsg) (tea.Model, tea.Cmd) {
	if !model.isActiveComparisonRequest(comparisonRequestStatus, message.requestID) {
		return model, nil
	}

	result := message.result
	model.statusComparison = &result
	model.diffComparison = nil
	model.comparisonError = RecoverableError{}
	model.state = statusReadyState
	return model, nil
}

func (model Model) handleDiffComparisonCompleted(message diffComparisonCompletedMsg) (tea.Model, tea.Cmd) {
	if !model.isActiveComparisonRequest(comparisonRequestDiff, message.requestID) || message.profile != model.comparisonProfileName {
		return model, nil
	}

	result := message.result
	model.diffComparison = &result
	model.statusComparison = nil
	model.comparisonError = RecoverableError{}
	model.state = diffReadyState
	return model, nil
}

func (model Model) handleComparisonFailed(message comparisonFailedMsg) (tea.Model, tea.Cmd) {
	if !model.isActiveComparisonRequest(message.kind, message.requestID) {
		return model, nil
	}
	if message.kind == comparisonRequestDiff && message.profile != model.comparisonProfileName {
		return model, nil
	}

	model.statusComparison = nil
	model.diffComparison = nil
	model.comparisonError = model.comparisonFailureError(message.kind, message.profile, message.err)
	model.state = comparisonErrorState
	return model, nil
}

func (model Model) handleApplyCompleted(message applyCompletedMsg) (tea.Model, tea.Cmd) {
	if message.requestID != model.applyRequestID || !model.isApplying() {
		return model, nil
	}

	profileName := model.applyingProfile
	model.applyingProfile = ""
	if message.err != nil {
		model.state = errorState
		model.recoverableError = model.applyFailureError(profileName, message.err)
		model.successResult = nil
		return model, nil
	}

	model.state = successState
	model.recoverableError = RecoverableError{}
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
	case matchesKey(message, keyReveal):
		return model.toggleValueVisibility(), nil
	case matchesKey(message, keyInspect):
		model.refreshProfiles()

		if _, ok := model.selectedProfile(); !ok {
			return model, nil
		}

		model.state = inspectState
		return model, nil
	case matchesKey(message, keyStatus):
		return model.startStatusComparison()
	case matchesKey(message, keyDiff):
		model.refreshProfiles()

		selectedProfile, ok := model.selectedProfile()
		if !ok {
			return model, nil
		}

		return model.startDiffComparison(selectedProfile.Name)
	case matchesKey(message, keyEnter):
		model.refreshProfiles()
		return model.applyOrConfirmSelectedProfile()
	default:
		return model, nil
	}
}

func (model Model) handleStatusComparisonKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(message, keyQuit, keyEscape):
		return model.returnToListFromComparison(), nil
	case matchesKey(message, keyRefresh):
		return model.startStatusComparison()
	default:
		return model, nil
	}
}

func (model Model) handleDiffComparisonKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(message, keyQuit, keyEscape):
		return model.returnToListFromComparison(), nil
	case matchesKey(message, keyRefresh):
		profileName := model.comparisonProfileName
		if profileName == "" {
			selectedProfile, ok := model.selectedProfile()
			if !ok {
				return model, nil
			}
			profileName = selectedProfile.Name
		}

		return model.startDiffComparison(profileName)
	case matchesKey(message, keyReveal):
		return model.toggleValueVisibility(), nil
	default:
		return model, nil
	}
}

func (model Model) handleComparisonErrorKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(message, keyQuit, keyEscape):
		return model.returnToListFromComparison(), nil
	case matchesKey(message, keyRefresh):
		switch model.comparisonRequestKind {
		case comparisonRequestStatus:
			return model.startStatusComparison()
		case comparisonRequestDiff:
			profileName := model.comparisonProfileName
			if profileName == "" {
				selectedProfile, ok := model.selectedProfile()
				if !ok {
					return model, nil
				}
				profileName = selectedProfile.Name
			}

			return model.startDiffComparison(profileName)
		}
	}

	return model, nil
}

func (model Model) handleInspectKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(message, keyQuit, keyInspect, keyEscape):
		model.state = listState
		return model, nil
	case matchesKey(message, keyReveal):
		return model.toggleValueVisibility(), nil
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
			model.recoverableError = model.unavailableProfileError(selectedProfile)
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
	if matchesKey(message, keyReveal) {
		return model, nil
	}

	model.state = listState
	model.recoverableError = RecoverableError{}
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
		model.recoverableError = model.unavailableProfileError(selectedProfile)
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
	model.recoverableError = RecoverableError{}
	model.successResult = nil
	model.state = listState

	return model, applySelectedProfile(model.application, selectedProfile.Name, model.applyRequestID)
}

func (model Model) startStatusComparison() (tea.Model, tea.Cmd) {
	model.comparisonRequestID++
	model.comparisonRequestKind = comparisonRequestStatus
	model.comparisonProfileName = ""
	model.statusComparison = nil
	model.diffComparison = nil
	model.comparisonError = RecoverableError{}
	model.state = statusLoadingState

	return model, compareStatus(model.application, model.comparisonRequestID)
}

func (model Model) startDiffComparison(profileName string) (tea.Model, tea.Cmd) {
	model.comparisonRequestID++
	model.comparisonRequestKind = comparisonRequestDiff
	model.comparisonProfileName = profileName
	model.statusComparison = nil
	model.diffComparison = nil
	model.comparisonError = RecoverableError{}
	model.state = diffLoadingState

	return model, compareDiff(model.application, profileName, model.comparisonRequestID)
}

func (model Model) returnToListFromComparison() Model {
	model.state = listState
	model.clearComparisonState()
	model.refreshProfiles()
	return model
}

func (model Model) toggleValueVisibility() Model {
	model.valuesVisible = !model.valuesVisible
	return model
}

func (model *Model) clearComparisonState() {
	model.statusComparison = nil
	model.diffComparison = nil
	model.comparisonError = RecoverableError{}
	model.comparisonRequestKind = comparisonRequestNone
	model.comparisonProfileName = ""
}

func (model Model) isActiveComparisonRequest(kind comparisonRequestKind, requestID int) bool {
	if model.comparisonRequestKind != kind || model.comparisonRequestID != requestID {
		return false
	}

	switch kind {
	case comparisonRequestStatus:
		return model.state == statusLoadingState
	case comparisonRequestDiff:
		return model.state == diffLoadingState
	default:
		return false
	}
}

func (model Model) applyFailureError(profileName string, err error) RecoverableError {
	if targetFailure, ok := app.TargetFailureFromError(err); ok {
		return model.targetFailureError(profileName, targetFailure, err)
	}

	if errors.Is(err, app.ErrProfileUnavailable) {
		if profileItem, inspectErr := model.application.InspectProfileByName(profileName); inspectErr == nil {
			return model.unavailableProfileError(profileItem)
		}
	}

	return model.genericRecoverableError(err)
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

func compareStatus(application app.Application, requestID int) tea.Cmd {
	return func() tea.Msg {
		result, err := application.CompareStatus()
		if err != nil {
			return comparisonFailedMsg{requestID: requestID, kind: comparisonRequestStatus, err: err}
		}

		return statusComparisonCompletedMsg{requestID: requestID, result: result}
	}
}

func compareDiff(application app.Application, profileName string, requestID int) tea.Cmd {
	return func() tea.Msg {
		result, err := application.DiffProfileByName(profileName)
		if err != nil {
			return comparisonFailedMsg{requestID: requestID, kind: comparisonRequestDiff, profile: profileName, err: err}
		}

		return diffComparisonCompletedMsg{requestID: requestID, profile: profileName, result: result}
	}
}
