package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestNew_InitializesProfilesAndSelection(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING"), Protected: true},
		},
	)

	model := New(application)

	if len(model.profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2", len(model.profiles))
	}
	if model.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", model.cursor)
	}
	if model.valuesVisible {
		t.Fatal("valuesVisible = true, want new model to start hidden")
	}

	view := model.View()
	if !strings.Contains(view, "> Local") {
		t.Fatalf("View() = %q, want selected Local profile", view)
	}
	if !strings.Contains(view, "x Production") {
		t.Fatalf("View() = %q, want unavailable row marker", view)
	}
	if strings.Contains(view, "x Production [protected]") || strings.Contains(view, "x Production [unavailable]") {
		t.Fatalf("View() = %q, main profile list must not show non-current badges", view)
	}
	if !strings.Contains(view, "* Profiles") {
		t.Fatalf("View() = %q, want focused profiles panel", view)
	}
}

func TestNewWithSelectedProfile_SelectsRequestedProfileWhenPresent(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Staging", Value: stringPointer("Server=staging;Database=App;")},
		},
	)

	model := NewWithSelectedProfile(application, "Staging")

	selectedProfileName, ok := model.SelectedProfileName()
	if !ok || selectedProfileName != "Staging" {
		t.Fatalf("SelectedProfileName() = %q, %t, want Staging, true", selectedProfileName, ok)
	}
	if model.cursor != 1 {
		t.Fatalf("cursor = %d, want requested profile index 1", model.cursor)
	}
	if !strings.Contains(model.View(), "> Staging") {
		t.Fatalf("View() = %q, want selected Staging profile", model.View())
	}
}

func TestNewWithSelectedProfile_FallsBackWhenRequestedProfileIsMissing(t *testing.T) {
	application := app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Staging", Value: stringPointer("Server=staging;Database=App;")},
		},
	)

	model := NewWithSelectedProfile(application, "Deleted")

	selectedProfileName, ok := model.SelectedProfileName()
	if !ok || selectedProfileName != "Local" {
		t.Fatalf("SelectedProfileName() = %q, %t, want Local fallback, true", selectedProfileName, ok)
	}
	if model.cursor != 0 {
		t.Fatalf("cursor = %d, want default clamped cursor", model.cursor)
	}
}

