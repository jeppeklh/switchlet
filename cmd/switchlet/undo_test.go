package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/jeppeklh/switchlet/internal/history"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// runSessionEffect executes the same independent batch commands as Bubble Tea.
func runSessionEffect(t *testing.T, session interactiveSessionModel, command tea.Cmd) interactiveSessionModel {
	t.Helper()
	if command == nil {
		return session
	}
	message := command()
	if _, quitting := message.(tea.QuitMsg); quitting {
		return session
	}
	if batch, ok := message.(tea.BatchMsg); ok {
		for _, effect := range batch {
			session = runSessionEffect(t, session, effect)
		}
		return session
	}
	updated, next := session.Update(message)
	return runSessionEffect(t, updated.(interactiveSessionModel), next)
}

func sessionKey(t *testing.T, session interactiveSessionModel, key tea.KeyMsg) interactiveSessionModel {
	t.Helper()
	updated, command := session.Update(key)
	return runSessionEffect(t, updated.(interactiveSessionModel), command)
}

func sessionRune(t *testing.T, session interactiveSessionModel, key rune) interactiveSessionModel {
	t.Helper()
	return sessionKey(t, session, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
}

func TestInteractiveUndoPreservesSelectionFilterAndRefreshesCurrentProfile(t *testing.T) {
	root, paths := undoProject(t)
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	before := readCommandFile(t, paths[0])
	application, err := loadApplication(root)
	if err != nil {
		t.Fatal(err)
	}
	session := newInteractiveSessionModel(root, application)
	session = runSessionEffect(t, session, session.Init())
	updated, _ := session.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	session = updated.(interactiveSessionModel)
	session = sessionRune(t, session, '/')
	for _, key := range "Two" {
		session = sessionRune(t, session, key)
	}
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeyEnter})
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeySpace})
	if bytes.Equal(before, readCommandFile(t, paths[0])) {
		t.Fatal("apply-and-stay did not change target")
	}
	session = sessionRune(t, session, 'u')
	if session.mode != interactiveSessionUndo || !strings.Contains(session.View(), "Review undo") {
		t.Fatal(session.View())
	}
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(session.View(), "Confirm undo") {
		t.Fatal(session.View())
	}
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(session.View(), "Recorded files restored") {
		t.Fatal(session.View())
	}
	updated, _ = session.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	session = updated.(interactiveSessionModel)
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeyEnter})
	if len(strings.Split(session.View(), "\n")) != 24 {
		t.Fatal("picker did not receive resize during undo")
	}
	if session.mode != interactiveSessionMain || !strings.Contains(session.View(), "> Two") || strings.Contains(session.View(), "One [current]") {
		t.Fatal("return changed filtered selection")
	}
	if !bytes.Equal(before, readCommandFile(t, paths[0])) {
		t.Fatal("interactive undo did not restore exact state")
	}
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(session.View(), "One [current]") || !strings.Contains(session.View(), "> Two") {
		t.Fatalf("current comparison or selection not refreshed:\n%s", session.View())
	}
}

func TestInteractiveUndoOpeningAndCancellingNeverCreatesStorage(t *testing.T) {
	root, _ := undoProject(t)
	application, err := loadApplication(root)
	if err != nil {
		t.Fatal(err)
	}
	session := newInteractiveSessionModel(root, application)
	session = runSessionEffect(t, session, session.Init())
	session = sessionRune(t, session, 'u')
	if session.mode != interactiveSessionUndo {
		t.Fatal("undo did not open")
	}
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeyEsc})
	if session.mode != interactiveSessionMain {
		t.Fatal("cancel did not resume picker")
	}
	if _, err := os.Stat(undoStore(t, root).Directory()); !os.IsNotExist(err) {
		t.Fatal("opening or cancelling created undo storage")
	}
}

