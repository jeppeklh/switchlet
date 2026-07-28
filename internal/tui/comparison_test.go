package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestUpdate_StatusActionRequestsComparisonRefreshesAndIgnoresStaleResults(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)
	configPath := writeTargetFile(t, projectRoot, ".switchlet.yaml", "version: 3\n")
	originalContents := readFile(t, targetPath)
	originalMode := fileMode(t, targetPath)
	originalConfigContents := readFile(t, configPath)
	originalConfigMode := fileMode(t, configPath)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{
			Name: "Local",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer("postgres://local")},
			},
		}},
	))

	updatedModel, command := model.Update(runeKey('s'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want status comparison command")
	}
	if model.state != statusLoadingState {
		t.Fatalf("state = %d, want statusLoadingState", model.state)
	}
	if model.comparisonRequestKind != comparisonRequestStatus || model.statusComparison != nil {
		t.Fatalf("comparison request = %d/%#v, want active status loading without result", model.comparisonRequestKind, model.statusComparison)
	}
	if !strings.Contains(model.View(), "Checking current managed values") || !strings.Contains(model.View(), "No files will be modified") {
		t.Fatalf("View() = %q, want immediate status loading feedback", model.View())
	}
	firstRequestID := model.comparisonRequestID

	updatedModel, refreshCommand := model.Update(runeKey('r'))
	model = updatedModel.(Model)
	if refreshCommand == nil {
		t.Fatal("refreshCommand is nil, want refreshed status comparison command")
	}
	if model.state != statusLoadingState || model.comparisonRequestID <= firstRequestID {
		t.Fatalf("state/requestID = %d/%d, want refreshed status loading with new request", model.state, model.comparisonRequestID)
	}

	staleMessage := command()
	updatedModel, staleCommand := model.Update(staleMessage)
	model = updatedModel.(Model)
	if staleCommand != nil {
		t.Fatal("staleCommand is not nil, want stale status result ignored")
	}
	if model.state != statusLoadingState || model.statusComparison != nil {
		t.Fatalf("model after stale status result = %#v, want current loading state unchanged", model)
	}

	message := refreshCommand()
	updatedModel, readyCommand := model.Update(message)
	model = updatedModel.(Model)
	if readyCommand != nil {
		t.Fatal("readyCommand is not nil, want no command after status result")
	}
	if model.state != statusReadyState || model.statusComparison == nil {
		t.Fatalf("model after status result = %#v, want status ready result", model)
	}
	if model.statusComparison.CurrentProfile != "Local" {
		t.Fatalf("CurrentProfile = %q, want Local", model.statusComparison.CurrentProfile)
	}
	if !strings.Contains(model.View(), "Current profile: Local") {
		t.Fatalf("View() = %q, want ready status feedback", model.View())
	}
	assertFileUnchanged(t, targetPath, originalContents, originalMode)
	assertNoTargetTempFile(t, targetPath)
	assertFileUnchanged(t, configPath, originalConfigContents, originalConfigMode)
}

func TestView_StatusExactMatchRendersCurrentProfileAndTargetsWithoutValues(t *testing.T) {
	projectRoot := t.TempDir()
	currentValue := "postgres://local-current-secret"
	targetPath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"`+currentValue+`"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{
			Name: "Local",
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer(currentValue)},
			},
		}},
	))

	model = openStatusReady(t, model)
	view := model.View()
	for _, expected := range []string{
		"Current profile: Local",
		"State: exact complete match",
		"Matched targets: 1 of 1",
		"database [json]",
		"File:",
		"jsonPath: database.url",
		"No files were modified.",
		"r Refresh",
		"Esc/q Return",
		"Ctrl+C Exit immediately",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want status detail %q", view, expected)
		}
	}
	if strings.Contains(view, currentValue) {
		t.Fatalf("View() = %q, must not contain raw current or resolved value", view)
	}
}

func TestView_StatusAmbiguousMatchRendersMatchesWithoutChoosingCurrentProfile(t *testing.T) {
	projectRoot := t.TempDir()
	currentValue := "postgres://shared-secret"
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"`+currentValue+`"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer(currentValue)}}},
			{Name: "Local Copy", Protected: true, Values: []config.ProfileValue{{Target: "database", Value: stringPointer(currentValue)}}},
		},
	))

	model = openStatusReady(t, model)
	view := model.View()
	for _, expected := range []string{
		"State: multiple complete profiles match",
		"Complete matches: 2",
		"Matches",
		"Local",
		"Local Copy",
		"[protected]",
		"Matched targets",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want ambiguous status detail %q", view, expected)
		}
	}
	if strings.Contains(view, "Current profile:") {
		t.Fatalf("View() = %q, must not choose one current profile for ambiguous status", view)
	}
	if strings.Contains(view, currentValue) {
		t.Fatalf("View() = %q, must not contain raw current or resolved value", view)
	}
}