func TestInit_DetectsCurrentProfileBadgeFromTargetFiles(t *testing.T) {
	projectRoot := t.TempDir()
	currentValue := "postgres://local-current"
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"`+currentValue+`"}}`)
	configPath := writeTargetFile(t, projectRoot, ".switchlet.yaml", "version: 3\n")
	originalContents := readFile(t, targetPath)
	originalMode := fileMode(t, targetPath)
	originalConfigContents := readFile(t, configPath)
	originalConfigMode := fileMode(t, configPath)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer(currentValue)}}},
			{Name: "Staging", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://staging")}}},
		},
	))
	if !strings.Contains(model.View(), "Checking current profile...") {
		t.Fatalf("View() = %q, want startup current-profile checking feedback", model.View())
	}

	command := model.Init()
	if command == nil {
		t.Fatal("Init() command is nil, want current-profile detection command")
	}
	updatedModel, nextCommand := model.Update(command())
	model = updatedModel.(Model)
	if nextCommand != nil {
		t.Fatal("nextCommand is not nil after current-profile detection")
	}
	if model.state != listState || !model.profileIsCurrent("Local") {
		t.Fatalf("model after current-profile detection = %#v, want Local current badge state", model)
	}
	view := model.View()
	if !strings.Contains(view, "> Local [current]") {
		t.Fatalf("View() = %q, want Local current badge", view)
	}
	if strings.Contains(view, "Checking current profile") || strings.Contains(view, "Current profile check unavailable") {
		t.Fatalf("View() = %q, want detection feedback removed after successful result", view)
	}
	if strings.Contains(view, "Staging [current]") {
		t.Fatalf("View() = %q, must not mark non-matching profile current", view)
	}
	assertFileUnchanged(t, targetPath, originalContents, originalMode)
	assertNoTargetTempFile(t, targetPath)
	assertFileUnchanged(t, configPath, originalConfigContents, originalConfigMode)
}

func TestInit_CurrentProfileDetectionFailureShowsUnavailableFeedback(t *testing.T) {
	projectRoot := t.TempDir()
	missingTargetPath := filepath.Join(projectRoot, "missing.json")
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: missingTargetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://local-secret")}}}},
	))

	updatedModel, command := model.Update(model.Init()())
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after current-profile detection failure")
	}
	if model.state != listState || model.currentDetection != currentProfileDetectionUnavailable || model.profileIsCurrent("Local") {
		t.Fatalf("model after current-profile detection failure = %#v, want non-current unavailable detection state", model)
	}
	view := model.View()
	if !strings.Contains(view, "Current profile check unavailable.") {
		t.Fatalf("View() = %q, want value-safe current-profile unavailable feedback", view)
	}
	if strings.Contains(view, "postgres://local-secret") {
		t.Fatalf("View() = %q, must not reveal profile value", view)
	}
}

func TestUpdate_CurrentProfileDetectionFailureDoesNotBlockListActions(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)
	baseModel := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://local")}}},
			{Name: "Staging", Values: []config.ProfileValue{{Target: "database", Value: stringPointer("postgres://staging")}}},
		},
	))
	updatedModel, command := baseModel.Update(currentProfileDetectedMsg{requestID: baseModel.currentRequestID, err: errors.New("status read failed")})
	baseModel = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after failed startup detection message")
	}
	if !strings.Contains(baseModel.View(), "Current profile check unavailable.") {
		t.Fatalf("View() = %q, want unavailable current-profile feedback before exercising actions", baseModel.View())
	}

	t.Run("selection", func(t *testing.T) {
		model := baseModel
		updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updatedModel.(Model)
		if command != nil || model.cursor != 1 {
			t.Fatalf("model after Down = %#v, command %v, want selection to move", model, command)
		}
	})

	t.Run("search", func(t *testing.T) {
		model := baseModel
		updatedModel, command := model.Update(runeKey('/'))
		model = updatedModel.(Model)
		if command != nil || model.state != searchState {
			t.Fatalf("model after search key = %#v, command %v, want search state", model, command)
		}
	})

	t.Run("apply", func(t *testing.T) {
		model := baseModel
		updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeySpace})
		model = updatedModel.(Model)
		if command == nil || model.applyingProfile != "Local" {
			t.Fatalf("model after Space = %#v, command %v, want apply command", model, command)
		}
	})

	t.Run("status", func(t *testing.T) {
		model := baseModel
		updatedModel, command := model.Update(runeKey('s'))
		model = updatedModel.(Model)
		if command == nil || model.state != statusLoadingState {
			t.Fatalf("model after status key = %#v, command %v, want status loading", model, command)
		}
	})

	t.Run("diff", func(t *testing.T) {
		model := baseModel
		updatedModel, command := model.Update(runeKey('d'))
		model = updatedModel.(Model)
		if command == nil || model.state != diffLoadingState {
			t.Fatalf("model after diff key = %#v, command %v, want diff loading", model, command)
		}
	})

	t.Run("config", func(t *testing.T) {
		model := baseModel
		updatedModel, command := model.Update(runeKey('c'))
		model = updatedModel.(Model)
		if command == nil || !model.ConfigRequested() {
			t.Fatalf("model after config key = %#v, command %v, want config handoff", model, command)
		}
	})

	t.Run("quit", func(t *testing.T) {
		model := baseModel
		updatedModel, command := model.Update(runeKey('q'))
		model = updatedModel.(Model)
		if command == nil || model.state != listState {
			t.Fatalf("model after quit key = %#v, command %v, want quit command from list", model, command)
		}
	})
}

func TestInit_ShowsCurrentBadgeForEveryMatchingProfile(t *testing.T) {
	projectRoot := t.TempDir()
	currentValue := "postgres://shared-current"
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"`+currentValue+`"}}`)
	model := New(app.NewWithTargets(
		[]config.Target{{Name: "database", File: targetPath, Type: config.TargetTypeJSON, JSONPath: "database.url"}},
		[]config.Profile{
			{Name: "Local", Values: []config.ProfileValue{{Target: "database", Value: stringPointer(currentValue)}}},
			{Name: "Local Copy", Values: []config.ProfileValue{{Target: "database", Value: stringPointer(currentValue)}}},
		},
	))

	updatedModel, nextCommand := model.Update(model.Init()())
	model = updatedModel.(Model)
	if nextCommand != nil {
		t.Fatal("nextCommand is not nil after ambiguous current-profile detection")
	}
	if !model.profileIsCurrent("Local") || !model.profileIsCurrent("Local Copy") {
		t.Fatalf("model after current-profile detection = %#v, want both matching profiles current", model)
	}
	view := model.View()
	for _, expected := range []string{"Local [current]", "Local Copy [current]"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want current badge %q", view, expected)
		}
	}
}

