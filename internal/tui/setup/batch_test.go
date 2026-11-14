package setup

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
	"strings"
	"testing"
)

func TestBatchReviewRenameTypeAndBack(t *testing.T) {
	target := app.InitTarget{Name: "FLAG", File: ".env", Type: app.InitTargetTypeDotenv, Key: "FLAG"}
	batch := Batch{}
	batch.Toggle(target)
	batch.BeginReview()
	batch.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	batch.Update(tea.KeyMsg{Type: tea.KeyEnd})
	batch.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	batch.Update(tea.KeyMsg{Type: tea.KeyEnter})
	batch.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if batch.Targets[0].Name != "FLAGq" || batch.Targets[0].ValueType != app.ManagedValueBoolean {
		t.Fatal("rename/type controls failed")
	}
	if batch.Update(tea.KeyMsg{Type: tea.KeyEsc}) != BatchBack || len(batch.Targets) != 1 {
		t.Fatal("back lost marks")
	}
	if batch.Label(target, "FLAG") != "[x] FLAG" {
		t.Fatal("renaming lost mark identity")
	}
	batch.Toggle(target)
	if len(batch.Targets) != 0 {
		t.Fatal("could not unmark renamed target")
	}
}

func TestBatchReviewBoundsAndPendingInput(t *testing.T) {
	batch := Batch{}
	for i := 0; i < 30; i++ {
		batch.Toggle(app.InitTarget{Name: strings.Repeat("長", 100) + string(rune('A'+i)), File: "long/path/config.json", Type: app.InitTargetTypeJSON, JSONPath: string(rune('A' + i))})
	}
	batch.BeginReview()
	batch.Update(tea.KeyMsg{Type: tea.KeyEnd})
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
		view := ui.RenderShell(ui.Shell{Title: "Batch", Panels: batch.Panels(), Actions: batch.Actions(), Width: size[0], Height: size[1]})
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("overflow at %v", size)
		}
	}
	batch.Pending = true
	if batch.Update(tea.KeyMsg{Type: tea.KeyEnter}) != BatchContinue {
		t.Fatal("pending accepted duplicate submit")
	}
	if batch.Update(tea.KeyMsg{Type: tea.KeyEsc}) != BatchBack {
		t.Fatal("pending could not cancel")
	}
}
