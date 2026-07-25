package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestView_ShowsTooSmallTerminalMessage(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	model = updatedModel.(Model)

	view := model.View()
	if !strings.Contains(view, "Terminal too small.") {
		t.Fatalf("View() = %q, want too-small message", view)
	}
	if !strings.Contains(view, "Minimum size: 80x24") {
		t.Fatalf("View() = %q, want minimum size guidance", view)
	}
	if !strings.Contains(view, "Current size: 79x23") {
		t.Fatalf("View() = %q, want current size guidance", view)
	}
	if !strings.Contains(view, "q Quit") {
		t.Fatalf("View() = %q, want quit guidance", view)
	}
}

func TestView_TooSmallTerminalMessageStaysWithinReportedWidth(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	model = updatedModel.(Model)

	for _, line := range strings.Split(model.View(), "\n") {
		if len([]rune(line)) > 20 {
			t.Fatalf("line %q has width %d, want at most 20", line, len([]rune(line)))
		}
	}
}

func TestUpdate_DoesNotApplyHiddenActionsWhenTerminalIsTooSmall(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
	  "service": {
	    "baseUrl": "https://old.example.test"
	  }
}
`)+"\n")

	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://new.example.test")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no hidden apply command")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}

	updatedContents := readFile(t, targetPath)
	if strings.Contains(string(updatedContents), "https://new.example.test") {
		t.Fatalf("updated target = %q, want no file modification while terminal is too small", string(updatedContents))
	}
}

func TestUpdate_TooSmallTerminalStillAllowsDocumentedQuitAndCancelKeys(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
	))

	updatedModel, _ := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if model.state != inspectState {
		t.Fatalf("state = %d, want inspectState", model.state)
	}

	updatedModel, _ = model.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when cancelling inspection from too-small terminal")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}

	updatedModel, command = model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want quit command from too-small list view")
	}
}
