package app

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jeppeklh/switchlet/internal/editor"
	"github.com/jeppeklh/switchlet/internal/history"
)

// WithUndoStore adds per-project recording and coordination to CLI and TUI applies.
func (application Application) WithUndoStore(store *history.Store) Application {
	application.undoStore = store
	return application
}

// WithUndoProject defers state-location resolution until a real apply is requested.
// Read-only commands remain independent of undo storage availability.
func (application Application) WithUndoProject(configPath string) Application {
	application.undoConfigPath = configPath
	return application
}

func (application Application) applyAndVerify(profileName string, changes []editor.TargetChange, planned []PlannedChange) error {
	if application.undoStore == nil && application.undoConfigPath != "" {
		store, err := history.New(application.undoConfigPath)
		if err != nil {
			return err
		}
		application.undoStore = store
	}
	if application.undoStore != nil {
		return application.applyTracked(profileName, changes, planned)
	}
	if err := application.applyResolvedTargetChanges(changes); err != nil {
		return fmt.Errorf("apply profile %q: %w", profileName, err)
	}
	if err := application.verifyAppliedTargetChanges(profileName, changes, planned); err != nil {
		return fmt.Errorf("verify applied profile %q: %w", profileName, err)
	}
	return nil
}

func (application Application) applyTracked(profileName string, changes []editor.TargetChange, planned []PlannedChange) (returnErr error) {
	lock, err := application.undoStore.Acquire()
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, lock.Close()) }()
	previous, err := lock.Read()
	if err != nil {
		return err
	}
	if previous.Record != nil && previous.Record.Status != "ready" {
		return fmt.Errorf("an interrupted operation requires attention; run switchlet undo --status, then undo --recover or undo --discard")
	}
	prepared, err := editor.PrepareTargetChanges(changes)
	if err != nil {
		return fmt.Errorf("apply profile %q: %w", profileName, err)
	}
	files := prepared.Snapshots()
	if err := application.undoStore.ValidateTargets(files); err != nil {
		return err
	}
	recording := previous.Enabled && len(files) > 0
	pending := previous
	if recording {
		pending.Record = &history.Record{Status: "pending", ProfileName: profileName, CreatedAt: time.Now().UTC(), Files: files}
		if err := lock.Save(pending); err != nil {
			return fmt.Errorf("save undo snapshot before apply; no target files were written: %w", err)
		}
	}
	if err := prepared.Write(); err != nil {
		if recording && canRestoreUndoState(prepared, err) {
			if saveErr := lock.Save(previous); saveErr != nil {
				return errors.Join(err, fmt.Errorf("could not restore prior undo record: %w", saveErr))
			}
		}
		return fmt.Errorf("apply profile %q: %w", profileName, err)
	}
	if err := application.verifyAppliedTargetChanges(profileName, changes, planned); err != nil {
		return fmt.Errorf("verify applied profile %q: %w", profileName, err)
	}
	if !recording {
		return nil
	}
	if err := prepared.VerifyUpdated(); err != nil {
		return fmt.Errorf("apply verification failed; undo snapshot remains pending: %w", err)
	}
	pending.Record.Status = "ready"
	if err := lock.Save(pending); err != nil {
		return fmt.Errorf("profile %q was applied, but undo recording could not be finalized; inspect undo --status: %w", profileName, err)
	}
	return nil
}

// UndoResult contains display-safe metadata only; snapshot bytes never leave storage/editor operations.
type UndoResult struct {
	Action           string    `json:"action"`
	Status           string    `json:"status"`
	Enabled          bool      `json:"enabled"`
	ProfileName      string    `json:"profileName,omitempty"`
	CreatedAt        time.Time `json:"createdAt,omitempty"`
	Files            []string  `json:"files"`
	StorageDirectory string    `json:"storageDirectory"`
	DryRun           bool      `json:"dryRun"`
	approval         *undoApproval
}

// UndoWorkflow coordinates opt-in, one-level undo, and interrupted-operation recovery.
type UndoWorkflow struct {
	store      *history.Store
	configPath string
}

func NewUndoWorkflow(store *history.Store) UndoWorkflow { return UndoWorkflow{store: store} }

// UndoWorkflow shares the application's project identity without accessing storage.
func (application Application) UndoWorkflow() UndoWorkflow {
	return UndoWorkflow{store: application.undoStore, configPath: application.undoConfigPath}
}