func TestUpdate_ValueRevealTogglesInProfileList(t *testing.T) {
	managedValue := "visible-managed-value"
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer(managedValue)}},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)

	if model.valuesVisible {
		t.Fatal("valuesVisible = true, want hidden by default")
	}
	hiddenView := model.View()
	if !strings.Contains(hiddenView, "v Reveal values") {
		t.Fatalf("View() = %q, want reveal command while values are hidden", hiddenView)
	}
	if !strings.Contains(hiddenView, hiddenValuePlaceholder) || strings.Contains(hiddenView, "values hidden") {
		t.Fatalf("View() = %q, want hidden value state in profile contents", hiddenView)
	}
	if strings.Contains(hiddenView, managedValue) {
		t.Fatalf("View() = %q, must not contain raw managed value while hidden", hiddenView)
	}

	updatedModel, command := model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want reveal toggle to stay local")
	}
	if !model.valuesVisible || model.state != listState || model.cursor != 0 {
		t.Fatalf("model after reveal = %#v, want list state with values visible and selection preserved", model)
	}
	shownView := model.View()
	if !strings.Contains(shownView, "v Hide values") {
		t.Fatalf("View() = %q, want hide command while values are shown", shownView)
	}
	if strings.Contains(shownView, "values shown") || !strings.Contains(shownView, managedValue) {
		t.Fatalf("View() = %q, want shown managed value in profile contents", shownView)
	}

	updatedModel, command = model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after hiding values")
	}
	if model.valuesVisible {
		t.Fatal("valuesVisible = true, want values hidden after second toggle")
	}
	if !strings.Contains(model.View(), "v Reveal values") {
		t.Fatalf("View() = %q, want reveal command after hiding values", model.View())
	}
}

func TestUpdate_ConfigKeyRequestsEditorHandoffFromProfileList(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, command := model.Update(runeKey('c'))
	model = updatedModel.(Model)

	if command == nil {
		t.Fatal("command is nil, want quit command for config handoff")
	}
	if !model.ConfigRequested() {
		t.Fatal("ConfigRequested() = false, want config handoff request")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}
}

func TestView_ListActionsExposeConfigWithoutReplacingApplyOrQuit(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	view := model.View()
	for _, expected := range []string{"Enter Apply+Exit", "Space Apply", "c Config", "q Quit"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want command bar entry %q", view, expected)
		}
	}
}

func TestView_ListViewUsesNeutralProtectedStatusAndApplyHelp(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
	))

	view := model.View()
	if !strings.Contains(view, `> Production`) || !strings.Contains(view, `Production`) {
		t.Fatalf("View() = %q, want selected profile context", view)
	}
	if strings.Contains(view, `[protected]`) {
		t.Fatalf("View() = %q, main picker must not show protected badges", view)
	}
	if strings.Contains(view, `[literal]`) {
		t.Fatalf("View() = %q, main picker must not show source badges", view)
	}
	if strings.Contains(view, `requires confirmation`) {
		t.Fatalf("View() = %q, must not show premature confirmation status copy", view)
	}
	if strings.Contains(view, `Ready to apply`) {
		t.Fatalf("View() = %q, profile contents must not show redundant readiness meta text", view)
	}
	if !strings.Contains(view, "Enter Apply+Exit") {
		t.Fatalf("View() = %q, want Enter help text that matches the protected flow", view)
	}
}

func TestView_SelectedProfilePanelStaysActionFocused(t *testing.T) {
	t.Setenv("PRODUCTION_DATABASE_URL", "Server=prod;Database=App;Password=super-secret;")

	model := New(app.New(
		config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("PRODUCTION_DATABASE_URL"), Protected: true}},
	))

	view := model.View()
	headerLine := visibleLines(view)[0]
	if strings.Contains(headerLine, "Switch a named profile safely") {
		t.Fatalf("header line = %q, must not show redundant task copy", headerLine)
	}
	if strings.Contains(headerLine, "config/development.json") || strings.Contains(headerLine, "database.primary.url") {
		t.Fatalf("header line = %q, must not duplicate selected target context", headerLine)
	}
	for _, expected := range []string{
		"Profile contents",
		"Production",
		"config/development.json",
		"default",
		"Target",
		"Selector",
		"database.primary.url",
		"Value",
		hiddenValuePlaceholder,
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want profile-contents context %q", view, expected)
		}
	}
	for _, forbidden := range []string{"Profile:", "State:", "Source:", "Changes:", "Values:", "Enter:", "Protection:", "Ready to apply", "[protected]", "[json]", "[env]", "Environment variable: PRODUCTION_DATABASE_URL", "Masked value:", "super-secret", "Password=****"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("View() = %q, profile contents must not duplicate inspection detail %q", view, forbidden)
		}
	}
}

