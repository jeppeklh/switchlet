package initwizard

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

func TestInitWizardModel_TOMLFileSelectionShowsPeerFileTypes(t *testing.T) {
	projectRoot := t.TempDir()
	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{
				{Path: filepath.Join(projectRoot, "backend", "settings.json"), RelativePath: filepath.Join("backend", "settings.json"), Type: app.InitTargetTypeJSON},
				{Path: filepath.Join(projectRoot, "worker", "config.yaml"), RelativePath: filepath.Join("worker", "config.yaml"), Type: app.InitTargetTypeYAML},
				{Path: filepath.Join(projectRoot, "services", "development.toml"), RelativePath: filepath.Join("services", "development.toml"), Type: app.InitTargetTypeTOML},
				{Path: filepath.Join(projectRoot, "frontend", ".env.local"), RelativePath: filepath.Join("frontend", ".env.local"), Type: app.InitTargetTypeDotenv},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	model = resizedWizardModel(t, model, 120, 32)

	view := model.View()
	for _, expected := range []string{
		"Choose configuration file",
		"JSON, YAML, TOML, and dotenv are supported.",
		"File format chooses the value step.",
		"backend/settings.json [json]",
		"worker/config.yaml [yaml]",
		"services/development.toml [toml]",
		"frontend/.env.local [dotenv]",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("file selection View() = %q, want %q", view, expected)
		}
	}
}

