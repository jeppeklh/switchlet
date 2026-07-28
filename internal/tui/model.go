package tui

import (
	"github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type viewState int

const (
	listState viewState = iota
	inspectState
	confirmState
	errorState
	successState
	statusLoadingState
	statusReadyState
	diffLoadingState
	diffReadyState
	comparisonErrorState
)

type comparisonRequestKind int

const (
	comparisonRequestNone comparisonRequestKind = iota
	comparisonRequestStatus
	comparisonRequestDiff
)

// Model is the Bubble Tea model for the Switchlet terminal interface.
type Model struct {
	application      app.Application
	profiles         []app.ProfileItem
	cursor           int
	width            int
	height           int
	state            viewState
	recoverableError RecoverableError
	successResult    *app.Result
	applyingProfile  string
	applyRequestID   int

	statusComparison      *app.StatusComparison
	diffComparison        *app.ProfileDiff
	comparisonError       RecoverableError
	comparisonRequestID   int
	comparisonRequestKind comparisonRequestKind
	comparisonProfileName string
}

// New creates the terminal model.
func New(application app.Application) Model {
	model := Model{application: application}
	model.refreshProfiles()

	return model
}

// Init starts the Bubble Tea model.
func (model Model) Init() tea.Cmd {
	return nil
}

func (model *Model) refreshProfiles() {
	model.profiles = model.application.Profiles()

	if len(model.profiles) == 0 {
		model.cursor = 0
		return
	}

	if model.cursor < 0 {
		model.cursor = 0
	}
	if model.cursor >= len(model.profiles) {
		model.cursor = len(model.profiles) - 1
	}
}

func (model Model) selectedProfile() (app.ProfileItem, bool) {
	if len(model.profiles) == 0 {
		return app.ProfileItem{}, false
	}

	return model.profiles[model.cursor], true
}
