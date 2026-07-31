package configeditor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

func TestModel_RendersOverviewProfilesManagedValuesAndReview(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)

	view := model.View()
	for _, expected := range []string{"Switchlet config", "Profiles", "Local", "Managed values", "database", "frontendApi", "Review changes"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view does not contain %q\n%s", expected, view)
		}
	}
	if strings.Contains(view, "postgres://local-secret") {
		t.Fatalf("view leaked literal profile value\n%s", view)
	}
}

func TestModel_VimNavigationMovesAndJumps(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)

	model, _ = pressRune(t, model, 'j')
	if selected := selectedLabel(model); selected != "Local" {
		t.Fatalf("selected row = %q, want Local", selected)
	}

	model, _ = pressRune(t, model, 'G')
	if selected := selectedLabel(model); selected != "Review changes" {
		t.Fatalf("selected row = %q, want Review changes", selected)
	}

	model, _ = pressRune(t, model, 'g')
	if selected := selectedLabel(model); selected != "Profiles" {
		t.Fatalf("selected row = %q, want Profiles", selected)
	}
}

func TestModel_FilterTreatsQAsLiteralInput(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)

	model, _ = pressRune(t, model, '/')
	if model.state != editorStateFilter {
		t.Fatalf("state = %v, want filter", model.state)
	}

	model, _ = pressRune(t, model, 'q')
	if model.state != editorStateFilter {
		t.Fatalf("state = %v, want filter after q input", model.state)
	}
	if model.inputValue != "q" {
		t.Fatalf("inputValue = %q, want q", model.inputValue)
	}
	if _, ok := model.Result(); ok {
		t.Fatal("Result returned ok=true, want editor to keep running")
	}
}

func TestModel_DirtyQuitConfirmationProtectsPendingChanges(t *testing.T) {
	model, _ := newTestModel(t, versionTwoConfig())
	model = resizeModel(t, model, 120, 32)

	model, _ = pressRune(t, model, 'q')
	if model.state != editorStateDirtyQuitConfirm {
		t.Fatalf("state = %v, want dirty quit confirmation", model.state)
	}
	if _, ok := model.Result(); ok {
		t.Fatal("Result returned ok=true before dirty quit confirmation")
	}

	model, _ = pressKey(t, model, tea.KeyEsc)
	if model.state != editorStateOverview {
		t.Fatalf("state = %v, want overview after Esc", model.state)
	}

	model, _ = pressRune(t, model, 'q')
	model, _ = pressKey(t, model, tea.KeyEnter)
	result, ok := model.Result()
	if !ok {
		t.Fatal("Result returned ok=false after confirmed discard")
	}
	if !result.Cancelled || result.Saved {
		t.Fatalf("result = %#v, want cancelled discard", result)
	}
}

func TestModel_ReviewSaveUnavailableForUnchangedVersionThreeDraft(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)
	model, _ = pressRune(t, model, 'G')

	view := model.View()
	if !strings.Contains(view, "No pending changes.") {
		t.Fatalf("view does not contain clean review state\n%s", view)
	}
	if strings.Contains(view, "s Save") {
		t.Fatalf("view advertises save for unchanged draft\n%s", view)
	}
}

func TestModel_SavePipelineConvertsCompatibilityConfigAfterExplicitReview(t *testing.T) {
	model, configPath := newTestModel(t, versionTwoConfig())
	model = resizeModel(t, model, 120, 32)
	model, _ = pressRune(t, model, 'G')

	view := model.View()
	if !strings.Contains(view, "version 3") || !strings.Contains(view, "s Save") {
		t.Fatalf("review view does not expose conversion and save action\n%s", view)
	}

	var cmd tea.Cmd
	model, cmd = pressRune(t, model, 's')
	if model.state != editorStateSaving {
		t.Fatalf("state = %v, want saving", model.state)
	}
	if cmd == nil {
		t.Fatal("save command is nil")
	}

	message := cmd()
	updatedModel, _ := model.Update(message)
	model = updatedModel.(Model)
	if model.state != editorStateSaveSuccess {
		t.Fatalf("state = %v, want save success", model.state)
	}

	contents := string(readTestFile(t, configPath))
	if !strings.Contains(contents, "version: 3") || !strings.Contains(contents, "targets:") {
		t.Fatalf("configuration was not converted to version 3\n%s", contents)
	}

	model, _ = pressRune(t, model, 'q')
	result, ok := model.Result()
	if !ok {
		t.Fatal("Result returned ok=false after save success exit")
	}
	if !result.Saved || result.ConfigPath != configPath {
		t.Fatalf("result = %#v, want saved config path %q", result, configPath)
	}
}

