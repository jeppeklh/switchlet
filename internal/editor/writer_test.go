package editor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateStringValue_PreservesOriginalPermissions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")

	if err := os.Chmod(targetPath, 0o600); err != nil {
		t.Fatalf("set file permissions: %v", err)
	}

	originalInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat original file: %v", err)
	}

	if err := UpdateStringValue(targetPath, "database.primary.url", "postgres://new"); err != nil {
		t.Fatalf("UpdateStringValue returned error: %v", err)
	}

	updatedInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat updated file: %v", err)
	}

	if updatedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("file permissions = %o, want %o", updatedInfo.Mode().Perm(), originalInfo.Mode().Perm())
	}
}

func TestUpdateStringValue_RenameFailureLeavesOriginalFileIntactAndCleansTemporaryFiles(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	originalReplaceFile := replaceFile
	replaceFile = func(oldPath string, newPath string) error {
		return errors.New("rename failed")
	}
	t.Cleanup(func() {
		replaceFile = originalReplaceFile
	})

	err := UpdateStringValue(targetPath, "database.primary.url", "postgres://new")
	if err == nil {
		t.Fatal("UpdateStringValue returned nil error, want rename failure")
	}
	if !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("UpdateStringValue returned error %q, want rename failure", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after rename failure")
	}

	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("temporary file was not cleaned up after rename failure")
	}
}

func TestUpdateStringValue_SyncFailureLeavesOriginalFileIntactAndCleansTemporaryFiles(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
  "database": {
    "primary": {
      "url": "postgres://old"
    }
  }
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	originalSyncFile := syncFile
	syncFile = func(*os.File) error {
		return errors.New("sync failed")
	}
	t.Cleanup(func() {
		syncFile = originalSyncFile
	})

	err := UpdateStringValue(targetPath, "database.primary.url", "postgres://new")
	if err == nil {
		t.Fatal("UpdateStringValue returned nil error, want sync failure")
	}
	if !strings.Contains(err.Error(), "sync failed") {
		t.Fatalf("UpdateStringValue returned error %q, want sync failure", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after sync failure")
	}

	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("temporary file was not cleaned up after sync failure")
	}
}
