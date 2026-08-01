package initwizard

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeppeklh/switchlet/internal/app"
)

func TestInitWizardModel_CompletesGuidedFlowWithFilterSearchAndLiteralProfile(t *testing.T) {
	projectRoot := t.TempDir()
	desiredCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json"),
		RelativePath: filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}
	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{
				{Path: filepath.Join(projectRoot, "packages", "one", "config.json"), RelativePath: filepath.Join("packages", "one", "config.json")},
				desiredCandidate,
			}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != desiredCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{
				Name:     "database",
				JSONPath: "database",
				Children: []app.InitStringTargetNode{
					{Name: "primary", JSONPath: "database.primary", Children: []app.InitStringTargetNode{{Name: "url", JSONPath: "database.primary.url", Selectable: true}}},
					{Name: "replica", JSONPath: "database.replica", Children: []app.InitStringTargetNode{{Name: "url", JSONPath: "database.replica.url", Selectable: true}}},
				},
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = updateWizardModel(t, model, runeKey('f'))
	typeWizardText(t, &model, "appsettings")

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse step", model.step)
	}

	model = updateWizardModel(t, model, runeKey('s'))
	typeWizardText(t, &model, "replica")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
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

	typeWizardText(t, &model, "Production")
	model = pressWizardEnter(t, model)
	chooseProfileSource(t, &model, false, true)
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

func TestInitWizardModel_OneManagedValueProfilesUseVisibleDecisionScreens(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	typeWizardText(t, &model, "database")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed-value-added decision after naming one managed value", model.step)
	}
	for _, expected := range []string{"Managed value added", "Create profiles", "Add another value"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("managed-value-added View() = %q, want %q", model.View(), expected)
		}
	}
	if strings.Contains(model.View(), "Target summary") {
		t.Fatalf("View() = %q, want no mandatory target summary after naming one managed value", model.View())
	}
	model = pressWizardEnter(t, model)

	typeWizardText(t, &model, "Local")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileValueSource {
		t.Fatalf("step = %d, want visible source selection after profile name", model.step)
	}
	for _, expected := range []string{"Choose value source", "Literal value", "Environment variable", "Protected: off"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("source View() = %q, want %q", model.View(), expected)
		}
	}
	if strings.Contains(model.View(), "Tab Options") {
		t.Fatalf("source View() = %q, want no hidden Tab Options path", model.View())
	}
	chooseProfileSource(t, &model, false, false)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.step != initWizardStepProfileValueSource {
		t.Fatalf("step = %d, want value entry Esc to return to source selection", model.step)
	}
	chooseProfileSource(t, &model, false, false)
	typeWizardText(t, &model, "postgres://local")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile-added decision after saving first literal profile", model.step)
	}
	for _, expected := range []string{"Profile added", "Review and create", "Add another profile", "Add another managed value", "Remove last profile"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("profile-added View() = %q, want %q", model.View(), expected)
		}
	}

	addAnotherProfileFromProfileAdded(t, &model)
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileName || !strings.Contains(model.View(), "Profile name must not be empty") {
		t.Fatalf("View() = %q, want empty profile name validation instead of hidden review", model.View())
	}
	typeWizardText(t, &model, "Test")
	model = pressWizardEnter(t, model)
	chooseProfileSource(t, &model, false, false)
	typeWizardText(t, &model, "postgres://test")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile-added decision after second literal profile", model.step)
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want explicit Review and create action to advance to review", model.step)
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
	jsonCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "backend", "settings.json"),
		RelativePath: filepath.Join("backend", "settings.json"),
		Type:         app.InitTargetTypeJSON,
	}
	dotenvCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "frontend", ".env.local"),
		RelativePath: filepath.Join("frontend", ".env.local"),
		Type:         app.InitTargetTypeDotenv,
	}

	newModel := func(t *testing.T) initWizardModel {
		t.Helper()
		model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
			DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
				return []app.InitTargetFileCandidate{jsonCandidate, dotenvCandidate}, nil
			},
			InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
				if path != jsonCandidate.Path {
					return nil, fmt.Errorf("unexpected JSON path %q", path)
				}

				return []app.InitStringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
			},
			InspectDotenvKeys: func(path string) ([]string, error) {
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
		"JSON, YAML, TOML, and dotenv are supported.",
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

func TestInitWizardModel_YAMLTargetSelectionCreatesYAMLTarget(t *testing.T) {
	projectRoot := t.TempDir()
	yamlCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "worker", "config.yaml"),
		RelativePath: filepath.Join("worker", "config.yaml"),
		Type:         app.InitTargetTypeYAML,
	}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{yamlCandidate}, nil
		},
		InspectYAMLStringTargets: func(path string) ([]app.InitYAMLStringTargetNode, error) {
			if path != yamlCandidate.Path {
				return nil, fmt.Errorf("unexpected YAML path %q", path)
			}

			return []app.InitYAMLStringTargetNode{{
				Name:     "queue",
				YAMLPath: "queue",
				Children: []app.InitYAMLStringTargetNode{{Name: "endpoint", YAMLPath: "queue.endpoint", Selectable: true}},
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want YAML path browse", model.step)
	}
	pathBrowseView := model.View()
	for _, expected := range []string{"Detected format: YAML", "* YAML strings", "Search YAML values", "Enter YAML value path manually"} {
		if !strings.Contains(pathBrowseView, expected) {
			t.Fatalf("YAML path browse View() = %q, want %q", pathBrowseView, expected)
		}
	}

	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueName || model.selectedYAMLPath != "queue.endpoint" {
		t.Fatalf("model after YAML value selection = %#v, want managed-value name with selected YAML path", model)
	}
	typeWizardText(t, &model, "workerQueue")
	model = pressWizardEnter(t, model)
	if len(model.targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(model.targets))
	}
	if model.targets[0].Type != app.InitTargetTypeYAML || model.targets[0].YAMLPath != "queue.endpoint" || model.targets[0].JSONPath != "" {
		t.Fatalf("target = %#v, want YAML target with yamlPath only", model.targets[0])
	}
	if !strings.Contains(model.View(), "YAML path: queue.endpoint") {
		t.Fatalf("checkpoint View() = %q, want YAML path summary", model.View())
	}
}