func TestModel_AddProfileFromExistingManagedValuesAndSave(t *testing.T) {
	model, configPath := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)

	model, _ = pressRune(t, model, 'a')
	if model.state != editorStateProfileNameInput {
		t.Fatalf("state = %v, want profile name input", model.state)
	}
	model = enterText(t, model, "QA")
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileIncludeValues {
		t.Fatalf("state = %v, want include-values state", model.state)
	}

	model, _ = pressKey(t, model, tea.KeySpace)
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileValueSource {
		t.Fatalf("state = %v, want value-source state", model.state)
	}
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileValueInput {
		t.Fatalf("state = %v, want value-input state", model.state)
	}
	model = enterText(t, model, "postgres://qa-secret")
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileReview {
		t.Fatalf("state = %v, want profile review", model.state)
	}

	view := model.View()
	if !strings.Contains(view, "QA") || strings.Contains(view, "postgres://qa-secret") {
		t.Fatalf("profile review should show profile name but hide literal value\n%s", view)
	}

	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateOverview {
		t.Fatalf("state = %v, want overview after profile draft save", model.state)
	}
	if !strings.Contains(model.View(), "QA") {
		t.Fatalf("overview does not contain added profile\n%s", model.View())
	}

	model, _ = pressRune(t, model, 'G')
	var cmd tea.Cmd
	model, cmd = pressRune(t, model, 's')
	if model.state != editorStateSaving || cmd == nil {
		t.Fatalf("state = %v cmd nil=%t, want saving command", model.state, cmd == nil)
	}
	updatedModel, _ := model.Update(cmd())
	model = updatedModel.(Model)
	if model.state != editorStateSaveSuccess {
		t.Fatalf("state = %v, want save success", model.state)
	}
	if !strings.Contains(string(readTestFile(t, configPath)), "name: QA") {
		t.Fatalf("saved configuration does not contain added profile\n%s", string(readTestFile(t, configPath)))
	}
}

func TestModel_EditProfileRenameProtectedAndEnvironmentValue(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)
	model = selectNavigationLabel(t, model, "Local")

	model, _ = pressRune(t, model, 'r')
	if model.state != editorStateProfileNameInput {
		t.Fatalf("state = %v, want name input", model.state)
	}
	model, _ = pressKey(t, model, tea.KeyCtrlU)
	model = enterText(t, model, "Development")
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileReview {
		t.Fatalf("state = %v, want profile review after rename", model.state)
	}
	model, _ = pressKey(t, model, tea.KeySpace)
	model, _ = pressKey(t, model, tea.KeyEnter)

	model = selectNavigationLabel(t, model, "Development")
	model, _ = pressRune(t, model, 'e')
	if model.state != editorStateProfileIncludeValues {
		t.Fatalf("state = %v, want include-values state", model.state)
	}
	model, _ = pressRune(t, model, 'j')
	model, _ = pressKey(t, model, tea.KeySpace)
	model, _ = pressRune(t, model, 'k')
	model, _ = pressRune(t, model, 'e')
	model, _ = pressRune(t, model, 'j')
	model, _ = pressKey(t, model, tea.KeyEnter)
	model = enterText(t, model, "DEV_DATABASE_URL")
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileReview {
		t.Fatalf("state = %v, want profile review", model.state)
	}
	model, _ = pressKey(t, model, tea.KeyEnter)

	overview := model.overview()
	var edited app.ConfigEditProfileItem
	for _, profile := range overview.Profiles {
		if profile.Name == "Development" {
			edited = profile
		}
	}
	if edited.Name == "" {
		t.Fatalf("overview profiles = %#v, want Development", overview.Profiles)
	}
	if !edited.Protected || edited.ValueCount != 1 || !edited.Partial {
		t.Fatalf("edited profile = %#v, want protected partial one-value profile", edited)
	}
	if edited.Values[0].Source != app.ProfileSourceEnvironment || edited.Values[0].EnvironmentVariableName != "DEV_DATABASE_URL" {
		t.Fatalf("edited values = %#v, want environment-backed database value", edited.Values)
	}
}

