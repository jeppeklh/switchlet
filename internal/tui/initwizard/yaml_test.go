package initwizard

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

func TestInitWizardModel_YAMLFileSelectionShowsPeerFileTypes(t *testing.T) {
	projectRoot := t.TempDir()
	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{
				{Path: filepath.Join(projectRoot, "backend", "settings.json"), RelativePath: filepath.Join("backend", "settings.json"), Type: app.InitTargetTypeJSON},
				{Path: filepath.Join(projectRoot, "worker", "config.yaml"), RelativePath: filepath.Join("worker", "config.yaml"), Type: app.InitTargetTypeYAML},
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
		"JSON, YAML, and dotenv files are supported.",
		"File format chooses the value step.",
		"backend/settings.json [json]",
		"worker/config.yaml [yaml]",
		"frontend/.env.local [dotenv]",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("file selection View() = %q, want %q", view, expected)
		}
	}
}

func TestInitWizardModel_YAMLFileInspectionRunsThroughCommandWithPendingView(t *testing.T) {
	projectRoot := t.TempDir()
	yamlCandidate := app.InitTargetFileCandidate{
		Path:         filepath.Join(projectRoot, "worker", "config.yaml"),
		RelativePath: filepath.Join("worker", "config.yaml"),
		Type:         app.InitTargetTypeYAML,
	}
	inspectionCount := 0

	model, err := newTestInitWizardModel(projectRoot, app.InitWorkflowDependencies{
		DiscoverTargetFileCandidates: func(string) ([]app.InitTargetFileCandidate, error) {
			return []app.InitTargetFileCandidate{yamlCandidate}, nil
		},
		InspectYAMLStringTargets: func(path string) ([]app.InitYAMLStringTargetNode, error) {
			inspectionCount++
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
	model = resizedWizardModel(t, model, 120, 32)

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want YAML file inspection command")
	}
	if inspectionCount != 0 {
		t.Fatalf("inspectionCount = %d, want no inspection before command execution", inspectionCount)
	}

	model = updatedModel.(initWizardModel)
	if !model.isPending() || model.pendingEffect.TargetType != app.InitTargetTypeYAML {
		t.Fatalf("model after YAML file select = %#v, want pending YAML inspection", model)
	}
	for _, expected := range []string{"Inspecting configuration file", "Inspecting worker/config.yaml.", "Detected format: YAML", "Esc Back", "q Cancel"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending YAML View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if inspectionCount != 1 {
		t.Fatalf("inspectionCount = %d, want one inspection after command execution", inspectionCount)
	}
	if model.step != initWizardStepPathBrowse || model.selectedFile.TargetType != app.InitTargetTypeYAML {
		t.Fatalf("model after YAML inspection = %#v, want YAML path browse", model)
	}
	for _, expected := range []string{"Detected format: YAML", "* YAML strings", "Only existing string values are shown.", "Rows ending in / open nested mappings.", "m Manual yamlPath"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("YAML path browse View() = %q, want %q", model.View(), expected)
		}
	}
}

func TestInitWizardModel_YAMLValueSearchSelectsExistingStringPath(t *testing.T) {
	projectRoot := t.TempDir()
	model := initWizardModel{
		workingDirectory: projectRoot,
		step:             initWizardStepPathBrowse,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			Path:        filepath.Join(projectRoot, "worker", "config.yaml"),
			DisplayPath: filepath.Join("worker", "config.yaml"),
			TargetType:  app.InitTargetTypeYAML,
			YAMLNodes: []app.InitYAMLStringTargetNode{
				{Name: "queue", YAMLPath: "queue", Children: []app.InitYAMLStringTargetNode{{Name: "endpoint", YAMLPath: "queue.endpoint", Selectable: true}}},
				{Name: "services", YAMLPath: "services", Children: []app.InitYAMLStringTargetNode{{Name: "worker", YAMLPath: "services.worker", Children: []app.InitYAMLStringTargetNode{{Name: "baseUrl", YAMLPath: "services.worker.baseUrl", Selectable: true}}}}},
			},
		},
		browseNodes: yamlTargetSelectorNodes([]app.InitYAMLStringTargetNode{
			{Name: "queue", YAMLPath: "queue", Children: []app.InitYAMLStringTargetNode{{Name: "endpoint", YAMLPath: "queue.endpoint", Selectable: true}}},
			{Name: "services", YAMLPath: "services", Children: []app.InitYAMLStringTargetNode{{Name: "worker", YAMLPath: "services.worker", Children: []app.InitYAMLStringTargetNode{{Name: "baseUrl", YAMLPath: "services.worker.baseUrl", Selectable: true}}}}},
		}),
	}

	model = updateWizardModel(t, model, runeKey('s'))
	typeWizardText(t, &model, "base")
	searchView := model.View()
	for _, expected := range []string{"Search YAML values", "Detected format: YAML", "services.worker.baseUrl"} {
		if !strings.Contains(searchView, expected) {
			t.Fatalf("YAML search View() = %q, want %q", searchView, expected)
		}
	}

	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueName || model.selectedYAMLPath != "services.worker.baseUrl" || model.selectedJSONPath != "" {
		t.Fatalf("model after YAML search = %#v, want selected YAML path", model)
	}
}

func TestInitWizardModel_YAMLManualValidationErrorShowsStructuredRecovery(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "worker", "config.yaml")
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateYAMLTarget: func(path string, yamlPath string) error {
				if path != targetPath || yamlPath != "queue.missing" {
					return fmt.Errorf("unexpected YAML validation %q %q", path, yamlPath)
				}

				return fmt.Errorf("missing segment %q", "missing")
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: filepath.Join("worker", "config.yaml"),
			TargetType:  app.InitTargetTypeYAML,
		},
	}
	model.setInputValue("queue.missing")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want YAML selector validation command")
	}
	model = updatedModel.(initWizardModel)
	for _, expected := range []string{"Validating YAML value path", "Detected format: YAML", "YAML path: queue.missing"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending YAML validation View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if model.step != initWizardStepManualPath || model.inputValue != "queue.missing" {
		t.Fatalf("model after YAML validation error = %#v, want manual YAML path with input preserved", model)
	}
	if model.errorDetail.Problem != "Could not use this YAML value." || !strings.Contains(model.errorDetail.Reason, `missing segment "missing"`) {
		t.Fatalf("errorDetail = %#v, want YAML validation recovery error", model.errorDetail)
	}
	for _, expected := range []string{"Enter YAML value path", "Error", "File: worker/config.yaml", "Type: YAML", "Selector: queue.missing", "Reason:", "Recovery:"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("YAML validation error View() = %q, want %q", model.View(), expected)
		}
	}
}

