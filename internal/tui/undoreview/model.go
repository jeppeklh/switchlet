// Package undoreview presents guarded undo and interrupted-operation recovery.
package undoreview

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeppeklh/switchlet/internal/app"
)

// Non-zero size gives each screen activation a distinct callback identity.
type requestScope byte

type viewState int

const (
	loadingStatus viewState = iota
	overview
	loadingReview
	confirmation
	committing
	completed
	failed
)

// Result tells the session host whether to resume the picker or quit.
type Result struct{ Quit bool }

// Model owns only presentation state; the application owns all restoration work.
type Model struct {
	workflow      app.UndoWorkflow
	scope         *requestScope
	projectRoot   string
	state         viewState
	requestID     int
	status        *app.UndoResult
	review        *app.UndoResult
	result        *app.UndoResult
	err           error
	statusErr     error
	width, height int
	scrollOffset  int
	finished      *Result
}

// New constructs a review screen without reading or creating storage.
func New(workflow app.UndoWorkflow, projectRoot string) Model {
	return Model{workflow: workflow, projectRoot: projectRoot, state: loadingStatus, requestID: 1, scope: new(requestScope)}
}

// Init requests optional recording/recovery status asynchronously.
func (model Model) Init() tea.Cmd { return readStatus(model.workflow, model.scope, model.requestID) }

// Result reports whether this screen has returned or requested exit.
func (model Model) Result() (Result, bool) {
	if model.finished == nil {
		return Result{}, false
	}
	return *model.finished, true
}

func (model Model) tooSmall() bool {
	return model.width > 0 && model.height > 0 && (model.width < 60 || model.height < 16)
}

func (model Model) restoreAction() string {
	if model.status == nil || model.statusErr != nil {
		return ""
	}
	switch model.status.Status {
	case "ready":
		return "undo"
	case "pending", "undoing":
		return "recover"
	default:
		return ""
	}
}
