package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestUpdate_OpensInspectionAndReturnsToList(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_CONNECTION_STRING", "Server=test;Database=App;Password=super-secret;")

	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Test", ValueFromEnv: stringPointer("MYAPPLICATION_TEST_CONNECTION_STRING")}},
	))

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	if model.state != inspectState {
		t.Fatalf("state = %d, want inspectState", model.state)
	}

	view := model.View()
	if !strings.Contains(view, "Inspect Profile") {
		t.Fatalf("View() = %q, want inspection title", view)
	}
	if !strings.Contains(view, "Profile: Test") {
		t.Fatalf("View() = %q, want selected profile name", view)
	}
	if !strings.Contains(view, "Source: Environment variable") {
		t.Fatalf("View() = %q, want source label", view)
	}
	if !strings.Contains(view, "Environment variable: MYAPPLICATION_TEST_CONNECTION_STRING") {
		t.Fatalf("View() = %q, want environment variable name", view)
	}
	if !strings.Contains(view, "Password=****") {
		t.Fatalf("View() = %q, want masked connection string", view)
	}
	if strings.Contains(view, "super-secret") {
		t.Fatalf("View() = %q, must not contain unmasked secret", view)
	}

	updatedModel, command = model.Update(runeKey('q'))
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no command when closing inspection")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}
	if !strings.Contains(model.View(), "Select a profile") {
		t.Fatalf("View() = %q, want profile list view", model.View())
	}
}

func TestUpdate_InspectionShowsResolutionErrorForUnavailableProfile(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING")}},
	))

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	if model.state != inspectState {
		t.Fatalf("state = %d, want inspectState", model.state)
	}

	view := model.View()
	if !strings.Contains(view, "Masked connection string:\nUnavailable") {
		t.Fatalf("View() = %q, want unavailable masked value message", view)
	}
	if !strings.Contains(view, "Resolution error:") {
		t.Fatalf("View() = %q, want resolution error heading", view)
	}
	if !strings.Contains(view, "MISSING_CONNECTION_STRING") {
		t.Fatalf("View() = %q, want unavailable reason", view)
	}
}

func TestUpdate_ProtectedProfileRequiresConfirmationAndCancels(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;Password=super-secret;"), Protected: true}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no apply command before confirmation")
	}
	if model.state != confirmState {
		t.Fatalf("state = %d, want confirmState", model.state)
	}

	view := model.View()
	if !strings.Contains(view, "Apply protected profile?") {
		t.Fatalf("View() = %q, want confirmation title", view)
	}
	if !strings.Contains(view, "Profile: Production") {
		t.Fatalf("View() = %q, want protected profile name", view)
	}
	if strings.Contains(view, "super-secret") {
		t.Fatalf("View() = %q, must not contain unmasked secret", view)
	}
	if strings.Contains(view, "Password=****") {
		t.Fatalf("View() = %q, must not contain masked connection string in confirmation", view)
	}

	updatedModel, command = model.Update(runeKey('n'))
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no command when cancelling confirmation")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}
}

func TestUpdate_ProtectedUnavailableProfileShowsRecoverableError(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING"), Protected: true}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no command for unavailable protected profile")
	}
	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}
	if !strings.Contains(model.errorMessage, "MISSING_CONNECTION_STRING") {
		t.Fatalf("errorMessage = %q, want unavailable reason", model.errorMessage)
	}
}
