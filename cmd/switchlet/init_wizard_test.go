package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeppeklh/switchlet/internal/config"
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
	if model.step != initWizardStepManagedValueName {
		t.Fatalf("step = %d, want managed value name step", model.step)
	}
	if model.selectedJSONPath != "database.replica.url" {
		t.Fatalf("selectedJSONPath = %q, want %q", model.selectedJSONPath, "database.replica.url")
	}
	nameCurrentTargetAndContinue(t, &model, "database")
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name step", model.step)
	}

	typeWizardText(t, &model, "Production")
	model = pressWizardEnter(t, model)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyTab})
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	typeWizardText(t, &model, "postgres://prod")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want review step", model.step)
	}
	if len(model.profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(model.profiles))
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
	if len(model.result.Targets) != 1 {
		t.Fatalf("len(result.Targets) = %d, want 1", len(model.result.Targets))
	}
	if model.result.Targets[0].File != desiredCandidate.Path {
		t.Fatalf("target file = %q, want %q", model.result.Targets[0].File, desiredCandidate.Path)
	}
	if model.result.Targets[0].Name != "database" {
		t.Fatalf("target name = %q, want database", model.result.Targets[0].Name)
	}
	if model.result.Targets[0].JSONPath != "database.replica.url" {
		t.Fatalf("target path = %q, want %q", model.result.Targets[0].JSONPath, "database.replica.url")
	}
	if !model.result.ShouldIgnoreConfig {
		t.Fatal("ShouldIgnoreConfig = false, want default literal-value protection")
	}
	if len(model.result.Profiles) != 1 || model.result.Profiles[0].Name != "Production" || !model.result.Profiles[0].Protected {
		t.Fatalf("result profiles = %#v, want one protected Production profile", model.result.Profiles)
	}
}

func TestInitWizardModel_OneManagedValueLiteralProfilesUseFastPath(t *testing.T) {
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

			return []editor.StringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	nameCurrentTargetAndContinue(t, &model, "database")
	if strings.Contains(model.View(), "Target summary") {
		t.Fatalf("View() = %q, want no mandatory target summary after naming one managed value", model.View())
	}

	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileValue {
		t.Fatalf("step = %d, want profile value directly after profile name", model.step)
	}
	if strings.Contains(model.View(), "Choose profile source") || strings.Contains(model.View(), "Protected profile confirmation") {
		t.Fatalf("View() = %q, want literal and unprotected defaults without choice screens", model.View())
	}
	typeWizardText(t, &model, "postgres://local")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want next profile name after saving first literal profile", model.step)
	}

	typeWizardText(t, &model, "Test")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileValue {
		t.Fatalf("step = %d, want profile value directly for second profile", model.step)
	}
	typeWizardText(t, &model, "postgres://test")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name loop after second literal profile", model.step)
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want blank profile name to advance to review after profiles exist", model.step)
	}
	if len(model.profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2", len(model.profiles))
	}
	for _, profile := range model.profiles {
		if profile.Protected {
			t.Fatalf("profile %#v is protected, want unprotected default", profile)
		}
		if len(profile.Values) != 1 || profile.Values[0].Value == nil {
			t.Fatalf("profile %#v should contain one literal value", profile)
		}
	}
}

