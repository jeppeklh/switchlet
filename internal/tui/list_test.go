package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestNew_InitializesProfilesAndSelection(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING"), Protected: true},
		},
	)

	model := New(application)

	if len(model.profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2", len(model.profiles))
	}
	if model.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", model.cursor)
	}

	view := model.View()
	if !strings.Contains(view, "> Local") {
		t.Fatalf("View() = %q, want selected Local profile", view)
	}
	if !strings.Contains(view, "Production [protected] [unavailable]") {
		t.Fatalf("View() = %q, want protected and unavailable indicators", view)
	}
}

func TestUpdate_MovesCursorDownUpAndWraps(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Test", Value: stringPointer("Server=test;Database=App;")},
		},
	))

	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)
	if model.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", model.cursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)
	if model.cursor != 0 {
		t.Fatalf("cursor after down wrap = %d, want 0", model.cursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updatedModel.(Model)
	if model.cursor != 1 {
		t.Fatalf("cursor after up wrap = %d, want 1", model.cursor)
	}
}

func TestUpdate_ShowsRecoverableErrorForUnavailableProfile(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING")}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no apply command for unavailable profile")
	}
	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}
	if !strings.Contains(model.errorMessage, "MISSING_CONNECTION_STRING") {
		t.Fatalf("errorMessage = %q, want unavailable reason", model.errorMessage)
	}
	if !strings.Contains(model.View(), "Error") {
		t.Fatalf("View() = %q, want error view", model.View())
	}
}

func TestUpdate_QuitsImmediately(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model = updatedModel.(Model)

	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}
}

func TestUpdate_CtrlCQuitsFromSecondaryViews(t *testing.T) {
	tests := []struct {
		name      string
		openKey   tea.KeyMsg
		wantState viewState
	}{
		{name: "inspection", openKey: runeKey('i'), wantState: inspectState},
		{name: "confirmation", openKey: tea.KeyMsg{Type: tea.KeyEnter}, wantState: confirmState},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			model := New(app.New(
				config.Target{},
				[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
			))

			updatedModel, _ := model.Update(testCase.openKey)
			model = updatedModel.(Model)
			if model.state != testCase.wantState {
				t.Fatalf("state after opening = %d, want %d", model.state, testCase.wantState)
			}

			updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			model = updatedModel.(Model)

			if command == nil {
				t.Fatal("command is nil, want quit command")
			}
		})
	}
}
