package editor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
)

// FileSnapshot contains the previous bytes and the expected post-apply identity.
// Contents may contain secrets and must never be rendered in normal output.
type FileSnapshot struct {
	Path        string
	Before      []byte
	AfterHash   string
	Permissions fs.FileMode
}

func fileHash(contents []byte) string {
	hash := sha256.Sum256(contents)
	return hex.EncodeToString(hash[:])
}

// Snapshots returns independent copies for files that will actually change.
func (prepared PreparedChanges) Snapshots() []FileSnapshot {
	files := make([]FileSnapshot, 0, len(prepared.fileChanges))
	for _, change := range prepared.fileChanges {
		if bytes.Equal(change.originalContents, change.updatedContents) {
			continue
		}
		files = append(files, FileSnapshot{Path: change.targetFile, Before: append([]byte(nil), change.originalContents...), AfterHash: fileHash(change.updatedContents), Permissions: change.originalPermissions})
	}
	return files
}

// VerifyOriginal checks whether an operation left every prepared file unchanged.
func (prepared PreparedChanges) VerifyOriginal() error {
	for _, change := range prepared.fileChanges {
		if err := verifyPreparedFileUnchanged(change); err != nil {
			return err
		}
	}
	return nil
}

// VerifyUpdated verifies exact bytes and permissions after a write or restore.
func (prepared PreparedChanges) VerifyUpdated() error {
	for _, change := range prepared.fileChanges {
		contents, info, err := readTargetFileFromDisk(change.targetFile)
		if err != nil {
			return fmt.Errorf("verify target file %q: %w", change.targetFile, err)
		}
		if !bytes.Equal(contents, change.updatedContents) || info.Mode().Perm() != change.originalPermissions {
			return fmt.Errorf("target file %q changed after replacement", change.targetFile)
		}
	}
	return nil
}

// PrepareRestore refuses any file that no longer matches the recorded apply.
func PrepareRestore(files []FileSnapshot) (PreparedChanges, error) {
	return prepareSnapshotRestore(files, restoreAfterOnly)
}

// PrepareRecovery also accepts files already restored before an interruption.
func PrepareRecovery(files []FileSnapshot) (PreparedChanges, error) {
	return prepareSnapshotRestore(files, restoreBeforeOrAfter)
}

type snapshotRestorePolicy int

const (
	restoreAfterOnly snapshotRestorePolicy = iota
	restoreBeforeOrAfter
)

func prepareSnapshotRestore(files []FileSnapshot, policy snapshotRestorePolicy) (PreparedChanges, error) {
	changes := make([]preparedFileChange, 0, len(files))
	for _, file := range files {
		contents, info, err := readTargetFileFromDisk(file.Path)
		if err != nil {
			return PreparedChanges{}, fmt.Errorf("inspect undo target %q: %w", file.Path, err)
		}
		matches := fileHash(contents) == file.AfterHash
		if policy == restoreBeforeOrAfter && bytes.Equal(contents, file.Before) {
			matches = true
		}
		if !matches || info.Mode().Perm() != file.Permissions {
			return PreparedChanges{}, fmt.Errorf("cannot undo: file %q changed since the recorded apply; no files were restored", file.Path)
		}
		changes = append(changes, preparedFileChange{targetFile: file.Path, originalContents: append([]byte(nil), contents...), updatedContents: append([]byte(nil), file.Before...), originalPermissions: file.Permissions})
	}
	return PreparedChanges{fileChanges: changes}, nil
}