func TestView_StatusUnmatchedRendersPartialClosestAndUnavailableProfilesSafely(t *testing.T) {
	projectRoot := t.TempDir()
	databaseCurrentValue := "postgres://current-database-secret"
	apiCurrentValue := "https://current-api-secret.example.test"
	apiStagingValue := "https://staging-api-secret.example.test"
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"`+databaseCurrentValue+`"}}`)
	apiPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL="+apiCurrentValue+"\n")
	environmentVariableName := "SWITCHLET_STATUS_TEST_EMPTY_DATABASE_URL"
	t.Setenv(environmentVariableName, "")

	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: apiPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
		},
		[]config.Profile{
			{Name: "Database only", Values: []config.ProfileValue{{Target: "database", Value: stringPointer(databaseCurrentValue)}}},
			{Name: "Almost", Values: []config.ProfileValue{{Target: "database", Value: stringPointer(databaseCurrentValue)}, {Target: "frontendApi", Value: stringPointer(apiStagingValue)}}},
			{Name: "Secret", Values: []config.ProfileValue{{Target: "database", ValueFromEnv: stringPointer(environmentVariableName)}, {Target: "frontendApi", Value: stringPointer(apiCurrentValue)}}},
		},
	))

	model = openStatusReady(t, model)
	view := model.View()
	for _, expected := range []string{
		"State: no complete profile match",
		"Configured targets: 2",
		"Partial matches",
		"Database only - 1/1 included match; 1 omitted",
		"Closest profiles",
		"Almost - 1/2 targets match",
		"Unavailable profiles",
		"Secret / database [json]",
		environmentVariableName,
		"Reason: profile \"Secret\" value for target \"database\"",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want unmatched status detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{databaseCurrentValue, apiCurrentValue, apiStagingValue} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain raw current or resolved value %q", view, forbidden)
		}
	}
}

func TestView_StatusScreenContainsLongContentAtHostileDimensions(t *testing.T) {
	model := comparisonKeyTestModel()
	model.state = statusReadyState
	model.statusComparison = &app.StatusComparison{
		Status:         app.StatusComparisonMatched,
		CurrentProfile: "Production profile with a very long display name",
		Matches:        []app.ProfileMatch{{ProfileName: "Production profile with a very long display name", Protected: true}},
		MatchedTargets: []app.TargetDescriptor{{
			TargetName:   "database-with-a-very-long-target-name",
			TargetFile:   "/very/long/project/path/backend/appsettings.Development.json",
			TargetType:   config.TargetTypeJSON,
			SelectorName: "jsonPath",
			Selector:     "services.database.primary.connectionStrings.defaultConnection.value",
		}},
		TargetCount: 1,
		Complete:    true,
	}

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
		t.Run(sizeLabel(size.width, size.height), func(t *testing.T) {
			resizedModel := resizedMainModel(t, model, size.width, size.height)
			view := resizedModel.View()
			assertVisibleWidth(t, view, size.width)
			assertVisibleHeight(t, view, size.height)
			if size.width < minimumTerminalWidth || size.height < minimumTerminalHeight {
				if !strings.Contains(view, "Terminal too small") {
					t.Fatalf("View() = %q, want intentional too-small state", view)
				}
				assertMainCommandBarAtBottom(t, view, "q Quit")
				return
			}

			assertMainCommandBarAtBottom(t, view, "Ctrl+C Exit immediately")
		})
	}
}

func TestUpdate_StatusReturnIgnoresLateResultAndPreservesSelection(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"service":{"url":"current"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "service", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "service.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("local")}}},
			{Name: "Current", Values: []config.ProfileValue{{Target: "service", Value: stringPointer("current")}}},
		},
	))
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)
	if model.cursor != 1 {
		t.Fatalf("cursor = %d, want second profile selected", model.cursor)
	}

	updatedModel, command := model.Update(runeKey('s'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want status comparison command")
	}

	updatedModel, returnCommand := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(Model)
	if returnCommand != nil {
		t.Fatal("returnCommand is not nil, want return without command")
	}
	if model.state != listState || model.cursor != 1 {
		t.Fatalf("state/cursor = %d/%d, want list with preserved selection", model.state, model.cursor)
	}

	lateMessage := command()
	updatedModel, lateCommand := model.Update(lateMessage)
	model = updatedModel.(Model)
	if lateCommand != nil {
		t.Fatal("lateCommand is not nil, want stale status result ignored after return")
	}
	if model.state != listState || model.cursor != 1 || model.statusComparison != nil {
		t.Fatalf("model after late status result = %#v, want list unchanged", model)
	}
}

func TestUpdate_DiffActionUsesSelectedProfileRefreshesAndIgnoresStaleResults(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)
	configPath := writeTargetFile(t, projectRoot, ".switchlet.yaml", "version: 3\n")
	originalContents := readFile(t, targetPath)
	originalMode := fileMode(t, targetPath)
	originalConfigContents := readFile(t, configPath)
	originalConfigMode := fileMode(t, configPath)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://local")}}},
			{Name: "Staging", Protected: true, Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://staging")}}},
		},
	))
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want diff comparison command")
	}
	if model.state != diffLoadingState || model.comparisonProfileName != "Staging" {
		t.Fatalf("state/profile = %d/%q, want diff loading for Staging", model.state, model.comparisonProfileName)
	}
	if !strings.Contains(model.View(), "Comparing selected profile") || strings.Contains(model.View(), "confirmation") {
		t.Fatalf("View() = %q, want read-only diff loading without confirmation", model.View())
	}

	updatedModel, returnCommand := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if returnCommand != nil {
		t.Fatal("returnCommand is not nil, want d to return from diff loading")
	}
	if model.state != listState || model.cursor != 1 {
		t.Fatalf("state/cursor = %d/%d, want list with Staging still selected", model.state, model.cursor)
	}

	updatedModel, staleReturnCommand := model.Update(command())
	model = updatedModel.(Model)
	if staleReturnCommand != nil {
		t.Fatal("staleReturnCommand is not nil, want stale diff result ignored after d return")
	}
	if model.state != listState || model.cursor != 1 || model.diffPreview != nil || model.comparisonRequestKind != comparisonRequestNone {
		t.Fatalf("model after stale diff result = %#v, want list unchanged after d return", model)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)
	if model.cursor != 0 {
		t.Fatalf("cursor = %d, want Local selected after movement", model.cursor)
	}
	updatedModel, currentCommand := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if currentCommand == nil {
		t.Fatal("currentCommand is nil, want diff command for Local")
	}
	if model.state != diffLoadingState || model.comparisonProfileName != "Local" {
		t.Fatalf("state/profile = %d/%q, want diff loading for Local", model.state, model.comparisonProfileName)
	}

	staleMessage := command()
	updatedModel, staleCommand := model.Update(staleMessage)
	model = updatedModel.(Model)
	if staleCommand != nil {
		t.Fatal("staleCommand is not nil, want stale diff result ignored")
	}
	if model.state != diffLoadingState || model.diffPreview != nil || model.comparisonProfileName != "Local" {
		t.Fatalf("model after stale diff result = %#v, want Local loading unchanged", model)
	}

	message := currentCommand()
	updatedModel, readyCommand := model.Update(message)
	model = updatedModel.(Model)
	if readyCommand != nil {
		t.Fatal("readyCommand is not nil, want no command after diff result")
	}
	if model.state != diffReadyState || model.diffPreview == nil || model.diffPreview.ProfileName != "Local" {
		t.Fatalf("model after diff result = %#v, want Local diff ready", model)
	}

	readyRequestID := model.comparisonRequestID
	updatedModel, refreshCommand := model.Update(runeKey('r'))
	model = updatedModel.(Model)
	if refreshCommand == nil {
		t.Fatal("refreshCommand is nil, want diff refresh command")
	}
	if model.state != diffLoadingState || model.comparisonRequestID <= readyRequestID || model.comparisonProfileName != "Local" {
		t.Fatalf("model after diff refresh = %#v, want new Local diff loading request", model)
	}
	assertFileUnchanged(t, targetPath, originalContents, originalMode)
	assertNoTargetTempFile(t, targetPath)
	assertFileUnchanged(t, configPath, originalConfigContents, originalConfigMode)
}

func TestUpdate_ValueRevealTogglesDiffWithoutChangingComparisonRequest(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Staging", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://staging")}}}},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want diff comparison command")
	}
	requestID := model.comparisonRequestID
	profileName := model.comparisonProfileName
	if !strings.Contains(model.View(), "v Reveal values") {
		t.Fatalf("diff loading View() = %q, want reveal command", model.View())
	}

	updatedModel, toggleCommand := model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if toggleCommand != nil {
		t.Fatal("toggleCommand is not nil, want local reveal toggle")
	}
	if !model.valuesVisible || model.state != diffLoadingState || model.comparisonRequestID != requestID || model.comparisonProfileName != profileName {
		t.Fatalf("model after diff loading reveal = %#v, want request state preserved", model)
	}
	if !strings.Contains(model.View(), "v Hide values") {
		t.Fatalf("diff loading View() = %q, want hide command", model.View())
	}

	updatedModel, readyCommand := model.Update(command())
	model = updatedModel.(Model)
	if readyCommand != nil {
		t.Fatal("readyCommand is not nil, want no command after diff result")
	}
	if model.state != diffReadyState || model.diffPreview == nil || !model.valuesVisible {
		t.Fatalf("model after diff result = %#v, want ready diff with reveal state preserved", model)
	}

	updatedModel, toggleCommand = model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if toggleCommand != nil {
		t.Fatal("toggleCommand is not nil after hiding values")
	}
	if model.valuesVisible || model.state != diffReadyState || model.diffPreview == nil || model.comparisonRequestID != requestID {
		t.Fatalf("model after diff ready hide = %#v, want ready diff request state preserved", model)
	}
	if !strings.Contains(model.View(), "v Reveal values") {
		t.Fatalf("diff ready View() = %q, want reveal command", model.View())
	}
}

func TestUpdate_ValueRevealIgnoredInStatusAndComparisonError(t *testing.T) {
	model := comparisonKeyTestModel()
	model.state = statusReadyState
	model.statusComparison = &app.StatusComparison{Status: app.StatusComparisonUnmatched, TargetCount: 1, Complete: true}

	updatedModel, command := model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want v ignored in status")
	}
	if model.valuesVisible || model.state != statusReadyState || model.statusComparison == nil {
		t.Fatalf("model after status v = %#v, want status unchanged and values hidden", model)
	}
	statusView := model.View()
	if strings.Contains(statusView, "v Reveal values") || strings.Contains(statusView, "v Hide values") {
		t.Fatalf("status View() = %q, must not advertise value reveal", statusView)
	}

	model.state = comparisonErrorState
	model.comparisonError = RecoverableError{Problem: "Could not compare profile."}
	model.valuesVisible = true
	updatedModel, command = model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want v ignored in comparison error")
	}
	if !model.valuesVisible || model.state != comparisonErrorState || model.comparisonError.IsZero() {
		t.Fatalf("model after comparison error v = %#v, want comparison error unchanged", model)
	}
	comparisonErrorView := model.View()
	if strings.Contains(comparisonErrorView, "v Reveal values") || strings.Contains(comparisonErrorView, "v Hide values") {
		t.Fatalf("comparison error View() = %q, must not advertise value reveal", comparisonErrorView)
	}
}

func TestView_DiffRendersManagedPatchHiddenValuesUnavailableAndOmittedTargets(t *testing.T) {
	projectRoot := t.TempDir()
	databaseCurrentValue := "postgres://current-database-secret"
	databaseResolvedValue := "postgres://staging-database-secret"
	apiCurrentValue := "https://current-api-secret.example.test"
	workerCurrentValue := "https://current-worker-secret.example.test"
	databasePath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"`+databaseCurrentValue+`"}}`)
	apiPath := writeTargetFile(t, projectRoot, "frontend/.env.local", "VITE_API_URL="+apiCurrentValue+"\n")
	workerPath := writeTargetFile(t, projectRoot, "worker/config.json", `{"queue":{"url":"`+workerCurrentValue+`"}}`)
	redisPath := writeTargetFile(t, projectRoot, "cache/config.json", `{"redis":{"url":"redis://current-secret"}}`)
	environmentVariableName := "SWITCHLET_DIFF_TEST_EMPTY_WORKER_QUEUE"
	t.Setenv(environmentVariableName, "")

	model := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database", File: databasePath, Type: config.TargetTypeJSON, JSONPath: "database.url"},
			{Name: "frontendApi", File: apiPath, Type: config.TargetTypeDotenv, Key: "VITE_API_URL"},
			{Name: "workerQueue", File: workerPath, Type: config.TargetTypeJSON, JSONPath: "queue.url"},
			{Name: "redis", File: redisPath, Type: config.TargetTypeJSON, JSONPath: "redis.url"},
		},
		[]config.Profile{{
			Name:      "Staging",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database", Value: stringPointer(databaseResolvedValue)},
				{Target: "frontendApi", Value: stringPointer(apiCurrentValue)},
				{Target: "workerQueue", ValueFromEnv: stringPointer(environmentVariableName)},
			},
		}},
	))

	model = openDiffReady(t, model)
	view := model.View()
	for _, expected := range []string{
		"Managed patch",
		"Staging",
		"[protected]",
		"some profile values unavailable | 3 of 4 | values hidden",
		"1 target unchanged",
		"Protected profile; read-only preview only.",
		"Affected files",
		"backend/appsettings.Development.json",
		"@@ database [json]",
		"jsonPath: database.url",
		"would update",
		"- current  " + hiddenValuePlaceholder,
		"+ profile  " + hiddenValuePlaceholder,
		"frontend/.env.local",
		"@@ frontendApi [dotenv]",
		"key: VITE_API_URL",
		"already matches",
		"= value    " + hiddenValuePlaceholder,
		"worker/config.json",
		"@@ workerQueue [json]",
		"unavailable",
		environmentVariableName,
		"Reason: profile \"Staging\" value for target \"workerQueue\"",
		"Omitted targets",
		"Unchanged by this partial profile.",
		"redis [json]",
		"No files were modified.",
		"r Refresh",
		"d Return",
		"Esc/q Return",
		"Ctrl+C Exit immediately",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want diff detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{databaseCurrentValue, databaseResolvedValue, apiCurrentValue, workerCurrentValue, "redis://current-secret"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain raw current or resolved value %q", view, forbidden)
		}
	}

	lines := visibleLines(view)
	if strings.Contains(lines[len(lines)-1], "Apply") {
		t.Fatalf("command bar = %q, must not include an apply action", lines[len(lines)-1])
	}
}