func TestView_PathTargetSingleTargetContextUsesSelectorFieldLabel(t *testing.T) {
	tests := []struct {
		name         string
		target       config.Target
		selectorName string
		selector     string
	}{
		{
			name:         "YAML",
			target:       config.Target{Name: "workerQueue", File: "worker/config.yaml", Type: config.TargetTypeYAML, YAMLPath: "queue.endpoint"},
			selectorName: "yamlPath",
			selector:     "queue.endpoint",
		},
		{
			name:         "TOML",
			target:       config.Target{Name: "serviceEndpoint", File: "services/development.toml", Type: config.TargetTypeTOML, TOMLPath: "services.api.endpoint"},
			selectorName: "tomlPath",
			selector:     "services.api.endpoint",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			model := New(app.New(
				testCase.target,
				[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Password=super-secret;"), Protected: true}},
			))

			view := model.View()
			for _, expected := range []string{
				"Profile contents",
				"Target",
				testCase.target.Name,
				testCase.target.File,
				"Selector",
				testCase.selector,
				"Value",
				hiddenValuePlaceholder,
			} {
				if !strings.Contains(view, expected) {
					t.Fatalf("View() = %q, want %s profile contents %q", view, testCase.name, expected)
				}
			}
			for _, forbidden := range []string{testCase.target.Name + " [" + string(testCase.target.Type) + "]", "Target JSON path", "Target selector", "super-secret", "Password=****"} {
				if strings.Contains(view, forbidden) {
					t.Fatalf("View() = %q, must not contain %q", view, forbidden)
				}
			}

			updatedModel, command := model.Update(runeKey('i'))
			model = updatedModel.(Model)
			if command != nil {
				t.Fatal("command is not nil, want no command when opening inspection")
			}

			inspectionView := model.View()
			for _, expected := range []string{
				"Profile detail",
				"Managed value  " + testCase.target.Name + " [" + string(testCase.target.Type) + "]",
				testCase.target.File,
				"Selector",
				testCase.selector,
				"Masked value: Server=prod;Password=****;",
			} {
				if !strings.Contains(inspectionView, expected) {
					t.Fatalf("inspection View() = %q, want %s inspection context %q", inspectionView, testCase.name, expected)
				}
			}
			if strings.Contains(inspectionView, "super-secret") {
				t.Fatalf("inspection View() = %q, must not contain unmasked secret", inspectionView)
			}

			updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			model = updatedModel.(Model)
			if command != nil {
				t.Fatal("command is not nil, want confirmation before apply")
			}
			if model.state != confirmState {
				t.Fatalf("state = %d, want confirmState", model.state)
			}

			confirmationView := model.View()
			for _, expected := range []string{
				"Confirmation",
				"Managed value  " + testCase.target.Name + " [" + string(testCase.target.Type) + "]",
				testCase.target.File,
				"Selector",
				testCase.selector,
				"resolved value is intentionally hidden",
			} {
				if !strings.Contains(confirmationView, expected) {
					t.Fatalf("confirmation View() = %q, want %s confirmation context %q", confirmationView, testCase.name, expected)
				}
			}
			for _, forbidden := range []string{"super-secret", "Password=****"} {
				if strings.Contains(confirmationView, forbidden) {
					t.Fatalf("confirmation View() = %q, must not contain resolved value %q", confirmationView, forbidden)
				}
			}
		})
	}
}

