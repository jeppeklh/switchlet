package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestCompactPathForDisplay_PreservesFilenameAndUsefulTail(t *testing.T) {
	path := "/very/long/project/path/with/many/segments/configuration/appsettings.Development.json"

	got := compactPathForDisplay(path, 52)

	if lipgloss.Width(got) > 52 {
		t.Fatalf("compactPathForDisplay() = %q with width %d, want at most 52", got, lipgloss.Width(got))
	}
	if !strings.Contains(got, "appsettings.Development.json") {
		t.Fatalf("compactPathForDisplay() = %q, want filename preserved", got)
	}
	if !strings.HasPrefix(got, "...") {
		t.Fatalf("compactPathForDisplay() = %q, want visible compaction marker", got)
	}
	if strings.Contains(got, "/very/long/project") {
		t.Fatalf("compactPathForDisplay() = %q, must not preserve only the absolute path prefix", got)
	}
}

func TestCompactPathForDisplay_KeepsRelativePathWhenItFits(t *testing.T) {
	path := "backend/appsettings.Development.json"

	got := compactPathForDisplay(path, 80)

	if got != path {
		t.Fatalf("compactPathForDisplay() = %q, want unchanged relative path %q", got, path)
	}
}

func TestView_LongTargetPathPreservesFilenameAcrossMainSurfaces(t *testing.T) {
	targetFile := "/very/long/project/path/with/many/segments/configuration/appsettings.Development.json"
	target := config.Target{File: targetFile, JSONPath: "services.database.primary.connectionStrings.defaultConnection.value"}
	profile := config.Profile{Name: "Production", Value: stringPointer("Server=prod;Database=App;Password=super-secret;"), Protected: true}
	model := New(app.New(target, []config.Profile{profile}))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)

	assertTargetFilenameVisible(t, "list", model.View(), "appsettings.Development.json")
	assertVisibleWidth(t, model.View(), 80)

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	assertTargetFilenameVisible(t, "inspection", model.View(), "appsettings.Development.json")
	assertVisibleWidth(t, model.View(), 80)

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want confirmation before apply")
	}
	assertTargetFilenameVisible(t, "confirmation", model.View(), "appsettings.Development.json")
	assertVisibleWidth(t, model.View(), 80)

	errorModel := model
	errorModel.state = errorState
	errorModel.recoverableError = errorModel.genericRecoverableError(errors.New("target file could not be updated"))
	assertTargetFilenameVisible(t, "error", errorModel.View(), "appsettings.Development.json")
	assertVisibleWidth(t, errorModel.View(), 80)

	successModel := model
	successModel.state = successState
	successModel.successResult = &app.Result{ProfileName: "Production", TargetFile: targetFile, TargetPath: target.JSONPath}
	assertTargetFilenameVisible(t, "success", successModel.View(), "appsettings.Development.json")
	assertVisibleWidth(t, successModel.View(), 80)
}

func TestView_LongYAMLTargetPathPreservesFilenameAcrossMainSurfaces(t *testing.T) {
	targetFile := "/very/long/project/path/with/many/segments/worker/configuration/config.yaml"
	target := config.Target{Name: "workerQueue", File: targetFile, Type: config.TargetTypeYAML, YAMLPath: "services.worker.queue.primary.endpoint.with.many.segments"}
	profile := config.Profile{Name: "Production", Value: stringPointer("Server=prod;Password=super-secret;"), Protected: true}
	model := New(app.New(target, []config.Profile{profile}))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)

	assertTargetFilenameVisible(t, "list", model.View(), "config.yaml")
	assertVisibleWidth(t, model.View(), 80)

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}
	assertTargetFilenameVisible(t, "inspection", model.View(), "config.yaml")
	assertVisibleWidth(t, model.View(), 80)

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want confirmation before apply")
	}
	assertTargetFilenameVisible(t, "confirmation", model.View(), "config.yaml")
	assertVisibleWidth(t, model.View(), 80)

	errorModel := model
	errorModel.state = errorState
	errorModel.recoverableError = errorModel.targetFailureError("Production", app.TargetFailure{
		TargetName:   "workerQueue",
		TargetFile:   targetFile,
		TargetType:   config.TargetTypeYAML,
		SelectorName: "yamlPath",
		Selector:     target.YAMLPath,
		Reason:       "missing segment \"endpoint\"",
	}, nil)
	assertTargetFilenameVisible(t, "error", errorModel.View(), "config.yaml")
	assertVisibleWidth(t, errorModel.View(), 80)

	successModel := model
	successModel.state = successState
	successModel.successResult = &app.Result{
		ProfileName: "Production",
		Changes: []app.PlannedChange{{
			TargetName:   "workerQueue",
			TargetFile:   targetFile,
			TargetType:   config.TargetTypeYAML,
			SelectorName: "yamlPath",
			Selector:     target.YAMLPath,
		}},
	}
	assertTargetFilenameVisible(t, "success", successModel.View(), "config.yaml")
	assertVisibleWidth(t, successModel.View(), 80)
}