func TestView_DiffManagedPatchRevealsManagedValuesWhenShown(t *testing.T) {
	projectRoot := t.TempDir()
	currentValue := "postgres://current-managed-value"
	profileValue := "postgres://staging-managed-value"
	targetPath := writeTargetFile(t, projectRoot, "backend/appsettings.Development.json", `{"database":{"url":"`+currentValue+`"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Staging", Values: []config.ProfileValue{{Target: "database", Value: stringPointer(profileValue)}}}},
	))

	model = openDiffReady(t, model)
	hiddenView := model.View()
	if strings.Contains(hiddenView, currentValue) || strings.Contains(hiddenView, profileValue) {
		t.Fatalf("View() = %q, must not reveal managed values while hidden", hiddenView)
	}

	updatedModel, command := model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want local reveal toggle")
	}

	shownView := model.View()
	for _, expected := range []string{
		"values shown",
		"v Hide values",
		"- current  " + currentValue,
		"+ profile  " + profileValue,
	} {
		if !strings.Contains(shownView, expected) {
			t.Fatalf("View() = %q, want revealed managed patch detail %q", shownView, expected)
		}
	}
}

func TestView_DiffLoadingShowsSelectedProfileContextAndReadOnlyCommands(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Staging", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://staging")}}}},
	))

	updatedModel, command := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want diff comparison command")
	}

	view := model.View()
	for _, expected := range []string{"Profile: Staging", "Comparing selected profile...", "No files will be modified.", "r Refresh", "d Return", "Esc/q Return", "Ctrl+C Exit immediately"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want diff loading detail %q", view, expected)
		}
	}
}

func TestView_DiffModeDoesNotShiftWorkspaceDown(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://staging")}}}},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)
	listPanelIndex := lineIndexContaining(model.View(), "* Profiles")
	if listPanelIndex < 0 {
		t.Fatalf("list View() = %q, want profile panel", model.View())
	}

	updatedModel, command := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want diff comparison command")
	}
	loadingPanelIndex := lineIndexContaining(model.View(), "* Managed patch")
	if loadingPanelIndex != listPanelIndex {
		t.Fatalf("diff loading panel index = %d, want same workspace start %d\nview: %q", loadingPanelIndex, listPanelIndex, model.View())
	}

	updatedModel, _ = model.Update(command())
	model = updatedModel.(Model)
	readyView := model.View()
	readyPanelIndex := lineIndexContaining(readyView, "* Managed patch")
	if readyPanelIndex != listPanelIndex {
		t.Fatalf("diff ready panel index = %d, want same workspace start %d\nview: %q", readyPanelIndex, listPanelIndex, readyView)
	}
	if strings.Contains(readyView, "* Profiles") {
		t.Fatalf("diff ready View() = %q, want managed patch preview to own the body width", readyView)
	}
}

func TestView_DiffScreenContainsLongContentAtHostileDimensions(t *testing.T) {
	model := comparisonKeyTestModel()
	model.state = diffReadyState
	model.comparisonRequestKind = comparisonRequestDiff
	model.comparisonProfileName = "Production profile with a very long display name"
	model.diffPreview = &app.ManagedPatchPreview{
		ProfileName:         "Production profile with a very long display name",
		Protected:           true,
		Complete:            true,
		TargetCount:         2,
		IncludedTargetCount: 1,
		OmittedTargetCount:  1,
		Files: []app.ManagedPatchFileGroup{{
			TargetFile: "/very/long/project/path/backend/appsettings.Development.json",
			Hunks: []app.ManagedPatchHunk{{
				TargetDescriptor: app.TargetDescriptor{
					TargetName:   "database-with-a-very-long-target-name",
					TargetFile:   "/very/long/project/path/backend/appsettings.Development.json",
					TargetType:   config.TargetTypeJSON,
					SelectorName: "jsonPath",
					Selector:     "services.database.primary.connectionStrings.defaultConnection.value",
				},
				Status:              app.ManagedPatchStatusWouldUpdate,
				Source:              app.ProfileSourceLiteral,
				CurrentValue:        strings.Repeat("current-value-", 12),
				CurrentValueVisible: true,
				ProfileValue:        strings.Repeat("profile-value-", 12),
				ProfileValueVisible: true,
			}},
		}},
		OmittedTargets: []app.TargetDescriptor{{
			TargetName:   "frontend-api-with-a-very-long-target-name",
			TargetFile:   "/very/long/project/path/frontend/.env.local",
			TargetType:   config.TargetTypeDotenv,
			SelectorName: "key",
			Selector:     "VITE_PUBLIC_BACKEND_API_BASE_URL_WITH_A_LONG_SUFFIX",
		}},
	}

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
		t.Run(sizeLabel(size.width, size.height), func(t *testing.T) {
			resizedModel := resizedMainModel(t, model, size.width, size.height)
			view := resizedModel.View()
			assertVisibleWidth(t, view, size.width)
			assertVisibleHeight(t, view, size.height)
			if size.width < minimumTerminalWidth || size.height < minimumTerminalHeight {
				if !strings.Contains(view, "Terminal too small") {
					t.Fatalf("View() = %q, want intentional too-small state", view)
				}
				assertMainCommandBarAtBottom(t, view, "q Quit")
				return
			}

			assertMainCommandBarAtBottom(t, view, "Ctrl+C Exit immediately")
		})
	}
}

func TestUpdate_ComparisonScreensReturnAndQuitWithDocumentedKeys(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		state viewState
		key   tea.KeyMsg
	}{
		{name: "status loading q", state: statusLoadingState, key: runeKey('q')},
		{name: "status ready esc", state: statusReadyState, key: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "diff loading q", state: diffLoadingState, key: runeKey('q')},
		{name: "diff loading d", state: diffLoadingState, key: runeKey('d')},
		{name: "diff ready esc", state: diffReadyState, key: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "diff ready d", state: diffReadyState, key: runeKey('d')},
		{name: "comparison error q", state: comparisonErrorState, key: runeKey('q')},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			model := comparisonKeyTestModel()
			model.cursor = 1
			model.state = testCase.state
			model.comparisonRequestKind = comparisonRequestStatus
			if testCase.state == diffLoadingState || testCase.state == diffReadyState {
				model.comparisonRequestKind = comparisonRequestDiff
				model.comparisonProfileName = "Production"
				model.diffPreview = &app.ManagedPatchPreview{ProfileName: "Production"}
			}

			updatedModel, command := model.Update(testCase.key)
			model = updatedModel.(Model)
			if command != nil {
				t.Fatal("command is not nil, want return without command")
			}
			if model.state != listState || model.cursor != 1 {
				t.Fatalf("state/cursor = %d/%d, want list with preserved selection", model.state, model.cursor)
			}
			if model.statusComparison != nil || model.diffPreview != nil || !model.comparisonError.IsZero() || model.comparisonRequestKind != comparisonRequestNone {
				t.Fatalf("comparison state was not cleared: %#v", model)
			}
		})
	}

	for _, state := range []viewState{statusLoadingState, statusReadyState, diffLoadingState, diffReadyState, comparisonErrorState} {
		model := comparisonKeyTestModel()
		model.state = state
		updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		model = updatedModel.(Model)
		if command == nil {
			t.Fatalf("Ctrl+C command is nil for state %d, want quit command", state)
		}
	}
}

func TestUpdate_ComparisonFailureShowsRecoverableErrorWithoutWritingOrLeakingValues(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"password":"current-secret"}}`)
	configPath := writeTargetFile(t, projectRoot, ".switchlet.yaml", "version: 3\n")
	originalContents := readFile(t, targetPath)
	originalMode := fileMode(t, targetPath)
	originalConfigContents := readFile(t, configPath)
	originalConfigMode := fileMode(t, configPath)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Staging", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("resolved-secret")}}}},
	))

	updatedModel, command := model.Update(runeKey('s'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want status comparison command")
	}

	message := command()
	updatedModel, errorCommand := model.Update(message)
	model = updatedModel.(Model)
	if errorCommand != nil {
		t.Fatal("errorCommand is not nil, want no command after comparison failure")
	}
	if model.state != comparisonErrorState {
		t.Fatalf("state = %d, want comparisonErrorState", model.state)
	}
	view := model.View()
	for _, expected := range []string{"Comparison error", "Could not compare current status.", "Action: Current status", "Target: database [json]", "Selector: database.url", "Reason:", "missing segment"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want comparison error detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{"current-secret", "resolved-secret"} {
		if strings.Contains(view, forbidden) || strings.Contains(model.comparisonError.Reason, forbidden) {
			t.Fatalf("comparison error leaked %q\nview: %q\nreason: %q", forbidden, view, model.comparisonError.Reason)
		}
	}
	assertFileUnchanged(t, targetPath, originalContents, originalMode)
	assertNoTargetTempFile(t, targetPath)
	assertFileUnchanged(t, configPath, originalConfigContents, originalConfigMode)

	updatedModel, returnCommand := model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if returnCommand != nil {
		t.Fatal("returnCommand is not nil, want q to return from comparison error")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState after comparison error return", model.state)
	}
}

func TestUpdate_ComparisonErrorRefreshRetriesStatusAndRejectsStaleFailure(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"password":"current-secret"}}`)
	originalContents := readFile(t, targetPath)
	originalMode := fileMode(t, targetPath)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("local-secret")}}},
			{Name: "Staging", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("resolved-secret")}}},
		},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updatedModel.(Model)
	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(runeKey('s'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want status comparison command")
	}
	failedRequestID := model.comparisonRequestID

	updatedModel, errorCommand := model.Update(command())
	model = updatedModel.(Model)
	if errorCommand != nil {
		t.Fatal("errorCommand is not nil, want no command after comparison failure")
	}
	if model.state != comparisonErrorState || model.cursor != 1 || model.width != 120 || model.height != 40 {
		t.Fatalf("model after comparison error = %#v, want error with preserved selection and size", model)
	}
	staleCause := model.comparisonError.Cause
	view := model.View()
	for _, expected := range []string{"Comparison error", "Could not compare current status.", "r Refresh", "Esc/q Return", "Ctrl+C Exit immediately"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want comparison error detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{"current-secret", "resolved-secret"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain raw value %q", view, forbidden)
		}
	}

	updatedModel, refreshCommand := model.Update(runeKey('r'))
	model = updatedModel.(Model)
	if refreshCommand == nil {
		t.Fatal("refreshCommand is nil, want retry status comparison command")
	}
	if model.state != statusLoadingState || model.comparisonRequestKind != comparisonRequestStatus || model.comparisonRequestID <= failedRequestID || model.cursor != 1 || model.width != 120 || model.height != 40 {
		t.Fatalf("model after error refresh = %#v, want refreshed status loading with preserved selection and size", model)
	}
	if !strings.Contains(model.View(), "Checking current managed values") {
		t.Fatalf("View() = %q, want refreshed loading feedback", model.View())
	}

	updatedModel, staleCommand := model.Update(comparisonFailedMsg{requestID: failedRequestID, kind: comparisonRequestStatus, err: staleCause})
	model = updatedModel.(Model)
	if staleCommand != nil {
		t.Fatal("staleCommand is not nil, want stale status failure ignored")
	}
	if model.state != statusLoadingState || !model.comparisonError.IsZero() {
		t.Fatalf("model after stale status failure = %#v, want refreshed loading unchanged", model)
	}

	updatedModel, errorCommand = model.Update(refreshCommand())
	model = updatedModel.(Model)
	if errorCommand != nil {
		t.Fatal("errorCommand is not nil after refreshed failure")
	}
	if model.state != comparisonErrorState {
		t.Fatalf("state = %d, want comparisonErrorState after refreshed failure", model.state)
	}
	assertFileUnchanged(t, targetPath, originalContents, originalMode)
	assertNoTargetTempFile(t, targetPath)
}

func TestUpdate_ComparisonErrorRefreshRetriesDiffForFailedProfileAndIgnoresStaleResult(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"password":"current-secret"}}`)
	originalContents := readFile(t, targetPath)
	originalMode := fileMode(t, targetPath)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("local-secret")}}},
			{Name: "Staging", Protected: true, Values: []config.ProfileValue{{Target: "database", Value: stringPointer("resolved-secret")}}},
		},
	))
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want diff comparison command")
	}
	failedRequestID := model.comparisonRequestID

	updatedModel, errorCommand := model.Update(command())
	model = updatedModel.(Model)
	if errorCommand != nil {
		t.Fatal("errorCommand is not nil, want no command after diff failure")
	}
	if model.state != comparisonErrorState || model.comparisonRequestKind != comparisonRequestDiff || model.comparisonProfileName != "Staging" || model.cursor != 1 {
		t.Fatalf("model after diff error = %#v, want Staging comparison error", model)
	}
	view := model.View()
	for _, expected := range []string{"Could not compare profile \"Staging\".", "Action: Selected-profile diff", "Profile: Staging", "r Refresh"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want diff error detail %q", view, expected)
		}
	}
	for _, forbidden := range []string{"current-secret", "resolved-secret"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, must not contain raw value %q", view, forbidden)
		}
	}

	updatedModel, refreshCommand := model.Update(runeKey('r'))
	model = updatedModel.(Model)
	if refreshCommand == nil {
		t.Fatal("refreshCommand is nil, want retry diff comparison command")
	}
	if model.state != diffLoadingState || model.comparisonRequestKind != comparisonRequestDiff || model.comparisonProfileName != "Staging" || model.comparisonRequestID <= failedRequestID || model.cursor != 1 {
		t.Fatalf("model after diff error refresh = %#v, want Staging diff loading", model)
	}

	updatedModel, staleCommand := model.Update(diffPreviewCompletedMsg{requestID: failedRequestID, profile: "Staging", result: app.ManagedPatchPreview{ProfileName: "Stale"}})
	model = updatedModel.(Model)
	if staleCommand != nil {
		t.Fatal("staleCommand is not nil, want stale diff result ignored")
	}
	if model.state != diffLoadingState || model.diffPreview != nil || model.comparisonProfileName != "Staging" {
		t.Fatalf("model after stale diff result = %#v, want refreshed Staging loading unchanged", model)
	}

	updatedModel, errorCommand = model.Update(refreshCommand())
	model = updatedModel.(Model)
	if errorCommand != nil {
		t.Fatal("errorCommand is not nil after refreshed diff failure")
	}
	if model.state != comparisonErrorState || model.comparisonProfileName != "Staging" {
		t.Fatalf("model after refreshed diff failure = %#v, want Staging comparison error", model)
	}
	assertFileUnchanged(t, targetPath, originalContents, originalMode)
	assertNoTargetTempFile(t, targetPath)
}

