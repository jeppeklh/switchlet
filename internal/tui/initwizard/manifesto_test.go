package initwizard

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

func TestInitWizardModel_PendingEffectsCanBeCancelledAndIgnoreLateResults(t *testing.T) {
	for _, testCase := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "q", key: runeKey('q')},
		{name: "ctrl-c", key: tea.KeyMsg{Type: tea.KeyCtrlC}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json", Type: app.InitTargetTypeJSON}
			inspectionCount := 0

			model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
				DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
					return []app.InitTargetFileCandidate{selectedCandidate}, nil
				},
				InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
					inspectionCount++
					if path != selectedCandidate.Path {
						return nil, fmt.Errorf("unexpected path %q", path)
					}

					return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
				},
			})
			if err != nil {
				t.Fatalf("newInitWizardModel returned error: %v", err)
			}
			model.width = 120
			model.height = 32

			updatedModel, inspectionCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			model = updatedModel.(initWizardModel)
			if inspectionCommand == nil {
				t.Fatal("inspectionCommand is nil, want pending file inspection command")
			}
			if !model.isPending() {
				t.Fatal("isPending() = false, want pending inspection before cancellation")
			}

			updatedModel, quitCommand := model.Update(testCase.key)
			model = updatedModel.(initWizardModel)
			if quitCommand == nil {
				t.Fatalf("quitCommand is nil after %s, want cancellation command", testCase.name)
			}
			if model.pendingEffect != nil {
				t.Fatalf("pendingEffect = %#v, want cleared after cancellation", model.pendingEffect)
			}
			if model.result == nil || !model.result.Cancelled {
				t.Fatalf("result = %#v, want cancelled result", model.result)
			}

			lateMessage := inspectionCommand()
			if inspectionCount != 1 {
				t.Fatalf("inspectionCount = %d, want late command to execute once", inspectionCount)
			}
			updatedModel, command := model.Update(lateMessage)
			model = updatedModel.(initWizardModel)
			if command != nil {
				t.Fatal("command is not nil, want late result ignored")
			}
			if model.result == nil || !model.result.Cancelled || model.step != initWizardStepFileSelect {
				t.Fatalf("model after late result = %#v, want cancelled file-selection state", model)
			}
		})
	}
}

func TestInitWizardModel_ExplicitFileTypeInspectionRunsThroughCommand(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "config.local")
	inspectionCount := 0

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return nil, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			inspectionCount++
			if path != targetPath {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	model.width = 120
	model.height = 32

	model = updateWizardModel(t, model, runeKey('m'))
	typeWizardText(t, &model, "config.local")
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want explicit type selection before inspection")
	}
	if model.step != initWizardStepTypeSelect {
		t.Fatalf("step = %d, want type selection for unknown file extension", model.step)
	}

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(initWizardModel)
	if command == nil {
		t.Fatal("command is nil, want file inspection command after choosing explicit type")
	}
	if inspectionCount != 0 {
		t.Fatalf("inspectionCount = %d, want no inspection before command execution", inspectionCount)
	}
	if !model.isPending() {
		t.Fatal("isPending() = false, want pending explicit type inspection")
	}
	for _, expected := range []string{"Inspecting configuration file", "Inspecting config.local.", "Detected format: JSON"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if inspectionCount != 1 {
		t.Fatalf("inspectionCount = %d, want one inspection after command execution", inspectionCount)
	}
	if model.step != initWizardStepPathBrowse || model.selectedFile.TargetType != app.InitTargetTypeJSON {
		t.Fatalf("model after explicit type inspection = %#v, want JSON path browse", model)
	}
}

func TestInitWizardModel_StaleDotenvValidationResultIsIgnoredAfterBacktrackingAndEditing(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, ".env")
	validationCount := 0
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateDotenvTarget: func(string, string) error {
				validationCount++
				return nil
			},
		}),
		step:   initWizardStepManualDotenvKey,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: ".env",
			TargetType:  app.InitTargetTypeDotenv,
		},
	}
	model.setInputValue("OLD_KEY")

	updatedModel, validationCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(initWizardModel)
	if validationCommand == nil {
		t.Fatal("validationCommand is nil, want dotenv validation command")
	}
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want pending cancellation without command")
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyCtrlU})
	typeWizardText(t, &model, "NEW_KEY")
	staleMessage := validationCommand()
	if validationCount != 1 {
		t.Fatalf("validationCount = %d, want stale command to execute once", validationCount)
	}
	updatedModel, command = model.Update(staleMessage)
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want stale dotenv result ignored")
	}
	if model.step != initWizardStepManualDotenvKey || model.inputValue != "NEW_KEY" || model.selectedDotenvKey != "" || !model.errorDetail.IsZero() {
		t.Fatalf("model after stale dotenv validation = %#v, want edited manual key with no selected key", model)
	}
}