func TestInitWizardModel_YAMLManualValidationCompletesToManagedValueName(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "worker", "config.yaml")
	validationCount := 0
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateYAMLTarget: func(path string, yamlPath string) error {
				validationCount++
				if path != targetPath || yamlPath != "queue.endpoint" {
					return fmt.Errorf("unexpected YAML validation %q %q", path, yamlPath)
				}

				return nil
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: filepath.Join("worker", "config.yaml"),
			TargetType:  app.InitTargetTypeYAML,
		},
	}
	model.setInputValue("queue.endpoint")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want YAML selector validation command")
	}
	model = updatedModel.(initWizardModel)
	for _, expected := range []string{"Validating YAML value path", "Detected format: YAML", "YAML path: queue.endpoint"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("pending YAML validation View() = %q, want %q", model.View(), expected)
		}
	}

	model = executeWizardEffectCommand(t, model, command)
	if validationCount != 1 {
		t.Fatalf("validationCount = %d, want one YAML validation", validationCount)
	}
	if model.step != initWizardStepManagedValueName {
		t.Fatalf("step = %d, want managed value name", model.step)
	}
	if model.selectedYAMLPath != "queue.endpoint" || model.selectedJSONPath != "" || model.selectedDotenvKey != "" {
		t.Fatalf("selected paths = json %q yaml %q dotenv %q, want only YAML path", model.selectedJSONPath, model.selectedYAMLPath, model.selectedDotenvKey)
	}
	for _, expected := range []string{"Name this managed value", "Selected file: worker/config.yaml", "Selected value: queue.endpoint"} {
		if !strings.Contains(model.View(), expected) {
			t.Fatalf("YAML managed-value name View() = %q, want %q", model.View(), expected)
		}
	}
}

