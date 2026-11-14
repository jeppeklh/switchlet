package undoreview

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

type statusCompletedMsg struct {
	requestID int
	scope     *requestScope
	status    app.UndoResult
	err       error
}

type reviewCompletedMsg struct {
	requestID int
	scope     *requestScope
	review    app.UndoResult
	err       error
}

type commitCompletedMsg struct {
	requestID      int
	scope          *requestScope
	result, status app.UndoResult
	err, statusErr error
}

// Update handles input and accepts results only for the active screen/request.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if model.finished != nil {
		return model, nil
	}
	switch message := message.(type) {
	case statusCompletedMsg:
		if model.state != loadingStatus || model.requestID != message.requestID || model.scope != message.scope {
			return model, nil
		}
		model.status, model.statusErr = &message.status, message.err
		model.state = overview
		if message.err != nil {
			model.state, model.err = failed, message.err
			model.status = nil
		}
	case reviewCompletedMsg:
		if model.state != loadingReview || model.requestID != message.requestID || model.scope != message.scope {
			return model, nil
		}
		if message.err != nil {
			model.state, model.err = failed, message.err
			// The old status is no longer evidence about the current record.
			model.status = nil
		} else {
			model.state, model.review = confirmation, &message.review
		}
	case commitCompletedMsg:
		if model.state != committing || model.requestID != message.requestID || model.scope != message.scope {
			return model, nil
		}
		model.review = nil
		model.status, model.statusErr = &message.status, message.statusErr
		model.state, model.result, model.err = completed, &message.result, nil
		if message.err != nil {
			model.state, model.result, model.err = failed, nil, message.err
		}
		model.scrollOffset = 0
	case tea.WindowSizeMsg:
		model.width, model.height = message.Width, message.Height
		model.scrollOffset = ui.PanelScrollMetricsForShell(model.shell(), 0).ClampOffset(model.scrollOffset)
	case tea.KeyMsg:
		return model.handleKey(message)
	}
	return model, nil
}

func (model Model) handleKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := message.String()
	if key == "q" || key == "ctrl+c" {
		model.finished = &Result{Quit: true}
		return model, tea.Quit
	}
	// Once confirmed, cancel cannot imply that an in-flight write was stopped.
	if model.state == committing {
		return model, nil
	}
	if key == "esc" {
		if model.state == confirmation || model.state == loadingReview {
			return model.refresh()
		}
		model.finished = &Result{}
		return model, nil
	}
	if model.tooSmall() {
		return model, nil
	}
	metrics := ui.PanelScrollMetricsForShell(model.shell(), 0)
	switch key {
	case "pgup":
		model.scrollOffset -= metrics.PageStep()
	case "pgdown":
		model.scrollOffset += metrics.PageStep()
	case "home":
		model.scrollOffset = 0
	case "end":
		model.scrollOffset = metrics.MaxOffset
	default:
		return model.handleAction(key)
	}
	model.scrollOffset = metrics.ClampOffset(model.scrollOffset)
	return model, nil
}

func (model Model) handleAction(key string) (tea.Model, tea.Cmd) {
	if key == "r" && (model.state == overview || model.state == failed || model.state == completed) {
		return model.refresh()
	}
	if model.state == completed && key == "enter" {
		model.finished = &Result{}
		return model, nil
	}
	if model.state == confirmation && (key == "enter" || key == "y") && model.review != nil {
		model.state = committing
		model.requestID++
		model.scrollOffset = 0
		return model, commitReview(model.workflow, model.scope, model.requestID, *model.review)
	}
	if model.state == confirmation && key == "n" {
		return model.refresh()
	}
	if model.state != overview || model.status == nil {
		return model, nil
	}
	action := ""
	switch key {
	case "enter":
		action = model.restoreAction()
	case "e":
		if !model.status.Enabled {
			action = "enable"
		}
	case "x":
		if model.restoreAction() != "" {
			action = "discard"
		}
	}
	if action == "" {
		return model, nil
	}
	model.state, model.review, model.err = loadingReview, nil, nil
	model.requestID++
	model.scrollOffset = 0
	return model, prepareReview(model.workflow, model.scope, model.requestID, action)
}

func (model Model) refresh() (tea.Model, tea.Cmd) {
	model.state = loadingStatus
	model.requestID++
	model.status, model.review, model.result, model.err, model.statusErr = nil, nil, nil, nil, nil
	model.scrollOffset = 0
	return model, readStatus(model.workflow, model.scope, model.requestID)
}

func readStatus(workflow app.UndoWorkflow, scope *requestScope, requestID int) tea.Cmd {
	return func() tea.Msg {
		result, err := workflow.Status()
		return statusCompletedMsg{requestID: requestID, scope: scope, status: result, err: err}
	}
}

func prepareReview(workflow app.UndoWorkflow, scope *requestScope, requestID int, action string) tea.Cmd {
	return func() tea.Msg {
		result, err := workflow.Review(action)
		return reviewCompletedMsg{requestID: requestID, scope: scope, review: result, err: err}
	}
}

func commitReview(workflow app.UndoWorkflow, scope *requestScope, requestID int, review app.UndoResult) tea.Cmd {
	return func() tea.Msg {
		result, err := workflow.Commit(review)
		status, statusErr := workflow.Status()
		return commitCompletedMsg{requestID: requestID, scope: scope, result: result, err: err, status: status, statusErr: statusErr}
	}
}