func TestInitWizardModel_FileInspectionRunsThroughCommandWithPendingView(t *testing.T) {
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

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want file inspection command")
	}
	if inspectionCount != 0 {
		t.Fatalf("inspectionCount = %d, want no inspection before command execution", inspectionCount)
	}

	model = updatedModel.(initWizardModel)
	if !model.isPending() {
		t.Fatal("isPending() = false, want pending file inspection")
	}
	for _, expected := range []string{"Inspecting configuration file", "Inspecting config.json.", "Esc Back", "q Cancel", "Ctrl+C Cancel immediately"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if inspectionCount != 1 {
		t.Fatalf("inspectionCount = %d, want one inspection after command execution", inspectionCount)
	}
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse after inspection completes", model.step)
	}
}

func TestInitWizardModel_ManualFileInspectionRunsThroughCommand(t *testing.T) {
	projectRoot := t.TempDir()
	manualTargetPath := filepath.Join(projectRoot, "config.json")
	inspectionCount := 0

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return nil, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			inspectionCount++
			if path != manualTargetPath {
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
	typeWizardText(t, &model, manualTargetPath)
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want manual file inspection command")
	}
	if inspectionCount != 0 {
		t.Fatalf("inspectionCount = %d, want no inspection before command execution", inspectionCount)
	}

	model = updatedModel.(initWizardModel)
	if !model.isPending() || !strings.Contains(model.View(), "Inspecting configuration file") {
		t.Fatalf("model after manual file submit = %#v, want pending inspection view", model)
	}

	model = executeWizardEffectCommand(t, model, command)
	if inspectionCount != 1 {
		t.Fatalf("inspectionCount = %d, want one inspection after command execution", inspectionCount)
	}
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse after manual file inspection completes", model.step)
	}
}

func TestInitWizardModel_ManualSelectorValidationRunsThroughCommands(t *testing.T) {
	projectRoot := t.TempDir()
	jsonTargetPath := filepath.Join(projectRoot, "config.json")
	yamlTargetPath := filepath.Join(projectRoot, "worker.yaml")
	dotenvTargetPath := filepath.Join(projectRoot, ".env")
	jsonValidationCount := 0
	yamlValidationCount := 0
	dotenvValidationCount := 0
	workflow := app.NewInitWorkflow(app.InitWorkflowDependencies{
		ValidateStringTarget: func(path string, jsonPath string) error {
			jsonValidationCount++
			if path != jsonTargetPath || jsonPath != "service.url" {
				return fmt.Errorf("unexpected JSON validation %q %q", path, jsonPath)
			}

			return nil
		},
		ValidateYAMLTarget: func(path string, yamlPath string) error {
			yamlValidationCount++
			if path != yamlTargetPath || yamlPath != "queue.endpoint" {
				return fmt.Errorf("unexpected YAML validation %q %q", path, yamlPath)
			}

			return nil
		},
		ValidateDotenvTarget: func(path string, key string) error {
			dotenvValidationCount++
			if path != dotenvTargetPath || key != "VITE_API_URL" {
				return fmt.Errorf("unexpected dotenv validation %q %q", path, key)
			}

			return nil
		},
	})

	jsonModel := initWizardModel{
		workingDirectory: projectRoot,
		workflow:         workflow,
		step:             initWizardStepManualPath,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			Path:        jsonTargetPath,
			DisplayPath: "config.json",
			TargetType:  app.InitTargetTypeJSON,
		},
	}
	jsonModel.setInputValue("service.url")
	updatedModel, command := jsonModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want JSON selector validation command")
	}
	if jsonValidationCount != 0 {
		t.Fatalf("jsonValidationCount = %d, want no validation before command execution", jsonValidationCount)
	}

	jsonModel = updatedModel.(initWizardModel)
	if !jsonModel.isPending() || !strings.Contains(jsonModel.View(), "Validating JSON value path") {
		t.Fatalf("pending JSON View() = %q, want validation pending view", jsonModel.View())
	}
	jsonModel = executeWizardEffectCommand(t, jsonModel, command)
	if jsonValidationCount != 1 {
		t.Fatalf("jsonValidationCount = %d, want one validation after command execution", jsonValidationCount)
	}
	if jsonModel.step != initWizardStepManagedValueName || jsonModel.selectedJSONPath != "service.url" {
		t.Fatalf("JSON model = %#v, want managed value name step with selected path", jsonModel)
	}

	yamlModel := initWizardModel{
		workingDirectory: projectRoot,
		workflow:         workflow,
		step:             initWizardStepManualPath,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			Path:        yamlTargetPath,
			DisplayPath: "worker.yaml",
			TargetType:  app.InitTargetTypeYAML,
		},
	}
	yamlModel.setInputValue("queue.endpoint")
	updatedModel, command = yamlModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want YAML selector validation command")
	}
	if yamlValidationCount != 0 {
		t.Fatalf("yamlValidationCount = %d, want no validation before command execution", yamlValidationCount)
	}

	yamlModel = updatedModel.(initWizardModel)
	if !yamlModel.isPending() || !strings.Contains(yamlModel.View(), "Validating YAML value path") {
		t.Fatalf("pending YAML View() = %q, want validation pending view", yamlModel.View())
	}
	yamlModel = executeWizardEffectCommand(t, yamlModel, command)
	if yamlValidationCount != 1 {
		t.Fatalf("yamlValidationCount = %d, want one validation after command execution", yamlValidationCount)
	}
	if yamlModel.step != initWizardStepManagedValueName || yamlModel.selectedYAMLPath != "queue.endpoint" {
		t.Fatalf("YAML model = %#v, want managed value name step with selected path", yamlModel)
	}

	dotenvModel := initWizardModel{
		workingDirectory: projectRoot,
		workflow:         workflow,
		step:             initWizardStepManualDotenvKey,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			Path:        dotenvTargetPath,
			DisplayPath: ".env",
			TargetType:  app.InitTargetTypeDotenv,
		},
	}
	dotenvModel.setInputValue("VITE_API_URL")
	updatedModel, command = dotenvModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want dotenv key validation command")
	}
	if dotenvValidationCount != 0 {
		t.Fatalf("dotenvValidationCount = %d, want no validation before command execution", dotenvValidationCount)
	}

	dotenvModel = updatedModel.(initWizardModel)
	if !dotenvModel.isPending() || !strings.Contains(dotenvModel.View(), "Validating dotenv key") {
		t.Fatalf("pending dotenv View() = %q, want validation pending view", dotenvModel.View())
	}
	dotenvModel = executeWizardEffectCommand(t, dotenvModel, command)
	if dotenvValidationCount != 1 {
		t.Fatalf("dotenvValidationCount = %d, want one validation after command execution", dotenvValidationCount)
	}
	if dotenvModel.step != initWizardStepManagedValueName || dotenvModel.selectedDotenvKey != "VITE_API_URL" {
		t.Fatalf("dotenv model = %#v, want managed value name step with selected key", dotenvModel)
	}
}