func TestView_PathTargetSuccessFinalMessageAndErrorsUseSafeTargetContext(t *testing.T) {
	tests := []struct {
		name            string
		targetName      string
		targetFile      string
		targetType      config.TargetType
		selectorName    string
		selector        string
		missingSelector string
		secretValue     string
	}{
		{
			name:            "YAML",
			targetName:      "workerQueue",
			targetFile:      "worker/config.yaml",
			targetType:      config.TargetTypeYAML,
			selectorName:    "yamlPath",
			selector:        "queue.endpoint",
			missingSelector: "queue.missing",
			secretValue:     "https://secret.queue.example.test",
		},
		{
			name:            "TOML",
			targetName:      "serviceEndpoint",
			targetFile:      "services/development.toml",
			targetType:      config.TargetTypeTOML,
			selectorName:    "tomlPath",
			selector:        "services.api.endpoint",
			missingSelector: "services.api.missing",
			secretValue:     "https://secret.service.example.test",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			model := New(app.New(
				config.Target{Name: testCase.targetName, File: testCase.targetFile, Type: testCase.targetType},
				[]config.Profile{{Name: "Local", Value: stringPointer(testCase.secretValue)}},
			))
			model.state = successState
			model.successResult = &app.Result{
				ProfileName: "Local",
				Changes: []app.PlannedChange{{
					TargetName:   testCase.targetName,
					TargetFile:   testCase.targetFile,
					TargetType:   testCase.targetType,
					SelectorName: testCase.selectorName,
					Selector:     testCase.selector,
				}},
			}

			view := model.View()
			for _, expected := range []string{
				"Applied profile: Local",
				"Updated target:",
				"updated " + testCase.targetFile,
				testCase.targetName + " [" + string(testCase.targetType) + "]",
				testCase.selectorName + ": " + testCase.selector,
			} {
				if !strings.Contains(view, expected) {
					t.Fatalf("View() = %q, want %s success context %q", view, testCase.name, expected)
				}
			}

			finalMessage := model.FinalMessage()
			for _, expected := range []string{
				"Applied profile \"Local\"",
				"Updated target:",
				"updated " + testCase.targetFile,
				testCase.targetName + " [" + string(testCase.targetType) + "]",
				testCase.selectorName + ": " + testCase.selector,
			} {
				if !strings.Contains(finalMessage, expected) {
					t.Fatalf("FinalMessage() = %q, want %s final context %q", finalMessage, testCase.name, expected)
				}
			}
			if strings.Contains(view, testCase.secretValue) || strings.Contains(finalMessage, testCase.secretValue) {
				t.Fatalf("%s success output must not contain resolved value %q\nview: %q\nfinal: %q", testCase.name, testCase.secretValue, view, finalMessage)
			}

			errorModel := New(app.New(
				config.Target{Name: testCase.targetName, File: testCase.targetFile, Type: testCase.targetType},
				[]config.Profile{{Name: "Local", Value: stringPointer(testCase.secretValue)}},
			))
			errorModel.state = errorState
			errorModel.recoverableError = errorModel.targetFailureError("Local", app.TargetFailure{
				TargetName:   testCase.targetName,
				TargetFile:   testCase.targetFile,
				TargetType:   testCase.targetType,
				SelectorName: testCase.selectorName,
				Selector:     testCase.missingSelector,
				Reason:       "missing segment \"missing\"",
			}, nil)

			errorView := errorModel.View()
			for _, expected := range []string{
				"Could not prepare target \"" + testCase.targetName + "\".",
				"Context:",
				"Profile: Local",
				"Managed value: " + testCase.targetName + " [" + string(testCase.targetType) + "]",
				"File: " + testCase.targetFile,
				"Selector: " + testCase.missingSelector,
				"Reason:",
				"missing segment \"missing\"",
				"Recovery:",
			} {
				if !strings.Contains(errorView, expected) {
					t.Fatalf("error View() = %q, want %s error context %q", errorView, testCase.name, expected)
				}
			}
			if strings.Contains(errorView, testCase.secretValue) {
				t.Fatalf("error View() = %q, must not contain resolved value", errorView)
			}
		})
	}
}

func TestView_ListViewUsesSplitLayoutAtComfortableWidth(t *testing.T) {
	model := New(app.New(
		config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("postgres://local")},
			{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING"), Protected: true},
		},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)

	view := model.View()
	if !lineContains(view, "* Profiles", "Profile contents") {
		t.Fatalf("View() = %q, want wide split layout with profile list and profile contents on the same row", view)
	}
	if !strings.Contains(view, "config/development.json") {
		t.Fatalf("View() = %q, want target file context in selected-profile details", view)
	}
	if !strings.Contains(view, "database.primary.url") {
		t.Fatalf("View() = %q, want target JSON path context in selected-profile details", view)
	}
}

