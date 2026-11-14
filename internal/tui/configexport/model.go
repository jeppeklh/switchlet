// Package configexport presents reviewed environment-only example export.
package configexport

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeppeklh/switchlet/internal/app"
)

type state int

const (
	editing state = iota
	nameInput
	preparing
	review
	writing
	completed
	failed
)

// Result returns control to the config editor or requests application exit.
type Result struct{ Quit bool }

// Model owns export interaction; app/config own validation and all file effects.
type Model struct {
	workflow       app.ConfigEditWorkflow
	document       app.ConfigEditDocument
	draft          app.ExampleExportDraft
	prepared       app.PreparedExampleExport
	state          state
	cursor         int
	input          []rune
	inputCursor    int
	width, height  int
	scroll         int
	help           bool
	sharedApproved bool
	request        *byte
	err            string
	result         *Result
}

func New(document app.ConfigEditDocument, workflow app.ConfigEditWorkflow) Model {
	draft, err := workflow.NewExampleExportDraft(document)
	model := Model{document: document, workflow: workflow, draft: draft}
	if err != nil {
		model.state, model.err = failed, err.Error()
	}
	return model
}
func (model Model) Init() tea.Cmd { return nil }
func (model Model) Result() (Result, bool) {
	if model.result == nil {
		return Result{}, false
	}
	return *model.result, true
}

type preparedMsg struct {
	request  *byte
	prepared app.PreparedExampleExport
	err      error
}
type writtenMsg struct {
	request *byte
	err     error
}

func (model *Model) prepare() tea.Cmd {
	model.request = new(byte)
	request, workflow, document := model.request, model.workflow, model.document
	draft := app.ExampleExportDraft{Entries: slices.Clone(model.draft.Entries)}
	model.state, model.scroll, model.err, model.sharedApproved = preparing, 0, "", false
	return func() tea.Msg {
		prepared, err := workflow.PrepareExampleExport(document, draft)
		return preparedMsg{request, prepared, err}
	}
}

func (model *Model) commit() tea.Cmd {
	model.request = new(byte)
	request, prepared := model.request, model.prepared
	var shared []string
	if model.sharedApproved {
		shared = slices.Clone(prepared.SharedVariables)
	}
	model.state, model.err = writing, ""
	return func() tea.Msg { return writtenMsg{request, prepared.Commit(shared)} }
}