func TestInitWizardModel_FileAndValueSelectionUsePhaseThreeCopy(t *testing.T) {
	projectRoot := t.TempDir()
	jsonCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "backend", "settings.json"),
		RelativePath: filepath.Join("backend", "settings.json"),
		Type:         config.TargetTypeJSON,
	}
	dotenvCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "frontend", ".env.local"),
		RelativePath: filepath.Join("frontend", ".env.local"),
		Type:         config.TargetTypeDotenv,
	}

	newModel := func(t *testing.T) initWizardModel {
		t.Helper()
		model, err := newInitWizardModel(projectRoot, initDependencies{
			discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
				return []editor.TargetFileCandidate{jsonCandidate, dotenvCandidate}, nil
			},
			inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
				if path != jsonCandidate.Path {
					return nil, fmt.Errorf("unexpected JSON path %q", path)
				}

				return []editor.StringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
			},
			inspectDotenvKeys: func(path string) ([]string, error) {
				if path != dotenvCandidate.Path {
					return nil, fmt.Errorf("unexpected dotenv path %q", path)
				}

				return []string{"VITE_API_URL", "VITE_FEATURES"}, nil
			},
		})
		if err != nil {
			t.Fatalf("newInitWizardModel returned error: %v", err)
		}

		updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
		return updatedModel.(initWizardModel)
	}

	model := newModel(t)
	fileView := model.View()
	for _, expected := range []string{
		"Choose configuration file",
		"JSON and dotenv files are both supported.",
		"File format chooses the value step.",
		"backend/settings.json [json]",
		"frontend/.env.local [dotenv]",
		"m Manual path",
	} {
		if !strings.Contains(fileView, expected) {
			t.Fatalf("file selection View() = %q, want %q", fileView, expected)
		}
	}

	model = pressWizardEnter(t, model)
	jsonValueView := model.View()
	for _, expected := range []string{
		"Choose value",
		"* JSON strings",
		"Detected format: JSON",
		"Only existing string values are shown.",
		"Switchlet does not create missing values.",
		"m Manual path",
	} {
		if !strings.Contains(jsonValueView, expected) {
			t.Fatalf("JSON value View() = %q, want %q", jsonValueView, expected)
		}
	}
	model = updateWizardModel(t, model, runeKey('m'))
	manualJSONView := model.View()
	if !strings.Contains(manualJSONView, "Enter JSON value path") || !strings.Contains(manualJSONView, "JSON value path") {
		t.Fatalf("manual JSON View() = %q, want JSON value path fallback", manualJSONView)
	}

	model = newModel(t)
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	dotenvValueView := model.View()
	for _, expected := range []string{
		"Choose value",
		"* Dotenv keys",
		"Detected format: dotenv",
		"Existing unique keys only.",
		"Switchlet does not create missing keys.",
		"m Manual key",
	} {
		if !strings.Contains(dotenvValueView, expected) {
			t.Fatalf("dotenv value View() = %q, want %q", dotenvValueView, expected)
		}
	}
	model = updateWizardModel(t, model, runeKey('m'))
	manualDotenvView := model.View()
	if !strings.Contains(manualDotenvView, "Enter dotenv value key") || !strings.Contains(manualDotenvView, "Dotenv value key") {
		t.Fatalf("manual dotenv View() = %q, want dotenv value key fallback", manualDotenvView)
	}
}

func TestInitWizardModel_ManagedValueNamingAndCheckpointUsePhaseFourCopy(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := editor.TargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json", Type: config.TargetTypeJSON}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(initWizardModel)

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	nameView := model.View()
	for _, expected := range []string{
		"Name this managed value",
		"* Name",
		"Selected file: config.json",
		"Selected value: database.url",
		"Profiles refer to this short name.",
		"Examples: database, frontendApi, redisUrl",
	} {
		if !strings.Contains(nameView, expected) {
			t.Fatalf("managed-value name View() = %q, want %q", nameView, expected)
		}
	}
	if strings.Contains(nameView, "Target name") {
		t.Fatalf("managed-value name View() = %q, want no unexplained target-name copy", nameView)
	}

	typeWizardText(t, &model, "database")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name after naming the first managed value", model.step)
	}
	if strings.Contains(model.View(), "Target summary") {
		t.Fatalf("View() = %q, want no mandatory target summary after naming one managed value", model.View())
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	checkpointView := model.View()
	for _, expected := range []string{
		"Managed values",
		"Switchlet now manages these values.",
		"Add another, or create profiles.",
		"Create profiles",
		"Add value",
		"Remove value",
		"database [json]",
		"File: config.json",
		"JSON path: database.url",
	} {
		if !strings.Contains(checkpointView, expected) {
			t.Fatalf("managed-values checkpoint View() = %q, want %q", checkpointView, expected)
		}
	}
	if strings.Contains(checkpointView, "Target summary") || strings.Contains(checkpointView, "Configuration summary") {
		t.Fatalf("managed-values checkpoint View() = %q, want no raw target/configuration summary copy", checkpointView)
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want Create profiles to return to profile entry", model.step)
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepFileSelect {
		t.Fatalf("step = %d, want Add value to return to file selection", model.step)
	}
}

func TestInitWizardModel_BacktrackingAndCancellationPreservePhaseTwoFlow(t *testing.T) {
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
	nameCurrentTargetAndContinue(t, &model, "service")
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed-values checkpoint after backing out of profile entry", model.step)
	}
	if len(model.targets) != 1 || model.targets[0].Name != "service" {
		t.Fatalf("targets = %#v, want configured managed value preserved", model.targets)
	}
	checkpointModel := model
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want Esc from optional checkpoint to return to profile entry", model.step)
	}

	updatedModel, command := checkpointModel.Update(runeKey('q'))
	if command == nil {
		t.Fatal("command is nil, want quit command for q cancellation")
	}
	model = updatedModel.(initWizardModel)
	if model.result == nil || !model.result.Cancelled {
		t.Fatalf("result = %#v, want cancelled result", model.result)
	}

	model, err = newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if command == nil {
		t.Fatal("command is nil, want quit command for Ctrl+C cancellation")
	}
	model = updatedModel.(initWizardModel)
	if model.result == nil || !model.result.Cancelled {
		t.Fatalf("result = %#v, want cancelled result", model.result)
	}
}

