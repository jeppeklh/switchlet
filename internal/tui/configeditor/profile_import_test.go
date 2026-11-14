package configeditor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/jeppeklh/switchlet/internal/app"
)

func beginTestImport(t *testing.T) (Model, string) {
	t.Helper()
	model, path := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 80, 24)
	model = selectNavigationLabel(t, model, "Frontend Only")
	model, _ = pressRune(t, model, 'I')
	if model.state != editorStateProfileNameInput {
		t.Fatal("import did not ask for name")
	}
	model = enterText(t, model, "Imported")
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileImportMapping {
		t.Fatal("import did not ask for mapping")
	}
	root := filepath.Dir(path)
	writeTestFile(t, root, "source.json", `{"database":{"url":"private-import-value"}}`)
	model, _ = pressRune(t, model, 'e')
	model = enterText(t, model, "source.json")
	model, _ = pressKey(t, model, tea.KeyEnter)
	return model, path
}

func TestProfileImport_ReviewedPartialDraftUsesExistingSave(t *testing.T) {
	model, path := beginTestImport(t)
	root := filepath.Dir(path)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cmd tea.Cmd
	model, cmd = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileImportLoading || cmd == nil {
		t.Fatal("read must be an async effect")
	}
	model = runCommand(t, model, cmd)
	if model.state != editorStateProfileImportSelection {
		t.Fatal("missing import selection")
	}
	if strings.Contains(model.View(), "private-import-value") {
		t.Fatal("selection leaked source")
	}
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileImportSelection || model.profileForm.errorMessage == "" {
		t.Fatal("empty selection accepted")
	}
	model, _ = pressKey(t, model, tea.KeySpace)
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileReview {
		t.Fatal("missing profile review")
	}
	model, _ = pressRune(t, model, 'p')
	model, _ = pressKey(t, model, tea.KeyEsc)
	model, _ = pressKey(t, model, tea.KeyEnter)
	if !model.profileForm.draft.Protected {
		t.Fatal("explicit protection lost on review return")
	}
	view := model.View()
	if strings.Contains(view, "private-import-value") {
		t.Fatal("review leaked source")
	}
	// Modify the source after review; the pending draft must retain the snapshot.
	writeTestFile(t, root, "source.json", `{"database":{"url":"later-private-value"}}`)
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateOverview {
		t.Fatal("draft not added")
	}
	current, err := os.ReadFile(path)
	if err != nil || string(current) != string(original) {
		t.Fatal("adding draft persisted early")
	}
	draft, err := model.workflow.ProfileDraft(model.document, "Imported")
	if err != nil {
		t.Fatal(err)
	}
	if draft.Values[0].LiteralValue != "private-import-value" || draft.Values[1].EnvironmentVariableName != "FRONTEND_API_URL" || !draft.Protected {
		t.Fatalf("unexpected draft: %#v", draft)
	}
	if draft.Values[1].Source != app.ProfileSourceEnvironment {
		t.Fatal("base source lost")
	}
	model.selectOverviewTab(overviewTabReview)
	model, cmd = pressRune(t, model, 's')
	model = runCommand(t, model, cmd)
	if model.state != editorStateSaveSuccess {
		t.Fatalf("save failed: %s", model.View())
	}
	saved, err := model.workflow.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	savedDraft, err := model.workflow.ProfileDraft(saved, "Imported")
	if err != nil || savedDraft.Values[0].LiteralValue != "private-import-value" {
		t.Fatal("snapshot not saved")
	}
	for file, want := range map[string]string{
		"source.json":                          `{"database":{"url":"later-private-value"}}`,
		"backend/appsettings.Development.json": `{"database":{"url":"postgres://old"}}`,
		"frontend/.env.local":                  "VITE_API_URL=http://localhost:5173\n",
	} {
		contents, err := os.ReadFile(filepath.Join(root, file))
		if err != nil || string(contents) != want {
			t.Fatalf("import changed %s", file)
		}
	}
}

