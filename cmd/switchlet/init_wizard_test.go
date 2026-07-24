package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/editor"
)

func TestInitWizardModel_CompletesGuidedFlowWithFilterSearchAndLiteralProfile(t *testing.T) {
	projectRoot := t.TempDir()
	desiredCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json"),
		RelativePath: filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}
	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{
				{Path: filepath.Join(projectRoot, "packages", "one", "config.json"), RelativePath: filepath.Join("packages", "one", "config.json")},
				desiredCandidate,
			}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != desiredCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{
				Name:     "database",
				JSONPath: "database",
				Children: []editor.StringTargetNode{
					{Name: "primary", JSONPath: "database.primary", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.primary.url", Selectable: true}}},
					{Name: "replica", JSONPath: "database.replica", Children: []editor.StringTargetNode{{Name: "url", JSONPath: "database.replica.url", Selectable: true}}},
				},
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = updateWizardModel(t, model, runeKey('f'))
	typeWizardText(t, &model, "appsettings")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("command is not nil, want no quit command while selecting a file")
	}
	model = updatedModel.(initWizardModel)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse step", model.step)
	}

	model = updateWizardModel(t, model, runeKey('s'))
	typeWizardText(t, &model, "replica")

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("command is not nil, want no quit command while selecting a JSON path")
	}
	model = updatedModel.(initWizardModel)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name step", model.step)
	}
	if model.selectedJSONPath != "database.replica.url" {
		t.Fatalf("selectedJSONPath = %q, want %q", model.selectedJSONPath, "database.replica.url")
	}

	typeWizardText(t, &model, "Production")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "postgres://prod")
	model = pressWizardEnter(t, model)
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile summary step", model.step)
	}
	if len(model.profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(model.profiles))
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want review step", model.step)
	}
	if !strings.Contains(model.View(), ".gitignore protection: Enabled") {
		t.Fatalf("View() = %q, want enabled gitignore protection in the review view", model.View())
	}

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want quit command when completing the wizard")
	}
	model = updatedModel.(initWizardModel)
	if model.result == nil {
		t.Fatal("result is nil, want completed init wizard result")
	}
	if model.result.Cancelled {
		t.Fatal("result.Cancelled = true, want completed result")
	}
	if model.result.Target.File != desiredCandidate.Path {
		t.Fatalf("target file = %q, want %q", model.result.Target.File, desiredCandidate.Path)
	}
	if model.result.Target.JSONPath != "database.replica.url" {
		t.Fatalf("target path = %q, want %q", model.result.Target.JSONPath, "database.replica.url")
	}
	if !model.result.ShouldIgnoreConfig {
		t.Fatal("ShouldIgnoreConfig = false, want default literal-value protection")
	}
	if len(model.result.Profiles) != 1 || model.result.Profiles[0].Name != "Production" || !model.result.Profiles[0].Protected {
		t.Fatalf("result profiles = %#v, want one protected Production profile", model.result.Profiles)
	}
}

func TestInitWizardModel_ManualFileAndPathEntryRemainAvailable(t *testing.T) {
	projectRoot := t.TempDir()
	manualTargetPath := filepath.Join(projectRoot, "..", "shared", "config.json")

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return nil, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != filepath.Clean(manualTargetPath) {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{Name: "service", JSONPath: "service", Children: []editor.StringTargetNode{{Name: "baseUrl", JSONPath: "service.baseUrl", Selectable: true}}}}, nil
		},
		validateStringTarget: func(path string, jsonPath string) error {
			if path != filepath.Clean(manualTargetPath) || jsonPath != "service.baseUrl" {
				return fmt.Errorf("unexpected validation %q %q", path, jsonPath)
			}

			return nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	if !strings.Contains(model.View(), "No target JSON files with selectable string values were discovered") {
		t.Fatalf("View() = %q, want empty-discovery guidance", model.View())
	}

	model = updateWizardModel(t, model, runeKey('m'))
	typeWizardText(t, &model, manualTargetPath)
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse step", model.step)
	}
	if model.selectedFile.path != filepath.Clean(manualTargetPath) {
		t.Fatalf("selectedFile.path = %q, want %q", model.selectedFile.path, filepath.Clean(manualTargetPath))
	}

	model = updateWizardModel(t, model, runeKey('m'))
	typeWizardText(t, &model, "service.baseUrl")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name step", model.step)
	}
	if model.selectedJSONPath != "service.baseUrl" {
		t.Fatalf("selectedJSONPath = %q, want %q", model.selectedJSONPath, "service.baseUrl")
	}
}