func TestInitWizardModel_CreatesMultipleTargetsWithDotenvAndPartialProfile(t *testing.T) {
	projectRoot := t.TempDir()
	jsonCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "config.json"),
		RelativePath: "config.json",
		Type:         config.TargetTypeJSON,
	}
	dotenvCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, "frontend", ".env.local"),
		RelativePath: filepath.Join("frontend", ".env.local"),
		Type:         config.TargetTypeDotenv,
	}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{jsonCandidate, dotenvCandidate}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != jsonCandidate.Path {
				return nil, fmt.Errorf("unexpected JSON path %q", path)
			}

			return []editor.StringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
		},
		inspectDotenvKeys: func(path string) ([]string, error) {
			if path != dotenvCandidate.Path {
				return nil, fmt.Errorf("unexpected dotenv path %q", path)
			}

			return []string{"VITE_API_URL", "VITE_FEATURES"}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want JSON path browse", model.step)
	}
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "database")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name after naming the first managed value", model.step)
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepFileSelect {
		t.Fatalf("step = %d, want file select for second target", model.step)
	}
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepDotenvKeySelect {
		t.Fatalf("step = %d, want dotenv key selection", model.step)
	}
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueName {
		t.Fatalf("step = %d, want managed value name after dotenv key", model.step)
	}

	typeWizardText(t, &model, "database")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueName {
		t.Fatalf("step = %d, want duplicate name to keep managed value name step", model.step)
	}
	if !strings.Contains(model.View(), `managed value name "database" is already configured`) {
		t.Fatalf("View() = %q, want duplicate managed-value-name error", model.View())
	}
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyCtrlU})
	typeWizardText(t, &model, "frontendApi")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name", model.step)
	}

	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	includeDatabaseView := model.View()
	for _, expected := range []string{
		"Values in Local",
		"Set database in Local? Yes",
		"Set database in Local? No, leave unchanged",
		"Managed value: database",
	} {
		if !strings.Contains(includeDatabaseView, expected) {
			t.Fatalf("profile scope View() = %q, want %q", includeDatabaseView, expected)
		}
	}
	if strings.Contains(includeDatabaseView, "Include this target") || strings.Contains(includeDatabaseView, "Omit this target") {
		t.Fatalf("profile scope View() = %q, want managed-value include language", includeDatabaseView)
	}
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "postgres://localhost:5432/app")
	model = pressWizardEnter(t, model)
	includeFrontendView := model.View()
	if !strings.Contains(includeFrontendView, "Set frontendApi in Local? No, leave unchanged") {
		t.Fatalf("profile scope View() = %q, want dynamic leave-unchanged choice for frontendApi", includeFrontendView)
	}
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile summary", model.step)
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want review", model.step)
	}
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want quit command when completing the wizard")
	}
	model = updatedModel.(initWizardModel)
	if model.result == nil {
		t.Fatal("result is nil, want completed init wizard result")
	}
	if len(model.result.Targets) != 2 {
		t.Fatalf("len(result.Targets) = %d, want 2", len(model.result.Targets))
	}
	if model.result.Targets[1].Type != config.TargetTypeDotenv || model.result.Targets[1].Key != "VITE_API_URL" {
		t.Fatalf("second target = %#v, want dotenv target with VITE_API_URL", model.result.Targets[1])
	}
	if len(model.result.Profiles) != 1 || len(model.result.Profiles[0].Values) != 1 {
		t.Fatalf("result profiles = %#v, want one partial profile with one value", model.result.Profiles)
	}
	if model.result.Profiles[0].Values[0].Target != "database" {
		t.Fatalf("profile values = %#v, want database-only partial profile", model.result.Profiles[0].Values)
	}
	if !model.result.ShouldIgnoreConfig {
		t.Fatal("ShouldIgnoreConfig = false, want default literal-value protection")
	}
}