func TestView_WideMainPickerPanelsUseAvailableHeight(t *testing.T) {
	model := New(app.New(
		config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Local", Value: stringPointer("postgres://local")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	model = updatedModel.(Model)

	lines := visibleLines(model.View())
	lineBeforeCommandSeparator := lines[len(lines)-3]
	if !strings.Contains(lineBeforeCommandSeparator, "┘") {
		t.Fatalf("line before command separator = %q, want wide panels to fill available body height", lineBeforeCommandSeparator)
	}
}

func TestView_ListViewStacksBeforeLayoutBecomesCramped(t *testing.T) {
	model := New(app.New(
		config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Local", Value: stringPointer("postgres://local")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)

	view := model.View()
	if lineContains(view, "* Profiles", "Profile contents") {
		t.Fatalf("View() = %q, want stacked layout at minimum supported width", view)
	}
	if !strings.Contains(view, "* Profiles") || !strings.Contains(view, "Profile contents") {
		t.Fatalf("View() = %q, want both stacked main regions", view)
	}
}

func TestView_LongMainScreenContentStaysWithinTerminalWidth(t *testing.T) {
	model := New(app.New(
		config.Target{
			File:     "/very/long/project/path/with/many/segments/configuration/appsettings.Development.json",
			JSONPath: "services.database.primary.connectionStrings.defaultConnection.value",
		},
		[]config.Profile{{Name: "A profile with a very long name that should not break the shell", Value: stringPointer("postgres://local")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)

	for _, line := range strings.Split(model.View(), "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("line %q has width %d, want at most 80", line, lipgloss.Width(line))
		}
	}
}

func TestView_WindowedProfileListShowsPositionContext(t *testing.T) {
	model := New(app.New(
		config.Target{},
		numberedProfiles(30),
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	model = updatedModel.(Model)
	start, end := model.visibleProfileRange()
	expected := fmt.Sprintf("Showing %d-%d of %d profiles", start+1, end, len(model.profiles))

	view := model.View()
	if !strings.Contains(view, expected) {
		t.Fatalf("View() = %q, want windowed profile-list position %q", view, expected)
	}
	if !strings.Contains(view, "PgUp/PgDn Page") || !strings.Contains(view, "Home/End Jump") {
		t.Fatalf("View() = %q, want long-list paging and jump command hints", view)
	}
}

func TestUpdate_MovesCursorDownUpAndWraps(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")},
			{Name: "Test", Value: stringPointer("Server=test;Database=App;")},
		},
	))

	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)
	if model.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", model.cursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updatedModel.(Model)
	if model.cursor != 0 {
		t.Fatalf("cursor after down wrap = %d, want 0", model.cursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updatedModel.(Model)
	if model.cursor != 1 {
		t.Fatalf("cursor after up wrap = %d, want 1", model.cursor)
	}
}

func TestUpdate_PageAndJumpKeysClampLongProfileList(t *testing.T) {
	model := New(app.New(
		config.Target{},
		numberedProfiles(20),
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)
	pageStep := model.profilePageStep()

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updatedModel.(Model)
	if model.cursor != pageStep {
		t.Fatalf("cursor after PgDn = %d, want %d", model.cursor, pageStep)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updatedModel.(Model)
	if model.cursor != len(model.profiles)-1 {
		t.Fatalf("cursor after End = %d, want last profile", model.cursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updatedModel.(Model)
	if model.cursor != len(model.profiles)-1 {
		t.Fatalf("cursor after PgDn at end = %d, want last profile", model.cursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model = updatedModel.(Model)
	wantCursor := len(model.profiles) - 1 - pageStep
	if model.cursor != wantCursor {
		t.Fatalf("cursor after PgUp = %d, want %d", model.cursor, wantCursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyHome})
	model = updatedModel.(Model)
	if model.cursor != 0 {
		t.Fatalf("cursor after Home = %d, want first profile", model.cursor)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model = updatedModel.(Model)
	if model.cursor != 0 {
		t.Fatalf("cursor after PgUp at start = %d, want first profile", model.cursor)
	}
}

func TestUpdate_ProfileSearchFiltersCaseInsensitiveAndAcceptsFilter(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("local")},
			{Name: "Staging", Value: stringPointer("staging")},
			{Name: "Production", Value: stringPointer("production")},
		},
	))
	updatedModel, command := model.Update(tea.WindowSizeMsg{Width: 140, Height: 32})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after window resize")
	}

	updatedModel, command = model.Update(runeKey('/'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after opening search")
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ST")})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after typing search")
	}
	if model.state != searchState || model.searchInput != "ST" {
		t.Fatalf("model after typing search = %#v, want search state with input ST", model)
	}
	selectedProfileName, ok := model.SelectedProfileName()
	if !ok || selectedProfileName != "Staging" {
		t.Fatalf("SelectedProfileName() = %q, %t, want Staging, true", selectedProfileName, ok)
	}
	searchView := model.View()
	commandLine := visibleLines(searchView)[len(visibleLines(searchView))-1]
	if !strings.Contains(commandLine, "/ST_") || !strings.Contains(searchView, "> Staging") || strings.Contains(searchView, "  Local") {
		t.Fatalf("View() = %q, want live filtered search for Staging only", searchView)
	}
	if strings.Contains(searchView, "Search: ST_") || strings.Contains(searchView, `Search "ST"`) || strings.Contains(searchView, `Filter "ST"`) || strings.Contains(commandLine, "Enter Apply filter") || strings.Contains(commandLine, "q Quit") {
		t.Fatalf("View() = %q, want search input to replace command bar contents", searchView)
	}

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after applying search filter")
	}
	if model.state != listState || model.profileFilter != "ST" || model.searchInput != "" {
		t.Fatalf("model after applying search = %#v, want accepted active filter", model)
	}
	filteredView := model.View()
	if strings.Contains(filteredView, `Filter "ST"`) || strings.Contains(filteredView, "Filter: ST") {
		t.Fatalf("View() = %q, active filter text must not appear in profile panel title", filteredView)
	}
	if !strings.Contains(filteredView, "n/N Matches") || !strings.Contains(filteredView, "Esc Clear filter") {
		t.Fatalf("View() = %q, want filtered command actions", filteredView)
	}
	if strings.Contains(filteredView, "  Local") || strings.Contains(filteredView, "  Production") {
		t.Fatalf("View() = %q, want non-matching profiles hidden", filteredView)
	}
}

func TestView_ProfileSearchDoesNotMoveProfileRowsDown(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("local")},
			{Name: "Staging", Value: stringPointer("staging")},
			{Name: "Production", Value: stringPointer("production")},
		},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 32})
	model = updatedModel.(Model)

	listView := model.View()
	listRowIndex := lineIndexContaining(listView, "> Local")

	updatedModel, command := model.Update(runeKey('/'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after opening search")
	}
	searchView := model.View()
	searchRowIndex := lineIndexContaining(searchView, "> Local")

	if searchRowIndex != listRowIndex {
		t.Fatalf("profile row moved from line %d to %d when search opened\nlist: %q\nsearch: %q", listRowIndex, searchRowIndex, listView, searchView)
	}
	if strings.Contains(searchView, "Search:") {
		t.Fatalf("View() = %q, want no search input in profile pane body", searchView)
	}
}

func TestUpdate_ProfileSearchCancelPreservesPriorFilterAndSelection(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("local")},
			{Name: "Staging", Value: stringPointer("staging")},
			{Name: "Production", Value: stringPointer("production")},
		},
	))
	model = acceptProfileSearch(t, model, "stag")
	selectedProfileName, ok := model.SelectedProfileName()
	if !ok || selectedProfileName != "Staging" {
		t.Fatalf("SelectedProfileName() after first filter = %q, %t, want Staging, true", selectedProfileName, ok)
	}

	updatedModel, command := model.Update(runeKey('/'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after reopening search")
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after clearing search input")
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("prod")})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after editing search")
	}
	selectedProfileName, ok = model.SelectedProfileName()
	if !ok || selectedProfileName != "Production" {
		t.Fatalf("SelectedProfileName() during edited search = %q, %t, want Production, true", selectedProfileName, ok)
	}

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after canceling search")
	}
	selectedProfileName, ok = model.SelectedProfileName()
	if model.state != listState || model.profileFilter != "stag" || !ok || selectedProfileName != "Staging" {
		t.Fatalf("model after canceling search = %#v, selected %q/%t, want previous filter and Staging selection", model, selectedProfileName, ok)
	}
}