func (workflow UndoWorkflow) resolved() (UndoWorkflow, error) {
	if workflow.store != nil {
		return workflow, nil
	}
	store, err := history.New(workflow.configPath)
	workflow.store = store
	return workflow, err
}

// undoApproval binds a value-safe review to the exact state and storage location.
// It contains no snapshot bytes and is never serialized in command output.
type undoApproval struct {
	action    string
	directory string
	revision  [sha256.Size]byte
}

func undoStateRevision(state history.State) ([sha256.Size]byte, error) {
	contents, err := json.Marshal(state)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("undo state could not be reviewed")
	}
	return sha256.Sum256(contents), nil
}

func (workflow UndoWorkflow) bindReview(state history.State, action string) (UndoResult, error) {
	revision, err := undoStateRevision(state)
	if err != nil {
		return UndoResult{}, err
	}
	result := workflow.summary(state, action)
	result.DryRun = true
	result.approval = &undoApproval{action: action, directory: workflow.store.Directory(), revision: revision}
	return result, nil
}

func (workflow UndoWorkflow) checkReview(state history.State, approval *undoApproval) error {
	if approval == nil {
		return nil
	}
	revision, err := undoStateRevision(state)
	if err != nil {
		return err
	}
	if approval.directory != workflow.store.Directory() || approval.revision != revision {
		return fmt.Errorf("undo record or recording settings changed; review again before confirming")
	}
	return nil
}

// Review prepares a read-only, value-safe confirmation for an interactive action.
func (workflow UndoWorkflow) Review(action string) (UndoResult, error) {
	switch action {
	case "undo":
		return workflow.Preview()
	case "recover":
		return workflow.PreviewRecovery()
	case "enable", "discard":
	default:
		return UndoResult{}, fmt.Errorf("unknown undo review action")
	}
	workflow, err := workflow.resolved()
	if err != nil {
		return UndoResult{}, err
	}
	state, err := workflow.store.Read()
	if err != nil {
		return UndoResult{}, err
	}
	if action == "discard" && state.Record == nil {
		return UndoResult{}, fmt.Errorf("no undo snapshot is available to discard")
	}
	return workflow.bindReview(state, action)
}

// Commit rechecks the reviewed state under the project lock and repeats all file
// guards. Exported display metadata cannot change the approved action or identity.
func (workflow UndoWorkflow) Commit(review UndoResult) (UndoResult, error) {
	approval := review.approval
	if approval == nil {
		return UndoResult{}, fmt.Errorf("review the undo action before confirming")
	}
	switch approval.action {
	case "undo", "recover":
		return workflow.restore(approval.action, executeUndo, approval)
	case "enable", "discard":
		return workflow.configure(approval.action, approval)
	default:
		return UndoResult{}, fmt.Errorf("unknown undo review action")
	}
}

func (workflow UndoWorkflow) summary(state history.State, action string) UndoResult {
	result := UndoResult{Action: action, Enabled: state.Enabled, Status: "empty", Files: []string{}, StorageDirectory: workflow.store.Directory()}
	if !state.Enabled {
		result.Status = "disabled"
	}
	if record := state.Record; record != nil {
		result.Status = record.Status
		result.ProfileName = record.ProfileName
		result.CreatedAt = record.CreatedAt
		for _, file := range record.Files {
			result.Files = append(result.Files, file.Path)
		}
	}
	return result
}

// Status never writes state or target files.
func (workflow UndoWorkflow) Status() (UndoResult, error) {
	workflow, err := workflow.resolved()
	if err != nil {
		return UndoResult{}, err
	}
	state, err := workflow.store.Read()
	if err != nil {
		return UndoResult{}, err
	}
	return workflow.summary(state, "status"), nil
}

// Configure accepts explicit enable/disable/discard actions. Disabling deletes the snapshot.
func (workflow UndoWorkflow) Configure(action string) (result UndoResult, returnErr error) {
	return workflow.configure(action, nil)
}

