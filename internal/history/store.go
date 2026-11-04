package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jeppeklh/switchlet/internal/editor"
)

// Record is a single exact-file undo snapshot. It is private storage data.
type Record struct {
	Status      string
	ProfileName string
	CreatedAt   time.Time
	Files       []editor.FileSnapshot
}

// State stores project opt-in and at most one current snapshot.
type State struct {
	Version    int
	ConfigPath string
	Enabled    bool
	Record     *Record
}

// Store identifies one configuration's private state outside its repository.
type Store struct{ directory, configPath string }

// New chooses the OS user-state directory; SWITCHLET_STATE_HOME can override it.
func New(configPath string) (*Store, error) {
	root := os.Getenv("SWITCHLET_STATE_HOME")
	if root == "" {
		var err error
		switch runtime.GOOS {
		case "windows":
			root, err = os.UserCacheDir()
		case "darwin":
			root, err = os.UserConfigDir()
		default:
			root = os.Getenv("XDG_STATE_HOME")
			if root == "" {
				var home string
				home, err = os.UserHomeDir()
				root = filepath.Join(home, ".local", "state")
			}
		}
		if err != nil {
			return nil, fmt.Errorf("locate undo state directory: %w", err)
		}
		root = filepath.Join(root, "switchlet")
	}
	return NewAt(configPath, root)
}

// NewAt supplies an explicit state root for isolated hosts and tests.
func NewAt(configPath, root string) (*Store, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("undo state directory must be absolute")
	}
	path, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	path = filepath.Clean(path)
	if strings.TrimSpace(configPath) == "" {
		return nil, fmt.Errorf("undo project configuration path must be set")
	}
	// Relative targets belong to the selected manifest's parent, including when
	// the manifest itself is a symlink shared by different projects.
	if parent, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		path = filepath.Join(parent, filepath.Base(path))
	}
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	hash := sha256.Sum256([]byte(path))
	return &Store{directory: filepath.Join(root, "undo", hex.EncodeToString(hash[:])), configPath: path}, nil
}

// Directory exposes metadata only, for safe path checks and status output.
func (store *Store) Directory() string { return store.directory }

func (store *Store) empty() State { return State{Version: 1, ConfigPath: store.configPath} }

// Read is side-effect free. Atomic state replacement provides a coherent view.
func (store *Store) Read() (State, error) {
	if _, err := os.Lstat(store.directory); errors.Is(err, os.ErrNotExist) {
		return store.empty(), nil
	} else if err != nil {
		return State{}, err
	}
	if err := checkPrivate(store.directory); err != nil {
		return State{}, err
	}
	path := filepath.Join(store.directory, "state.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return store.empty(), nil
	}
	if err != nil {
		return State{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return State{}, fmt.Errorf("undo state is not a regular file within the supported size limit")
	}
	if err := checkPrivate(path); err != nil {
		return State{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return State{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<20))
	decoder.DisallowUnknownFields()
	var state State
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("undo state is unreadable or corrupt; inspect the files, then use undo --discard")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return State{}, fmt.Errorf("undo state contains trailing data")
	}
	if err := store.validate(state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (store *Store) validate(state State) error {
	if state.Version != 1 || state.ConfigPath != store.configPath {
		return fmt.Errorf("undo state has an incompatible version or project identity")
	}
	if state.Record == nil {
		return nil
	}
	if !state.Enabled {
		return fmt.Errorf("disabled undo state contains a snapshot")
	}
	switch state.Record.Status {
	case "pending", "ready", "undoing":
	default:
		return fmt.Errorf("undo snapshot has an invalid operation state")
	}
	if len(state.Record.Files) == 0 {
		return fmt.Errorf("undo snapshot contains no files")
	}
	seen := map[string]bool{}
	for _, file := range state.Record.Files {
		if !filepath.IsAbs(file.Path) || filepath.Clean(file.Path) != file.Path || seen[file.Path] || file.Permissions&^0o777 != 0 {
			return fmt.Errorf("undo snapshot contains invalid or duplicate file metadata")
		}
		seen[file.Path] = true
		if _, err := hex.DecodeString(file.AfterHash); err != nil || len(file.AfterHash) != 64 {
			return fmt.Errorf("undo snapshot contains an invalid file fingerprint")
		}

	}
	return store.ValidateTargets(state.Record.Files)
}

// ValidateTargets keeps Switchlet's private journal namespace out of managed writes.
func (store *Store) ValidateTargets(files []editor.FileSnapshot) error {
	for _, file := range files {
		relative, err := filepath.Rel(filepath.Dir(store.directory), file.Path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("undo storage files cannot be managed targets")
		}
	}
	return nil
}

// Lock holds the process lock across preparation, writes, and journal updates.
type Lock struct {
	store *Store
	file  *os.File
}

// Acquire serializes apply, undo, and preference changes for the same project.
func (store *Store) Acquire() (*Lock, error) {
	if _, err := os.Lstat(store.directory); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(store.directory, 0o700); err != nil {
			return nil, fmt.Errorf("create private undo directory: %w", err)
		}
		if err := secureDirectory(store.directory); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if err := checkPrivate(store.directory); err != nil {
		return nil, err
	}
	path := filepath.Join(store.directory, "lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if errors.Is(err, os.ErrExist) {
		if err := checkPrivate(path); err != nil {
			return nil, err
		}
		file, err = os.OpenFile(path, os.O_RDWR, 0)
	} else if err == nil {
		if privateErr := secureNewFile(path); privateErr != nil {
			file.Close()
			return nil, privateErr
		}
	}
	if err != nil {
		return nil, err
	}
	if err := lockFile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("another Switchlet operation is using this project: %w", err)
	}
	return &Lock{store: store, file: file}, nil
}

func (lock *Lock) Close() error {
	if lock.file == nil {
		return nil
	}
	file := lock.file
	lock.file = nil
	return errors.Join(unlockFile(file), file.Close())
}

func (lock *Lock) Read() (State, error) { return lock.store.Read() }

// Save publishes state only while the project lock is held.
func (lock *Lock) Save(state State) error {
	if lock.file == nil {
		return fmt.Errorf("undo state lock is closed")
	}
	state.Version = 1
	state.ConfigPath = lock.store.configPath
	if err := lock.store.validate(state); err != nil {
		return err
	}
	contents, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode undo state")
	}
	if len(contents) > 64<<20 {
		return fmt.Errorf("undo snapshot exceeds the supported 64 MiB state limit")
	}
	return writeState(lock.store.directory, contents)
}

func writeState(directory string, contents []byte) (returnErr error) {
	temporary, err := os.CreateTemp(directory, ".state-*")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() {
		temporary.Close()
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, err)
		}
	}()
	if err := secureNewFile(path); err != nil {
		return err
	}
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceState(path, filepath.Join(directory, "state.json")); err != nil {
		return fmt.Errorf("replace undo state: %w", err)
	}
	return syncStateDirectory(directory)
}