func TestInitWizardModel_ManifestoScreensFitHostileDimensions(t *testing.T) {
	projectRoot := t.TempDir()
	longLiteralValue := "postgres://very-long-host-name.example.test/database-name-with-a-long-suffix"
	longEnvironmentVariable := "STAGING_DATABASE_URL"
	screens := []struct {
		name                  string
		model                 initWizardModel
		supportedBottomAction string
	}{
		{
			name: "file-selection",
			model: initWizardModel{
				workingDirectory: projectRoot,
				step:             initWizardStepFileSelect,
				fileCandidates: []app.InitTargetFileCandidate{{
					Path:         filepath.Join(projectRoot, "services", "backend", "configuration", "appsettings.Development.json"),
					RelativePath: filepath.Join("services", "backend", "configuration", "appsettings.Development.json"),
					Type:         app.InitTargetTypeJSON,
				}},
			},
			supportedBottomAction: "q Cancel",
		},
		{
			name: "pending-inspection",
			model: initWizardModel{
				workingDirectory: projectRoot,
				step:             initWizardStepFileSelect,
				pendingEffect: &initWizardPendingEffect{
					Kind:        initWizardEffectFileInspection,
					StepNumber:  1,
					Title:       "Inspecting configuration file",
					Message:     "Inspecting services/backend/configuration/appsettings.Development.json.",
					ReturnStep:  initWizardStepFileSelect,
					TargetPath:  filepath.Join(projectRoot, "services", "backend", "configuration", "appsettings.Development.json"),
					DisplayPath: filepath.Join("services", "backend", "configuration", "appsettings.Development.json"),
					TargetType:  app.InitTargetTypeJSON,
				},
			},
			supportedBottomAction: "q Cancel",
		},
		{
			name: "profile-value-entry",
			model: initWizardModel{
				workingDirectory: projectRoot,
				step:             initWizardStepProfileValue,
				targets:          []app.InitTarget{{Name: "database", File: filepath.Join(projectRoot, "config.json"), Type: app.InitTargetTypeJSON, JSONPath: "services.database.primary.connectionStrings.defaultConnection.value"}},
				draftProfile:     initWizardProfileDraft{Name: "Local profile with a long display name", TargetIndex: 0},
				inputValue:       longLiteralValue,
				inputCursor:      len([]rune(longLiteralValue)),
			},
			supportedBottomAction: "Ctrl+C Cancel",
		},
		{
			name: "structured-error",
			model: initWizardModel{
				workingDirectory: projectRoot,
				step:             initWizardStepManualPath,
				selectedFile: app.InitTargetFileSelection{
					Path:        filepath.Join(projectRoot, "services", "backend", "configuration", "appsettings.Development.json"),
					DisplayPath: filepath.Join("services", "backend", "configuration", "appsettings.Development.json"),
					TargetType:  app.InitTargetTypeJSON,
				},
				inputValue:  "services.database.primary.connectionStrings.defaultConnection.value",
				inputCursor: len([]rune("services.database.primary.connectionStrings.defaultConnection.value")),
				errorDetail: ui.RecoverableError{
					Problem:  "Could not use this JSON value.",
					Context:  []string{"File: services/backend/configuration/appsettings.Development.json", "Selector: services.database.primary.connectionStrings.defaultConnection.value"},
					Reason:   strings.Repeat("the selected JSON path does not resolve to one existing string value ", 8),
					Recovery: "Choose an existing string value or enter another path.",
				},
			},
			supportedBottomAction: "Ctrl+C Cancel",
		},
		{
			name: "review",
			model: initWizardModel{
				workingDirectory:   projectRoot,
				step:               initWizardStepReview,
				shouldIgnoreConfig: true,
				targets: []app.InitTarget{
					{Name: "database", File: filepath.Join(projectRoot, "backend", "appsettings.Development.json"), Type: app.InitTargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
					{Name: "frontendApi", File: filepath.Join(projectRoot, "frontend", ".env.local"), Type: app.InitTargetTypeDotenv, Key: "VITE_API_URL"},
				},
				profiles: []app.InitProfile{
					{Name: "Local", Values: []app.InitProfileValue{{Target: "database", Value: stringPointer(longLiteralValue)}}},
					{Name: "Staging", Protected: true, Values: []app.InitProfileValue{{Target: "database", ValueFromEnv: stringPointer(longEnvironmentVariable)}, {Target: "frontendApi", Value: stringPointer("https://api.staging.example.test/with/a/long/path")}}},
				},
			},
			supportedBottomAction: "q Cancel",
		},
	}

	for _, screen := range screens {
		for _, size := range []struct {
			width  int
			height int
		}{
			{width: 200, height: 60},
			{width: 120, height: 40},
			{width: 80, height: 24},
			{width: 60, height: 20},
			{width: 40, height: 15},
		} {
			t.Run(screen.name+"_"+fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
				model := resizedWizardModel(t, screen.model, size.width, size.height)
				view := model.View()
				assertWizardViewWidth(t, view, size.width)
				assertWizardViewHeight(t, view, size.height)

				expectedAction := screen.supportedBottomAction
				if size.width < initWizardMinimumTerminalWidth || size.height < initWizardMinimumTerminalHeight {
					expectedAction = "q Cancel"
				}
				assertWizardCommandBarAtBottom(t, view, expectedAction)

				for _, forbidden := range []string{longLiteralValue, "https://api.staging.example.test/with/a/long/path"} {
					if screen.name != "profile-value-entry" && strings.Contains(view, forbidden) {
						t.Fatalf("%s View() = %q, must not contain resolved literal value %q", screen.name, view, forbidden)
					}
				}
			})
		}
	}
}

func resizedWizardModel(t *testing.T, model initWizardModel, width int, height int) initWizardModel {
	t.Helper()

	updatedModel, command := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	if command != nil {
		t.Fatal("command is not nil after window resize")
	}

	return updatedModel.(initWizardModel)
}