func assertTargetFilenameVisible(t *testing.T, surface string, view string, filename string) {
	t.Helper()

	if !strings.Contains(view, filename) {
		t.Fatalf("%s View() = %q, want compacted path to preserve filename", surface, view)
	}
	if strings.Contains(view, "/very/long/project/path/with/many") {
		t.Fatalf("%s View() = %q, must not preserve only the absolute path prefix", surface, view)
	}
}

func TestView_MultiTargetListShowsTargetAwareSummary(t *testing.T) {
	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: "backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: "frontend/.env.local", Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{
			{
				Name: "Database Only",
				Values: []config.ProfileValue{
					{Target: "database", Value: stringPointer("postgres://local")},
				},
			},
			{
				Name:      "Staging",
				Protected: true,
				Values: []config.ProfileValue{
					{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")},
					{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
				},
			},
		},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)

	view := model.View()
	headerLine := visibleLines(view)[0]
	if strings.Contains(headerLine, "2 configured targets") || strings.Contains(headerLine, "Selected: 1 of 2 targets") {
		t.Fatalf("header line = %q, must not duplicate selected-profile target context", headerLine)
	}
	for _, expected := range []string{
		"> Database Only [1 target] [partial]",
		"Changes: 1 of 2 targets",
		"Affected targets",
		"database",
		"Enter: Apply this profile.",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want target-aware list detail %q", view, expected)
		}
	}
}

func TestView_MultiTargetInspectionGroupsTargetsAndMasksValues(t *testing.T) {
	t.Setenv("STAGING_DATABASE_URL", "Server=staging;Database=App;Password=super-secret;")

	databasePath := "/workspace/backend/appsettings.Development.json"
	workerPath := "/workspace/worker/config.yaml"
	frontendPath := "/workspace/frontend/.env.local"
	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "workerQueue", File: workerPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name:      "Staging",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")},
				{Target: "workerQueue", Value: stringPointer("https://queue.staging.example.test")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	))

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when opening inspection")
	}

	view := model.View()
	for _, expected := range []string{
		"Inspect Profile",
		"Changes: 3 targets",
		databasePath,
		"database [json] -> database.url",
		"Environment variable: STAGING_DATABASE_URL",
		"Value: Server=staging;Database=App;Password=****;",
		workerPath,
		"workerQueue [yaml] -> yamlPath: queue.endpoint",
		"Value: https://queue.staging.example.test",
		frontendPath,
		"frontendApi [dotenv] -> VITE_API_URL",
		"Value: https://api.staging.example.test",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want multi-target inspection detail %q", view, expected)
		}
	}
	if strings.Contains(view, "super-secret") {
		t.Fatalf("View() = %q, must not include unmasked secret", view)
	}
}