func TestInteractivePickerToleratesUnavailableStorageAndReportsPendingRecovery(t *testing.T) {
	root, _ := undoProject(t)
	application, err := loadApplication(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SWITCHLET_STATE_HOME", "invalid-relative-location")
	session := newInteractiveSessionModel(root, application)
	session = runSessionEffect(t, session, session.Init())
	if session.mode != interactiveSessionMain || !strings.Contains(session.View(), "Undo status unavailable") || !strings.Contains(session.View(), "> One") {
		t.Fatal("optional storage failure blocked the picker")
	}
	t.Setenv("SWITCHLET_STATE_HOME", t.TempDir())
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	store := undoStore(t, root)
	lock, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	state, err := lock.Read()
	if err != nil {
		t.Fatal(err)
	}
	state.Record.Status = "pending"
	if err := lock.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	session = sessionRune(t, session, 'r')
	if !strings.Contains(session.View(), "Interrupted operation") {
		t.Fatal(session.View())
	}
	session = sessionKey(t, session, tea.KeyMsg{Type: tea.KeySpace})
	if !strings.Contains(session.View(), "interrupted operation requires attention") {
		t.Fatal("pending recovery did not refuse apply")
	}
	session = sessionRune(t, session, 'u')
	if !strings.Contains(session.View(), "Review recovery") {
		t.Fatal(session.View())
	}
}

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "switchlet-test-state-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("SWITCHLET_STATE_HOME", root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	if err := os.RemoveAll(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func undoProject(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	paths := []string{writeFile(t, root, "settings.json", "{ \"value\": \"before-secret\", \"unmanaged\": 42 }\r\n"), writeFile(t, root, ".env", "TOKEN=before-token # keep\r\n")}
	writeFile(t, root, ".switchlet.yaml", `version: 3
targets:
  - name: api
    file: settings.json
    jsonPath: value
  - name: token
    file: .env
    key: TOKEN
profiles:
  - name: One
    values:
      - target: api
        value: first-secret
      - target: token
        valueFromEnv: SWITCHLET_UNDO_TOKEN
  - name: Two
    values:
      - target: api
        value: second-secret
`)
	t.Setenv("SWITCHLET_UNDO_TOKEN", "after-token")
	return root, paths
}

func undoCommand(t *testing.T, root string, args ...string) commandResult {
	t.Helper()
	result := runCommandForTest(t, args, root)
	if result.exitCode != 0 {
		t.Fatalf("command %v failed: %s", args, result.stderr)
	}
	for _, secret := range []string{"before-secret", "before-token", "first-secret", "after-token", base64.StdEncoding.EncodeToString([]byte("before-secret"))} {
		if strings.Contains(result.stdout+result.stderr, secret) {
			t.Fatalf("command %v exposed stored values", args)
		}
	}
	return result
}