func TestInitWizardModel_StaleFileInspectionResultIsIgnoredAfterReplacementSelection(t *testing.T) {
	projectRoot := t.TempDir()
	firstCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "first.json"), RelativePath: "first.json", Type: app.InitTargetTypeJSON}
	secondCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "second.json"), RelativePath: "second.json", Type: app.InitTargetTypeJSON}
	inspectedPaths := make([]string, 0, 2)

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{firstCandidate, secondCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			inspectedPaths = append(inspectedPaths, path)
			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	model.width = 120
	model.height = 32

	updatedModel, firstCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if firstCommand == nil {
		t.Fatal("firstCommand is nil, want first inspection command")
	}
	model = updatedModel.(initWizardModel)
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if command != nil {
		t.Fatal("command is not nil, want pending cancellation without command")
	}
	model = updatedModel.(initWizardModel)
	model = updateWizardModel(t, model, runeKey('j'))

	updatedModel, secondCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if secondCommand == nil {
		t.Fatal("secondCommand is nil, want replacement inspection command")
	}
	model = updatedModel.(initWizardModel)

	staleMessage := firstCommand()
	updatedModel, command = model.Update(staleMessage)
	if command != nil {
		t.Fatal("command is not nil, want stale result ignored without command")
	}
	model = updatedModel.(initWizardModel)
	if !model.isPending() || model.pendingEffect.TargetPath != secondCandidate.Path {
		t.Fatalf("model after stale result = %#v, want still-pending second inspection", model)
	}

	model = executeWizardEffectCommand(t, model, secondCommand)
	if len(inspectedPaths) != 2 || inspectedPaths[0] != firstCandidate.Path || inspectedPaths[1] != secondCandidate.Path {
		t.Fatalf("inspectedPaths = %#v, want first then second command execution", inspectedPaths)
	}
	if model.step != initWizardStepPathBrowse || model.selectedFile.Path != secondCandidate.Path {
		t.Fatalf("model after second inspection = %#v, want second file selected", model)
	}
}

func TestInitWizardModel_StaleValidationErrorIsIgnoredAfterBacktrackingAndEditing(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "config.json")
	validationCount := 0
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateStringTarget: func(string, string) error {
				validationCount++
				return fmt.Errorf("old path is invalid")
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: "config.json",
			TargetType:  app.InitTargetTypeJSON,
		},
	}
	model.setInputValue("old.path")

	updatedModel, validationCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if validationCommand == nil {
		t.Fatal("validationCommand is nil, want validation command")
	}
	model = updatedModel.(initWizardModel)
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if command != nil {
		t.Fatal("command is not nil, want pending cancellation without command")
	}
	model = updatedModel.(initWizardModel)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyCtrlU})
	typeWizardText(t, &model, "new.path")

	staleMessage := validationCommand()
	if validationCount != 1 {
		t.Fatalf("validationCount = %d, want stale command to execute once", validationCount)
	}
	updatedModel, command = model.Update(staleMessage)
	if command != nil {
		t.Fatal("command is not nil, want stale validation result ignored without command")
	}
	model = updatedModel.(initWizardModel)
	if !model.errorDetail.IsZero() || model.inputValue != "new.path" || model.step != initWizardStepManualPath {
		t.Fatalf("model after stale validation = %#v, want edited manual path with no error", model)
	}
}

func TestInitWizardModel_FailedInspectionAndValidationReturnToSourceScreens(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json", Type: app.InitTargetTypeJSON}
	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(string) ([]app.InitStringTargetNode, error) {
			return nil, fmt.Errorf("invalid JSON")
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	model.width = 120
	model.height = 32

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want inspection command")
	}
	model = executeWizardEffectCommand(t, updatedModel.(initWizardModel), command)
	if model.step != initWizardStepFileSelect {
		t.Fatalf("step = %d, want file selection after failed inspection", model.step)
	}
	if model.errorDetail.Problem != "Could not inspect configuration file." || !strings.Contains(model.errorDetail.Reason, "invalid JSON") {
		t.Fatalf("errorDetail = %#v, want recoverable inspection error", model.errorDetail)
	}
	if !strings.Contains(model.View(), "Choose configuration file") || !strings.Contains(model.View(), "Error") || !strings.Contains(model.View(), "Recovery:") {
		t.Fatalf("View() = %q, want file selection error view", model.View())
	}

	targetPath := filepath.Join(projectRoot, "manual.json")
	validationModel := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateStringTarget: func(string, string) error {
				return fmt.Errorf("missing string value")
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: "manual.json",
			TargetType:  app.InitTargetTypeJSON,
		},
	}
	validationModel.setInputValue("missing.path")
	updatedModel, command = validationModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want validation command")
	}
	validationModel = executeWizardEffectCommand(t, updatedModel.(initWizardModel), command)
	if validationModel.step != initWizardStepManualPath || validationModel.inputValue != "missing.path" {
		t.Fatalf("validation model = %#v, want manual path with input preserved", validationModel)
	}
	if validationModel.errorDetail.Problem != "Could not use this JSON value." || !strings.Contains(validationModel.errorDetail.Reason, "missing string value") {
		t.Fatalf("errorDetail = %#v, want recoverable validation error", validationModel.errorDetail)
	}
	if !strings.Contains(validationModel.View(), "Enter JSON value path") || !strings.Contains(validationModel.View(), "Error") || !strings.Contains(validationModel.View(), "Selector: missing.path") {
		t.Fatalf("View() = %q, want manual path error view", validationModel.View())
	}

	dotenvValidationModel := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateDotenvTarget: func(string, string) error {
				return fmt.Errorf("missing dotenv key")
			},
		}),
		step:   initWizardStepManualDotenvKey,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        filepath.Join(projectRoot, ".env"),
			DisplayPath: ".env",
			TargetType:  app.InitTargetTypeDotenv,
		},
	}
	dotenvValidationModel.setInputValue("MISSING_KEY")
	updatedModel, command = dotenvValidationModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want dotenv validation command")
	}
	dotenvValidationModel = executeWizardEffectCommand(t, updatedModel.(initWizardModel), command)
	if dotenvValidationModel.step != initWizardStepManualDotenvKey || dotenvValidationModel.inputValue != "MISSING_KEY" {
		t.Fatalf("dotenv validation model = %#v, want manual dotenv key with input preserved", dotenvValidationModel)
	}
	if dotenvValidationModel.errorDetail.Problem != "Could not use this dotenv key." || !strings.Contains(dotenvValidationModel.errorDetail.Reason, "missing dotenv key") {
		t.Fatalf("errorDetail = %#v, want recoverable dotenv validation error", dotenvValidationModel.errorDetail)
	}
	if !strings.Contains(dotenvValidationModel.View(), "Enter dotenv value key") || !strings.Contains(dotenvValidationModel.View(), "Selector: MISSING_KEY") || !strings.Contains(dotenvValidationModel.View(), "Recovery:") {
		t.Fatalf("View() = %q, want manual dotenv key error view", dotenvValidationModel.View())
	}
}

