package tui

import (
	"fmt"
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
	if !strings.Contains(view, "Profile detail") {
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
	if !strings.Contains(view, "Masked value:") {
		t.Fatalf("View() = %q, want generic masked-value label", view)
	}
	if strings.Contains(view, "Masked connection string") {
		t.Fatalf("View() = %q, must not contain ASP.NET-specific masked-value label", view)
	}
	if !strings.Contains(view, "Password=****") {
		t.Fatalf("View() = %q, want masked connection string", view)
	}
	if strings.Contains(view, "super-secret") {
		t.Fatalf("View() = %q, must not contain unmasked secret", view)
	}
	if !strings.Contains(view, "~ Test") {
		t.Fatalf("View() = %q, want inactive selected profile context", view)
	}

	updatedModel, command = model.Update(runeKey('i'))
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no command when returning from inspection")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}
	if !strings.Contains(model.View(), "* Profiles") {
		t.Fatalf("View() = %q, want profile list view", model.View())
	}

	updatedModel, command = model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when reopening inspection")
	}
	updatedModel, command = model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want q to quit from inspection")
	}
}

func TestUpdate_ValueRevealTogglesInInspectionAndIsIgnoredInConfirmationAndError(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;Password=super-secret;"), Protected: true}},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	if model.state != inspectState {
		t.Fatalf("state = %d, want inspectState", model.state)
	}
	if !strings.Contains(model.View(), "v Reveal values") {
		t.Fatalf("inspection View() = %q, want reveal command", model.View())
	}

	updatedModel, command = model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want inspection reveal toggle to stay local")
	}
	if !model.valuesVisible || model.state != inspectState {
		t.Fatalf("model after inspection reveal = %#v, want inspection with values visible", model)
	}
	if !strings.Contains(model.View(), "v Hide values") {
		t.Fatalf("inspection View() = %q, want hide command", model.View())
	}

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want protected profile confirmation before apply")
	}
	if model.state != confirmState {
		t.Fatalf("state = %d, want confirmState", model.state)
	}
	confirmationView := model.View()
	if strings.Contains(confirmationView, "v Reveal values") || strings.Contains(confirmationView, "v Hide values") {
		t.Fatalf("confirmation View() = %q, must not advertise value reveal", confirmationView)
	}

	updatedModel, command = model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want v ignored in confirmation")
	}
	if !model.valuesVisible || model.state != confirmState {
		t.Fatalf("model after confirmation v = %#v, want reveal state and confirmation unchanged", model)
	}

	model.state = errorState
	model.recoverableError = RecoverableError{Problem: "Action could not continue."}
	errorView := model.View()
	if strings.Contains(errorView, "v Reveal values") || strings.Contains(errorView, "v Hide values") {
		t.Fatalf("error View() = %q, must not advertise value reveal", errorView)
	}

	updatedModel, command = model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want v ignored in recoverable error")
	}
	if !model.valuesVisible || model.state != errorState || model.recoverableError.IsZero() {
		t.Fatalf("model after error v = %#v, want error state unchanged", model)
	}
}

func TestView_InspectionAddsDetailsNotPresentInSelectedPanel(t *testing.T) {
	t.Setenv("PRODUCTION_DATABASE_URL", "Server=prod;Database=App;Password=super-secret;")

	tests := []struct {
		name            string
		profile         config.Profile
		selectedMissing []string
		inspectDetails  []string
	}{
		{
			name:            "literal value",
			profile:         config.Profile{Name: "Local", Value: stringPointer("Server=local;Database=App;Password=literal-secret;")},
			selectedMissing: []string{"Masked value:", "Password=****", "literal-secret"},
			inspectDetails:  []string{"Source: Literal value", "Masked value:", "Password=****"},
		},
		{
			name:            "environment value",
			profile:         config.Profile{Name: "Production", ValueFromEnv: stringPointer("PRODUCTION_DATABASE_URL"), Protected: true},
			selectedMissing: []string{"Environment variable: PRODUCTION_DATABASE_URL", "Masked value:", "super-secret"},
			inspectDetails:  []string{"Source: Environment variable", "Environment variable: PRODUCTION_DATABASE_URL", "Masked value:", "Password=****"},
		},
		{
			name:            "unavailable value",
			profile:         config.Profile{Name: "QA", ValueFromEnv: stringPointer("MISSING_DATABASE_URL")},
			selectedMissing: []string{"Resolution error:", "Masked value:"},
			inspectDetails:  []string{"Masked value: Unavailable", "Resolution error:", "MISSING_DATABASE_URL"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			model := New(app.New(
				config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
				[]config.Profile{testCase.profile},
			))

			selectedView := model.View()
			for _, forbidden := range testCase.selectedMissing {
				if strings.Contains(selectedView, forbidden) {
					t.Fatalf("selected View() = %q, must not include inspection-only detail %q", selectedView, forbidden)
				}
			}

			updatedModel, command := model.Update(runeKey('i'))
			model = updatedModel.(Model)
			if command != nil {
				t.Fatal("command is not nil, want no command when opening inspection")
			}

			inspectView := model.View()
			for _, expected := range testCase.inspectDetails {
				if !strings.Contains(inspectView, expected) {
					t.Fatalf("inspection View() = %q, want unique inspection detail %q", inspectView, expected)
				}
			}
		})
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
	if !strings.Contains(view, "Masked value:") || !strings.Contains(view, "Unavailable") {
		t.Fatalf("View() = %q, want unavailable masked value message", view)
	}
	if !strings.Contains(view, "Resolution error:") {
		t.Fatalf("View() = %q, want resolution error heading", view)
	}
	if !strings.Contains(view, "MISSING_CONNECTION_STRING") {
		t.Fatalf("View() = %q, want unavailable reason", view)
	}
}

func TestUpdate_InspectionScrollsLongProfileDetailWithoutMovingProfileCursor(t *testing.T) {
	targets := make([]config.Target, 0, 14)
	values := make([]config.ProfileValue, 0, 14)
	for index := 1; index <= 14; index++ {
		name := fmt.Sprintf("target%02d", index)
		targets = append(targets, config.Target{Name: name, File: fmt.Sprintf("config/%02d.json", index), Type: config.TargetTypeJSON, JSONPath: "service.url"})
		values = append(values, config.ProfileValue{Target: name, Value: stringPointer(fmt.Sprintf("value-%02d", index))})
	}
	model := New(app.NewWithTargets(targets, []config.Profile{{Name: "Large", Values: values}}))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	model = updatedModel.(Model)
	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	originalCursor := model.cursor
	if !strings.Contains(model.View(), "PgUp/PgDn Scroll") {
		t.Fatalf("View() = %q, want scroll command for long profile detail", model.View())
	}

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after profile detail scroll")
	}
	if model.cursor != originalCursor {
		t.Fatalf("cursor after profile detail scroll = %d, want %d", model.cursor, originalCursor)
	}
	if model.scrollOffset == 0 {
		t.Fatalf("scrollOffset after PgDn = %d, want scrolled detail panel", model.scrollOffset)
	}
	if !strings.Contains(model.View(), "earlier") {
		t.Fatalf("View() = %q, want hidden-before overflow indicator after scrolling", model.View())
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyHome})
	model = updatedModel.(Model)
	if model.scrollOffset != 0 {
		t.Fatalf("scrollOffset after Home = %d, want top", model.scrollOffset)
	}
}

