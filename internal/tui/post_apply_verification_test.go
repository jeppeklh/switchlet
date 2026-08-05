package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestUpdate_PostApplyVerificationFailureShowsRecoverableValueSafeError(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)

	model := NewWithProjectRoot(app.New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://expected-secret.example.test")}},
	), projectRoot)

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want apply command")
	}
	model = updatedModel.(Model)

	verificationErr := app.PostApplyVerificationError{
		ProfileName: "Local",
		Failures: []app.PostApplyVerificationFailure{{
			TargetDescriptor: app.TargetDescriptor{
				TargetName:   "default",
				TargetFile:   targetPath,
				TargetType:   config.TargetTypeJSON,
				SelectorName: "jsonPath",
				Selector:     "service.baseUrl",
			},
			Reason: "current value does not match selected profile value",
		}},
	}
	updatedModel, _ = model.Update(applyCompletedMsg{requestID: model.applyRequestID, err: verificationErr})
	model = updatedModel.(Model)

	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}
	if model.successResult != nil {
		t.Fatalf("successResult = %#v, want nil", model.successResult)
	}

	view := model.View()
	for _, expected := range []string{
		"Writes completed, but Switchlet could not confirm the final managed state.",
		"Context:",
		"Profile: Local",
		"Managed value: default [json]",
		"config.json",
		"Selector: service.baseUrl",
		"Reason:",
		"current value does not match selected profile value",
		"Recovery:",
		"status or diff",
		"Press any key",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want verification error detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{"expected-secret", "current-secret"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain raw value %q", view, forbidden)
		}
	}
	assertVisibleWidth(t, view, 80)
}