func TestInitWizardModel_JSONBrowseUsesEscInsteadOfSelectableBackRow(t *testing.T) {
	model := initWizardModel{
		workingDirectory: t.TempDir(),
		step:             initWizardStepPathBrowse,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			DisplayPath: "config.json",
			TargetType:  app.InitTargetTypeJSON,
		},
		browseNodes: jsonTargetSelectorNodes([]app.InitStringTargetNode{{
			Name:     "database",
			JSONPath: "database",
			Children: []app.InitStringTargetNode{{Name: "url", JSONPath: "database.url", Selectable: true}},
		}}),
	}

	model = pressWizardEnter(t, model)
	view := model.View()
	if strings.Contains(view, "Back up one level") {
		t.Fatalf("nested JSON browse View() = %q, want Esc command instead of selectable back row", view)
	}
	if !strings.Contains(view, "Esc Back") {
		t.Fatalf("nested JSON browse View() = %q, want command bar back action", view)
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.step != initWizardStepPathBrowse || len(model.browseAncestors) != 0 || len(model.browseNodes) != 1 || model.browseNodes[0].name != "database" {
		t.Fatalf("model after Esc = %#v, want root JSON browse context", model)
	}
}

func TestInitWizardModel_ManagedValueNamingAndCheckpointUsePhaseFourCopy(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json", Type: app.InitTargetTypeJSON}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
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
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed-value-added decision after naming the first managed value", model.step)
	}
	if strings.Contains(model.View(), "Target summary") {
		t.Fatalf("View() = %q, want no mandatory target summary after naming one managed value", model.View())
	}

	checkpointView := model.View()
	for _, expected := range []string{
		"Managed value added",
		"Switchlet now manages this value.",
		"Add another, or create profiles.",
		"Create profiles",
		"Add another value",
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
		t.Fatalf("step = %d, want Add another value to return to file selection", model.step)
	}
}

func TestInitWizardModel_BacktrackingAndCancellationPreservePhaseTwoFlow(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(string) ([]app.InitStringTargetNode, error) {
			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
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

	model, err = newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
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

func TestInitWizardModel_TextEntryScreensPreserveLiteralQAndOmitQCancel(t *testing.T) {
	tests := []struct {
		name         string
		model        initWizardModel
		wantInput    string
		wantViewText string
	}{
		{
			name: "manual file",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepManualFile,
				width:            120,
				height:           32,
			},
			wantInput:    "q",
			wantViewText: "Configuration file: q_",
		},
		{
			name: "file filter",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepFileFilter,
				width:            120,
				height:           32,
			},
			wantInput:    "q",
			wantViewText: "/q_",
		},
		{
			name: "path search",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepPathSearch,
				width:            120,
				height:           32,
				selectedFile: app.InitTargetFileSelection{
					DisplayPath: "config.json",
					TargetType:  app.InitTargetTypeJSON,
				},
			},
			wantInput:    "q",
			wantViewText: "/q_",
		},
		{
			name: "manual JSON path",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepManualPath,
				width:            120,
				height:           32,
				selectedFile: app.InitTargetFileSelection{
					DisplayPath: "config.json",
					TargetType:  app.InitTargetTypeJSON,
				},
			},
			wantInput:    "q",
			wantViewText: "JSON value path: q_",
		},
		{
			name: "manual dotenv key",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepManualDotenvKey,
				width:            120,
				height:           32,
				selectedFile: app.InitTargetFileSelection{
					DisplayPath: ".env",
					TargetType:  app.InitTargetTypeDotenv,
				},
			},
			wantInput:    "q",
			wantViewText: "Dotenv value key: q_",
		},
		{
			name: "managed value name",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepManagedValueName,
				width:            120,
				height:           32,
				selectedFile: app.InitTargetFileSelection{
					DisplayPath: "config.json",
					TargetType:  app.InitTargetTypeJSON,
				},
				selectedJSONPath: "database.url",
			},
			wantInput:    "q",
			wantViewText: "Name: q_",
		},
		{
			name: "profile name",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepProfileName,
				width:            120,
				height:           32,
				targets:          []app.InitTarget{{Name: "database"}},
			},
			wantInput:    "q",
			wantViewText: "Profile name: q_",
		},
		{
			name: "profile value",
			model: initWizardModel{
				workingDirectory: t.TempDir(),
				step:             initWizardStepProfileValue,
				width:            120,
				height:           32,
				targets:          []app.InitTarget{{Name: "database"}},
				draftProfile:     initWizardProfileDraft{Name: "Local", TargetIndex: 0},
			},
			wantInput:    "q",
			wantViewText: "Literal value: q_",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			updatedModel, command := testCase.model.Update(runeKey('q'))
			model := updatedModel.(initWizardModel)

			if command != nil {
				t.Fatal("command is not nil, want literal q input without cancellation")
			}
			if model.result != nil {
				t.Fatalf("result = %#v, want no cancelled result after literal q input", model.result)
			}
			if model.inputValue != testCase.wantInput {
				t.Fatalf("inputValue = %q, want %q", model.inputValue, testCase.wantInput)
			}

			view := model.View()
			if !strings.Contains(view, testCase.wantViewText) {
				t.Fatalf("View() = %q, want literal q input %q", view, testCase.wantViewText)
			}
			if strings.Contains(view, "q Cancel") {
				t.Fatalf("View() = %q, text-entry command bar must not advertise q Cancel", view)
			}
			if isCommandLineSearchStep(model.step) {
				if strings.Contains(view, "Ctrl+C Cancel") {
					t.Fatalf("View() = %q, command-line search must replace command bar guidance", view)
				}
				return
			}

			if !strings.Contains(view, "Ctrl+C Cancel") {
				t.Fatalf("View() = %q, want Ctrl+C cancellation guidance", view)
			}
		})
	}
}