func TestUpdate_ProfileSearchTreatsQAsLiteralInput(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "QA", Value: stringPointer("qa")}},
	))

	updatedModel, command := model.Update(runeKey('/'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after opening search")
	}
	updatedModel, command = model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after q in search input, want literal text")
	}
	if model.state != searchState || model.searchInput != "q" {
		t.Fatalf("model after q search input = %#v, want literal q in search field", model)
	}
	commandLine := visibleLines(model.View())[len(visibleLines(model.View()))-1]
	if !strings.Contains(commandLine, "/q_") {
		t.Fatalf("View() = %q, want q rendered in search field", model.View())
	}
}

func TestUpdate_ProfileFilterClearRestoresFullList(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("local")},
			{Name: "Staging", Value: stringPointer("staging")},
			{Name: "Production", Value: stringPointer("production")},
		},
	))
	model = acceptProfileSearch(t, model, "stag")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after clearing filter")
	}
	if model.profileFilter != "" {
		t.Fatalf("profileFilter = %q, want cleared", model.profileFilter)
	}
	if selectedProfileName, ok := model.SelectedProfileName(); !ok || selectedProfileName == "" {
		t.Fatalf("SelectedProfileName() = %q, %t, want valid selection after clearing filter", selectedProfileName, ok)
	}
	view := model.View()
	for _, expected := range []string{"Local", "Staging", "Production"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want full list restored with %q", view, expected)
		}
	}
}

func TestUpdate_ProfileFilterNoMatchesShowsEmptyState(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("local")},
			{Name: "Staging", Value: stringPointer("staging")},
		},
	))
	model = acceptProfileSearch(t, model, "missing")

	if selectedProfileName, ok := model.SelectedProfileName(); ok || selectedProfileName != "" {
		t.Fatalf("SelectedProfileName() = %q, %t, want no selection for empty filter result", selectedProfileName, ok)
	}
	view := model.View()
	if !strings.Contains(view, "No profiles match this filter.") || !strings.Contains(view, "Esc Clear filter") {
		t.Fatalf("View() = %q, want empty filtered state with clear action", view)
	}
	if strings.Contains(view, `Filter "missing"`) || strings.Contains(view, "missing_") {
		t.Fatalf("View() = %q, must not duplicate filter query in the profile panel", view)
	}
	if strings.Contains(view, "Enter Apply") || strings.Contains(view, "Space Apply") {
		t.Fatalf("View() = %q, must not advertise apply when no filtered profile is selected", view)
	}

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil || model.state != listState {
		t.Fatalf("model after Enter with no matches = %#v, command %v, want no-op list state", model, command)
	}
}