func TestInitWizardModel_ProfileScopeUsesManagedValueNamesAndRejectsOmittingAll(t *testing.T) {
	model := initWizardModel{
		workingDirectory: t.TempDir(),
		step:             initWizardStepProfileTargetInclude,
		width:            120,
		height:           32,
		targets: []config.Target{
			{Name: "primaryUrl"},
			{Name: "clientBaseUrl"},
		},
		draftProfile: initWizardProfileDraft{Name: "Local", Values: make([]config.ProfileValue, 0, 2)},
	}

	view := model.View()
	for _, expected := range []string{
		"Set primaryUrl in Local? Yes",
		"Set primaryUrl in Local? No, leave unchanged",
		"Managed value: primaryUrl",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("profile scope View() = %q, want %q", view, expected)
		}
	}
	if strings.Contains(view, "database") || strings.Contains(view, "frontendApi") {
		t.Fatalf("profile scope View() = %q, want names from wizard state only", view)
	}

	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	if model.draftProfile.TargetIndex != 1 {
		t.Fatalf("TargetIndex = %d, want second managed value", model.draftProfile.TargetIndex)
	}
	view = model.View()
	if !strings.Contains(view, "Set clientBaseUrl in Local? No, leave unchanged") {
		t.Fatalf("profile scope View() = %q, want second managed value leave-unchanged choice", view)
	}

	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileTargetInclude || model.draftProfile.TargetIndex != 0 {
		t.Fatalf("step = %d, TargetIndex = %d, want first include decision after omitting all", model.step, model.draftProfile.TargetIndex)
	}
	if !strings.Contains(model.View(), "a profile must include at least one managed") {
		t.Fatalf("View() = %q, want all-omitted managed-value error", model.View())
	}
}

func TestInitWizardModel_ProfileScopeBacktrackingRemovesRevisitedValues(t *testing.T) {
	model := initWizardModel{
		workingDirectory: t.TempDir(),
		step:             initWizardStepProfileTargetInclude,
		width:            120,
		height:           32,
		targets: []config.Target{
			{Name: "primaryUrl"},
			{Name: "clientBaseUrl"},
		},
		draftProfile: initWizardProfileDraft{Name: "Local", Values: make([]config.ProfileValue, 0, 2)},
	}

	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "https://primary.example.test")
	model = pressWizardEnter(t, model)
	if len(model.draftProfile.Values) != 1 || model.draftProfile.Values[0].Target != "primaryUrl" {
		t.Fatalf("draft values = %#v, want primaryUrl before backtracking", model.draftProfile.Values)
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if len(model.draftProfile.Values) != 0 {
		t.Fatalf("draft values = %#v, want revisited values removed after backtracking", model.draftProfile.Values)
	}

	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "https://client.example.test")
	model = pressWizardEnter(t, model)

	if len(model.profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(model.profiles))
	}
	if len(model.profiles[0].Values) != 1 || model.profiles[0].Values[0].Target != "clientBaseUrl" {
		t.Fatalf("profile values = %#v, want only clientBaseUrl after changing inclusion", model.profiles[0].Values)
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
	if !strings.Contains(model.View(), "No supported configuration files were discovered.") || !strings.Contains(model.View(), "Need existing JSON strings or unique dotenv keys.") {
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
	if model.step != initWizardStepManagedValueName {
		t.Fatalf("step = %d, want managed value name step", model.step)
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
	nameCurrentTargetAndContinue(t, &model, "service")
	typeWizardText(t, &model, "Test")
	model = pressWizardEnter(t, model)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyTab})
	model = pressWizardEnter(t, model)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	typeWizardText(t, &model, "MYAPP_TEST_URL")
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
	nameCurrentTargetAndContinue(t, &model, "service")
	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "https://new.example.test")
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
	nameCurrentTargetAndContinue(t, &model, "service")
	typeWizardText(t, &model, "Local")
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
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want next profile name step after submitting the edited value", model.step)
	}
	if len(model.profiles) != 1 || len(model.profiles[0].Values) != 1 || model.profiles[0].Values[0].Value == nil || *model.profiles[0].Values[0].Value != "value" {
		t.Fatalf("profiles = %#v, want one profile with one literal value", model.profiles)
	}
}

