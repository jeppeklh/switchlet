package history

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStorePrivateStateAndExclusiveLock(t *testing.T) {
	store, err := NewAt(filepath.Join(t.TempDir(), ".switchlet.yaml"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Read()
	if err != nil || state.Enabled {
		t.Fatalf("empty status: %v", err)
	}
	if _, err := os.Stat(store.Directory()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read created undo storage")
	}
	lock, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Save(State{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := checkPrivate(filepath.Join(store.Directory(), "state.json")); err != nil {
		t.Fatalf("state is not private: %v", err)
	}
	if _, err := store.Acquire(); err == nil {
		t.Fatal("second lock succeeded")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestStoreLockHelper$")
	command.Env = append(os.Environ(), "SWITCHLET_TEST_LOCK_DIRECTORY="+store.directory, "SWITCHLET_TEST_LOCK_CONFIG="+store.configPath)
	err = command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child lock result = %v, want lock refusal", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	unlockedChild := exec.Command(os.Args[0], "-test.run=^TestStoreLockHelper$")
	unlockedChild.Env = command.Env
	if err := unlockedChild.Run(); err != nil {
		t.Fatalf("child could not lock released project: %v", err)
	}
	next, err := store.Acquire()
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	state, err = store.Read()
	if err != nil || !state.Enabled {
		t.Fatalf("state did not persist: %v", err)
	}
}

func TestStoreLockHelper(t *testing.T) {
	directory := os.Getenv("SWITCHLET_TEST_LOCK_DIRECTORY")
	if directory == "" {
		return
	}
	store := &Store{directory: directory, configPath: os.Getenv("SWITCHLET_TEST_LOCK_CONFIG")}
	lock, err := store.Acquire()
	if err != nil {
		os.Exit(3)
	}
	if err := lock.Close(); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestStoreRejectsCorruptStateWithoutPrintingContents(t *testing.T) {
	store, err := NewAt(filepath.Join(t.TempDir(), ".switchlet.yaml"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lock, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := lock.Save(State{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Directory(), "state.json")
	if err := os.WriteFile(path, []byte("secret-invalid-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(); err == nil {
		t.Fatal("corrupt state accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private.json")
	if err := os.WriteFile(outside, []byte("outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := store.Read(); err == nil {
		t.Fatal("state symlink accepted")
	}
	contents, err := os.ReadFile(outside)
	if err != nil || string(contents) != "outside-secret" {
		t.Fatal("symlink destination changed")
	}
}

func TestStoreIdentityPreservesManifestLocation(t *testing.T) {
	root := t.TempDir()
	common := filepath.Join(root, "shared.yaml")
	if err := os.WriteFile(common, []byte("shared"), 0o600); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(root, "first.yaml")
	secondPath := filepath.Join(root, "second.yaml")
	if err := os.Symlink(common, firstPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(common, secondPath); err != nil {
		t.Fatal(err)
	}
	first, err := NewAt(firstPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAt(secondPath, filepath.Dir(filepath.Dir(first.Directory())))
	if err != nil {
		t.Fatal(err)
	}
	if first.Directory() == second.Directory() {
		t.Fatal("distinct manifest identities share undo state")
	}
}