func TestInitWizardModel_FileFilterDoesNotMoveFileRowsDown(t *testing.T) {
	projectRoot := t.TempDir()
	desiredCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json"),
		RelativePath: filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}
	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{
				desiredCandidate,
				{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	model.width = 120
	model.height = 32

	listView := model.View()
	listRowIndex := wizardLineIndexContaining(listView, "> src")
	if listRowIndex < 0 {
		t.Fatalf("View() = %q, want source file row", listView)
	}

	model = updateWizardModel(t, model, runeKey('/'))
	filterView := model.View()
	filterRowIndex := wizardLineIndexContaining(filterView, "> src")
	if filterRowIndex < 0 {
		t.Fatalf("View() = %q, want source file row while filtering", filterView)
	}
	if filterRowIndex != listRowIndex {
		t.Fatalf("file row moved from line %d to %d when filter opened\nlist: %q\nfilter: %q", listRowIndex, filterRowIndex, listView, filterView)
	}
	if strings.Contains(filterView, "Filter:") {
		t.Fatalf("View() = %q, want no filter input in file panel body", filterView)
	}
	assertWizardCommandBarAtBottom(t, filterView, "/_")

	typeWizardText(t, &model, "appsettings")
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	filteredListView := model.View()
	filteredListRowIndex := wizardLineIndexContaining(filteredListView, "> src")
	if filteredListRowIndex < 0 {
		t.Fatalf("View() = %q, want source file row after accepting filter", filteredListView)
	}
	if filteredListRowIndex != listRowIndex {
		t.Fatalf("file row moved from line %d to %d when filter was accepted\nlist: %q\nfiltered list: %q", listRowIndex, filteredListRowIndex, listView, filteredListView)
	}
	if strings.Contains(filteredListView, "Active filter") || strings.Contains(filteredListView, "Filter:") {
		t.Fatalf("View() = %q, want active filter shown without a file panel body prefix", filteredListView)
	}
	if !strings.Contains(filteredListView, `Filter "appsettings"`) {
		t.Fatalf("View() = %q, want active filter in panel title", filteredListView)
	}
}

func TestInitWizardModel_PathSearchDoesNotMovePathRowsDown(t *testing.T) {
	nodes := []app.InitStringTargetNode{
		{Name: "databaseUrl", JSONPath: "database.url", Selectable: true},
		{Name: "redisUrl", JSONPath: "redis.url", Selectable: true},
	}
	model := initWizardModel{
		workingDirectory: t.TempDir(),
		step:             initWizardStepPathBrowse,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			DisplayPath: "config.json",
			TargetType:  app.InitTargetTypeJSON,
			Nodes:       nodes,
		},
		browseNodes: jsonTargetSelectorNodes(nodes),
	}

	model = updateWizardModel(t, model, runeKey('/'))
	searchView := model.View()
	searchPanelTitleIndex := wizardLineIndexContaining(searchView, "* Search results")
	if searchPanelTitleIndex < 0 {
		t.Fatalf("View() = %q, want search results panel title", searchView)
	}
	searchRowIndex := wizardLineIndexContaining(searchView, "> database.url")
	if searchRowIndex < 0 {
		t.Fatalf("View() = %q, want database.url search row", searchView)
	}
	if searchRowIndex != searchPanelTitleIndex+1 {
		t.Fatalf("path search row rendered on line %d, want first body line after panel title %d\nsearch: %q", searchRowIndex, searchPanelTitleIndex+1, searchView)
	}
	if strings.Contains(searchView, "Search:") {
		t.Fatalf("View() = %q, want no search input in path panel body", searchView)
	}
	assertWizardCommandBarAtBottom(t, searchView, "/_")
}

func TestInitWizardModel_CommandBarsDescribeActualEscDestinations(t *testing.T) {
	projectRoot := t.TempDir()
	literalValue := "postgres://local"
	model := initWizardModel{
		workingDirectory: projectRoot,
		step:             initWizardStepManagedValueCheckpoint,
		width:            120,
		height:           32,
		targets: []app.InitTarget{{
			Name:     "database",
			File:     filepath.Join(projectRoot, "config.json"),
			Type:     app.InitTargetTypeJSON,
			JSONPath: "database.url",
		}},
	}

	view := model.View()
	if !strings.Contains(view, "Esc Profiles") || strings.Contains(view, "Esc Back") {
		t.Fatalf("managed-value checkpoint View() = %q, want Esc Profiles copy", view)
	}
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want no command for Esc profile transition")
	}
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name after Esc Profiles", model.step)
	}

	model.step = initWizardStepProfileSummary
	model.profiles = []app.InitProfile{{Name: "Local", Values: []app.InitProfileValue{{Target: "database", Value: &literalValue}}}}
	view = model.View()
	if !strings.Contains(view, "Esc Profiles") || strings.Contains(view, "Esc Back") {
		t.Fatalf("profile summary View() = %q, want Esc Profiles copy", view)
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want no command for Esc profile transition")
	}
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name after Esc Profiles", model.step)
	}

	model.step = initWizardStepReview
	model.profiles = []app.InitProfile{{Name: "Local", Values: []app.InitProfileValue{{Target: "database", Value: &literalValue}}}}
	view = model.View()
	if !strings.Contains(view, "Esc Profiles") || strings.Contains(view, "Esc Back") {
		t.Fatalf("review View() = %q, want Esc Profiles copy", view)
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want no command for Esc profile transition")
	}
	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile-added summary after Esc Profiles", model.step)
	}
}

