package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	if !strings.Contains(view, "x Production [protected] [unavailable]") {
		t.Fatalf("View() = %q, want protected and unavailable indicators", view)
	}
	if !strings.Contains(view, "* Profiles") {
		t.Fatalf("View() = %q, want focused profiles panel", view)
	}
}

func TestView_ListViewUsesNeutralProtectedStatusAndContinueHelp(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
	))

	view := model.View()
	if !strings.Contains(view, `Status: Selected "Production"`) {
		t.Fatalf("View() = %q, want neutral protected status copy", view)
	}
	if strings.Contains(view, `requires confirmation`) {
		t.Fatalf("View() = %q, must not show premature confirmation status copy", view)
	}
	if !strings.Contains(view, "Enter Continue") {
		t.Fatalf("View() = %q, want Enter help text that matches the protected flow", view)
	}
}

func TestView_ListViewUsesSplitLayoutAtComfortableWidth(t *testing.T) {
	model := New(app.New(
		config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("postgres://local")},
			{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING"), Protected: true},
		},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)

	view := model.View()
	if !lineContains(view, "* Profiles", "Selected") {
		t.Fatalf("View() = %q, want wide split layout with profile list and selected context on the same row", view)
	}
	if !strings.Contains(view, "config/development.json") {
		t.Fatalf("View() = %q, want target file context in header or details", view)
	}
	if !strings.Contains(view, "database.primary.url") {
		t.Fatalf("View() = %q, want target JSON path context in header or details", view)
	}
}

func TestView_ListViewStacksBeforeLayoutBecomesCramped(t *testing.T) {
	model := New(app.New(
		config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Local", Value: stringPointer("postgres://local")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)

	view := model.View()
	if lineContains(view, "* Profiles", "Selected") {
		t.Fatalf("View() = %q, want stacked layout at minimum supported width", view)
	}
	if !strings.Contains(view, "* Profiles") || !strings.Contains(view, "Selected") {
		t.Fatalf("View() = %q, want both stacked main regions", view)
	}
}

func TestView_LongMainScreenContentStaysWithinTerminalWidth(t *testing.T) {
	model := New(app.New(
		config.Target{
			File:     "/very/long/project/path/with/many/segments/configuration/appsettings.Development.json",
			JSONPath: "services.database.primary.connectionStrings.defaultConnection.value",
		},
		[]config.Profile{{Name: "A profile with a very long name that should not break the shell", Value: stringPointer("postgres://local")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)

	for _, line := range strings.Split(model.View(), "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("line %q has width %d, want at most 80", line, lipgloss.Width(line))
		}
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