func (workflow UndoWorkflow) configure(action string, approval *undoApproval) (result UndoResult, returnErr error) {
	switch action {
	case "enable", "disable", "discard":
	default:
		return result, fmt.Errorf("unknown undo action")
	}
	workflow, err := workflow.resolved()
	if err != nil {
		return result, err
	}
	lock, err := workflow.store.Acquire()
	if err != nil {
		return result, err
	}
	defer func() { returnErr = errors.Join(returnErr, lock.Close()) }()
	state, err := lock.Read()
	if err != nil {
		if approval != nil || action != "discard" && action != "disable" {
			return result, err
		}
		state = history.State{}
	}
	if err := workflow.checkReview(state, approval); err != nil {
		return result, err
	}
	switch action {
	case "enable":
		state.Enabled = true
	case "disable":
		state.Enabled = false
		state.Record = nil
	case "discard":
		state.Record = nil
	}
	if err := lock.Save(state); err != nil {
		return result, err
	}
	return workflow.summary(state, action), nil
}

// Preview checks every file before reporting a read-only undo plan.
func (workflow UndoWorkflow) Preview() (UndoResult, error) {
	return workflow.restore("undo", previewUndo, nil)
}
func (workflow UndoWorkflow) Undo() (UndoResult, error) {
	return workflow.restore("undo", executeUndo, nil)
}
func (workflow UndoWorkflow) PreviewRecovery() (UndoResult, error) {
	return workflow.restore("recover", previewUndo, nil)
}
func (workflow UndoWorkflow) Recover() (UndoResult, error) {
	return workflow.restore("recover", executeUndo, nil)
}

type undoExecution int

const (
	previewUndo undoExecution = iota
	executeUndo
)

func (workflow UndoWorkflow) restore(action string, execution undoExecution, approval *undoApproval) (result UndoResult, returnErr error) {
	workflow, err := workflow.resolved()
	if err != nil {
		return result, err
	}
	initial, err := workflow.store.Read()
	if err != nil {
		return result, err
	}
	if initial.Record == nil {
		return result, fmt.Errorf("no undo snapshot is available; enable recording with switchlet undo --enable before applying a profile")
	}
	lock, err := workflow.store.Acquire()
	if err != nil {
		return result, err
	}
	defer func() { returnErr = errors.Join(returnErr, lock.Close()) }()
	state, err := lock.Read()
	if err != nil {
		return result, err
	}
	if state.Record == nil {
		return result, fmt.Errorf("no undo snapshot is available")
	}
	if err := workflow.checkReview(state, approval); err != nil {
		return result, err
	}
	record := state.Record
	if action == "undo" && record.Status != "ready" {
		return result, fmt.Errorf("undo snapshot belongs to an interrupted operation; preview undo --recover --dry-run before recovery")
	}
	if action == "recover" && record.Status == "ready" {
		return result, fmt.Errorf("the last apply completed; use switchlet undo")
	}
	var prepared editor.PreparedChanges
	if action == "recover" {
		prepared, err = editor.PrepareRecovery(record.Files)
	} else {
		prepared, err = editor.PrepareRestore(record.Files)
	}
	if err != nil {
		return result, err
	}
	result = workflow.summary(state, action)
	result.DryRun = execution == previewUndo
	if execution == previewUndo {
		return workflow.bindReview(state, action)
	}
	previousStatus := record.Status
	record.Status = "undoing"
	if err := lock.Save(state); err != nil {
		return result, fmt.Errorf("record undo intent before restoration; no files were restored: %w", err)
	}
	if err := prepared.Write(); err != nil {
		if canRestoreUndoState(prepared, err) {
			record.Status = previousStatus
			if saveErr := lock.Save(state); saveErr != nil {
				return result, errors.Join(err, saveErr)
			}
		}
		return result, err
	}
	if err := prepared.VerifyUpdated(); err != nil {
		return result, fmt.Errorf("restoration could not be verified; snapshot retained for recovery: %w", err)
	}
	state.Record = nil
	if err := lock.Save(state); err != nil {
		return result, fmt.Errorf("files were restored, but undo state could not be cleared; inspect undo --status and use undo --recover or --discard: %w", err)
	}
	result.Status = "restored"
	return result, nil
}

func canRestoreUndoState(prepared editor.PreparedChanges, writeErr error) bool {
	var recovery editor.RecoveryError
	if errors.As(writeErr, &recovery) && len(recovery.UnrestoredFiles) > 0 {
		return false
	}
	return prepared.VerifyOriginal() == nil
}