func TestInitWizardModel_StaleYAMLValidationResultIsIgnoredAfterBacktrackingAndEditing(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := filepath.Join(projectRoot, "worker.yaml")
	validationCount := 0
	model := initWizardModel{
		workingDirectory: projectRoot,
		workflow: app.NewInitWorkflow(app.InitWorkflowDependencies{
			ValidateYAMLTarget: func(string, string) error {
				validationCount++
				return fmt.Errorf("old YAML path is invalid")
			},
		}),
		step:   initWizardStepManualPath,
		width:  120,
		height: 32,
		selectedFile: app.InitTargetFileSelection{
			Path:        targetPath,
			DisplayPath: "worker.yaml",
			TargetType:  app.InitTargetTypeYAML,
		},
	}
	model.setInputValue("queue.old")

	updatedModel, validationCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if validationCommand == nil {
		t.Fatal("validationCommand is nil, want YAML validation command")
	}
	model = updatedModel.(initWizardModel)
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if command != nil {
		t.Fatal("command is not nil, want pending cancellation without command")
	}
	model = updatedModel.(initWizardModel)
	model = updateWizardModel(t, model, tea.KeyMsg{Type: tea.KeyCtrlU})
	typeWizardText(t, &model, "queue.new")

	staleMessage := validationCommand()
	if validationCount != 1 {
		t.Fatalf("validationCount = %d, want stale command to execute once", validationCount)
	}
	updatedModel, command = model.Update(staleMessage)
	model = updatedModel.(initWizardModel)
	if command != nil {
		t.Fatal("command is not nil, want stale YAML result ignored")
	}
	if model.step != initWizardStepManualPath || model.inputValue != "queue.new" || model.selectedYAMLPath != "" || !model.errorDetail.IsZero() {
		t.Fatalf("model after stale YAML validation = %#v, want edited manual path with no selected YAML path", model)
	}
}

func TestInitWizardModel_YAMLNamingAndReviewSummariesStayManagedValueFocused(t *testing.T) {
	projectRoot := t.TempDir()
	queueValue := "https://secret-queue.example.test"
	environmentVariableName := "STAGING_DATABASE_URL"
	model := initWizardModel{
		workingDirectory: projectRoot,
		step:             initWizardStepManagedValueName,
		width:            160,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			Path:        filepath.Join(projectRoot, "worker", "config.yaml"),
			DisplayPath: filepath.Join("worker", "config.yaml"),
			TargetType:  app.InitTargetTypeYAML,
		},
		selectedYAMLPath: "queue.endpoint",
	}

	nameView := model.View()
	for _, expected := range []string{"Name this managed value", "Selected file: worker/config.yaml", "Selected value: queue.endpoint"} {
		if !strings.Contains(nameView, expected) {
			t.Fatalf("YAML managed-value name View() = %q, want %q", nameView, expected)
		}
	}
	if strings.Contains(nameView, "Target name") {
		t.Fatalf("YAML managed-value name View() = %q, want managed-value language", nameView)
	}

	typeWizardText(t, &model, "workerQueue")
	model = pressWizardEnter(t, model)
	if model.step != initWizardStepManagedValueCheckpoint {
		t.Fatalf("step = %d, want managed value checkpoint", model.step)
	}
	checkpointView := model.View()
	for _, expected := range []string{"workerQueue [yaml]", "File: worker/config.yaml", "YAML path: queue.endpoint"} {
		if !strings.Contains(checkpointView, expected) {
			t.Fatalf("YAML checkpoint View() = %q, want %q", checkpointView, expected)
		}
	}

	model.step = initWizardStepReview
	model.targets = []app.InitTarget{
		{Name: "database", File: filepath.Join(projectRoot, "backend", "appsettings.Development.json"), Type: app.InitTargetTypeJSON, JSONPath: "ConnectionStrings.DefaultConnection"},
		{Name: "workerQueue", File: filepath.Join(projectRoot, "worker", "config.yaml"), Type: app.InitTargetTypeYAML, YAMLPath: "queue.endpoint"},
	}
	model.profiles = []app.InitProfile{
		{Name: "Worker Queue Only", Values: []app.InitProfileValue{{Target: "workerQueue", Value: &queueValue}}},
		{Name: "Staging", Protected: true, Values: []app.InitProfileValue{{Target: "database", ValueFromEnv: &environmentVariableName}, {Target: "workerQueue", Value: &queueValue}}},
	}
	model.shouldIgnoreConfig = true

	reviewView := model.View()
	for _, expected := range []string{
		"Managed values",
		"workerQueue [yaml]",
		"File: worker/config.yaml",
		"YAML path: queue.endpoint",
		"Profiles",
		"Worker Queue Only [partial] [literal]",
		"workerQueue: literal",
		"Staging [protected] [mixed]",
		"database: env STAGING_DATABASE_URL",
	} {
		if !strings.Contains(reviewView, expected) {
			t.Fatalf("YAML review View() = %q, want %q", reviewView, expected)
		}
	}
	for _, forbidden := range []string{queueValue, "secret-queue", "targets:", "profiles:", "yamlPath:"} {
		if strings.Contains(reviewView, forbidden) {
			t.Fatalf("YAML review View() = %q, want no %q", reviewView, forbidden)
		}
	}
}

