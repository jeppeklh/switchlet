package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestDiscover_FindsConfigurationInCurrentDirectory(t *testing.T) {
	projectRoot := t.TempDir()
	copyTree(t, fixturePath(t, "valid", "basic"), projectRoot)

	configPath, err := config.Discover(projectRoot)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	wantPath := filepath.Join(projectRoot, ".switchlet.yaml")
	if configPath != wantPath {
		t.Fatalf("Discover returned %q, want %q", configPath, wantPath)
	}
}

func TestDiscover_FindsConfigurationInParentDirectory(t *testing.T) {
	projectRoot := t.TempDir()
	copyTree(t, fixturePath(t, "valid", "basic"), projectRoot)

	nestedDirectory := filepath.Join(projectRoot, "src", "MyApplication", "Data")
	if err := os.MkdirAll(nestedDirectory, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}

	configPath, err := config.Discover(nestedDirectory)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	wantPath := filepath.Join(projectRoot, ".switchlet.yaml")
	if configPath != wantPath {
		t.Fatalf("Discover returned %q, want %q", configPath, wantPath)
	}
}

func TestDiscover_ReturnsConfigNotFoundWhenMissing(t *testing.T) {
	startDirectory := filepath.Join(t.TempDir(), "one", "two")
	if err := os.MkdirAll(startDirectory, 0o755); err != nil {
		t.Fatalf("create start directory: %v", err)
	}

	_, err := config.Discover(startDirectory)
	if err == nil {
		t.Fatal("Discover returned nil error, want config-not-found error")
	}

	if !errors.Is(err, config.ErrConfigNotFound) {
		t.Fatalf("Discover returned error %q, want ErrConfigNotFound", err)
	}
}