func TestProfileImport_RefreshMappingsCancelAndLateResults(t *testing.T) {
	model, path := beginTestImport(t)
	var first, second tea.Cmd
	model, first = pressKey(t, model, tea.KeyEnter)
	oldResult := first()
	model = runCommand(t, model, func() tea.Msg { return oldResult })
	model, _ = pressKey(t, model, tea.KeySpace)
	writeTestFile(t, filepath.Dir(path), "source.json", `{"database":{"url":"refreshed-private-value"}}`)
	model, second = pressRune(t, model, 'r')
	if model.profileForm.importForm.selected != nil || len(model.profileForm.importForm.prepared.Values) != 0 {
		t.Fatal("refresh retained prior review")
	}
	model = runCommand(t, model, func() tea.Msg { return oldResult })
	if model.state != editorStateProfileImportLoading {
		t.Fatal("old read replaced refresh")
	}
	model = runCommand(t, model, second)
	refreshed, err := model.workflow.ProfileDraftFromImport(model.document, model.profileForm.importForm.prepared, "Refreshed", []string{"database"})
	if err != nil || refreshed.Values[0].LiteralValue != "refreshed-private-value" {
		t.Fatal("refresh did not use current source")
	}
	if len(model.profileForm.importForm.selected) != 0 {
		t.Fatal("refresh retained selections")
	}
	model, _ = pressKey(t, model, tea.KeySpace)
	model, _ = pressRune(t, model, 'm')
	if len(model.profileForm.importForm.prepared.Values) != 0 || model.profileForm.importForm.selected != nil {
		t.Fatal("mapping change retained review")
	}
	model, _ = pressRune(t, model, 'e')
	model, _ = pressKey(t, model, tea.KeyCtrlU)
	model = enterText(t, model, "missing.json")
	model, _ = pressKey(t, model, tea.KeyEnter)
	model, second = pressKey(t, model, tea.KeyEnter)
	model = runCommand(t, model, second)
	row := model.profileForm.importForm.prepared.Values[0]
	if row.Available || row.Error == "" {
		t.Fatal("missing source was selectable")
	}
	model, _ = pressKey(t, model, tea.KeySpace)
	if model.profileForm.importForm.selected[row.TargetName] {
		t.Fatal("selected failed read")
	}
	model, _ = pressKey(t, model, tea.KeyEsc)
	model, second = pressKey(t, model, tea.KeyEnter)
	model, _ = pressKey(t, model, tea.KeyEsc)
	model, _ = pressKey(t, model, tea.KeyEsc)
	if model.state != editorStateOverview {
		t.Fatal("cancel did not return to overview")
	}
	model = runCommand(t, model, second)
	if model.state != editorStateOverview || model.overview().Dirty {
		t.Fatal("cancelled result changed document")
	}
	fresh := NewModel(model.document, model.workflow)
	fresh = resizeModel(t, fresh, 80, 24)
	fresh.beginImportProfile("Local")
	fresh.profileForm.importForm.mappings[0].SourceFile = "source.json"
	newRead := fresh.readProfileImport()
	fresh = runCommand(t, fresh, first)
	if fresh.state != editorStateProfileImportLoading {
		t.Fatal("previous editor result was accepted")
	}
	fresh, _ = pressKey(t, fresh, tea.KeyCtrlC)
	fresh = runCommand(t, fresh, newRead)
	if fresh.profileForm.importForm.prepared.Values != nil {
		t.Fatal("closed editor accepted result")
	}
	saved, err := model.workflow.LoadDocument(path)
	if err != nil || len(saved.Profiles) != 2 {
		t.Fatal("cancel modified config")
	}
}

func TestProfileImport_ResizeHelpAndScrollableReview(t *testing.T) {
	model, _ := beginTestImport(t)
	var cmd tea.Cmd
	model, cmd = pressKey(t, model, tea.KeyEnter)
	model = runCommand(t, model, cmd)
	model, _ = pressKey(t, model, tea.KeySpace)
	model = resizeModel(t, model, 60, 16)
	if !strings.Contains(model.View(), "Resize required") {
		t.Fatal("minimum size not enforced")
	}
	model, _ = pressKey(t, model, tea.KeySpace)
	if !model.profileForm.importForm.selected["database"] {
		t.Fatal("small terminal changed selection")
	}
	model = resizeModel(t, model, 80, 24)
	model, _ = pressRune(t, model, '?')
	if !strings.Contains(model.View(), "Refresh sources") {
		t.Fatal("import help missing actions")
	}
	model, _ = pressKey(t, model, tea.KeyEsc)
	model, _ = pressKey(t, model, tea.KeyEnter)
	model.profileForm.draft.Values[0].TargetFile = strings.Repeat("long-directory/", 40) + "config.json"
	if !model.scrollablePanelMetrics().CanScroll() {
		t.Fatal("long import review must scroll")
	}
	model, _ = pressKey(t, model, tea.KeyEnd)
	if model.scrollOffset == 0 {
		t.Fatal("review did not scroll")
	}
	if !strings.Contains(model.View(), "FRONTEND_API_URL") {
		t.Fatal("retained environment source not reachable")
	}
	for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 40}} {
		model = resizeModel(t, model, size[0], size[1])
		lines := strings.Split(strings.TrimSuffix(ansi.Strip(model.View()), "\n"), "\n")
		if len(lines) > size[1] {
			t.Fatalf("view height %d > %d", len(lines), size[1])
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("view exceeds width %d", size[0])
			}
		}
		if strings.Contains(model.View(), "private-import-value") {
			t.Fatal("resize leaked value")
		}
	}
}

func TestProfileImport_UniqueNameAndEmptyMapping(t *testing.T) {
	model, _ := newTestModel(t, versionThreeConfig())
	model = resizeModel(t, model, 80, 24)
	model, _ = pressRune(t, model, 'I')
	model = enterText(t, model, "Local")
	model, _ = pressKey(t, model, tea.KeyEnter)
	if model.state != editorStateProfileNameInput || model.profileForm.errorMessage == "" {
		t.Fatal("duplicate name accepted")
	}
	model, _ = pressKey(t, model, tea.KeyCtrlU)
	model = enterText(t, model, "Imported")
	model, _ = pressKey(t, model, tea.KeyEnter)
	model, cmd := pressKey(t, model, tea.KeyEnter)
	if cmd != nil || model.state != editorStateProfileImportMapping || model.profileForm.errorMessage == "" {
		t.Fatal("empty mapping accepted")
	}
}