func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if model.result != nil {
		return model, nil
	}
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width, model.height = message.Width, message.Height
		model.scroll = model.metrics().ClampOffset(model.scroll)
	case preparedMsg:
		if model.state != preparing || message.request != model.request {
			return model, nil
		}
		model.request = nil
		if message.err != nil {
			model.state, model.err = editing, message.err.Error()
		} else {
			model.state, model.prepared = review, message.prepared
		}
	case writtenMsg:
		if model.state != writing || message.request != model.request {
			return model, nil
		}
		model.request = nil
		if message.err != nil {
			model.state, model.err = failed, message.err.Error()
		} else {
			model.state = completed
		}
	case tea.KeyMsg:
		if model.state == writing {
			return model, nil
		}
		if message.Type == tea.KeyCtrlC {
			model.result = &Result{Quit: true}
			return model, nil
		}
		if model.tooSmall() {
			if message.String() == "q" {
				model.result = &Result{}
			}
			return model, nil
		}
		if model.help {
			if message.Type == tea.KeyEsc || message.String() == "?" {
				model.help = false
			}
			if message.String() == "q" {
				model.result = &Result{}
			}
			return model, nil
		}
		if model.state == nameInput {
			return model.editName(message), nil
		}
		if message.String() == "?" && model.state != preparing {
			model.help = true
			return model, nil
		}
		if message.String() == "q" {
			model.result = &Result{}
			return model, nil
		}
		if message.Type == tea.KeyEsc || message.String() == "h" {
			if model.state == review || model.state == preparing || model.state == failed {
				model.state, model.err, model.request, model.scroll = editing, "", nil, 0
				model.prepared = app.PreparedExampleExport{}
			} else {
				model.result = &Result{}
			}
			return model, nil
		}
		if model.scrollKey(message) {
			return model, nil
		}
		switch model.state {
		case editing:
			switch message.String() {
			case "j", "down":
				if len(model.draft.Entries) > 0 {
					model.cursor = (model.cursor + 1) % len(model.draft.Entries)
				}
				model.scroll = 0
			case "k", "up":
				if len(model.draft.Entries) > 0 {
					model.cursor = (model.cursor + len(model.draft.Entries) - 1) % len(model.draft.Entries)
				}
				model.scroll = 0
			case "g":
				model.cursor, model.scroll = 0, 0
			case "G":
				model.cursor, model.scroll = max(0, len(model.draft.Entries)-1), 0
			case "e":
				if len(model.draft.Entries) > 0 && !model.draft.Entries[model.cursor].ExistingReference {
					model.input = []rune(model.draft.Entries[model.cursor].EnvironmentName)
					model.inputCursor, model.state, model.err = len(model.input), nameInput, ""
				}
			case "enter", "l":
				return model, model.prepare()
			}
		case review:
			if message.Type == tea.KeySpace && len(model.prepared.SharedVariables) > 0 {
				model.sharedApproved = !model.sharedApproved
				model.err = ""
			}
			if message.Type == tea.KeyEnter {
				if len(model.prepared.SharedVariables) > 0 && !model.sharedApproved {
					model.err = "Review and explicitly approve the shared environment sources with Space."
					return model, nil
				}
				return model, model.commit()
			}
		case completed:
			if message.Type == tea.KeyEnter {
				model.result = &Result{}
			}
		case failed:
			if message.Type == tea.KeyEnter {
				model.state, model.err, model.scroll = editing, "", 0
				model.prepared = app.PreparedExampleExport{}
			}
		}
	}
	return model, nil
}

func (model Model) editName(key tea.KeyMsg) Model {
	switch key.Type {
	case tea.KeyEsc:
		model.state, model.err = editing, ""
		model.input = nil
	case tea.KeyEnter:
		name := strings.TrimSpace(string(model.input))
		if err := app.ValidateExampleEnvironmentName(name); err != nil {
			model.err = err.Error()
			return model
		}
		model.draft.Entries[model.cursor].EnvironmentName = name
		model.input = nil
		model.state, model.err, model.sharedApproved = editing, "", false
	case tea.KeyLeft, tea.KeyCtrlB:
		model.inputCursor = max(0, model.inputCursor-1)
	case tea.KeyRight, tea.KeyCtrlF:
		model.inputCursor = min(len(model.input), model.inputCursor+1)
	case tea.KeyHome, tea.KeyCtrlA:
		model.inputCursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		model.inputCursor = len(model.input)
	case tea.KeyCtrlU:
		model.input = nil
		model.inputCursor = 0
	case tea.KeyCtrlK:
		model.input = model.input[:model.inputCursor]
	case tea.KeyBackspace:
		if model.inputCursor > 0 {
			model.input = append(model.input[:model.inputCursor-1], model.input[model.inputCursor:]...)
			model.inputCursor--
		}
	case tea.KeyDelete:
		if model.inputCursor < len(model.input) {
			model.input = append(model.input[:model.inputCursor], model.input[model.inputCursor+1:]...)
		}
	case tea.KeySpace:
		model.insertName([]rune{' '})
	case tea.KeyRunes:
		model.insertName(key.Runes)
	}
	return model
}
func (model *Model) insertName(value []rune) {
	next := append([]rune(nil), model.input[:model.inputCursor]...)
	next = append(next, value...)
	next = append(next, model.input[model.inputCursor:]...)
	model.input = next
	model.inputCursor += len(value)
}
func (model Model) tooSmall() bool {
	return model.width > 0 && model.height > 0 && (model.width < 80 || model.height < 24)
}