func TestInitWizardModel_CreatesMultipleTargetsWithDotenvAndPartialProfile(t *testing.T) {
	projectRoot := t.TempDir()
	jsonCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "config.json"),
		RelativePath: "config.json",
		Type:         app.InitTargetTypeJSON,
	}
	dotenvCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "frontend", ".env.local"),
		RelativePath: filepath.Join("frontend", ".env.local"),
		Type:         app.InitTargetTypeDotenv,
	}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{jsonCandidate, dotenvCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != jsonCandidate.Path {
				return nil, fmt.Errorf("unexpected JSON path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "databaseUrl", JSONPath: "database.url", Selectable: true}}, nil
		},
		InspectDotenvKeys: func(path string) ([]string, error) {
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
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed-value-added decision after naming the first managed value", model.step)
	}

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
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed-value-added decision", model.step)
	}
	model = pressWizardEnter(t, model)

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
	chooseProfileSource(t, &model, false, false)
	typeWizardText(t, &model, "postgres://localhost:5432/app")
	model = pressWizardEnter(t, model)
	includeFrontendView := model.View()
	if !strings.Contains(includeFrontendView, "Set frontendApi in Local? No, leave unchanged") {
		t.Fatalf("profile scope View() = %q, want dynamic leave-unchanged choice for frontendApi", includeFrontendView)
	}
	model = updateWizardModel(t, model, runeKey('j'))
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile-added decision", model.step)
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
	if model.result.Targets[1].Type != app.InitTargetTypeDotenv || model.result.Targets[1].Key != "VITE_API_URL" {
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
		targets: []app.InitTarget{
			{Name: "primaryUrl"},
			{Name: "clientBaseUrl"},
		},
		draftProfile: initWizardProfileDraft{Name: "Local", Values: make([]app.InitProfileValue, 0, 2)},
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
	if !strings.Contains(model.View(), "A profile must include at least one manage") {
		t.Fatalf("View() = %q, want all-omitted managed-value error", model.View())
	}
}

func TestInitWizardModel_ProfileScopeBacktrackingRemovesRevisitedValues(t *testing.T) {
	model := initWizardModel{
		workingDirectory: t.TempDir(),
		step:             initWizardStepProfileTargetInclude,
		width:            120,
		height:           32,
		targets: []app.InitTarget{
			{Name: "primaryUrl"},
			{Name: "clientBaseUrl"},
		},
		draftProfile: initWizardProfileDraft{Name: "Local", Values: make([]app.InitProfileValue, 0, 2)},
	}

	model = pressWizardEnter(t, model)
	chooseProfileSource(t, &model, false, false)
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
	chooseProfileSource(t, &model, false, false)
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

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return nil, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != filepath.Clean(manualTargetPath) {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "service", JSONPath: "service", Children: []app.InitStringTargetNode{{Name: "baseUrl", JSONPath: "service.baseUrl", Selectable: true}}}}, nil
		},
		ValidateStringTarget: func(path string, jsonPath string) error {
			if path != filepath.Clean(manualTargetPath) || jsonPath != "service.baseUrl" {
				return fmt.Errorf("unexpected validation %q %q", path, jsonPath)
			}

			return nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	if !strings.Contains(model.View(), "No supported configuration files were discovered.") || !strings.Contains(model.View(), "Need existing JSON/YAML/TOML strings or unique dotenv keys.") {
		t.Fatalf("View() = %q, want empty-discovery guidance", model.View())
	}

	model = updateWizardModel(t, model, runeKey('m'))
	typeWizardText(t, &model, manualTargetPath)
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse step", model.step)
	}
	if model.selectedFile.Path != filepath.Clean(manualTargetPath) {
		t.Fatalf("selectedFile.Path = %q, want %q", model.selectedFile.Path, filepath.Clean(manualTargetPath))
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
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
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
	chooseProfileSource(t, &model, true, false)
	typeWizardText(t, &model, "MYAPP_TEST_URL")
	model = pressWizardEnter(t, model)
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepReview {
		t.Fatalf("step = %d, want review step", model.step)
	}
	if strings.Contains(model.View(), ".gitignore") {
		t.Fatalf("View() = %q, want no gitignore block for env-only profiles", model.View())
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
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
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
	chooseProfileSource(t, &model, false, false)
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
	desiredCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "src", "MyApplication", "appsettings.Development.json"),
		RelativePath: filepath.Join("src", "MyApplication", "appsettings.Development.json"),
	}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{
				{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"},
				desiredCandidate,
			}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != desiredCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
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

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepPathBrowse {
		t.Fatalf("step = %d, want path browse step", model.step)
	}
	if model.selectedFile.Path != desiredCandidate.Path {
		t.Fatalf("selectedFile.Path = %q, want %q", model.selectedFile.Path, desiredCandidate.Path)
	}
}

func TestInitWizardModel_ProfileValueSupportsEditingPastedText(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
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
	chooseProfileSource(t, &model, false, false)
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
	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile-added decision after submitting the edited value", model.step)
	}
	if len(model.profiles) != 1 || len(model.profiles[0].Values) != 1 || model.profiles[0].Values[0].Value == nil || *model.profiles[0].Values[0].Value != "value" {
		t.Fatalf("profiles = %#v, want one profile with one literal value", model.profiles)
	}
}

func TestInitWizardModel_CanRemoveLastManagedValueBeforeReview(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(string) ([]app.InitStringTargetNode, error) {
			return []app.InitStringTargetNode{{Name: "serviceUrl", JSONPath: "serviceUrl", Selectable: true}}, nil
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
	chooseProfileSource(t, &model, false, false)
	typeWizardText(t, &model, "https://new.example.test")
	model = pressWizardEnter(t, model)

	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile-added decision", model.step)
	}

	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
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
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(initWizardModel)
	widView := model.View()
	assertWizardViewHeight(t, widView, 32)
	assertWizardCommandBarAtBottom(t, widView, "q Cancel")
	for _, expected := range []string{"Switchlet init", "Step 1 of 5", "[1 File]", "2 Value", "3 Name", "4 Profiles", "5 Review", "* Configuration files", "> config.json", "Guidance", "Enter Select", "m Manual path"} {
		if !strings.Contains(widView, expected) {
			t.Fatalf("wide View() = %q, want %q", widView, expected)
		}
	}
	if !wizardLineContains(widView, "* Configuration files", "Guidance") {
		t.Fatalf("wide View() = %q, want split file and guidance panels", widView)
	}
	wideLines := strings.Split(strings.TrimSuffix(widView, "\n"), "\n")
	lineBeforeCommandSeparator := wideLines[len(wideLines)-3]
	if !strings.Contains(lineBeforeCommandSeparator, "┘") {
		t.Fatalf("line before command separator = %q, want wide init wizard panels to fill available body height", lineBeforeCommandSeparator)
	}

	updatedModel, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(initWizardModel)
	narrowView := model.View()
	assertWizardViewHeight(t, narrowView, 24)
	assertWizardCommandBarAtBottom(t, narrowView, "q Cancel")
	if wizardLineContains(narrowView, "* Configuration files", "Guidance") {
		t.Fatalf("narrow View() = %q, want stacked panels at minimum width", narrowView)
	}
	if !strings.Contains(narrowView, "* Configuration files") || !strings.Contains(narrowView, "Guidance") {
		t.Fatalf("narrow View() = %q, want both stacked panels", narrowView)
	}
}