func TestView_MultiTargetConfirmationListsTargetsWithoutValues(t *testing.T) {
	databasePath := "/workspace/backend/appsettings.Development.json"
	workerPath := "/workspace/worker/config.yaml"
	frontendPath := "/workspace/frontend/.env.local"
	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "workerQueue", File: workerPath, Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name:      "Production",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("Server=prod;Database=App;Password=super-secret;")},
				{Target: "workerQueue", Value: stringPointer("https://queue.production.example.test")},
				{Target: "frontendApi", Value: stringPointer("https://api.production.example.test")},
			},
		}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want confirmation before apply")
	}
	if model.state != confirmState {
		t.Fatalf("state = %d, want confirmState", model.state)
	}

	view := model.View()
	for _, expected := range []string{
		"Apply protected profile?",
		"Changes: 3 targets",
		"This will update configured targets only.",
		"Resolved values are intentionally hidden.",
		databasePath,
		"database [json] -> database.url",
		workerPath,
		"workerQueue [yaml] -> yamlPath: queue.endpoint",
		frontendPath,
		"frontendApi [dotenv] -> VITE_API_URL",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want confirmation detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{"super-secret", "Password=****", "https://queue.production.example.test", "https://api.production.example.test"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain resolved value %q", view, forbidden)
		}
	}
}

func TestUpdate_MultiTargetApplyShowsTargetAwareSuccessAndFinalMessage(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\nVITE_FEATURES=local\n")

	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://staging")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want apply command")
	}

	message := command()
	updatedModel, quitCommand := updatedModel.Update(message)
	model = updatedModel.(Model)
	if quitCommand == nil {
		t.Fatal("quitCommand is nil, want success quit command")
	}
	if model.state != successState {
		t.Fatalf("state = %d, want successState", model.state)
	}

	view := model.View()
	for _, expected := range []string{
		"Applied profile: Staging",
		"Updated targets:",
		"updated",
		"appsettings.Development.json",
		"database [json]",
		"database.url",
		".env.local",
		"frontendApi [dotenv]",
		"VITE_API_URL",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want target-aware success detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{"postgres://staging", "https://api.staging.example.test"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain resolved value %q", view, forbidden)
		}
	}

	finalMessage := model.FinalMessage()
	for _, expected := range []string{
		"Applied profile \"Staging\"",
		"Updated targets:",
		"updated " + databasePath,
		"database [json]",
		"database.url",
		"updated " + frontendPath,
		"frontendApi [dotenv]",
		"VITE_API_URL",
	} {
		if !strings.Contains(finalMessage, expected) {
			t.Fatalf("FinalMessage() = %q, want target-aware final detail %q", finalMessage, expected)
		}
	}
	for _, forbidden := range []string{"postgres://staging", "https://api.staging.example.test"} {
		if strings.Contains(finalMessage, forbidden) {
			t.Fatalf("FinalMessage() = %q, must not contain resolved value %q", finalMessage, forbidden)
		}
	}
	if !strings.Contains(string(readFile(t, databasePath)), "postgres://staging") {
		t.Fatalf("database file was not updated")
	}
	if !strings.Contains(string(readFile(t, frontendPath)), "VITE_API_URL=https://api.staging.example.test") {
		t.Fatalf("frontend file was not updated")
	}
}

func TestUpdate_UnavailableMultiTargetProfileIdentifiesFailingTarget(t *testing.T) {
	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: "backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: "frontend/.env.local", Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", ValueFromEnv: stringPointer("STAGING_DATABASE_URL")},
				{Target: "frontendApi", Value: stringPointer("https://api.staging.example.test")},
			},
		}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no apply command for unavailable profile")
	}
	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}

	view := model.View()
	for _, expected := range []string{"Profile \"Staging\" is unavailable.", "Affected target: database [json] -> database.url", "Environment variable: STAGING_DATABASE_URL", "Reason:"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want unavailable target detail %q", view, expected)
		}
	}
}

