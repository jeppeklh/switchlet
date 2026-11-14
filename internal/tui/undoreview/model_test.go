package undoreview

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/history"
)

func TestRecordingReviewCancelAndOptIn(t *testing.T) {
	model, store, _, _ := fixture(t, "disabled")
	model = finish(t, model, model.Init())
	if !strings.Contains(model.View(), "Recording: Disabled") {
		t.Fatal(model.View())
	}
	model, command := press(model, "e")
	model = finish(t, model, command)
	for _, want := range []string{"secrets", "NOT encrypted", "Confirm"} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("missing disclosure %q: %s", want, model.View())
		}
	}
	model, command = press(model, "esc")
	model = finish(t, model, command)
	if _, err := os.Stat(store.Directory()); !os.IsNotExist(err) {
		t.Fatal("opening or cancelling created storage")
	}
	model, command = press(model, "e")
	model = finish(t, model, command)
	model, command = press(model, "enter")
	if _, extra := press(model, "enter"); extra != nil {
		t.Fatal("duplicate confirmation started another commit")
	}
	model = finish(t, model, command)
	if model.state != completed || !strings.Contains(model.View(), "recording enabled") {
		t.Fatal(model.View())
	}
	state, err := store.Read()
	if err != nil || !state.Enabled || state.Record != nil {
		t.Fatal("opt-in created an unexpected record")
	}
	model, command = press(model, "r")
	model = finish(t, model, command)
	if !strings.Contains(model.View(), "No recorded apply") || strings.Contains(model.View(), "e Enable recording") {
		t.Fatal("enabled recording without a snapshot was misrepresented")
	}
	if _, command := press(model, "enter"); command != nil {
		t.Fatal("empty recording offered restoration")
	}
}

func TestRestoreAndRecoveryUsePreviewThenConfirmation(t *testing.T) {
	for _, status := range []string{"ready", "pending", "undoing"} {
		t.Run(status, func(t *testing.T) {
			model, store, path, before := fixture(t, status)
			model = finish(t, model, model.Init())
			if status != "ready" && !strings.Contains(model.View(), "Review recovery") {
				t.Fatal(model.View())
			}
			after := read(t, path)
			model, command := press(model, "enter")
			if model.state != loadingReview {
				t.Fatal("restoration skipped loading review")
			}
			model = finish(t, model, command)
			if model.state != confirmation || !bytes.Equal(after, read(t, path)) {
				t.Fatal("preview wrote files")
			}
			assertHidden(t, model.View())
			model, ignored := press(model, "v")
			if ignored != nil {
				t.Fatal("reveal is not supported in undo")
			}
			model, command = press(model, "y")
			model = finish(t, model, command)
			if model.state != completed || !bytes.Equal(before, read(t, path)) {
				t.Fatalf("restore failed: %s", model.View())
			}
			state, err := store.Read()
			if err != nil || state.Record != nil {
				t.Fatal("restore retained consumed snapshot")
			}
		})
	}
}