func TestUpdate_FilteredListApplyUsesHighlightedProfile(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"local"}}`)
	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "database.url"},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("local")},
			{Name: "Staging", Value: stringPointer("staging")},
		},
	))
	model = acceptProfileSearch(t, model, "stag")

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want apply command from filtered list")
	}
	if model.applyingProfile != "Staging" {
		t.Fatalf("applyingProfile = %q, want filtered profile Staging", model.applyingProfile)
	}

	updatedModel, nextCommand := model.Update(command())
	model = updatedModel.(Model)
	if nextCommand == nil {
		t.Fatal("nextCommand is nil, want current-profile refresh after Space apply")
	}
	if contents := string(readFile(t, targetPath)); !strings.Contains(contents, `"url": "staging"`) {
		t.Fatalf("target contents = %q, want Staging value applied", contents)
	}
}

func TestUpdate_FilteredListSecondaryActionsUseFilteredSelection(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{
			{Name: "Local", Value: stringPointer("local")},
			{Name: "Production", Value: stringPointer("production")},
		},
	))
	model = acceptProfileSearch(t, model, "prod")

	updatedModel, command := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after opening inspection from filtered list")
	}
	if model.state != inspectState || !strings.Contains(model.View(), "Profile: Production") {
		t.Fatalf("model after filtered inspection = %#v, view %q, want Production inspection", model, model.View())
	}
	updatedModel, command = model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if command != nil || model.state != listState {
		t.Fatalf("model after returning from inspection = %#v, command %v, want filtered list", model, command)
	}

	updatedModel, command = model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command == nil || model.state != diffLoadingState || model.comparisonProfileName != "Production" {
		t.Fatalf("model after filtered diff = %#v, command %v, want diff for Production", model, command)
	}
	updatedModel, command = model.Update(runeKey('d'))
	model = updatedModel.(Model)
	if command != nil || model.state != listState {
		t.Fatalf("model after returning from diff = %#v, command %v, want filtered list", model, command)
	}

	updatedModel, command = model.Update(runeKey('v'))
	model = updatedModel.(Model)
	if command != nil || !model.valuesVisible {
		t.Fatalf("model after filtered reveal = %#v, command %v, want local reveal toggle", model, command)
	}

	updatedModel, command = model.Update(runeKey('c'))
	model = updatedModel.(Model)
	if command == nil || !model.ConfigRequested() {
		t.Fatalf("model after filtered config handoff = %#v, command %v, want config request", model, command)
	}
}

func TestUpdate_ShowsRecoverableErrorForUnavailableProfile(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING")}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no apply command for unavailable profile")
	}
	if model.state != errorState {
		t.Fatalf("state = %d, want errorState", model.state)
	}
	if !strings.Contains(model.recoverableError.Reason, "MISSING_CONNECTION_STRING") {
		t.Fatalf("recoverableError.Reason = %q, want unavailable reason", model.recoverableError.Reason)
	}
	view := model.View()
	for _, expected := range []string{"Error", "Profile \"Production\" is unavailable.", "Context:", "Reason:", "Recovery", "MISSING_CONNECTION_STRING"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want recoverable error detail %q", view, expected)
		}
	}
}

func numberedProfiles(count int) []config.Profile {
	profiles := make([]config.Profile, 0, count)
	for index := 1; index <= count; index++ {
		profiles = append(profiles, config.Profile{
			Name:  fmt.Sprintf("Profile %02d", index),
			Value: stringPointer(fmt.Sprintf("value-%02d", index)),
		})
	}

	return profiles
}

func acceptProfileSearch(t *testing.T, model Model, filter string) Model {
	t.Helper()

	updatedModel, command := model.Update(runeKey('/'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after opening profile search")
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(filter)})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after typing profile search")
	}
	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil after accepting profile search")
	}

	return model
}

func TestUpdate_QuitsImmediately(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model = updatedModel.(Model)

	if command == nil {
		t.Fatal("command is nil, want quit command")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}
}

func TestUpdate_CtrlCQuitsFromSecondaryViews(t *testing.T) {
	tests := []struct {
		name      string
		openKey   tea.KeyMsg
		wantState viewState
	}{
		{name: "inspection", openKey: runeKey('i'), wantState: inspectState},
		{name: "confirmation", openKey: tea.KeyMsg{Type: tea.KeyEnter}, wantState: confirmState},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			model := New(app.New(
				config.Target{},
				[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
			))

			updatedModel, _ := model.Update(testCase.openKey)
			model = updatedModel.(Model)
			if model.state != testCase.wantState {
				t.Fatalf("state after opening = %d, want %d", model.state, testCase.wantState)
			}

			updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			model = updatedModel.(Model)

			if command == nil {
				t.Fatal("command is nil, want quit command")
			}
		})
	}
}