func TestInitWizardModel_ManualYAMLTextEntryPreservesLiteralQAndOmitQCancel(t *testing.T) {
	model := initWizardModel{
		workingDirectory: t.TempDir(),
		step:             initWizardStepManualPath,
		width:            120,
		height:           32,
		selectedFile: app.InitTargetFileSelection{
			DisplayPath: "worker.yaml",
			TargetType:  app.InitTargetTypeYAML,
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
	if !strings.Contains(view, "YAML value path: q_") {
		t.Fatalf("manual YAML View() = %q, want literal q input", view)
	}
	if strings.Contains(view, "q Cancel") {
		t.Fatalf("manual YAML View() = %q, text-entry command bar must not advertise q Cancel", view)
	}
	if !strings.Contains(view, "Ctrl+C Cancel") {
		t.Fatalf("manual YAML View() = %q, want Ctrl+C cancellation guidance", view)
	}
}

func TestInitWizardModel_YAMLViewsFitHostileDimensions(t *testing.T) {
	projectRoot := t.TempDir()
	secretQueueValue := "https://secret-queue.example.test/with/a/very/long/path"
	longYAMLPath := "services.worker.queues.primary.connection.endpoint"
	screens := []struct {
		name                  string
		model                 initWizardModel
		supportedBottomAction string
	}{
		{
			name: "yaml-value-browse",
			model: initWizardModel{
				workingDirectory: projectRoot,
				step:             initWizardStepPathBrowse,
				selectedFile: app.InitTargetFileSelection{
					Path:        filepath.Join(projectRoot, "services", "worker", "configuration", "config.yaml"),
					DisplayPath: filepath.Join("services", "worker", "configuration", "config.yaml"),
					TargetType:  app.InitTargetTypeYAML,
				},
				browseNodes: []targetSelectorNode{{
					name:       longYAMLPath,
					selector:   longYAMLPath,
					selectable: true,
				}},
			},
			supportedBottomAction: "q Cancel",
		},
		{
			name: "yaml-review",
			model: initWizardModel{
				workingDirectory:   projectRoot,
				step:               initWizardStepReview,
				shouldIgnoreConfig: true,
				targets: []app.InitTarget{{
					Name:     "workerQueueWithLongName",
					File:     filepath.Join(projectRoot, "services", "worker", "configuration", "config.yaml"),
					Type:     app.InitTargetTypeYAML,
					YAMLPath: longYAMLPath,
				}},
				profiles: []app.InitProfile{{
					Name:   "Local worker queue profile with a long name",
					Values: []app.InitProfileValue{{Target: "workerQueueWithLongName", Value: &secretQueueValue}},
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
				if strings.Contains(view, secretQueueValue) || strings.Contains(view, "secret-queue") {
					t.Fatalf("%s View() = %q, must not contain resolved literal YAML value", screen.name, view)
				}
			})
		}
	}
}