func TestInitWizardModel_TOMLFileInspectionRunsThroughCommandWithPendingView(t *testing.T) {
	projectRoot := t.TempDir()
	tomlCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "services", "development.toml"),
		RelativePath: filepath.Join("services", "development.toml"),
		Type:         app.InitTargetTypeTOML,
	}
	inspectionCount := 0

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{tomlCandidate}, nil
		},
		InspectTOMLStringTargets: func(path string) ([]app.InitTOMLStringTargetNode, error) {
			inspectionCount++
			if path != tomlCandidate.Path {
				return nil, fmt.Errorf("unexpected TOML path %q", path)
			}

			return []app.InitTOMLStringTargetNode{{
				Name:     "services",
				TOMLPath: "services",
				Children: []app.InitTOMLStringTargetNode{{Name: "api", TOMLPath: "services.api", Children: []app.InitTOMLStringTargetNode{{Name: "endpoint", TOMLPath: "services.api.endpoint", Selectable: true}}}},
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	model = resizedWizardModel(t, model, 120, 32)

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want TOML file inspection command")
	}
	if inspectionCount != 0 {
		t.Fatalf("inspectionCount = %d, want no inspection before command execution", inspectionCount)
	}

	model = updatedModel.(initWizardModel)
	if !model.isPending() || model.pendingEffect.TargetType != app.InitTargetTypeTOML {
		t.Fatalf("model after TOML file select = %#v, want pending TOML inspection", model)
	}
	for _, expected := range []string{"Inspecting configuration file", "Inspecting services/development.toml.", "Detected format: TOML", "Esc Back", "q Cancel"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending TOML View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if inspectionCount != 1 {
		t.Fatalf("inspectionCount = %d, want one inspection after command execution", inspectionCount)
	}
	if model.step != initWizardStepPathBrowse || model.selectedFile.TargetType != app.InitTargetTypeTOML {
		t.Fatalf("model after TOML inspection = %#v, want TOML path browse", model)
	}
	for _, expected := range []string{"Detected format: TOML", "* TOML strings", "Only existing string values are shown.", "Rows ending in / open nested tables.", "m Manual tomlPath"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("TOML path browse View() = %q, want %q", model.View(), expected)
		}
	}
}

func TestInitWizardModel_TOMLExplicitFileTypeInspectionRunsThroughCommand(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "config.local")
	inspectionCount := 0

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return nil, nil
		},
		InspectTOMLStringTargets: func(path string) ([]app.InitTOMLStringTargetNode, error) {
			inspectionCount++
			if path != targetPath {
				return nil, fmt.Errorf("unexpected TOML path %q", path)
			}

			return []app.InitTOMLStringTargetNode{{Name: "serviceUrl", TOMLPath: "serviceUrl", Selectable: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("newInitWizardModel returned error: %v", err)
	}
	model = resizedWizardModel(t, model, 120, 32)

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
	if !strings.Contains(model.View(), "TOML") {
		t.Fatalf("type selection View() = %q, want TOML choice", model.View())
	}

	model = updateWizardModel(t, model, runeKey('j'))
	model = updateWizardModel(t, model, runeKey('j'))
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(initWizardModel)
	if command == nil {
		t.Fatal("command is nil, want TOML inspection command after choosing explicit type")
	}
	if inspectionCount != 0 {
		t.Fatalf("inspectionCount = %d, want no inspection before command execution", inspectionCount)
	}
	if !model.isPending() || model.pendingEffect.TargetType != app.InitTargetTypeTOML {
		t.Fatalf("model after explicit TOML type = %#v, want pending TOML inspection", model)
	}
	if !strings.Contains(model.View(), "Detected format: TOML") {
		t.Fatalf("pending View() = %q, want TOML format context", model.View())
	}

	model = executeWizardEffectCommand(t, model, command)
	if inspectionCount != 1 {
		t.Fatalf("inspectionCount = %d, want one TOML inspection after command execution", inspectionCount)
	}
	if model.step != initWizardStepPathBrowse || model.selectedFile.TargetType != app.InitTargetTypeTOML {
		t.Fatalf("model after explicit TOML inspection = %#v, want TOML path browse", model)
	}
}

func TestInitWizardModel_TOMLValueSearchSelectsExistingStringPath(t *testing.T) {
	projectRoot := t.TempDir()
	model := initWizardModel{
		workingDirectory: projectRoot,
		step:             initWizardStepPathBrowse,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			Path:        filepath.Join(projectRoot, "services", "development.toml"),
			DisplayPath: filepath.Join("services", "development.toml"),
			TargetType:  app.InitTargetTypeTOML,
			TOMLNodes: []app.InitTOMLStringTargetNode{
				{Name: "services", TOMLPath: "services", Children: []app.InitTOMLStringTargetNode{{Name: "api", TOMLPath: "services.api", Children: []app.InitTOMLStringTargetNode{{Name: "endpoint", TOMLPath: "services.api.endpoint", Selectable: true}}}}},
				{Name: "features", TOMLPath: "features", Children: []app.InitTOMLStringTargetNode{{Name: "defaultMode", TOMLPath: "features.defaultMode", Selectable: true}}},
			},
		},
		browseNodes: tomlTargetSelectorNodes([]app.InitTOMLStringTargetNode{
			{Name: "services", TOMLPath: "services", Children: []app.InitTOMLStringTargetNode{{Name: "api", TOMLPath: "services.api", Children: []app.InitTOMLStringTargetNode{{Name: "endpoint", TOMLPath: "services.api.endpoint", Selectable: true}}}}},
			{Name: "features", TOMLPath: "features", Children: []app.InitTOMLStringTargetNode{{Name: "defaultMode", TOMLPath: "features.defaultMode", Selectable: true}}},
		}),
	}

	model = updateWizardModel(t, model, runeKey('s'))
	typeWizardText(t, &model, "endpoint")
	searchView := model.View()
	for _, expected := range []string{"Search TOML values", "Detected format: TOML", "services.api.endpoint"} {
		if !strings.Contains(searchView, expected) {
			t.Fatalf("TOML search View() = %q, want %q", searchView, expected)
		}
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueName || model.selectedTOMLPath != "services.api.endpoint" || model.selectedJSONPath != "" || model.selectedYAMLPath != "" {
		t.Fatalf("model after TOML search = %#v, want selected TOML path", model)
	}
}

func TestInitWizardModel_TOMLManualValidationErrorShowsStructuredRecovery(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "services", "development.toml")
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateTOMLTarget: func(path string, tomlPath string) error {
				if path != targetPath || tomlPath != "services.api.missing" {
					return fmt.Errorf("unexpected TOML validation %q %q", path, tomlPath)
				}

				return fmt.Errorf("missing segment %q", "missing")
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: filepath.Join("services", "development.toml"),
			TargetType:  app.InitTargetTypeTOML,
		},
	}
	model.setInputValue("services.api.missing")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want TOML selector validation command")
	}
	model = updatedModel.(initWizardModel)
	for _, expected := range []string{"Validating TOML value path", "Detected format: TOML", "TOML path: services.api.missing"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending TOML validation View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if model.step != initWizardStepManualPath || model.inputValue != "services.api.missing" {
		t.Fatalf("model after TOML validation error = %#v, want manual TOML path with input preserved", model)
	}
	if model.errorDetail.Problem != "Could not use this TOML value." || !strings.Contains(model.errorDetail.Reason, `missing segment "missing"`) {
		t.Fatalf("errorDetail = %#v, want TOML validation recovery error", model.errorDetail)
	}
	for _, expected := range []string{"Enter TOML value path", "Error", "File: services/development.toml", "Type: TOML", "Selector: services.api.missing", "Reason:", "Recovery:"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("TOML validation error View() = %q, want %q", model.View(), expected)
		}
	}
}

