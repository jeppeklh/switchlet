package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePreparedReplacement_SyncFailureLeavesOriginalConfigurationIntactAndCleansTemporaryFiles(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := filepath.Join(projectRoot, configFileName)
	originalContents := []byte("version: 3\nprofiles: []\n")
	if err := os.WriteFile(configPath, originalContents, 0o640); err != nil {
		t.Fatalf("write original configuration: %v", err)
	}

	originalSyncConfigFile := syncConfigFile
	syncConfigFile = func(*os.File) error {
		return errors.New("sync failed")
	}
	t.Cleanup(func() {
		syncConfigFile = originalSyncConfigFile
	})

	err := writePreparedReplacement(configPath, []byte("version: 3\nprofiles:\n  - name: Local\n"), 0o640)
	if err == nil {
		t.Fatal("writePreparedReplacement returned nil error, want sync failure")
	}
	if !strings.Contains(err.Error(), "sync failed") {
		t.Fatalf("writePreparedReplacement returned error %q, want sync failure", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read configuration after sync failure: %v", err)
	}
	if !bytes.Equal(contents, originalContents) {
		t.Fatalf("configuration changed after sync failure: %q", string(contents))
	}

	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		t.Fatalf("read project root: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "."+configFileName+".switchlet-") {
			t.Fatalf("temporary configuration file was not cleaned up after sync failure: %s", entry.Name())
		}
	}
}