func TestInitWizardModel_TooSmallTerminalStateIsSafe(t *testing.T) {
	projectRoot := t.TempDir()
	selectedCandidate := app.InitTargetFileCandidate{Path: filepath.Join(projectRoot, "config.json"), RelativePath: "config.json"}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	model = updatedModel.(initWizardModel)
	view := model.View()
	assertWizardViewWidth(t, view, 79)
	assertWizardViewHeight(t, view, 23)
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
		targets:            []app.InitTarget{{Name: "database", File: filepath.Join(projectRoot, "config.json"), Type: app.InitTargetTypeJSON, JSONPath: "database.primary.url"}},
		shouldIgnoreConfig: true,
		profiles: []app.InitProfile{
			{Name: "Local", Values: []app.InitProfileValue{{Target: "database", Value: &literalValue}}},
			{Name: "Production", Values: []app.InitProfileValue{{Target: "database", ValueFromEnv: &environmentVariableName}}, Protected: true},
		},
	}

	summaryView := model.View()
	if !wizardLineContains(summaryView, "* Next action", "Configured profiles") {
		t.Fatalf("summary View() = %q, want focused next action as the primary panel", summaryView)
	}
	for _, expected := range []string{"Profile added", "Review and create", "Add another profile", "Add another managed value", "Remove last profile", "Local [literal]", "Production [protected] [env]", "database: env MYAPP_PRODUCTION_URL"} {
		if !strings.Contains(summaryView, expected) {
			t.Fatalf("summary View() = %q, want profile badge summary %q", summaryView, expected)
		}
	}
	for _, unexpected := range []string{"Back to targets", "Back to profiles", "Review and create configuration"} {
		if strings.Contains(summaryView, unexpected) {
			t.Fatalf("summary View() = %q, want no %q", summaryView, unexpected)
		}
	}

	model.step = initWizardStepReview
	reviewView := model.View()
	if !wizardLineContains(reviewView, "* Create", "Setup summary") {
		t.Fatalf("review View() = %q, want focused create decision as the primary panel", reviewView)
	}
	for _, expected := range []string{"Step 5 of 5", "1 File", "2 Value", "3 Name", "4 Profiles", "[5 Review]", ".gitignore protection: Enabled", "Create .switchlet.yaml", "Toggle ignore"} {
		if !strings.Contains(reviewView, expected) {
			t.Fatalf("review View() = %q, want %q", reviewView, expected)
		}
	}
	for _, unexpected := range []string{"Configuration summary", "Back to profiles"} {
		if strings.Contains(reviewView, unexpected) {
			t.Fatalf("review View() = %q, want no %q", reviewView, unexpected)
		}
	}
	assertWizardViewWidth(t, reviewView, 120)
}

func TestInitWizardModel_ReviewUsesReplaceCopyWhenOverwriting(t *testing.T) {
	projectRoot := t.TempDir()
	model := initWizardModel{
		workingDirectory:        projectRoot,
		step:                    initWizardStepReview,
		width:                   120,
		height:                  24,
		overwriteExistingConfig: true,
		shouldIgnoreConfig:      false,
		shouldIgnoreConfigSet:   false,
		targets: []app.InitTarget{{
			Name:     "database",
			File:     filepath.Join(projectRoot, "config.json"),
			Type:     app.InitTargetTypeJSON,
			JSONPath: "database.primary.url",
		}},
		profiles: []app.InitProfile{{
			Name:   "Local",
			Values: []app.InitProfileValue{{Target: "database", ValueFromEnv: stringPointer("LOCAL_DATABASE_URL")}},
		}},
	}

	view := model.View()
	for _, expected := range []string{"* Replace", "Ready to replace .switchlet.yaml.", "Replace .switchlet.yaml"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want %q", view, expected)
		}
	}
	if strings.Contains(view, "Create .switchlet.yaml") {
		t.Fatalf("View() = %q, want replacement copy instead of create copy", view)
	}
}

func TestInitWizardModel_ReviewSummarizesScopeWithoutSecretValues(t *testing.T) {
	projectRoot := t.TempDir()
	databaseValue := "Server=db;Database=App;Password=secret;"
	frontendValue := "https://api.staging.example.test/token-secret"
	environmentVariableName := "STAGING_DATABASE_URL"
	model := initWizardModel{
		workingDirectory:   projectRoot,
		step:               initWizardStepReview,
		width:              160,
		height:             24,
		shouldIgnoreConfig: true,
		targets: []app.InitTarget{
			{Name: "database", File: filepath.Join(projectRoot, "backend", "appsettings.Development.json"), Type: app.InitTargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
			{Name: "frontendApi", File: filepath.Join(projectRoot, "frontend", ".env.local"), Type: app.InitTargetTypeDotenv, Key: "VITE_API_URL"},
		},
		profiles: []app.InitProfile{
			{Name: "Local", Values: []app.InitProfileValue{{Target: "database", Value: &databaseValue}}},
			{Name: "Staging", Values: []app.InitProfileValue{{Target: "database", ValueFromEnv: &environmentVariableName}, {Target: "frontendApi", Value: &frontendValue}}, Protected: true},
		},
	}

	view := model.View()
	for _, expected := range []string{
		"Managed values",
		"database [json]",
		"File: backend/appsettings.Development.json",
		"JSON path: ConnectionStrings.DefaultConnection",
		"frontendApi [dotenv]",
		"File: frontend/.env.local",
		"Key: VITE_API_URL",
		"Profiles",
		"Local [partial] [literal]",
		"Scope: 1 of 2 managed values",
		"database: literal",
		"Staging [protected] [mixed]",
		"Scope: 2 of 2 managed values",
		"database: env STAGING_DATABASE_URL",
		"frontendApi: literal",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("review View() = %q, want %q", view, expected)
		}
	}
	for _, forbidden := range []string{databaseValue, frontendValue, "Password=secret", "token-secret", "Back to profiles", "Configuration summary"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("review View() = %q, want no %q", view, forbidden)
		}
	}
	assertWizardViewWidth(t, view, 160)
	assertWizardViewHeight(t, view, 24)
}

func TestInitWizardModel_ReviewOverflowKeepsCommandBarVisible(t *testing.T) {
	projectRoot := t.TempDir()
	model := initWizardModel{
		workingDirectory:   projectRoot,
		step:               initWizardStepReview,
		width:              80,
		height:             24,
		shouldIgnoreConfig: true,
		targets: []app.InitTarget{
			{Name: "database", File: filepath.Join(projectRoot, "backend", "appsettings.Development.json"), Type: app.InitTargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
			{Name: "frontendApi", File: filepath.Join(projectRoot, "frontend", ".env.local"), Type: app.InitTargetTypeDotenv, Key: "VITE_API_URL"},
			{Name: "workerQueue", File: filepath.Join(projectRoot, "worker", "config.json"), Type: app.InitTargetTypeJSON, JSONPath: "queue.endpoint"},
		},
		profiles: []app.InitProfile{
			{Name: "Local", Values: []app.InitProfileValue{{Target: "database", Value: stringPointer("local database")}}},
			{Name: "Staging", Values: []app.InitProfileValue{{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")}, {Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")}}, Protected: true},
			{Name: "Worker", Values: []app.InitProfileValue{{Target: "workerQueue", Value: stringPointer("https://queue.example.test")}}},
		},
	}

	view := model.View()
	assertWizardViewWidth(t, view, 80)
	assertWizardViewHeight(t, view, 24)
	assertWizardCommandBarAtBottom(t, view, "q Cancel")
	if !strings.Contains(view, "... ") {
		t.Fatalf("review View() = %q, want intentional overflow marker", view)
	}
}