func undoStore(t *testing.T, root string) *history.Store {
	t.Helper()
	store, err := history.New(filepath.Join(root, ".switchlet.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestUndoRecordsOnlyOptedInProjectsAndRestoresExactBytes(t *testing.T) {
	root, paths := undoProject(t)
	store := undoStore(t, root)
	undoCommand(t, root, "undo", "--status")
	if _, err := os.Stat(store.Directory()); !os.IsNotExist(err) {
		t.Fatal("status created state directory")
	}
	before := [][]byte{readCommandFile(t, paths[0]), readCommandFile(t, paths[1])}
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	state, err := store.Read()
	if err != nil || state.Record == nil || state.Record.Status != "ready" {
		t.Fatalf("snapshot not ready: %v", err)
	}
	snapshot := readCommandFile(t, filepath.Join(store.Directory(), "state.json"))
	applied := readCommandFile(t, paths[0])
	undoCommand(t, root, "undo", "--dry-run", "--json")
	undoCommand(t, root, "apply", "One", "--dry-run")
	undoCommand(t, root, "apply", "One")
	if !bytes.Equal(snapshot, readCommandFile(t, filepath.Join(store.Directory(), "state.json"))) || !bytes.Equal(applied, readCommandFile(t, paths[0])) {
		t.Fatal("preview or no-op apply replaced the previous snapshot")
	}
	t.Setenv("SWITCHLET_UNDO_TOKEN", "")
	if err := os.WriteFile(filepath.Join(root, ".switchlet.yaml"), []byte("invalid: [yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	undoCommand(t, root, "undo")
	for index, path := range paths {
		if !bytes.Equal(before[index], readCommandFile(t, path)) {
			t.Fatal("undo did not restore exact previous bytes")
		}
	}
	state, err = store.Read()
	if err != nil || state.Record != nil {
		t.Fatal("successful undo did not consume snapshot")
	}
	repeated := runCommandForTest(t, []string{"undo"}, root)
	if repeated.exitCode == 0 {
		t.Fatal("repeated undo toggled the state")
	}
}

func TestUndoRefusesExternalChangesBeforeRestoringAnyFile(t *testing.T) {
	root, paths := undoProject(t)
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	firstAfter := readCommandFile(t, paths[0])
	external := []byte("TOKEN=external-edit\r\n")
	if err := os.WriteFile(paths[1], external, 0o600); err != nil {
		t.Fatal(err)
	}
	result := runCommandForTest(t, []string{"undo"}, root)
	if result.exitCode == 0 || !strings.Contains(result.stderr, "changed since") {
		t.Fatalf("external edit accepted: %s", result.stderr)
	}
	if !bytes.Equal(firstAfter, readCommandFile(t, paths[0])) || !bytes.Equal(external, readCommandFile(t, paths[1])) {
		t.Fatal("conflicting undo changed a file")
	}
	if result := runCommandForTest(t, []string{"undo", "--recover"}, root); result.exitCode == 0 {
		t.Fatal("recovery bypassed conflict checks")
	}
}

func TestUndoRecoversInterruptedOperationsAndBlocksApply(t *testing.T) {
	root, paths := undoProject(t)
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	store := undoStore(t, root)
	lock, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	state, err := lock.Read()
	if err != nil {
		t.Fatal(err)
	}
	state.Record.Status = "pending"
	original := append([]byte(nil), state.Record.Files[0].Before...)
	if err := lock.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], original, state.Record.Files[0].Permissions); err != nil {
		t.Fatal(err)
	}
	if result := runCommandForTest(t, []string{"apply", "Two"}, root); result.exitCode == 0 {
		t.Fatal("apply ignored pending recovery")
	}
	if result := runCommandForTest(t, []string{"undo"}, root); result.exitCode == 0 {
		t.Fatal("normal undo accepted unfinished apply")
	}
	undoCommand(t, root, "undo", "--recover", "--dry-run", "--json")
	undoCommand(t, root, "undo", "--recover")
	for _, file := range state.Record.Files {
		if !bytes.Equal(file.Before, readCommandFile(t, file.Path)) {
			t.Fatal("recovery did not restore all files")
		}
	}
	state, err = store.Read()
	if err != nil || state.Record != nil {
		t.Fatal("recovery did not consume snapshot")
	}
}

func TestUndoDisableDiscardAndRecordingDefaults(t *testing.T) {
	root, _ := undoProject(t)
	undoCommand(t, root, "apply", "One")
	state, err := undoStore(t, root).Read()
	if err != nil || state.Enabled || state.Record != nil {
		t.Fatal("recording was not opt-in")
	}
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "Two")
	undoCommand(t, root, "undo", "--discard")
	state, err = undoStore(t, root).Read()
	if err != nil || !state.Enabled || state.Record != nil {
		t.Fatal("discard changed opt-in")
	}
	undoCommand(t, root, "apply", "One")
	undoCommand(t, root, "undo", "--disable")
	state, err = undoStore(t, root).Read()
	if err != nil || state.Enabled || state.Record != nil {
		t.Fatal("disable retained a snapshot")
	}
	for _, args := range [][]string{{"undo", "--enable", "--disable"}, {"undo", "--status", "--dry-run"}, {"undo", "--enable", "--dry-run"}, {"undo", "extra"}} {
		if result := runCommandForTest(t, args, root); result.exitCode != usageExitCode {
			t.Fatalf("invalid flags accepted: %v", args)
		}
	}
}

func TestUndoKeepsTheLastChangedApplyAndFailedApplyKeepsItsSnapshot(t *testing.T) {
	root, paths := undoProject(t)
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	store := undoStore(t, root)
	before := readCommandFile(t, filepath.Join(store.Directory(), "state.json"))
	t.Setenv("SWITCHLET_UNDO_TOKEN", "")
	if result := runCommandForTest(t, []string{"apply", "One"}, root); result.exitCode == 0 {
		t.Fatal("missing environment value applied")
	}
	if !bytes.Equal(before, readCommandFile(t, filepath.Join(store.Directory(), "state.json"))) {
		t.Fatal("failed apply replaced last good snapshot")
	}
	t.Setenv("SWITCHLET_UNDO_TOKEN", "after-token")
	firstApplied := readCommandFile(t, paths[0])
	undoCommand(t, root, "apply", "Two")
	state, err := store.Read()
	if err != nil || state.Record == nil || state.Record.ProfileName != "Two" || len(state.Record.Files) != 1 {
		t.Fatal("last changed partial apply was not recorded")
	}
	undoCommand(t, root, "undo")
	if !bytes.Equal(firstApplied, readCommandFile(t, paths[0])) {
		t.Fatal("undo restored an older apply")
	}
	if result := runCommandForTest(t, []string{"undo"}, root); result.exitCode == 0 {
		t.Fatal("undo exposed a second history level")
	}
}

func TestUndoCorruptStateCanBeDiscardedAndReadOnlyCommandsNeedNoStorage(t *testing.T) {
	root, _ := undoProject(t)
	t.Setenv("SWITCHLET_STATE_HOME", "relative-invalid-path")
	undoCommand(t, root, "list")
	if result := runCommandForTest(t, []string{"apply", "One"}, root); result.exitCode == 0 {
		t.Fatal("apply ignored invalid coordination storage")
	}
	t.Setenv("SWITCHLET_STATE_HOME", t.TempDir())
	undoCommand(t, root, "undo", "--enable")
	store := undoStore(t, root)
	if err := os.WriteFile(filepath.Join(store.Directory(), "state.json"), []byte("corrupt-secret-state"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := runCommandForTest(t, []string{"undo", "--status", "--json"}, root)
	if result.exitCode == 0 || strings.Contains(result.stderr+result.stdout, "corrupt-secret-state") {
		t.Fatal("corrupt state was accepted or exposed")
	}
	undoCommand(t, root, "undo", "--discard")
	state, err := store.Read()
	if err != nil || state.Enabled || state.Record != nil {
		t.Fatal("corrupt state did not reset safely")
	}
}

func TestUndoRefusesChangedFileModes(t *testing.T) {
	root, paths := undoProject(t)
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	before := readCommandFile(t, paths[0])
	info, err := os.Stat(paths[1])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(paths[1], info.Mode().Perm()) })
	if err := os.Chmod(paths[1], 0o400); err != nil {
		t.Fatal(err)
	}
	result := runCommandForTest(t, []string{"undo"}, root)
	if result.exitCode == 0 || !bytes.Equal(before, readCommandFile(t, paths[0])) {
		t.Fatal("changed mode did not refuse the whole undo")
	}
}

func TestUndoRefusesMissingOrSymlinkTargets(t *testing.T) {
	for _, kind := range []string{"missing", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root, paths := undoProject(t)
			undoCommand(t, root, "undo", "--enable")
			undoCommand(t, root, "apply", "One")
			first := readCommandFile(t, paths[0])
			second := readCommandFile(t, paths[1])
			if err := os.Remove(paths[1]); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside.env")
			if err := os.WriteFile(outside, second, 0o600); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				if err := os.Symlink(outside, paths[1]); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if result := runCommandForTest(t, []string{"undo"}, root); result.exitCode == 0 {
				t.Fatal("unsafe restore target accepted")
			}
			if !bytes.Equal(first, readCommandFile(t, paths[0])) || !bytes.Equal(second, readCommandFile(t, outside)) {
				t.Fatal("unsafe undo changed a file")
			}
		})
	}
}

func TestUndoRecoveryRefusesUnknownExternalBytes(t *testing.T) {
	root, paths := undoProject(t)
	undoCommand(t, root, "undo", "--enable")
	undoCommand(t, root, "apply", "One")
	store := undoStore(t, root)
	lock, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	state, err := lock.Read()
	if err != nil {
		t.Fatal(err)
	}
	state.Record.Status = "pending"
	if err := lock.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	first := readCommandFile(t, paths[0])
	external := []byte("TOKEN=unknown-edit\r\n")
	if err := os.WriteFile(paths[1], external, 0o600); err != nil {
		t.Fatal(err)
	}
	if result := runCommandForTest(t, []string{"undo", "--recover"}, root); result.exitCode == 0 {
		t.Fatal("recovery forced away external edits")
	}
	if !bytes.Equal(first, readCommandFile(t, paths[0])) || !bytes.Equal(external, readCommandFile(t, paths[1])) {
		t.Fatal("conflicting recovery changed files")
	}
}