func TestInitWizardModel_TOMLManualValidationCompletesToManagedValueName(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "services", "development.toml")
	validationCount := 0
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateTOMLTarget: func(path string, tomlPath string) error {
				validationCount++
				if path != targetPath || tomlPath != "services.api.endpoint" {
					return fmt.Errorf("unexpected TOML validation %q %q", path, tomlPath)
				}

				return nil
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: filepath.Join("services", "development.toml"),
			TargetType:  app.InitTargetTypeTOML,
		},
	}
	model.setInputValue("services.api.endpoint")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want TOML selector validation command")
	}
	model = updatedModel.(initWizardModel)
	for _, expected := range []string{"Validating TOML value path", "Detected format: TOML", "TOML path: services.api.endpoint"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending TOML validation View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if validationCount != 1 {
		t.Fatalf("validationCount = %d, want one TOML validation", validationCount)
	}
	if model.step != initWizardStepManagedValueName {
		t.Fatalf("step = %d, want managed value name", model.step)
	}
	if model.selectedTOMLPath != "services.api.endpoint" || model.selectedJSONPath != "" || model.selectedYAMLPath != "" || model.selectedDotenvKey != "" {
		t.Fatalf("selected paths = json %q yaml %q toml %q dotenv %q, want only TOML path", model.selectedJSONPath, model.selectedYAMLPath, model.selectedTOMLPath, model.selectedDotenvKey)
	}
	for _, expected := range []string{"Name this managed value", "Selected file: services/development.toml", "Selected value: services.api.endpoint"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("TOML managed-value name View() = %q, want %q", model.View(), expected)
		}
	}
}

func TestInitWizardModel_StaleTOMLValidationResultIsIgnoredAfterBacktrackingAndEditing(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "development.toml")
	validationCount := 0
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateTOMLTarget: func(string, string) error {
				validationCount++
				return fmt.Errorf("old TOML path is invalid")
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: "development.toml",
			TargetType:  app.InitTargetTypeTOML,
		},
	}
	model.setInputValue("services.old")

	updatedModel, validationCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if validationCommand == nil {
		t.Fatal("validationCommand is nil, want TOML validation command")
	}
	model = updatedModel.(initWizardModel)
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if command != nil {
		t.Fatal("command is not nil, want pending cancellation without command")
	}
	model = updatedModel.(initWizardModel)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyCtrlU})
	typeWizardText(t, &model, "services.new")

	staleMessage := validationCommand()
	if validationCount != 1 {
		t.Fatalf("validationCount = %d, want stale command to execute once", validationCount)
	}
	updatedModel, command = model.Update(staleMessage)
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want stale TOML result ignored")
	}
	if model.step != initWizardStepManualPath || model.inputValue != "services.new" || model.selectedTOMLPath != "" || !model.errorDetail.IsZero() {
		t.Fatalf("model after stale TOML validation = %#v, want edited manual path with no selected TOML path", model)
	}
}

func TestInitWizardModel_TOMLNamingAndReviewSummariesStayManagedValueFocused(t *testing.T) {
	projectRoot := t.TempDir()
	serviceEndpointValue := "https://secret-service.example.test"
	environmentVariableName := "STAGING_DATABASE_URL"
	model := initWizardModel{
		workingDirectory: projectRoot,
		step:             initWizardStepManagedValueName,
		width:            160,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			Path:        filepath.Join(projectRoot, "services", "development.toml"),
			DisplayPath: filepath.Join("services", "development.toml"),
			TargetType:  app.InitTargetTypeTOML,
		},
		selectedTOMLPath: "services.api.endpoint",
	}

	nameView := model.View()
	for _, expected := range []string{"Name this managed value", "Selected file: services/development.toml", "Selected value: services.api.endpoint"} {
		if !strings.Contains(nameView, expected) {
			t.Fatalf("TOML managed-value name View() = %q, want %q", nameView, expected)
		}
	}
	if strings.Contains(nameView, "Target name") {
		t.Fatalf("TOML managed-value name View() = %q, want managed-value language", nameView)
	}

	typeWizardText(t, &model, "serviceEndpoint")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed value checkpoint", model.step)
	}
	checkpointView := model.View()
	for _, expected := range []string{"serviceEndpoint [toml]", "File: services/development.toml", "TOML path: services.api.endpoint"} {
		if !strings.Contains(checkpointView, expected) {
			t.Fatalf("TOML checkpoint View() = %q, want %q", checkpointView, expected)
		}
	}

	model.step = initWizardStepReview
	model.targets = []app.InitTarget{
		{Name: "database", File: filepath.Join(projectRoot, "backend", "appsettings.Development.json"), Type: app.InitTargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
		{Name: "serviceEndpoint", File: filepath.Join(projectRoot, "services", "development.toml"), Type: app.InitTargetTypeTOML, TOMLPath: "services.api.endpoint"},
	}
	model.profiles = []app.InitProfile{
		{Name: "Service Endpoint Only", Values: []app.InitProfileValue{{Target: "serviceEndpoint", Value: &serviceEndpointValue}}},
		{Name: "Staging", Protected: true, Values: []app.InitProfileValue{{Target: "database", ValueFromEnv: &environmentVariableName}, {Target: "serviceEndpoint", Value: &serviceEndpointValue}}},
	}
	model.shouldIgnoreConfig = true

	reviewView := model.View()
	for _, expected := range []string{
		"Managed values",
		"serviceEndpoint [toml]",
		"File: services/development.toml",
		"TOML path: services.api.endpoint",
		"Profiles",
		"Service Endpoint Only [partial] [literal]",
		"serviceEndpoint: literal",
		"Staging [protected] [mixed]",
		"database: env STAGING_DATABASE_URL",
	} {
		if !strings.Contains(reviewView, expected) {
			t.Fatalf("TOML review View() = %q, want %q", reviewView, expected)
		}
	}
	for _, forbidden := range []string{serviceEndpointValue, "secret-service", "targets:", "profiles:", "tomlPath:"} {
		if strings.Contains(reviewView, forbidden) {
			t.Fatalf("TOML review View() = %q, want no %q", reviewView, forbidden)
		}
	}
}

