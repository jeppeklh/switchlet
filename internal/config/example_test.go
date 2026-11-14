package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeppeklh/switchlet/internal/config"
)

func exampleInputs(t *testing.T) (string, string, []config.Target, []config.Profile) {
	t.Helper()
	root := t.TempDir()
	active := filepath.Join(root, ".switchlet.yaml")
	if err := os.WriteFile(active, []byte("active-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	target := config.Target{Name: "url", File: filepath.Join(root, "settings.json"), Type: config.TargetTypeJSON, JSONPath: "url"}
	if err := os.WriteFile(target.File, []byte("target-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	name := "EXAMPLE_URL"
	return root, active, []config.Target{target}, []config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "url", ValueFromEnv: &name}}}}
}

func TestPrepareExample_RejectsAliasesAndNonRegularFiles(t *testing.T) {
	for _, kind := range []string{"active path", "relative active path", "missing active path", "target path", "active hardlink", "target hardlink", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root, active, targets, profiles := exampleInputs(t)
			destination := filepath.Join(root, ".switchlet.example.yaml")
			switch kind {
			case "active path":
				active = destination
			case "relative active path":
				active = ".switchlet.example.yaml"
			case "missing active path":
				active = ""
			case "target path":
				targets[0].File = destination
			case "active hardlink", "target hardlink":
				from := active
				if kind == "target hardlink" {
					from = targets[0].File
				}
				if err := os.Link(from, destination); err != nil {
					t.Skipf("hardlinks unavailable: %v", err)
				}
			case "symlink":
				if err := os.Symlink(active, destination); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "directory":
				if err := os.Mkdir(destination, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := config.PrepareExample(root, active, targets, profiles); err == nil {
				t.Fatal("unsafe destination accepted")
			}
		})
	}
}

func TestPreparedExample_ReviewedReplacementAndConflicts(t *testing.T) {
	for _, change := range []string{"none", "changed", "removed", "appeared", "alias after review"} {
		t.Run(change, func(t *testing.T) {
			root, active, targets, profiles := exampleInputs(t)
			destination := filepath.Join(root, ".switchlet.example.yaml")
			if change != "appeared" {
				if err := os.WriteFile(destination, []byte("old-example-secret"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			prepared, err := config.PrepareExample(root, active, targets, profiles)
			if err != nil {
				t.Fatal(err)
			}
			if prepared.ReplacesExisting() != (change != "appeared") {
				t.Fatal("incorrect replacement status")
			}
			switch change {
			case "changed", "appeared":
				if err := os.WriteFile(destination, []byte("external-content"), 0600); err != nil {
					t.Fatal(err)
				}
			case "removed":
				if err := os.Remove(destination); err != nil {
					t.Fatal(err)
				}
			case "alias after review":
				if err := os.Remove(destination); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(active, destination); err != nil {
					t.Skipf("hardlink unavailable: %v", err)
				}
			}
			err = prepared.Commit()
			if change == "none" {
				if err != nil {
					t.Fatal(err)
				}
				contents, err := os.ReadFile(destination)
				if err != nil || !strings.Contains(string(contents), "valueFromEnv: EXAMPLE_URL") || strings.Contains(string(contents), "old-example-secret") {
					t.Fatal("replacement failed")
				}
			} else if err == nil {
				t.Fatal("stale or aliased destination replaced")
			}
			if change == "changed" || change == "appeared" {
				contents, _ := os.ReadFile(destination)
				if string(contents) != "external-content" {
					t.Fatal("conflicting contents changed")
				}
			}
			contents, _ := os.ReadFile(active)
			if string(contents) != "active-secret" {
				t.Fatal("active configuration changed")
			}
			contents, _ = os.ReadFile(targets[0].File)
			if string(contents) != "target-secret" {
				t.Fatal("managed file changed")
			}
		})
	}
}

func TestPrepareExample_RejectsLiteralsAtPersistenceBoundary(t *testing.T) {
	root, active, targets, profiles := exampleInputs(t)
	secret := "private-literal"
	profiles[0].Values[0].Value = &secret
	profiles[0].Values[0].ValueFromEnv = nil
	if _, err := config.PrepareExample(root, active, targets, profiles); err == nil {
		t.Fatal("literal export accepted")
	}
}

func TestPrepareExample_RejectsDanglingProtectedAliases(t *testing.T) {
	for _, kind := range []string{"target", "active", "parent"} {
		t.Run(kind, func(t *testing.T) {
			root, active, targets, profiles := exampleInputs(t)
			destination := filepath.Join(root, ".switchlet.example.yaml")
			protected := targets[0].File
			if kind == "active" {
				protected = active
			}
			if kind == "parent" {
				alias := filepath.Join(root, "alias-directory")
				if err := os.Symlink(root, alias); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				targets[0].File = filepath.Join(alias, ".switchlet.example.yaml")
			} else {
				if err := os.Remove(protected); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(destination, protected); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			if _, err := config.PrepareExample(root, active, targets, profiles); err == nil {
				t.Fatal("dangling protected alias accepted")
			}
			if _, err := os.Lstat(destination); !os.IsNotExist(err) {
				t.Fatal("alias rejection created example")
			}
		})
	}
}

func TestPreparedExample_RechecksProtectedAliasesBeforeCreating(t *testing.T) {
	root, active, targets, profiles := exampleInputs(t)
	prepared, err := config.PrepareExample(root, active, targets, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(targets[0].File); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(prepared.Path(), targets[0].File); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := prepared.Commit(); err == nil {
		t.Fatal("new dangling alias accepted after review")
	}
	if _, err := os.Lstat(prepared.Path()); !os.IsNotExist(err) {
		t.Fatal("commit created aliased example")
	}
}
