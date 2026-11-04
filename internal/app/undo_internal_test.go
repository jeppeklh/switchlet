package app

import (
	"errors"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/history"
	"os"
	"path/filepath"
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
	if _, err := undo.Recover(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != `{"url":"before"}` {
		t.Fatal("recovery required application-level value reads")
	}
}