func TestInitWizardModel_CanRemoveLastManagedValueBeforeReview(t *testing.T) {
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
	nameCurrentTargetAndContinue(t, &model, "service")
	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "https://new.example.test")
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name step", model.step)
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	model = updateWizardModel(t, model, runeKey('j'))
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepFileSelect {
		t.Fatalf("step = %d, want file selection after removing the only managed value", model.step)
	}
	if len(model.targets) != 0 {
		t.Fatalf("len(targets) = %d, want 0 after removing the only managed value", len(model.targets))
	}
	if len(model.profiles) != 0 {
		t.Fatalf("len(profiles) = %d, want 0 after removing values that referenced the removed managed value", len(model.profiles))
	}
}

func TestInitWizardModel_UsesSharedShellAndResponsivePanels(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := editor.TargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(initWizardModel)
	widView := model.View()
	for _, expected := range []string{"Switchlet init", "Step 1 of 5", "[1 File]", "2 Value", "3 Name", "4 Profiles", "5 Review", "* Configuration files", "> config.json", "Guidance", "Enter Select", "m Manual path"} {
		if !strings.Contains(widView, expected) {
			t.Fatalf("wide View() = %q, want %q", widView, expected)
		}
	}
	if !wizardLineContains(widView, "* Configuration files", "Guidance") {
		t.Fatalf("wide View() = %q, want split file and guidance panels", widView)
	}

	updatedModel, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(initWizardModel)
	narrowView := model.View()
	if wizardLineContains(narrowView, "* Configuration files", "Guidance") {
		t.Fatalf("narrow View() = %q, want stacked panels at minimum width", narrowView)
	}
	if !strings.Contains(narrowView, "* Configuration files") || !strings.Contains(narrowView, "Guidance") {
		t.Fatalf("narrow View() = %q, want both stacked panels", narrowView)
	}
}

func TestInitWizardModel_TooSmallTerminalStateIsSafe(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := editor.TargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	model = updatedModel.(initWizardModel)
	view := model.View()
	assertWizardViewWidth(t, view, 79)
	for _, expected := range []string{"Terminal too small.", "Resize required", "Minimum size: 80x24", "Current size: 79x23"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("too-small View() = %q, want %q", view, expected)
		}
	}

	updatedModel, command := model.Update(runeKey('q'))
	if command == nil {
		t.Fatal("command is nil, want quit command for q cancellation in too-small state")
	}
	model = updatedModel.(initWizardModel)
	if model.result == nil || !model.result.Cancelled {
		t.Fatalf("result = %#v, want cancelled result", model.result)
	}
}

