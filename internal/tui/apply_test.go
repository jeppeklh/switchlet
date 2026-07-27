package tui

import (
	"bytes"
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
	if !strings.Contains(model.View(), "Updated target:") || !strings.Contains(model.View(), "service.baseUrl") {
		t.Fatalf("View() = %q, want updated target path", model.View())
	}
	assertVisibleWidth(t, model.View(), 80)
	if !strings.Contains(model.FinalMessage(), "Applied profile \"Local\"") {
		t.Fatalf("FinalMessage() = %q, want applied profile summary", model.FinalMessage())
	}
	for _, expected := range []string{"Updated target:", "updated " + targetPath, "default [json]", "service.baseUrl"} {
		if !strings.Contains(model.FinalMessage(), expected) {
			t.Fatalf("FinalMessage() = %q, want updated target detail %q", model.FinalMessage(), expected)
		}
	}

	updatedContents := readFile(t, targetPath)
	if !strings.Contains(string(updatedContents), "https://new.example.test") {
		t.Fatalf("updated target = %q, want applied profile value", string(updatedContents))
	}
}

func TestUpdate_AppliesYAMLSelectedProfileThroughCommand(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "worker/config.yaml", strings.TrimSpace(`
queue:
  endpoint: old-queue
  retries: 3
`)+"\n")
	originalContents := readFile(t, targetPath)

	model := New(app.NewWithTargets(
		[]config.Target{{Name: "workerQueue", File: targetPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "workerQueue", Value: stringPointer("staging-queue")},
			},
		}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want YAML apply command")
	}
	if model.applyingProfile != "Staging" {
		t.Fatalf("applyingProfile = %q, want Staging", model.applyingProfile)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("YAML target changed before returned apply command was executed")
	}

	message := command()
	updatedModel, quitCommand := model.Update(message)
	model = updatedModel.(Model)
	if quitCommand == nil {
		t.Fatal("quitCommand is nil, want YAML success quit command")
	}
	if model.state != successState {
		t.Fatalf("state = %d, want successState", model.state)
	}
	if model.successResult == nil {
		t.Fatal("successResult is nil, want YAML success result")
	}

	view := model.View()
	for _, expected := range []string{"Applied profile: Staging", "Updated target:", "workerQueue [yaml]", "queue.endpoint"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want YAML success detail %q", view, expected)
		}
	}
	finalMessage := model.FinalMessage()
	for _, expected := range []string{`Applied profile "Staging"`, "Updated target:", "updated " + targetPath, "workerQueue [yaml]", "queue.endpoint"} {
		if !strings.Contains(finalMessage, expected) {
			t.Fatalf("FinalMessage() = %q, want YAML final detail %q", finalMessage, expected)
		}
	}
	for _, forbidden := range []string{"staging-queue"} {
		if strings.Contains(view, forbidden) || strings.Contains(finalMessage, forbidden) {
			t.Fatalf("YAML success output must not contain resolved value %q\nview: %q\nfinal: %q", forbidden, view, finalMessage)
		}
	}
	updatedContents := string(readFile(t, targetPath))
	if !strings.Contains(updatedContents, "endpoint: staging-queue") {
		t.Fatalf("YAML target = %q, want updated endpoint", updatedContents)
	}
	if !strings.Contains(updatedContents, "retries: 3") {
		t.Fatalf("YAML target = %q, want unrelated value preserved", updatedContents)
	}
}

func TestUpdate_AppliesTOMLSelectedProfileThroughCommand(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "services/development.toml", strings.TrimSpace(`
[services.api]
endpoint = "http://old.example.test"
retries = 3
`)+"\n")
	originalContents := readFile(t, targetPath)

	model := New(app.NewWithTargets(
		[]config.Target{{Name: "serviceEndpoint", File: targetPath, Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"}},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "serviceEndpoint", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want TOML apply command")
	}
	if model.applyingProfile != "Staging" {
		t.Fatalf("applyingProfile = %q, want Staging", model.applyingProfile)
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("TOML target changed before returned apply command was executed")
	}

	message := command()
	updatedModel, quitCommand := model.Update(message)
	model = updatedModel.(Model)
	if quitCommand == nil {
		t.Fatal("quitCommand is nil, want TOML success quit command")
	}
	if model.state != successState {
		t.Fatalf("state = %d, want successState", model.state)
	}
	if model.successResult == nil {
		t.Fatal("successResult is nil, want TOML success result")
	}

	view := model.View()
	for _, expected := range []string{"Applied profile: Staging", "Updated target:", "serviceEndpoint [toml]", "services.api.endpoint"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want TOML success detail %q", view, expected)
		}
	}
	finalMessage := model.FinalMessage()
	for _, expected := range []string{`Applied profile "Staging"`, "Updated target:", "updated " + targetPath, "serviceEndpoint [toml]", "services.api.endpoint"} {
		if !strings.Contains(finalMessage, expected) {
			t.Fatalf("FinalMessage() = %q, want TOML final detail %q", finalMessage, expected)
		}
	}
	if strings.Contains(view, "https://api.staging.example.test") || strings.Contains(finalMessage, "https://api.staging.example.test") {
		t.Fatalf("TOML success output must not contain resolved value\nview: %q\nfinal: %q", view, finalMessage)
	}
	updatedContents := string(readFile(t, targetPath))
	if !strings.Contains(updatedContents, `endpoint = "https://api.staging.example.test"`) {
		t.Fatalf("TOML target = %q, want updated endpoint", updatedContents)
	}
	if !strings.Contains(updatedContents, "retries = 3") {
		t.Fatalf("TOML target = %q, want unrelated value preserved", updatedContents)
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
	if !strings.Contains(model.recoverableError.Reason, "contains invalid JSON") {
		t.Fatalf("recoverableError.Reason = %q, want editor error", model.recoverableError.Reason)
	}
	for _, expected := range []string{"Could not prepare target", "Context:", "Profile: Local", "Target: default [json]", "config.json", "Selector: service.baseUrl", "Reason:", "contains invalid JSON", "Recovery:", "Press any key"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("View() = %q, want recoverable error detail %q", model.View(), expected)
		}
	}
	if strings.Contains(model.View(), "https://new.example.test") {
		t.Fatalf("View() = %q, must not contain resolved replacement value", model.View())
	}
	if !strings.Contains(model.recoverableError.Recovery, "Press any key to return.") {
		t.Fatalf("View() = %q, want recoverable error guidance", model.View())
	}
	assertVisibleWidth(t, model.View(), 80)
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