func TestView_ComparisonErrorContainsLongContentAtHostileDimensions(t *testing.T) {
	model := comparisonKeyTestModel()
	model.state = comparisonErrorState
	model.comparisonRequestKind = comparisonRequestDiff
	model.comparisonProfileName = "Production profile with a very long display name"
	model.comparisonError = RecoverableError{
		Problem: "Could not compare profile \"Production profile with a very long display name\".",
		Context: []string{
			RenderKeyValue("Action", "Selected-profile diff"),
			RenderKeyValue("Profile", "Production profile with a very long display name"),
			RenderKeyValue("Target", "database-with-a-very-long-target-name [json]"),
			RenderKeyValue("File", "/very/long/project/path/backend/appsettings.Development.json"),
			RenderKeyValue("Selector", "services.database.primary.connectionStrings.defaultConnection.value"),
		},
		Reason:   strings.Repeat("target read failed because the configured selector could not be resolved safely ", 8),
		Recovery: "Fix the target or profile, then return and try again.",
	}

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
		t.Run(sizeLabel(size.width, size.height), func(t *testing.T) {
			resizedModel := resizedMainModel(t, model, size.width, size.height)
			view := resizedModel.View()
			assertVisibleWidth(t, view, size.width)
			assertVisibleHeight(t, view, size.height)
			if size.width < minimumTerminalWidth || size.height < minimumTerminalHeight {
				if !strings.Contains(view, "Terminal too small") {
					t.Fatalf("View() = %q, want intentional too-small state", view)
				}
				assertMainCommandBarAtBottom(t, view, "q Quit")
				return
			}

			assertMainCommandBarAtBottom(t, view, "Ctrl+C Exit immediately")
			if strings.Contains(view, "super-secret") {
				t.Fatalf("View() = %q, must not contain raw secret values", view)
			}
		})
	}
}

func TestView_ListActionsExposeStatusAndDiff(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	view := model.View()
	for _, expected := range []string{"Enter Apply", "i Inspect", "s Status", "d Diff", "q Quit"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want command action %q", view, expected)
		}
	}
}

func comparisonKeyTestModel() Model {
	return New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Production", Value: stringPointer("Server=prod;Database=App;")},
		},
	))
}

func openStatusReady(t *testing.T, model Model) Model {
	t.Helper()

	updatedModel, command := model.Update(runeKey('s'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want status comparison command")
	}

	updatedModel, command = model.Update(command())
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after status result")
	}
	if model.state != statusReadyState {
		t.Fatalf("state = %d, want statusReadyState", model.state)
	}

	return model
}

func openDiffReady(t *testing.T, model Model) Model {
	t.Helper()

	updatedModel, command := model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want diff comparison command")
	}

	updatedModel, command = model.Update(command())
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after diff result")
	}
	if model.state != diffReadyState {
		t.Fatalf("state = %d, want diffReadyState", model.state)
	}

	return model
}