func TestInitWizardModel_ManualTOMLTextEntryPreservesLiteralQAndOmitQCancel(t *testing.T) {
	model := initWizardModel{
		workingDirectory: t.TempDir(),
		step:             initWizardStepManualPath,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			DisplayPath: "development.toml",
			TargetType:  app.InitTargetTypeTOML,
		},
	}

	updatedModel, command := model.Update(runeKey('q'))
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want literal q input without cancellation")
	}
	if model.inputValue != "q" {
		t.Fatalf("inputValue = %q, want q", model.inputValue)
	}
	view := model.View()
	if !strings.Contains(view, "TOML value path: q_") {
		t.Fatalf("manual TOML View() = %q, want literal q input", view)
	}
	if strings.Contains(view, "q Cancel") {
		t.Fatalf("manual TOML View() = %q, text-entry command bar must not advertise q Cancel", view)
	}
	if !strings.Contains(view, "Ctrl+C Cancel") {
		t.Fatalf("manual TOML View() = %q, want Ctrl+C cancellation guidance", view)
	}
}

func TestInitWizardModel_TOMLViewsFitHostileDimensions(t *testing.T) {
	projectRoot := t.TempDir()
	secretServiceValue := "https://secret-service.example.test/with/a/very/long/path"
	longTOMLPath := "services.worker.queues.primary.connection.endpoint"
	screens := []struct {
		name                  string
		model                 initWizardModel
		supportedBottomAction string
	}{
		{
			name: "toml-value-browse",
			model: initWizardModel{
				workingDirectory: projectRoot,
				step:             initWizardStepPathBrowse,
				selectedFile: app.InitTargetFileSelection{
					Path:        filepath.Join(projectRoot, "services", "worker", "configuration", "development.toml"),
					DisplayPath: filepath.Join("services", "worker", "configuration", "development.toml"),
					TargetType:  app.InitTargetTypeTOML,
				},
				browseNodes: []targetSelectorNode{{
					name:       longTOMLPath,
					selector:   longTOMLPath,
					selectable: true,
				}},
			},
			supportedBottomAction: "q Cancel",
		},
		{
			name: "toml-review",
			model: initWizardModel{
				workingDirectory:   projectRoot,
				step:               initWizardStepReview,
				shouldIgnoreConfig: true,
				targets: []app.InitTarget{{
					Name:     "serviceEndpointWithLongName",
					File:     filepath.Join(projectRoot, "services", "worker", "configuration", "development.toml"),
					Type:     app.InitTargetTypeTOML,
					TOMLPath: longTOMLPath,
				}},
				profiles: []app.InitProfile{{
					Name:   "Local service endpoint profile with a long name",
					Values: []app.InitProfileValue{{Target: "serviceEndpointWithLongName", Value: &secretServiceValue}},
				}},
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
				if strings.Contains(view, secretServiceValue) || strings.Contains(view, "secret-service") {
					t.Fatalf("%s View() = %q, must not contain resolved literal TOML value", screen.name, view)
				}
			})
		}
	}
}
