package editor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateConnectionString_PreservesOriginalPermissions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
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

	if err := UpdateConnectionString(targetPath, "DefaultConnection", "Server=test;Database=NewDatabase;"); err != nil {
		t.Fatalf("UpdateConnectionString returned error: %v", err)
	}

	updatedInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat updated file: %v", err)
	}

	if updatedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("file permissions = %o, want %o", updatedInfo.Mode().Perm(), originalInfo.Mode().Perm())
	}
}

func TestUpdateConnectionString_RenameFailureLeavesOriginalFileIntactAndCleansTemporaryFiles(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")
	originalContents := readFile(t, targetPath)

	originalRenameFile := renameFile
	renameFile = func(oldPath string, newPath string) error {
		return errors.New("rename failed")
	}
	t.Cleanup(func() {
		renameFile = originalRenameFile
	})

	err := UpdateConnectionString(targetPath, "DefaultConnection", "Server=test;Database=NewDatabase;")
	if err == nil {
		t.Fatal("UpdateConnectionString returned nil error, want rename failure")
	}
	if !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("UpdateConnectionString returned error %q, want rename failure", err)
	}

	updatedContents := readFile(t, targetPath)
	if !bytes.Equal(updatedContents, originalContents) {
		t.Fatal("target file changed after rename failure")
	}

	if containsTempFile(t, filepath.Dir(targetPath), tempFilePrefix(targetPath)) {
		t.Fatal("temporary file was not cleaned up after rename failure")
	}
}
