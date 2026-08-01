package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/tui/configeditor"
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

	err := runInteractiveCommand(workingDirectory, func(model tea.Model) error {
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

func TestRun_InteractiveModelDisplaysPathsRelativeToDiscoveredProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: app/config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
`)+"\n")
	writeFile(t, projectRoot, "app/config.json", `{"database":{"url":"postgres://old"}}`)
	workingDirectory := filepath.Join(projectRoot, "app")

	programStarted := false
	err := runInteractiveCommand(workingDirectory, func(model tea.Model) error {
		programStarted = true

		updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 32})
		view := updatedModel.View()
		if !strings.Contains(view, "app/config.json") {
			t.Fatalf("View() = %q, want project-relative target path", view)
		}
		if strings.Contains(view, projectRoot) {
			t.Fatalf("View() = %q, must not show absolute project root %q", view, projectRoot)
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

func TestRun_StartsProgramForValidVersionTwoProject(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 2

target:
  file: config/runtime.json
  jsonPath: services.backend.baseUrl

profiles:
  - name: Local
    value: http://localhost:8080
`)+"\n")
	writeFile(t, projectRoot, "config/runtime.json", strings.TrimSpace(`
{
  "services": {
    "backend": {
      "baseUrl": "https://old.example.test"
    }
  }
}
`)+"\n")

	programStarted := false

	err := runInteractiveCommand(projectRoot, func(model tea.Model) error {
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

	err := runInteractiveCommand(workingDirectory, func(model tea.Model) error {
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
	err := runInteractiveCommand(projectRoot, func(model tea.Model) error {
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
			wantError:      `does not contain JSON path "ConnectionStrings.DefaultConnection": missing segment "DefaultConnection"`,
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
			err := runInteractiveCommand(projectRoot, func(model tea.Model) error {
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

	err := runInteractiveCommand(projectRoot, func(model tea.Model) error {
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

func TestInteractiveSession_ConfigKeyTogglesBetweenPickerAndConfigEditor(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
`)+"\n")
	writeFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://old"}}`)

	application, err := loadApplication(projectRoot)
	if err != nil {
		t.Fatalf("loadApplication returned error: %v", err)
	}
	session := newInteractiveSessionModel(projectRoot, application)

	updatedModel, _ := session.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	session = updatedModel.(interactiveSessionModel)
	if session.mode != interactiveSessionConfig {
		t.Fatalf("session mode = %v, want config editor", session.mode)
	}

	updatedModel, command := session.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	session = updatedModel.(interactiveSessionModel)
	if session.mode != interactiveSessionMain {
		t.Fatalf("session mode = %v, want main picker", session.mode)
	}
	if command == nil {
		t.Fatal("command is nil, want main picker init command after returning")
	}
}

func TestInteractiveSession_ConfigReturnPreservesSelectedProfileAndCurrentBadgeSemantics(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
  - name: Staging
    values:
      - target: database
        value: postgres://staging
`)+"\n")
	writeFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)

	application, err := loadApplication(projectRoot)
	if err != nil {
		t.Fatalf("loadApplication returned error: %v", err)
	}
	session := newInteractiveSessionModel(projectRoot, application)
	updatedModel, _ := session.Update(tea.KeyMsg{Type: tea.KeyDown})
	session = updatedModel.(interactiveSessionModel)
	updatedModel, _ = session.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	session = updatedModel.(interactiveSessionModel)
	if session.mode != interactiveSessionConfig {
		t.Fatalf("session mode = %v, want config editor", session.mode)
	}

	updatedModel, command := session.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	session = updatedModel.(interactiveSessionModel)
	if command == nil {
		t.Fatal("command is nil, want main picker init command after returning")
	}
	updatedModel, _ = session.Update(command())
	session = updatedModel.(interactiveSessionModel)

	view := session.View()
	if !strings.Contains(view, "> Staging") {
		t.Fatalf("returned picker view = %q, want Staging cursor preserved", view)
	}
	if !strings.Contains(view, "Local [current]") {
		t.Fatalf("returned picker view = %q, want Local current badge from file contents", view)
	}
	if strings.Contains(view, "Staging [current]") {
		t.Fatalf("returned picker view = %q, selected Staging must not become current badge", view)
	}
}

func TestInteractiveSession_ConfigSavePreservesSelectedProfileWhenStillPresent(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
  - name: Staging
    values:
      - target: database
        value: postgres://staging
`)+"\n")
	writeFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)

	application, err := loadApplication(projectRoot)
	if err != nil {
		t.Fatalf("loadApplication returned error: %v", err)
	}
	session := newInteractiveSessionModel(projectRoot, application)
	updatedModel, _ := session.Update(tea.KeyMsg{Type: tea.KeyDown})
	session = updatedModel.(interactiveSessionModel)
	updatedModel, _ = session.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	session = updatedModel.(interactiveSessionModel)

	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
  - name: Staging
    values:
      - target: database
        value: postgres://staging
  - name: QA
    values:
      - target: database
        value: postgres://qa
`)+"\n")

	updatedModel, command := session.handleConfigResult(configeditor.Result{Saved: true, ConfigPath: filepath.Join(projectRoot, ".switchlet.yaml")})
	session = updatedModel.(interactiveSessionModel)
	if command == nil {
		t.Fatal("command is nil, want main picker init command after saved reload")
	}
	if !strings.Contains(session.View(), "> Staging") {
		t.Fatalf("reloaded picker view = %q, want Staging cursor preserved", session.View())
	}
}

func TestInteractiveSession_ConfigSaveFallsBackWhenSelectedProfileWasDeleted(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
  - name: Staging
    values:
      - target: database
        value: postgres://staging
`)+"\n")
	writeFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)

	application, err := loadApplication(projectRoot)
	if err != nil {
		t.Fatalf("loadApplication returned error: %v", err)
	}
	session := newInteractiveSessionModel(projectRoot, application)
	updatedModel, _ := session.Update(tea.KeyMsg{Type: tea.KeyDown})
	session = updatedModel.(interactiveSessionModel)
	updatedModel, _ = session.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	session = updatedModel.(interactiveSessionModel)

	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
`)+"\n")

	updatedModel, _ = session.handleConfigResult(configeditor.Result{Saved: true, ConfigPath: filepath.Join(projectRoot, ".switchlet.yaml")})
	session = updatedModel.(interactiveSessionModel)
	view := session.View()
	if !strings.Contains(view, "> Local") {
		t.Fatalf("reloaded picker view = %q, want valid fallback selection", view)
	}
	if strings.Contains(view, "> Staging") {
		t.Fatalf("reloaded picker view = %q, must not select deleted profile", view)
	}
}

func TestInteractiveSession_ConfigSaveReloadsBeforeReturningToPicker(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
`)+"\n")
	writeFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://old"}}`)

	application, err := loadApplication(projectRoot)
	if err != nil {
		t.Fatalf("loadApplication returned error: %v", err)
	}
	session := newInteractiveSessionModel(projectRoot, application)

	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
  - name: Staging
    values:
      - target: database
        value: postgres://staging
`)+"\n")

	updatedModel, _ := session.handleConfigResult(configeditor.Result{Saved: true, ConfigPath: filepath.Join(projectRoot, ".switchlet.yaml")})
	session = updatedModel.(interactiveSessionModel)
	if session.mode != interactiveSessionMain {
		t.Fatalf("session mode = %v, want main picker", session.mode)
	}
	view := session.View()
	if !strings.Contains(view, "Staging") {
		t.Fatalf("reloaded picker view = %q, want saved Staging profile", view)
	}
}

func TestInteractiveSession_ConfigSaveShowsReloadErrorWithoutStaleProfiles(t *testing.T) {
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".switchlet.yaml", strings.TrimSpace(`
version: 3

targets:
  - name: database
    file: config.json
    type: json
    jsonPath: database.url

profiles:
  - name: Local
    values:
      - target: database
        value: postgres://local
`)+"\n")
	writeFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://old"}}`)

	application, err := loadApplication(projectRoot)
	if err != nil {
		t.Fatalf("loadApplication returned error: %v", err)
	}
	session := newInteractiveSessionModel(projectRoot, application)

	writeFile(t, projectRoot, ".switchlet.yaml", "version: 3\nprofiles: [\n")
	updatedModel, _ := session.handleConfigResult(configeditor.Result{Saved: true, ConfigPath: filepath.Join(projectRoot, ".switchlet.yaml")})
	session = updatedModel.(interactiveSessionModel)

	view := session.View()
	if !strings.Contains(view, "Configuration was saved, but Switchlet could not reload it.") {
		t.Fatalf("reload failure view = %q, want focused reload error", view)
	}
	if strings.Contains(view, "Local") {
		t.Fatalf("reload failure view = %q, must not show stale Local profile", view)
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