func TestChangedFilesAndRecordsInvalidateConfirmation(t *testing.T) {
	for _, change := range []string{"file", "record"} {
		t.Run(change, func(t *testing.T) {
			model, store, path, _ := fixture(t, "ready")
			model = finish(t, model, model.Init())
			model, command := press(model, "enter")
			model = finish(t, model, command)
			if change == "file" {
				if err := os.WriteFile(path, []byte(`{"url":"external-secret"}`), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				changeRecordStatus(t, store, "pending")
			}
			contents := read(t, path)
			model, command = press(model, "enter")
			model = finish(t, model, command)
			if model.state != failed || model.review != nil || model.result != nil {
				t.Fatal("failure retained an executable confirmation")
			}
			if !bytes.Equal(contents, read(t, path)) {
				t.Fatal("refused undo wrote files")
			}
			if strings.Contains(model.View(), "Recorded files restored") {
				t.Fatal("failure claimed success")
			}
			if change == "record" && !strings.Contains(model.View(), "Interrupted apply") {
				t.Fatal("failure did not report actual recovery state")
			}
			if _, command := press(model, "enter"); command != nil {
				t.Fatal("failure allowed an obsolete retry")
			}
			assertHidden(t, model.View())
		})
	}
}

func TestDiscardIsSeparateAndNeverRestoresFiles(t *testing.T) {
	model, store, path, _ := fixture(t, "pending")
	model = finish(t, model, model.Init())
	after := read(t, path)
	model, command := press(model, "x")
	model = finish(t, model, command)
	if !strings.Contains(model.View(), "does NOT restore") || model.review.Action != "discard" {
		t.Fatal(model.View())
	}
	model, command = press(model, "n")
	model = finish(t, model, command)
	state, err := store.Read()
	if err != nil || state.Record == nil {
		t.Fatal("cancelling discarded the record")
	}
	model, command = press(model, "x")
	model = finish(t, model, command)
	model, command = press(model, "enter")
	model = finish(t, model, command)
	state, err = store.Read()
	if err != nil || state.Record != nil || !state.Enabled || !bytes.Equal(after, read(t, path)) {
		t.Fatal("discard changed targets or recording")
	}
	if !strings.Contains(model.View(), "Target files were not changed") {
		t.Fatal(model.View())
	}
}

func TestCorruptOrUnavailableStatusCannotAuthorizeActions(t *testing.T) {
	model, store, path, _ := fixture(t, "ready")
	if err := os.WriteFile(filepath.Join(store.Directory(), "state.json"), []byte("corrupt-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	model = finish(t, model, model.Init())
	if model.state != failed || !strings.Contains(model.View(), "switchlet undo --status") {
		t.Fatal(model.View())
	}
	for _, key := range []string{"enter", "e", "x", "y"} {
		if _, command := press(model, key); command != nil {
			t.Fatalf("%q authorized action without a readable record", key)
		}
	}
	assertHidden(t, model.View())
	if !strings.Contains(string(read(t, path)), "after-secret") {
		t.Fatal("corrupt status changed target")
	}
}

func TestLateResultsAreRejectedAfterCancelRefreshAndReopen(t *testing.T) {
	model, _, _, _ := fixture(t, "ready")
	oldStatus := model.Init()()
	newModel := New(model.workflow, model.projectRoot)
	updated, _ := newModel.Update(oldStatus)
	if updated.(Model).state != loadingStatus {
		t.Fatal("result from a previous activation was accepted")
	}
	model = finish(t, model, model.Init())
	model, command := press(model, "enter")
	oldReview := command()
	model, command = press(model, "esc")
	model = finish(t, model, command)
	updated, _ = model.Update(oldReview)
	model = updated.(Model)
	if model.state != overview || model.review != nil {
		t.Fatal("cancelled review became a confirmation")
	}
	model, _ = press(model, "esc")
	updated, _ = model.Update(oldStatus)
	if _, done := updated.(Model).Result(); !done {
		t.Fatal("late status reopened a closed review")
	}
}

func TestCommitFailureDisplaysRefreshedRecoveryState(t *testing.T) {
	model, _, _, _ := fixture(t, "ready")
	model = finish(t, model, model.Init())
	model, command := press(model, "enter")
	model = finish(t, model, command)
	model, _ = press(model, "enter")
	// The engine can retain undoing intent after an unverified or partial restore.
	message := commitCompletedMsg{requestID: model.requestID, scope: model.scope,
		err:    errors.New("restoration could not be verified; snapshot retained for recovery"),
		status: app.UndoResult{Enabled: true, Status: "undoing", ProfileName: "Local"},
	}
	updated, _ := model.Update(message)
	model = updated.(Model)
	if model.state != failed || !strings.Contains(model.View(), "Interrupted restoration") || strings.Contains(model.View(), "Recorded files restored") {
		t.Fatalf("failure misrepresented recovery state:\n%s", model.View())
	}
	other := New(model.workflow, model.projectRoot)
	other.state, other.requestID = committing, model.requestID
	updated, _ = other.Update(message)
	if updated.(Model).state != committing {
		t.Fatal("previous screen's commit result was accepted")
	}
}

func TestUndoLayoutsAreBoundedAndAllFilesCanBeReviewed(t *testing.T) {
	model, _, _, _ := fixture(t, "ready")
	model = finish(t, model, model.Init())
	model, command := press(model, "enter")
	model = finish(t, model, command)
	model.review.Files = append([]string{filepath.Join(model.projectRoot, strings.Repeat("長い-directory/", 15), "first.json")}, model.review.Files...)
	for _, size := range [][2]int{{60, 16}, {80, 24}, {120, 32}, {59, 16}, {60, 15}} {
		updated, _ := model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		resized := updated.(Model)
		view := resized.View()
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > size[0] {
				t.Fatalf("line exceeds width %d: %q", size[0], line)
			}
		}
		if len(strings.Split(view, "\n")) > size[1] {
			t.Fatal("view exceeds height")
		}
		if resized.tooSmall() {
			if _, command := press(resized, "enter"); command != nil {
				t.Fatal("hidden confirmation executed")
			}
			continue
		}
		if !strings.Contains(view, "Enter/y Confirm") || !strings.Contains(view, "n/Esc Cancel") {
			t.Fatal(view)
		}
		resized, _ = press(resized, "end")
		if !strings.Contains(resized.View(), "settings.json") {
			t.Fatal("last affected file is not reachable")
		}
		assertHidden(t, resized.View())
	}
}

func fixture(t *testing.T, status string) (Model, *history.Store, string, []byte) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	before := []byte("{ \"url\": \"before-secret\", \"unmanaged\": true }\r\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := history.NewAt(filepath.Join(root, ".switchlet.yaml"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	value := "after-secret"
	application := app.New(config.Target{File: path, JSONPath: "url"}, []config.Profile{{Name: "Local", Value: &value}}).WithUndoStore(store)
	workflow := application.UndoWorkflow()
	if status != "disabled" {
		if _, err := workflow.Configure("enable"); err != nil {
			t.Fatal(err)
		}
		if _, err := application.ApplyProfileByName("Local"); err != nil {
			t.Fatal(err)
		}
		if status != "ready" {
			changeRecordStatus(t, store, status)
		}
	}
	return New(workflow, root), store, path, before
}

func changeRecordStatus(t *testing.T, store *history.Store, status string) {
	t.Helper()
	lock, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	state, err := lock.Read()
	if err != nil {
		t.Fatal(err)
	}
	state.Record.Status = status
	if err := lock.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

func press(model Model, key string) (Model, tea.Cmd) {
	message := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "enter":
		message.Type = tea.KeyEnter
	case "esc":
		message.Type = tea.KeyEsc
	case "end":
		message.Type = tea.KeyEnd
	}
	updated, command := model.Update(message)
	return updated.(Model), command
}

func finish(t *testing.T, model Model, command tea.Cmd) Model {
	t.Helper()
	if command == nil {
		t.Fatal("expected an effect")
	}
	updated, next := model.Update(command())
	if next != nil {
		t.Fatal("unexpected follow-up effect")
	}
	return updated.(Model)
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func assertHidden(t *testing.T, view string) {
	t.Helper()
	for _, secret := range []string{"before-secret", "after-secret", "external-secret", "corrupt-secret"} {
		if strings.Contains(view, secret) {
			t.Fatalf("undo view exposed %q", secret)
		}
	}
}
