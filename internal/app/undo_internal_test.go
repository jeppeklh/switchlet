package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/history"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrackedApplyVerificationFailureRetainsRecoverySnapshot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte(`{"url":"before"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := history.NewAt(filepath.Join(root, ".switchlet.yaml"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	undo := NewUndoWorkflow(store)
	if _, err := undo.Configure("enable"); err != nil {
		t.Fatal(err)
	}
	application := NewWithTargets([]config.Target{{Name: "url", File: path, Type: config.TargetTypeJSON, JSONPath: "url"}}, []config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "url", Value: stringPointer("after")}}}}).WithUndoStore(store)
	application.readTargetValues = func([]config.Target) (map[string]string, error) {
		return nil, errors.New("simulated verification failure")
	}
	if _, err := application.ApplyProfileByName("Local"); !errors.Is(err, ErrPostApplyVerificationFailed) {
		t.Fatalf("apply error=%v", err)
	}
	state, err := store.Read()
	if err != nil || state.Record == nil || state.Record.Status != "pending" {
		t.Fatal("verification failure lost recovery data")
	}
	review, err := undo.Review("recover")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := undo.Commit(review); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != `{"url":"before"}` {
		t.Fatal("recovery required application-level value reads")
	}
}

func TestUndoReview_EnableIsReadOnlyAndBoundToRecordingSettings(t *testing.T) {
	store, err := history.NewAt(filepath.Join(t.TempDir(), ".switchlet.yaml"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workflow := NewUndoWorkflow(store)
	review, err := workflow.Review("enable")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Directory()); !os.IsNotExist(err) {
		t.Fatal("review created storage")
	}
	if _, err := workflow.Commit(UndoResult{Action: "enable", DryRun: true}); err == nil {
		t.Fatal("unreviewed action accepted")
	}
	if _, err := workflow.Configure("enable"); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Commit(review); err == nil {
		t.Fatal("stale recording settings accepted")
	}
}

func TestUndoReview_RejectsChangedRecordForRestoreAndDiscard(t *testing.T) {
	for _, action := range []string{"undo", "discard"} {
		t.Run(action, func(t *testing.T) {
			workflow, store, path := reviewedUndoFixture(t)
			review, err := workflow.Review(action)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lock, err := store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			state, err := lock.Read()
			if err != nil {
				t.Fatal(err)
			}
			// Keep all public metadata identical: matching names/timestamps are not identity.
			state.Record.Files[0].Before = []byte(`{"url":"another-secret"}`)
			if err := lock.Save(state); err != nil {
				t.Fatal(err)
			}
			if err := lock.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := workflow.Commit(review); err == nil || !strings.Contains(err.Error(), "changed") {
				t.Fatalf("stale commit error = %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("stale commit changed target")
			}
			current, err := store.Read()
			if err != nil || current.Record == nil || !bytes.Equal(current.Record.Files[0].Before, state.Record.Files[0].Before) {
				t.Fatal("stale commit discarded replacement record")
			}
		})
	}
}

func TestUndoReview_RechecksFilesAndKeepsOutputValueSafe(t *testing.T) {
	workflow, store, path := reviewedUndoFixture(t)
	review, err := workflow.Review("undo")
	if err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"before-secret", "after-secret", "approval", "revision", "Before", "AfterHash"} {
		if bytes.Contains(output, []byte(secret)) {
			t.Fatalf("review exposed %q", secret)
		}
	}
	external := []byte(`{"url":"external-secret"}`)
	if err := os.WriteFile(path, external, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Commit(review); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("external edit accepted: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(contents, external) {
		t.Fatal("external edit overwritten")
	}
	state, err := store.Read()
	if err != nil || state.Record == nil || state.Record.Status != "ready" {
		t.Fatal("refusal lost recovery record")
	}
}

func TestUndoReview_CommitUsesApprovedActionAndConsumesSnapshot(t *testing.T) {
	workflow, store, path := reviewedUndoFixture(t)
	review, err := workflow.Review("undo")
	if err != nil {
		t.Fatal(err)
	}
	review.Action, review.ProfileName, review.Files = "discard", "untrusted display", nil
	result, err := workflow.Commit(review)
	if err != nil || result.Action != "undo" || result.Status != "restored" {
		t.Fatalf("commit result %v, error %v", result, err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "{ \"url\": \"before-secret\" }\r\n" {
		t.Fatal("review did not restore exact bytes")
	}
	state, err := store.Read()
	if err != nil || state.Record != nil {
		t.Fatal("successful undo did not consume snapshot")
	}
	if _, err := workflow.Commit(review); err == nil {
		t.Fatal("review was reusable after consumption")
	}
}

func TestUndoReview_ChangedStorageLocationRefusesCommit(t *testing.T) {
	workflow, store, _ := reviewedUndoFixture(t)
	review, err := workflow.Review("discard")
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	other, err := history.NewAt(state.ConfigPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lock, err := other.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewUndoWorkflow(other).Commit(review); err == nil {
		t.Fatal("review accepted at another storage location")
	}
}

func reviewedUndoFixture(t *testing.T) (UndoWorkflow, *history.Store, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte("{ \"url\": \"before-secret\" }\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := history.NewAt(filepath.Join(root, ".switchlet.yaml"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workflow := NewUndoWorkflow(store)
	if _, err := workflow.Configure("enable"); err != nil {
		t.Fatal(err)
	}
	application := New(config.Target{File: path, JSONPath: "url"}, []config.Profile{{Name: "Local", Value: stringPointer("after-secret")}}).WithUndoStore(store)
	if _, err := application.ApplyProfileByName("Local"); err != nil {
		t.Fatal(err)
	}
	return workflow, store, path
}
