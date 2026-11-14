package configexport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/jeppeklh/switchlet/internal/app"
)

func testModel(t *testing.T) Model {
	t.Helper()
	root := t.TempDir()
	contents := `# comment-secret-sentinel
version: 3
targets:
  - name: url
    file: settings.json
    type: json
    jsonPath: url
profiles:
  - name: Local
    protected: true
    values:
      - target: url
        value: literal-secret-sentinel
  - name: Staging
    values:
      - target: url
        valueFromEnv: EXISTING_URL
`
	path := filepath.Join(root, ".switchlet.yaml")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	workflow := app.DefaultConfigEditWorkflow()
	document, err := workflow.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	model := New(document, workflow)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(Model)
}
func key(model Model, key string) (Model, tea.Cmd) {
	var message tea.KeyMsg
	switch key {
	case "enter":
		message.Type = tea.KeyEnter
	case "esc":
		message.Type = tea.KeyEsc
	case "space":
		message.Type = tea.KeySpace
	case "ctrl+c":
		message.Type = tea.KeyCtrlC
	case "end":
		message.Type = tea.KeyEnd
	default:
		message = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, cmd := model.Update(message)
	return updated.(Model), cmd
}
func execute(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("missing command")
	}
	updated, _ := model.Update(cmd())
	return updated.(Model)
}
func nameEntry(model Model, name string) Model {
	model, _ = key(model, "e")
	model, _ = key(model, name)
	model, _ = key(model, "enter")
	return model
}