func TestInitWizardModel_ProfileSummaryAndReviewKeepFocusedTaskPrimary(t *testing.T) {
	projectRoot := t.TempDir()
	literalValue := "postgres://local"
	environmentVariableName := "MYAPP_PRODUCTION_URL"
	model := initWizardModel{
		workingDirectory:   projectRoot,
		step:               initWizardStepProfileSummary,
		width:              120,
		height:             32,
		targets:            []config.Target{{Name: "database", File: filepath.Join(projectRoot, "config.json"), Type: config.TargetTypeJSON, JSONPath: "database.primary.url"}},
		shouldIgnoreConfig: true,
		profiles: []config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: &literalValue}}},
			{Name: "Production", Values: []config.ProfileValue{{Target: "database", ValueFromEnv: &environmentVariableName}}, Protected: true},
		},
	}

	summaryView := model.View()
	if !wizardLineContains(summaryView, "* Next action", "Configured profiles") {
		t.Fatalf("summary View() = %q, want focused next action as the primary panel", summaryView)
	}
	for _, expected := range []string{"Local [literal]", "Production [protected] [env]", "database: env MYAPP_PRODUCTION_URL"} {
		if !strings.Contains(summaryView, expected) {
			t.Fatalf("summary View() = %q, want profile badge summary %q", summaryView, expected)
		}
	}

	model.step = initWizardStepReview
	reviewView := model.View()
	if !wizardLineContains(reviewView, "* Create", "Configuration summary") {
		t.Fatalf("review View() = %q, want focused create decision as the primary panel", reviewView)
	}
	for _, expected := range []string{"Step 5 of 5", "1 File", "2 Value", "3 Name", "4 Profiles", "[5 Review]", ".gitignore protection: Enabled", "Create .switchlet.yaml"} {
		if !strings.Contains(reviewView, expected) {
			t.Fatalf("review View() = %q, want %q", reviewView, expected)
		}
	}
	assertWizardViewWidth(t, reviewView, 120)
}

func TestInitWizardModel_LongInputAndPathsStayWithinTerminalWidth(t *testing.T) {
	projectRoot := t.TempDir()
	longRelativePath := filepath.Join("services", "backend", "configuration", "very-long-directory-name", "appsettings.Development.json")
	longJSONPath := "services.database.primary.connectionStrings.defaultConnection.value"
	selectedCandidate := editor.TargetFileCandidate{
		Path:         filepath.Join(projectRoot, longRelativePath),
		RelativePath: longRelativePath,
	}

	model, err := newInitWizardModel(projectRoot, initDependencies{
		discoverTargetFileCandidates: func(string) ([]editor.TargetFileCandidate, error) {
			return []editor.TargetFileCandidate{selectedCandidate}, nil
		},
		inspectStringTargets: func(path string) ([]editor.StringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []editor.StringTargetNode{{Name: longJSONPath, JSONPath: longJSONPath, Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(initWizardModel)
	assertWizardViewWidth(t, model.View(), 80)
	model = pressWizardEnter(t, model)
	pathView := model.View()
	assertWizardViewWidth(t, pathView, 80)
	if !strings.Contains(pathView, "Detected format: JSON") {
		t.Fatalf("View() = %q, want detected JSON format in value-selection context", pathView)
	}

	model.step = initWizardStepProfileValue
	model.targets = []config.Target{{Name: "database", File: selectedCandidate.Path, Type: config.TargetTypeJSON, JSONPath: longJSONPath}}
	model.draftProfile = initWizardProfileDraft{Name: "Local", TargetIndex: 0}
	model.setInputValue("postgres://very-long-host-name.example.test/database-name-with-a-long-suffix")

	view := model.View()
	assertWizardViewWidth(t, view, 80)
	if !strings.Contains(view, "* Value") {
		t.Fatalf("View() = %q, want focused value input panel", view)
	}
	if !strings.Contains(view, "Literal value: ...") {
		t.Fatalf("View() = %q, want deliberately truncated long input field", view)
	}
	if !strings.Contains(view, "_") {
		t.Fatalf("View() = %q, want visible cursor for long input field", view)
	}
}

func pressWizardEnter(t *testing.T, model initWizardModel) initWizardModel {
	t.Helper()
	return updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
}

func nameCurrentTargetAndContinue(t *testing.T, model *initWizardModel, targetName string) {
	t.Helper()
	typeWizardText(t, model, targetName)
	*model = pressWizardEnter(t, *model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name after naming managed value", model.step)
	}
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

func wizardLineContains(view string, values ...string) bool {
	for _, line := range strings.Split(view, "\n") {
		containsAllValues := true
		for _, value := range values {
			if !strings.Contains(line, value) {
				containsAllValues = false
				break
			}
		}
		if containsAllValues {
			return true
		}
	}

	return false
}

func assertWizardViewWidth(t *testing.T, view string, width int) {
	t.Helper()

	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("line %q has width %d, want at most %d", line, lipgloss.Width(line), width)
		}
	}
}