func TestUpdate_MultiTargetPreparationErrorShowsTargetContextWithoutResolvedValues(t *testing.T) {
	projectRoot := t.TempDir()
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"postgres://old"}}`)
	frontendPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL=http://localhost:5173\n")
	originalDatabaseContents := readFile(t, databasePath)
	originalFrontendContents := readFile(t, frontendPath)

	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: frontendPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{{
			Name: "Staging",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://database-secret")},
				{Target: "frontendApi", Value: stringPointer("https://api.secret.example.test\nNEXT=value")},
			},
		}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("command is nil, want apply command")
	}

	message := command()
	updatedModel, _ = updatedModel.Update(message)
	model = updatedModel.(Model)
	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}

	view := model.View()
	for _, expected := range []string{
		"Could not prepare target \"frontendApi\".",
		"Context:",
		"Profile: Staging",
		"Target: frontendApi [dotenv]",
		"Selector: VITE_API_URL",
		"Reason:",
		"replacement value must not contain newline",
		"characters",
		"Recovery:",
		"Press any key",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want target-aware error detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{"postgres://database-secret", "api.secret.example.test"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain resolved value %q", view, forbidden)
		}
	}
	if string(readFile(t, databasePath)) != string(originalDatabaseContents) {
		t.Fatal("database target changed after preparation failure")
	}
	if string(readFile(t, frontendPath)) != string(originalFrontendContents) {
		t.Fatal("frontend target changed after preparation failure")
	}
}

func TestUpdate_StartsApplyThroughCommandAndShowsImmediateFeedback(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://new.example.test")}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want apply command")
	}
	if model.applyingProfile != "Local" {
		t.Fatalf("applyingProfile = %q, want Local", model.applyingProfile)
	}
	if strings.Contains(string(readFile(t, targetPath)), "https://new.example.test") {
		t.Fatal("target changed before returned apply command was executed")
	}

	view := model.View()
	for _, expected := range []string{"State: Applying", "Enter: Applying now.", "Ctrl+C Exit immediately"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want immediate apply feedback %q", view, expected)
		}
	}
}

func TestUpdate_IgnoresStaleApplyResult(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"service":{"baseUrl":"https://old.example.test"}}`)
	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://new.example.test")}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want apply command")
	}

	staleMessage := applyCompletedMsg{
		requestID: model.applyRequestID - 1,
		result:    app.Result{ProfileName: "Stale", TargetPath: "stale.path"},
	}
	updatedModel, staleCommand := model.Update(staleMessage)
	model = updatedModel.(Model)
	if staleCommand != nil {
		t.Fatal("staleCommand is not nil, want ignored stale result")
	}
	if model.state != listState || model.successResult != nil || model.applyingProfile != "Local" {
		t.Fatalf("model after stale result = %#v, want in-progress Local apply", model)
	}

	message := command()
	updatedModel, quitCommand := model.Update(message)
	model = updatedModel.(Model)
	if quitCommand == nil {
		t.Fatal("quitCommand is nil, want success quit command")
	}
	if model.state != successState || model.successResult == nil || model.successResult.ProfileName != "Local" {
		t.Fatalf("model after current result = %#v, want Local success", model)
	}
}

func TestView_MultiTargetViewsFitHostileDimensions(t *testing.T) {
	t.Setenv("LONG_DATABASE_URL", "Server=very-long-hostname.example.test;Database=Application;Password=super-secret;")

	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database-with-a-very-long-target-name", File: "/very/long/project/path/backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "services.database.primary.connectionStrings.defaultConnection.value"},
			{Name: "frontend-api-with-a-very-long-target-name", File: "/very/long/project/path/frontend/.env.local", Type: config.TargetTypeDotenv, Key: "VITE_APPLICATION_BACKEND_API_URL"},
		},
		[]config.Profile{{
			Name:      "Production profile with a very long display name",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database-with-a-very-long-target-name", ValueFromEnv: stringPointer("LONG_DATABASE_URL")},
				{Target: "frontend-api-with-a-very-long-target-name", Value: stringPointer("https://api.production.example.test/with/a/long/path")},
			},
		}},
	))

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
		updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		resizedModel := updatedModel.(Model)
		assertVisibleWidth(t, resizedModel.View(), size.width)
		assertVisibleHeight(t, resizedModel.View(), size.height)

		if size.width < minimumTerminalWidth || size.height < minimumTerminalHeight {
			continue
		}

		updatedModel, _ = resizedModel.Update(runeKey('i'))
		inspectionModel := updatedModel.(Model)
		assertVisibleWidth(t, inspectionModel.View(), size.width)
		assertVisibleHeight(t, inspectionModel.View(), size.height)

		updatedModel, _ = inspectionModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
		confirmationModel := updatedModel.(Model)
		assertVisibleWidth(t, confirmationModel.View(), size.width)
		assertVisibleHeight(t, confirmationModel.View(), size.height)
	}
}
