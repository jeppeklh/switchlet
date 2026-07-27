package initwizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOverwriteConfirmationModel_RendersWizardQualityDecisionSurface(t *testing.T) {
	model := NewOverwriteConfirmationModel("/workspace/.switchlet.yaml").(overwriteConfirmationModel)
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	model = updatedModel.(overwriteConfirmationModel)

	view := model.View()
	for _, expected := range []string{
		"Switchlet init",
		"Existing configuration",
		"Choose whether setup should continue.",
		"* Switchlet is already configured in this directory.",
		"File: /workspace/.switchlet.yaml",
		"> Keep existing configuration",
		"Replace .switchlet.yaml",
		"Enter Select",
		"y Replace",
		"n Keep",
		"q Cancel",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want %q", view, expected)
		}
	}
	if strings.Contains(view, "Overwrite it and create a new configuration? [y/N]") {
		t.Fatalf("View() = %q, want TUI confirmation instead of line prompt", view)
	}
	if strings.Contains(view, "* Decision") {
		t.Fatalf("View() = %q, want one focused confirmation card instead of a separate decision panel", view)
	}
	if !viewContainsCenteredHeader(view, "* Switchlet is already configured in this directory.") {
		t.Fatalf("View() = %q, want centered confirmation card", view)
	}
}

func viewContainsCenteredHeader(view string, header string) bool {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, header) && strings.HasPrefix(line, " ") {
			return true
		}
	}

	return false
}

func TestOverwriteConfirmationModel_DefaultEnterKeepsExistingConfiguration(t *testing.T) {
	model := NewOverwriteConfirmationModel("/workspace/.switchlet.yaml").(overwriteConfirmationModel)
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
	model = updatedModel.(overwriteConfirmationModel)

	result, ok := model.Result()
	if !ok {
		t.Fatal("Result returned ok=false")
	}
	if result.Replace || result.Cancelled {
		t.Fatalf("result = %#v, want keep existing configuration", result)
	}
}

func TestOverwriteConfirmationModel_CanSelectReplacement(t *testing.T) {
	model := NewOverwriteConfirmationModel("/workspace/.switchlet.yaml").(overwriteConfirmationModel)
	updatedModel, command := model.Update(runeKey('y'))
	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
	model = updatedModel.(overwriteConfirmationModel)

	result, ok := model.Result()
	if !ok {
		t.Fatal("Result returned ok=false")
	}
	if !result.Replace || result.Cancelled {
		t.Fatalf("result = %#v, want replacement", result)
	}
}

func TestOverwriteConfirmationModel_CanKeepExistingWithN(t *testing.T) {
	model := NewOverwriteConfirmationModel("/workspace/.switchlet.yaml").(overwriteConfirmationModel)
	updatedModel, command := model.Update(runeKey('n'))
	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
	model = updatedModel.(overwriteConfirmationModel)

	result, ok := model.Result()
	if !ok {
		t.Fatal("Result returned ok=false")
	}
	if result.Replace || result.Cancelled {
		t.Fatalf("result = %#v, want keep existing configuration", result)
	}
}

func TestOverwriteConfirmationModel_CancelIsDistinctFromKeep(t *testing.T) {
	model := NewOverwriteConfirmationModel("/workspace/.switchlet.yaml").(overwriteConfirmationModel)
	updatedModel, command := model.Update(runeKey('q'))
	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
	model = updatedModel.(overwriteConfirmationModel)

	result, ok := model.Result()
	if !ok {
		t.Fatal("Result returned ok=false")
	}
	if !result.Cancelled || result.Replace {
		t.Fatalf("result = %#v, want cancelled result", result)
	}
}

func TestOverwriteConfirmationModel_TooSmallTerminalStateIsSafe(t *testing.T) {
	model := NewOverwriteConfirmationModel("/workspace/.switchlet.yaml").(overwriteConfirmationModel)
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	model = updatedModel.(overwriteConfirmationModel)

	view := model.View()
	for _, expected := range []string{"Terminal too small.", "Resize required", "Minimum size: 80x24", "Current size: 79x23"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want %q", view, expected)
		}
	}

	updatedModel, command := model.Update(runeKey('q'))
	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
	model = updatedModel.(overwriteConfirmationModel)
	result, ok := model.Result()
	if !ok || !result.Cancelled {
		t.Fatalf("result = %#v, ok = %v, want cancelled result", result, ok)
	}
}
