package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestNew_InitializesProfilesAndSelection(t *testing.T) {
	application := app.New().WithConfig(
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
	model := New(app.New().WithConfig(
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
	model := New(app.New().WithConfig(
		config.Target{},
		[]config.Profile{
			{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING")},
		},
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

func TestUpdate_OpensInspectionAndReturnsToList(t *testing.T) {
	t.Setenv("MYAPPLICATION_TEST_CONNECTION_STRING", "Server=test;Database=App;Password=super-secret;")

	model := New(app.New().WithConfig(
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
	model := New(app.New().WithConfig(
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
	model := New(app.New().WithConfig(
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

func TestUpdate_ProtectedProfileConfirmationAppliesSelectedProfile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	model := New(app.New().WithConfig(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=NewDatabase;"), Protected: true}},
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
	if !strings.Contains(string(updatedContents), "NewDatabase") {
		t.Fatalf("updated target = %q, want applied protected profile", string(updatedContents))
	}
}

func TestUpdate_ProtectedUnavailableProfileShowsRecoverableError(t *testing.T) {
	model := New(app.New().WithConfig(
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

func TestUpdate_AppliesSelectedProfileSuccessfully(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	model := New(app.New().WithConfig(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=NewDatabase;")}},
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

	updatedContents := readFile(t, targetPath)
	if !strings.Contains(string(updatedContents), "NewDatabase") {
		t.Fatalf("updated target = %q, want applied connection string", string(updatedContents))
	}
}

func TestUpdate_ShowsRecoverableApplicationError(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", `{`)

	model := New(app.New().WithConfig(
		config.Target{File: targetPath, ConnectionName: "DefaultConnection"},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=NewDatabase;")}},
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

func TestUpdate_QuitsImmediately(t *testing.T) {
	model := New(app.New().WithConfig(
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
		{
			name:      "inspection",
			openKey:   runeKey('i'),
			wantState: inspectState,
		},
		{
			name:      "confirmation",
			openKey:   tea.KeyMsg{Type: tea.KeyEnter},
			wantState: confirmState,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			model := New(app.New().WithConfig(
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

func writeTargetFile(t *testing.T, rootDir string, relativePath string, contents string) string {
	t.Helper()

	fullPath := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create parent directories for %q: %v", fullPath, err)
	}

	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write target file %q: %v", fullPath, err)
	}

	return fullPath
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}

func stringPointer(value string) *string {
	return &value
}

func runeKey(value rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}}
}
