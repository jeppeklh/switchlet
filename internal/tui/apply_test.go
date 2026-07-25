package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestUpdate_ProtectedProfileConfirmationAppliesSelectedProfileWithEnter(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
	  "database": {
	    "primary": {
	      "url": "postgres://old"
	    }
	  }
}
`)+"\n")

	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Production", Value: stringPointer("postgres://new"), Protected: true}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want confirmation before apply")
	}
	if model.state != confirmState {
		t.Fatalf("state = %d, want confirmState", model.state)
	}

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want apply command after confirmation")
	}

	message := command()
	updatedModel, quitCommand := updatedModel.Update(message)
	model = updatedModel.(Model)

	if quitCommand == nil {
		t.Fatal("quitCommand is nil, want success quit command")
	}
	if model.state != successState {
		t.Fatalf("state = %d, want successState", model.state)
	}
	if model.successResult == nil {
		t.Fatal("successResult is nil, want success result")
	}

	updatedContents := readFile(t, targetPath)
	if !strings.Contains(string(updatedContents), "postgres://new") {
		t.Fatalf("updated target = %q, want applied protected profile", string(updatedContents))
	}
}

func TestUpdate_ProtectedProfileConfirmationStillAcceptsY(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Production", Value: stringPointer("postgres://new"), Protected: true}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want confirmation before apply")
	}
	if model.state != confirmState {
		t.Fatalf("state = %d, want confirmState", model.state)
	}

	updatedModel, command = model.Update(runeKey('y'))
	if command == nil {
		t.Fatal("command is nil, want apply command after y confirmation")
	}
}

func TestUpdate_AppliesSelectedProfileSuccessfully(t *testing.T) {
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

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want apply command")
	}

	message := command()
	updatedModel, quitCommand := updatedModel.Update(message)
	model = updatedModel.(Model)

	if quitCommand == nil {
		t.Fatal("quitCommand is nil, want success quit command")
	}
	if model.state != successState {
		t.Fatalf("state = %d, want successState", model.state)
	}
	if model.successResult == nil {
		t.Fatal("successResult is nil, want success result")
	}
	if !strings.Contains(model.View(), "Applied profile: Local") {
		t.Fatalf("View() = %q, want success message", model.View())
	}
	if !strings.Contains(model.View(), "Updated target:\nservice.baseUrl") {
		t.Fatalf("View() = %q, want updated target path", model.View())
	}
	if !strings.Contains(model.FinalMessage(), "Applied profile: Local") {
		t.Fatalf("FinalMessage() = %q, want applied profile summary", model.FinalMessage())
	}
	if !strings.Contains(model.FinalMessage(), "Updated target:\nservice.baseUrl") {
		t.Fatalf("FinalMessage() = %q, want updated target path", model.FinalMessage())
	}

	updatedContents := readFile(t, targetPath)
	if !strings.Contains(string(updatedContents), "https://new.example.test") {
		t.Fatalf("updated target = %q, want applied profile value", string(updatedContents))
	}
}

func TestFinalMessage_IsEmptyUnlessApplicationSucceeded(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://local.example.test")}},
	))

	if got := model.FinalMessage(); got != "" {
		t.Fatalf("FinalMessage() = %q, want empty message before success", got)
	}
}

func TestUpdate_ShowsRecoverableApplicationError(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{`)

	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://new.example.test")}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want apply command")
	}

	message := command()
	updatedModel, _ = updatedModel.Update(message)
	model = updatedModel.(Model)

	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}
	if !strings.Contains(model.errorMessage, "contains invalid JSON") {
		t.Fatalf("errorMessage = %q, want editor error", model.errorMessage)
	}
	if !strings.Contains(model.View(), "Press any key to return") {
		t.Fatalf("View() = %q, want recoverable error guidance", model.View())
	}
}

func TestUpdate_CtrlCQuitsFromErrorView(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING")}},
	))

	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model = updatedModel.(Model)

	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
}