func TestInitWizardModel_EnvironmentOnlyProfilesSkipGitignoreProtection(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := editor.TargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "Test")
	model = pressWizardEnter(t, model)
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "MYAPP_TEST_URL")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want review step", model.step)
	}
	if strings.Contains(model.View(), ".gitignore protection") {
		t.Fatalf("View() = %q, want no gitignore protection block for env-only profiles", model.View())
	}

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want quit command when completing the wizard")
	}
	model = updatedModel.(initWizardModel)
	if model.result == nil {
		t.Fatal("result is nil, want completed result")
	}
	if model.result.ShouldIgnoreConfig {
		t.Fatal("ShouldIgnoreConfig = true, want false for env-only profiles")
	}
}

func TestInitWizardModel_AllowsDisablingGitignoreProtectionForLiteralProfiles(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := editor.TargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "https://new.example.test")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want review step", model.step)
	}
	if !strings.Contains(model.View(), ".gitignore protection: Enabled") {
		t.Fatalf("View() = %q, want enabled gitignore protection by default", model.View())
	}

	model = updateWizardModel(t, model, runeKey('j'))
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("command is not nil, want no quit command when toggling gitignore protection")
	}
	model = updatedModel.(initWizardModel)
	if !strings.Contains(model.View(), ".gitignore protection: Disabled") {
		t.Fatalf("View() = %q, want disabled gitignore protection after toggling", model.View())
	}

	model = updateWizardModel(t, model, runeKey('k'))
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want quit command when completing the wizard")
	}
	model = updatedModel.(initWizardModel)
	if model.result == nil {
		t.Fatal("result is nil, want completed result")
	}
	if model.result.ShouldIgnoreConfig {
		t.Fatal("ShouldIgnoreConfig = true, want false after disabling gitignore protection")
	}
}

func TestInitWizardModel_FileFilterSupportsEditingInsideTheInput(t *testing.T) {
	projectRoot := t.TempDir()
	desiredCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json"),
		RelativePath: filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{
				{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"},
				desiredCandidate,
			}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != desiredCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = updateWizardModel(t, model, runeKey('f'))
	pasteWizardText(t, &model, "appsettngs")
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyLeft})
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyLeft})
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyLeft})
	model = updateWizardModel(t, model, runeKey('i'))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("command is not nil, want no quit command while selecting a file")
	}
	model = updatedModel.(initWizardModel)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse step", model.step)
	}
	if model.selectedFile.path != desiredCandidate.Path {
		t.Fatalf("selectedFile.path = %q, want %q", model.selectedFile.path, desiredCandidate.Path)
	}
}

func TestInitWizardModel_ProfileValueSupportsEditingPastedText(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := editor.TargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	pasteWizardText(t, &model, "[value]")

	for range 7 {
		model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyLeft})
	}
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyDelete})
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnd})
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyBackspace})

	if !strings.Contains(model.View(), "Literal value: value_") {
		t.Fatalf("View() = %q, want editable pasted value without surrounding brackets", model.View())
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileProtected {
		t.Fatalf("step = %d, want protected-profile step after submitting the edited value", model.step)
	}
	if model.draftProfile.Value != "value" {
		t.Fatalf("draftProfile.Value = %q, want %q", model.draftProfile.Value, "value")
	}
}

func TestInitWizardModel_CanRemoveLastProfileBeforeReview(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := editor.TargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
		inspectStringTargets: func(string) ([]editor.StringTargetNode, error) {
			return []editor.StringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "https://new.example.test")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile summary step", model.step)
	}

	model = updateWizardModel(t, model, runeKey('j'))
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name step after removing the last profile", model.step)
	}
	if len(model.profiles) != 0 {
		t.Fatalf("len(profiles) = %d, want 0 after removing the only profile", len(model.profiles))
	}
}

func pressWizardEnter(t *testing.T, model initWizardModel) initWizardModel {
	t.Helper()
	return updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
}

func typeWizardText(t *testing.T, model *initWizardModel, value string) {
	t.Helper()
	for _, character := range value {
		*model = updateWizardModel(t, *model, runeKey(character))
	}
}

func pasteWizardText(t *testing.T, model *initWizardModel, value string) {
	t.Helper()
	*model = updateWizardModel(t, *model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
}

func updateWizardModel(t *testing.T, model initWizardModel, message tea.KeyMsg) initWizardModel {
	t.Helper()
	updatedModel, _ := model.Update(message)
	return updatedModel.(initWizardModel)
}

func runeKey(value rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}}
}
