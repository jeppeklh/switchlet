package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/config"
)

func TestRun_StartsProgramForValidProject(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: src/MyApplication/appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`)+"\n")
	writeFile(t, projectRoot, "src/MyApplication/appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	workingDirectory := filepath.Join(projectRoot, "src", "MyApplication")
	programStarted := false

	err := run(workingDirectory, func(model tea.Model) error {
		programStarted = true
		if model == nil {
			t.Fatal("runProgram received nil model")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !programStarted {
		t.Fatal("runProgram was not called")
	}
}

func TestRun_ReturnsDiscoveryErrorWithoutStartingProgram(t *testing.T) {
	workingDirectory := t.TempDir()
	programStarted := false

	err := run(workingDirectory, func(model tea.Model) error {
		programStarted = true
		return nil
	})
	if err == nil {
		t.Fatal("run returned nil error, want discovery error")
	}
	if !errors.Is(err, config.ErrConfigNotFound) {
		t.Fatalf("run returned error %q, want ErrConfigNotFound", err)
	}
	if programStarted {
		t.Fatal("runProgram was called for discovery failure")
	}
}

func TestRun_ReturnsConfigurationErrorWithoutStartingProgram(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`)+"\n")

	programStarted := false
	err := run(projectRoot, func(model tea.Model) error {
		programStarted = true
		return nil
	})
	if err == nil {
		t.Fatal("run returned nil error, want configuration error")
	}
	if !strings.Contains(err.Error(), "validate configuration file") {
		t.Fatalf("run returned error %q, want configuration validation error", err)
	}
	if programStarted {
		t.Fatal("runProgram was called for configuration failure")
	}
}

func TestRun_ReturnsTargetValidationErrorWithoutStartingProgram(t *testing.T) {
	tests := []struct {
		name           string
		targetPath     string
		targetContents *string
		wantError      string
	}{
		{
			name:      "missing target file",
			wantError: `stat target file`,
		},
		{
			name:           "invalid target json",
			targetContents: stringPointer(`{`),
			wantError:      `contains invalid JSON`,
		},
		{
			name:           "missing connection string",
			targetContents: stringPointer(`{"ConnectionStrings":{"Reporting":"Server=localhost;Database=Reporting;"}}`),
			wantError:      `does not contain connection string "DefaultConnection"`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`)+"\n")

			if testCase.targetContents != nil {
				targetRelativePath := testCase.targetPath
				if targetRelativePath == "" {
					targetRelativePath = "appsettings.Development.json"
				}

				writeFile(t, projectRoot, targetRelativePath, *testCase.targetContents)
			}

			programStarted := false
			err := run(projectRoot, func(model tea.Model) error {
				programStarted = true
				return nil
			})
			if err == nil {
				t.Fatal("run returned nil error, want target validation error")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("run returned error %q, want target validation error %q", err, testCase.wantError)
			}
			if programStarted {
				t.Fatal("runProgram was called for target validation failure")
			}
		})
	}
}

func TestRun_ReturnsProgramError(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 1

target:
  file: appsettings.Development.json
  connectionName: DefaultConnection

profiles:
  - name: Local
    value: "Server=localhost;Database=MyApplication;"
`)+"\n")
	writeFile(t, projectRoot, "appsettings.Development.json", strings.TrimSpace(`
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=localhost;Database=OldDatabase;"
  }
}
`)+"\n")

	err := run(projectRoot, func(model tea.Model) error {
		return errors.New("program failed")
	})
	if err == nil {
		t.Fatal("run returned nil error, want program error")
	}
	if !strings.Contains(err.Error(), "run terminal UI") {
		t.Fatalf("run returned error %q, want contextual program error", err)
	}
	if !strings.Contains(err.Error(), "program failed") {
		t.Fatalf("run returned error %q, want original program error", err)
	}
}

func writeFile(t *testing.T, rootDir string, relativePath string, contents string) string {
	t.Helper()

	fullPath := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create parent directories for %q: %v", fullPath, err)
	}

	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %q: %v", fullPath, err)
	}

	return fullPath
}

func stringPointer(value string) *string {
	return &value
}