func TestModel_RemoveProfileUpdatesReviewSummary(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)
	model = selectNavigationLabel(t, model, "Frontend Only")

	model, _ = pressRune(t, model, 'd')
	if model.state != editorStateProfileRemoveConfirm {
		t.Fatalf("state = %v, want remove confirm", model.state)
	}
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateOverview {
		t.Fatalf("state = %v, want overview after removal", model.state)
	}

	view := model.View()
	if !strings.Contains(view, "Removed profile \"Frontend Only\"") {
		t.Fatalf("review summary does not contain removed profile\n%s", view)
	}
}

func TestModel_ProfileTextEntryTreatsQAsLiteralInput(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 120, 32)

	model, _ = pressRune(t, model, 'a')
	model, _ = pressRune(t, model, 'q')
	if model.state != editorStateProfileNameInput {
		t.Fatalf("state = %v, want profile name input", model.state)
	}
	if model.inputValue != "q" {
		t.Fatalf("inputValue = %q, want q", model.inputValue)
	}
	if _, ok := model.Result(); ok {
		t.Fatal("Result returned ok=true, want editor to keep running")
	}
}

func TestModel_TooSmallTerminalRendersResizeState(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 40, 10)

	view := model.View()
	if !strings.Contains(view, "Resize required") || !strings.Contains(view, "Minimum size") {
		t.Fatalf("view does not contain too-small state\n%s", view)
	}
}

func newTestModel(t *testing.T, configContents string) (Model, string) {
	t.Helper()

	projectRoot := t.TempDir()
	writeTestFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	writeTestFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	writeTestFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	configPath := writeTestFile(t, projectRoot, ".switchlet.yaml", configContents)

	workflow := app.DefaultConfigEditWorkflow()
	document, err := workflow.LoadDocument(configPath)
	if err != nil {
		t.Fatalf("LoadDocument returned error: %v", err)
	}

	return NewModel(document, workflow), configPath
}

func versionThreeConfig() string {
	return strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: backend/appsettings.Development.json
    type: json
    jsonPath: database.url
  - name: frontendApi
    file: frontend/.env.local
    type: dotenv
    key: VITE_API_URL

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local-secret
      - target: frontendApi
        value: http://localhost:5173
  - name: Frontend Only
    values:
      - target: frontendApi
        valueFromEnv: FRONTEND_API_URL
`) + "\n"
}

func versionTwoConfig() string {
	return strings.TrimSpace(`
version: 2

target:
  file: config.json
  jsonPath: service.baseUrl

profiles:
  - name: Local
    value: https://local.example.test
`) + "\n"
}

func resizeModel(t *testing.T, model Model, width int, height int) Model {
	t.Helper()

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updatedModel.(Model)
}

func pressRune(t *testing.T, model Model, key rune) (Model, tea.Cmd) {
	t.Helper()

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
	return updatedModel.(Model), cmd
}

func pressKey(t *testing.T, model Model, keyType tea.KeyType) (Model, tea.Cmd) {
	t.Helper()

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: keyType})
	return updatedModel.(Model), cmd
}

func enterText(t *testing.T, model Model, text string) Model {
	t.Helper()

	for _, key := range text {
		var cmd tea.Cmd
		model, cmd = pressRune(t, model, key)
		if cmd != nil {
			t.Fatalf("unexpected command while entering %q", text)
		}
	}

	return model
}

func selectNavigationLabel(t *testing.T, model Model, label string) Model {
	t.Helper()

	rows := model.navigationRows(model.overview())
	for index, row := range rows {
		if row.Label == label {
			model.cursor = index
			return model
		}
	}
	t.Fatalf("navigation label %q not found in %#v", label, rows)
	return model
}

func selectedLabel(model Model) string {
	rows := model.navigationRows(model.overview())
	return model.selectedRow(rows).Label
}

func writeTestFile(t *testing.T, rootDir string, relativePath string, contents string) string {
	t.Helper()

	path := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}

	return path
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	return contents
}