func TestExport_RequiresExplicitSharedReviewAndKeepsLocalConfig(t *testing.T) {
	model := testModel(t)
	original, err := os.ReadFile(model.document.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	model = nameEntry(model, "EXISTING_URL")
	model, cmd := key(model, "enter")
	model = execute(t, model, cmd)
	if model.state != review || len(model.prepared.SharedVariables) != 1 {
		t.Fatal("missing shared review")
	}
	if strings.Contains(model.View(), "secret-sentinel") {
		t.Fatal("review leaked source")
	}
	model, cmd = key(model, "enter")
	if cmd != nil || model.err == "" {
		t.Fatal("implicit shared source accepted")
	}
	model, _ = key(model, "space")
	model, cmd = key(model, "enter")
	if model.state != writing {
		t.Fatal("commit did not use an effect")
	}
	model = execute(t, model, cmd)
	if model.state != completed {
		t.Fatalf("export failed: %s", model.err)
	}
	contents, err := os.ReadFile(model.prepared.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "secret-sentinel") || strings.Count(string(contents), "valueFromEnv: EXISTING_URL") != 2 {
		t.Fatal("unsafe export")
	}
	current, err := os.ReadFile(model.document.ConfigPath)
	if err != nil || string(current) != string(original) {
		t.Fatal("export changed active config")
	}
	model, _ = key(model, "enter")
	if _, done := model.Result(); !done {
		t.Fatal("success did not return to editor")
	}
}

func TestExport_CancelLateResultsAndExplicitOverwrite(t *testing.T) {
	model := testModel(t)
	model = nameEntry(model, "LOCAL_URL")
	destination := filepath.Join(model.document.ProjectRoot, ".switchlet.example.yaml")
	if err := os.WriteFile(destination, []byte("existing-example-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	model, cmd := key(model, "enter")
	message := cmd()
	model, _ = key(model, "esc")
	updated, _ := model.Update(message)
	model = updated.(Model)
	if model.state != editing {
		t.Fatal("cancelled preparation changed state")
	}
	model, cmd = key(model, "enter")
	model = execute(t, model, cmd)
	if !model.prepared.ReplacesExisting || !strings.Contains(model.View(), "Replace existing") && !strings.Contains(model.View(), "Replace the existing") {
		t.Fatal("replacement not reviewed")
	}
	if strings.Contains(model.View(), "existing-example-secret") {
		t.Fatal("existing contents leaked")
	}
	model, _ = key(model, "esc")
	contents, _ := os.ReadFile(destination)
	if string(contents) != "existing-example-secret" {
		t.Fatal("cancel wrote file")
	}
	model, cmd = key(model, "enter")
	model = execute(t, model, cmd)
	if err := os.WriteFile(destination, []byte("external-edit"), 0600); err != nil {
		t.Fatal(err)
	}
	model, cmd = key(model, "enter")
	model = execute(t, model, cmd)
	if model.state != failed || !strings.Contains(model.err, "changed since review") {
		t.Fatal("stale export was not refused")
	}
	contents, _ = os.ReadFile(destination)
	if string(contents) != "external-edit" {
		t.Fatal("stale export overwrote external edit")
	}
	model, _ = key(model, "enter")
	model, cmd = key(model, "enter")
	model = execute(t, model, cmd)
	model, cmd = key(model, "enter")
	model = execute(t, model, cmd)
	if model.state != completed {
		t.Fatal("fresh reviewed replacement failed")
	}
}

func TestExport_CancelAndReopenedSessionRejectsOldPreparation(t *testing.T) {
	model := testModel(t)
	model = nameEntry(model, "LOCAL_URL")
	model, cmd := key(model, "enter")
	old := cmd()
	model, _ = key(model, "q")
	updated, _ := model.Update(old)
	model = updated.(Model)
	if _, done := model.Result(); !done {
		t.Fatal("cancelled export reopened")
	}
	if _, err := os.Stat(filepath.Join(model.document.ProjectRoot, ".switchlet.example.yaml")); !os.IsNotExist(err) {
		t.Fatal("cancel created output")
	}
	fresh := New(model.document, model.workflow)
	fresh = nameEntry(fresh, "LOCAL_URL")
	fresh, cmd = key(fresh, "enter")
	updated, _ = fresh.Update(old)
	fresh = updated.(Model)
	if fresh.state != preparing {
		t.Fatal("previous session result accepted")
	}
	fresh = execute(t, fresh, cmd)
	if fresh.state != review {
		t.Fatal("current result rejected")
	}
}

func TestExport_NameInputHelpResizeAndReviewScrolling(t *testing.T) {
	model := testModel(t)
	model, _ = key(model, "e")
	model, _ = key(model, "q")
	if string(model.input) != "q" {
		t.Fatal("q did not enter text")
	}
	model, _ = key(model, "esc")
	model = nameEntry(model, "BAD-NAME")
	if model.state != nameInput || model.err == "" {
		t.Fatal("invalid name accepted")
	}
	model, _ = key(model, "esc")
	model = nameEntry(model, "LOCAL_URL")
	model, _ = key(model, "j")
	model, _ = key(model, "e")
	if model.state != editing {
		t.Fatal("existing reference was editable")
	}
	model, _ = key(model, "?")
	if !strings.Contains(model.View(), "Review export") {
		t.Fatal("help missing")
	}
	model, _ = key(model, "esc")
	model, cmd := key(model, "enter")
	model = execute(t, model, cmd)
	model.draft.Entries[0].TargetFile = strings.Repeat("long-directory/", 45) + "settings.json"
	model, _ = key(model, "end")
	if model.scroll == 0 {
		t.Fatal("long review not scrollable")
	}
	for _, size := range [][2]int{{80, 24}, {120, 32}} {
		updated, _ := model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		model = updated.(Model)
		lines := strings.Split(strings.TrimSuffix(ansi.Strip(model.View()), "\n"), "\n")
		if len(lines) > size[1] {
			t.Fatal("screen exceeds height")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("screen exceeds width")
			}
		}
		if strings.Contains(model.View(), "secret-sentinel") {
			t.Fatal("screen leaked values")
		}
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	model = updated.(Model)
	if !strings.Contains(model.View(), "Resize required") {
		t.Fatal("missing resize guard")
	}
	model, cmd = key(model, "enter")
	if cmd != nil {
		t.Fatal("small terminal committed export")
	}
	model, _ = key(model, "ctrl+c")
	if result, done := model.Result(); !done || !result.Quit {
		t.Fatal("Ctrl+C did not request exit")
	}
}
