// Package setup contains interaction shared by init and configuration editing.
package setup

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

// BatchAction requests a transition owned by the hosting workflow.
type BatchAction int

const (
	BatchContinue BatchAction = iota
	BatchAdd
	BatchBack
	BatchQuit
)

// Batch retains marks independently of the current filter or navigation level.
// Targets are drafts; the host must validate them before accepting BatchAdd.
type Batch struct {
	Targets     []app.InitTarget
	Review      bool
	Pending     bool
	Error       string
	cursor      int
	editing     bool
	input       []rune
	inputCursor int
}

// Toggle marks or unmarks one location, retaining other edited names and types.
func (batch *Batch) Toggle(target app.InitTarget) {
	for index, existing := range batch.Targets {
		if sameLocation(existing, target) {
			batch.Targets = append(append([]app.InitTarget(nil), batch.Targets[:index]...), batch.Targets[index+1:]...)
			return
		}
	}
	batch.Targets = append(append([]app.InitTarget(nil), batch.Targets...), target)
}

// Label adds a visible mark to a selectable setting.
func (batch Batch) Label(target app.InitTarget, label string) string {
	for _, existing := range batch.Targets {
		if sameLocation(existing, target) {
			return "[x] " + label
		}
	}
	return "[ ] " + label
}

func sameLocation(left, right app.InitTarget) bool {
	return left.File == right.File && left.Type == right.Type && left.JSONPath == right.JSONPath && left.YAMLPath == right.YAMLPath && left.TOMLPath == right.TOMLPath && left.Key == right.Key
}

// BeginReview opens the marked batch without changing selection context.
func (batch *Batch) BeginReview() {
	if len(batch.Targets) == 0 {
		return
	}
	batch.Review = true
	batch.cursor = min(batch.cursor, len(batch.Targets)-1)
	batch.Error = ""
}

// Update handles batch review input; persistent effects belong to the host.
func (batch *Batch) Update(key tea.KeyMsg) BatchAction {
	if batch.editing {
		batch.editName(key)
		return BatchContinue
	}
	if key.Type == tea.KeyEsc {
		return BatchBack
	}
	if key.String() == "q" {
		return BatchQuit
	}
	if batch.Pending || len(batch.Targets) == 0 {
		return BatchContinue
	}
	batch.Targets = append([]app.InitTarget(nil), batch.Targets...)
	switch key.String() {
	case "up", "k":
		batch.cursor = (batch.cursor + len(batch.Targets) - 1) % len(batch.Targets)
	case "down", "j":
		batch.cursor = (batch.cursor + 1) % len(batch.Targets)
	case "home", "g":
		batch.cursor = 0
	case "end", "G":
		batch.cursor = len(batch.Targets) - 1
	case "enter":
		return BatchAdd
	case "r":
		batch.editing = true
		batch.input = []rune(batch.Targets[batch.cursor].Name)
		batch.inputCursor = len(batch.input)
	case "t":
		target := &batch.Targets[batch.cursor]
		if target.Type == app.InitTargetTypeDotenv {
			kinds := []app.ManagedValueType{"", app.ManagedValueBoolean, app.ManagedValueInteger, app.ManagedValueNumber}
			for index, kind := range kinds {
				if target.ValueType == kind || (kind == "" && target.ValueType == app.ManagedValueString) {
					target.ValueType = kinds[(index+1)%len(kinds)]
					break
				}
			}
		}
	}
	return BatchContinue
}

func (batch *Batch) editName(key tea.KeyMsg) {
	input := append([]rune(nil), batch.input...)
	switch key.Type {
	case tea.KeyEsc:
		batch.editing = false
	case tea.KeyEnter:
		name := strings.TrimSpace(string(input))
		if name == "" {
			batch.Error = "Managed value name must be set."
			return
		}
		batch.Targets = append([]app.InitTarget(nil), batch.Targets...)
		batch.Targets[batch.cursor].Name = name
		batch.editing = false
		batch.Error = ""
	case tea.KeyLeft, tea.KeyCtrlB:
		batch.inputCursor = max(0, batch.inputCursor-1)
	case tea.KeyRight, tea.KeyCtrlF:
		batch.inputCursor = min(len(input), batch.inputCursor+1)
	case tea.KeyHome, tea.KeyCtrlA:
		batch.inputCursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		batch.inputCursor = len(input)
	case tea.KeyCtrlU:
		input = nil
		batch.inputCursor = 0
	case tea.KeyCtrlK:
		input = input[:batch.inputCursor]
	case tea.KeyBackspace:
		if batch.inputCursor > 0 {
			input = append(input[:batch.inputCursor-1], input[batch.inputCursor:]...)
			batch.inputCursor--
		}
	case tea.KeyDelete:
		if batch.inputCursor < len(input) {
			input = append(input[:batch.inputCursor], input[batch.inputCursor+1:]...)
		}
	case tea.KeyRunes, tea.KeySpace:
		runes := key.Runes
		if key.Type == tea.KeySpace {
			runes = []rune{' '}
		}
		updated := append([]rune(nil), input[:batch.inputCursor]...)
		updated = append(updated, runes...)
		input = append(updated, input[batch.inputCursor:]...)
		batch.inputCursor += len(runes)
	}
	batch.input = input
}

// Panels renders a bounded, value-safe review using the shared design system.
func (batch Batch) Panels() []ui.Panel {
	start := max(0, batch.cursor-5)
	end := min(len(batch.Targets), start+6)
	rows := make([]ui.ListRow, 0, end-start)
	for index := start; index < end; index++ {
		target := batch.Targets[index]
		kind := string(target.ValueType)
		if kind == "" {
			kind = "string"
		}
		state := ui.RowNormal
		if index == batch.cursor {
			state = ui.RowSelected
		}
		rows = append(rows, ui.ListRow{Label: target.Name, State: state, Badges: []ui.Badge{{Label: kind}}})
	}
	lines := ui.RenderListRows(rows)
	lines = append(lines, fmt.Sprintf("%d marked", len(batch.Targets)))
	if batch.editing {
		lines = append(lines, ui.RenderInput("Name", string(batch.input), batch.inputCursor))
	}
	if batch.Pending {
		lines = append(lines, "Validating selected values…")
	}
	if batch.Error != "" {
		lines = append(lines, batch.Error)
	}
	panels := []ui.Panel{{Title: "Review managed values", Lines: lines, Focused: true}}
	if len(batch.Targets) > 0 {
		target := batch.Targets[batch.cursor]
		selector := target.JSONPath + target.YAMLPath + target.TOMLPath + target.Key
		panels = append(panels, ui.Panel{Title: "Selected location", Lines: []string{ui.RenderKeyValue("File", target.File), ui.RenderKeyValue("Selector", selector)}})
	}
	return panels
}

// Actions describes the active review controls.
func (batch Batch) Actions() []ui.Action {
	if batch.editing {
		return []ui.Action{{Key: "Enter", Label: "Rename"}, {Key: "Esc", Label: "Back"}, {Key: "Ctrl+C", Label: "Cancel"}}
	}
	if batch.Pending {
		return []ui.Action{{Key: "Esc", Label: "Back"}, {Key: "Ctrl+C", Label: "Cancel"}}
	}
	return []ui.Action{{Key: "Enter", Label: "Add all"}, {Key: "↑/↓", Label: "Move"}, {Key: "r", Label: "Rename"}, {Key: "t", Label: "Dotenv type"}, {Key: "Esc", Label: "Selections"}, {Key: "q", Label: "Cancel"}}
}