func TestInitWizardModel_LongInputAndPathsStayWithinTerminalWidth(t *testing.T) {
	projectRoot := t.TempDir()
	longRelativePath := filepath.Join("services", "backend", "configuration", "very-long-directory-name", "appsettings.Development.json")
	longJSONPath := "services.database.primary.connectionStrings.defaultConnection.value"
	selectedCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, longRelativePath),
		RelativePath: longRelativePath,
	}

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{selectedCandidate}, nil
		},
		InspectStringTargets: func(path string) ([]app.InitStringTargetNode, error) {
			if path != selectedCandidate.Path {
				return nil, fmt.Errorf("unexpected path %q", path)
			}

			return []app.InitStringTargetNode{{Name: longJSONPath, JSONPath: longJSONPath, Selectable: true}}, nil
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
	model.targets = []app.InitTarget{{Name: "database", File: selectedCandidate.Path, Type: app.InitTargetTypeJSON, JSONPath: longJSONPath}}
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

func newTestInitWizardModel(workingDirectory string, dependencies app.InitWorkflowDependencies) (initWizardModel, error) {
	return newInitWizardModel(workingDirectory, app.NewInitWorkflow(dependencies))
}

func stringPointer(value string) *string {
	return &value
}

func pressWizardEnter(t *testing.T, model initWizardModel) initWizardModel {
	t.Helper()
	return updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
}

func nameCurrentTargetAndContinue(t *testing.T, model *initWizardModel, targetName string) {
	t.Helper()
	typeWizardText(t, model, targetName)
	*model = pressWizardEnter(t, *model)
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed-value-added decision after naming managed value", model.step)
	}
	*model = pressWizardEnter(t, *model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name after naming managed value", model.step)
	}
}

func chooseProfileSource(t *testing.T, model *initWizardModel, useEnvironment bool, protected bool) {
	t.Helper()
	if model.step != initWizardStepProfileValueSource {
		t.Fatalf("step = %d, want profile value source step", model.step)
	}

	if protected {
		*model = updateWizardModel(t, *model, runeKey('j'))
		*model = updateWizardModel(t, *model, runeKey('j'))
		*model = pressWizardEnter(t, *model)
		if !model.draftProfile.Protected {
			t.Fatal("draftProfile.Protected = false, want visible protected toggle to enable protection")
		}
	}

	if useEnvironment {
		for model.cursor != 1 {
			*model = updateWizardModel(t, *model, runeKey('k'))
		}
	} else {
		for model.cursor != 0 {
			*model = updateWizardModel(t, *model, runeKey('k'))
		}
	}

	*model = pressWizardEnter(t, *model)
	if model.step != initWizardStepProfileValue {
		t.Fatalf("step = %d, want profile value input after choosing source", model.step)
	}
}

func addAnotherProfileFromProfileAdded(t *testing.T, model *initWizardModel) {
	t.Helper()
	if model.step != initWizardStepProfileSummary {
		t.Fatalf("step = %d, want profile-added decision", model.step)
	}
	*model = updateWizardModel(t, *model, runeKey('j'))
	*model = pressWizardEnter(t, *model)
	if model.step != initWizardStepProfileName {
		t.Fatalf("step = %d, want profile name after choosing add another profile", model.step)
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
	updatedModel, command := model.Update(message)
	return executeWizardEffectCommand(t, updatedModel.(initWizardModel), command)
}

func executeWizardEffectCommand(t *testing.T, model initWizardModel, command tea.Cmd) initWizardModel {
	t.Helper()
	if command == nil {
		return model
	}

	message := command()
	switch message := message.(type) {
	case fileInspectedMsg:
		updatedModel, nextCommand := model.Update(message)
		if nextCommand != nil {
			t.Fatal("effect completion returned an unexpected command")
		}
		return updatedModel.(initWizardModel)
	case jsonSelectorValidatedMsg:
		updatedModel, nextCommand := model.Update(message)
		if nextCommand != nil {
			t.Fatal("effect completion returned an unexpected command")
		}
		return updatedModel.(initWizardModel)
	case yamlSelectorValidatedMsg:
		updatedModel, nextCommand := model.Update(message)
		if nextCommand != nil {
			t.Fatal("effect completion returned an unexpected command")
		}
		return updatedModel.(initWizardModel)
	case tomlSelectorValidatedMsg:
		updatedModel, nextCommand := model.Update(message)
		if nextCommand != nil {
			t.Fatal("effect completion returned an unexpected command")
		}
		return updatedModel.(initWizardModel)
	case dotenvKeyValidatedMsg:
		updatedModel, nextCommand := model.Update(message)
		if nextCommand != nil {
			t.Fatal("effect completion returned an unexpected command")
		}
		return updatedModel.(initWizardModel)
	default:
		return model
	}
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

func wizardLineIndexContaining(view string, value string) int {
	for index, line := range strings.Split(view, "\n") {
		if strings.Contains(line, value) {
			return index
		}
	}

	return -1
}

func isCommandLineSearchStep(step initWizardStep) bool {
	return step == initWizardStepFileFilter || step == initWizardStepPathSearch
}

func assertWizardViewHeight(t *testing.T, view string, height int) {
	t.Helper()

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) != height {
		t.Fatalf("View() rendered %d lines, want %d", len(lines), height)
	}
}

func assertWizardCommandBarAtBottom(t *testing.T, view string, expectedAction string) {
	t.Helper()

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("View() rendered no lines")
	}
	if !strings.Contains(lines[len(lines)-1], expectedAction) {
		t.Fatalf("last line = %q, want command bar action %q", lines[len(lines)-1], expectedAction)
	}
}

func assertWizardViewWidth(t *testing.T, view string, width int) {
	t.Helper()

	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("line %q has width %d, want at most %d", line, lipgloss.Width(line), width)
		}
	}
}