func TestUpdate_ProtectedProfileRequiresConfirmationAndCancels(t *testing.T) {
	target := config.Target{File: "/tmp/config.json", JSONPath: "service.baseUrl"}
	model := New(app.New(
		target,
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
	if !strings.Contains(view, "Confirmation") {
		t.Fatalf("View() = %q, want confirmation title", view)
	}
	if !strings.Contains(view, "Profile: Production") {
		t.Fatalf("View() = %q, want protected profile name", view)
	}
	if !strings.Contains(view, "File") || !strings.Contains(view, "/tmp/config.json") {
		t.Fatalf("View() = %q, want target file", view)
	}
	if !strings.Contains(view, "Selector") || !strings.Contains(view, "service.baseUrl") {
		t.Fatalf("View() = %q, want target JSON path", view)
	}
	if !strings.Contains(view, "configured target value") {
		t.Fatalf("View() = %q, want generic confirmation text", view)
	}
	if !strings.Contains(view, "Press Enter or y to confirm.") {
		t.Fatalf("View() = %q, want explicit Enter confirmation guidance", view)
	}
	if !strings.Contains(view, "Enter/y Confirm  n/Esc Cancel  q Quit") {
		t.Fatalf("View() = %q, want confirmation help that documents Enter", view)
	}
	if strings.Contains(view, "configured connection string") {
		t.Fatalf("View() = %q, must not contain ASP.NET-specific confirmation text", view)
	}
	if strings.Contains(view, "super-secret") {
		t.Fatalf("View() = %q, must not contain unmasked secret", view)
	}
	if strings.Contains(view, "Password=****") {
		t.Fatalf("View() = %q, must not contain masked connection string in confirmation", view)
	}
	if !strings.Contains(view, "~ Production") || strings.Contains(view, "~ Production [protected]") {
		t.Fatalf("View() = %q, want inactive selected profile context", view)
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
	if !strings.Contains(model.recoverableError.Reason, "MISSING_CONNECTION_STRING") {
		t.Fatalf("recoverableError.Reason = %q, want unavailable reason", model.recoverableError.Reason)
	}
}

func TestView_InspectionUsesApplyAndReturnHelpForProtectedProfiles(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
	))

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	if model.state != inspectState {
		t.Fatalf("state = %d, want inspectState", model.state)
	}
	if !strings.Contains(model.View(), "Enter Apply+Exit") || !strings.Contains(model.View(), "Space Apply") || !strings.Contains(model.View(), "i Return") || !strings.Contains(model.View(), "Esc Return") || !strings.Contains(model.View(), "q Quit") {
		t.Fatalf("View() = %q, want protected inspection help text that matches Enter behavior", model.View())
	}
}

func TestView_InspectionAndConfirmationStayWithinTerminalWidth(t *testing.T) {
	t.Setenv("LONG_PROTECTED_PROFILE_VALUE", "Server=prod.example.test;Database=Application;Password=super-secret;")

	model := New(app.New(
		config.Target{
			File:     "/very/long/project/path/with/many/segments/configuration/appsettings.Development.json",
			JSONPath: "services.database.primary.connectionStrings.defaultConnection.value",
		},
		[]config.Profile{{
			Name:         "Production profile with a long display name",
			ValueFromEnv: stringPointer("LONG_PROTECTED_PROFILE_VALUE"),
			Protected:    true,
		}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	assertVisibleWidth(t, model.View(), 80)

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want confirmation before apply")
	}
	if model.state != confirmState {
		t.Fatalf("state = %d, want confirmState", model.state)
	}
	assertVisibleWidth(t, model.View(), 80)
}
