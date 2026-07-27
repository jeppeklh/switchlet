package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestView_ManifestoSurfacesFitHostileDimensions(t *testing.T) {
	t.Setenv("LONG_DATABASE_URL", "Server=very-long-host.example.test;Database=Application;Password=super-secret;")

	baseModel := New(app.NewWithTargets(
		[]config.Target{
			{Name: "database-with-a-very-long-target-name", File: "/very/long/project/path/backend/appsettings.Development.json", Type: config.TargetTypeJSON, JSONPath: "services.database.primary.connectionStrings.defaultConnection.value"},
			{Name: "worker-queue-with-a-very-long-target-name", File: "/very/long/project/path/worker/configuration/config.yaml", Type: config.TargetTypeYAML, YAMLPath: "services.worker.queue.primary.endpoint.with.many.segments"},
			{Name: "frontend-api-with-a-very-long-target-name", File: "/very/long/project/path/frontend/.env.local", Type: config.TargetTypeDotenv, Key: "VITE_APPLICATION_BACKEND_API_URL"},
		},
		[]config.Profile{{
			Name:      "Production profile with a very long display name",
			Protected: true,
			Values: []config.ProfileValue{
				{Target: "database-with-a-very-long-target-name", ValueFromEnv: stringPointer("LONG_DATABASE_URL")},
				{Target: "worker-queue-with-a-very-long-target-name", Value: stringPointer("https://queue.production.example.test/with/a/long/path")},
				{Target: "frontend-api-with-a-very-long-target-name", Value: stringPointer("https://api.production.example.test/with/a/long/path")},
			},
		}},
	))

	sizes := []struct {
		width  int
		height int
	}{
		{width: 200, height: 60},
		{width: 120, height: 40},
		{width: 80, height: 24},
		{width: 60, height: 20},
		{width: 40, height: 15},
	}

	for _, size := range sizes {
		t.Run(strings.Join([]string{"list", sizeLabel(size.width, size.height)}, "_"), func(t *testing.T) {
			model := resizedMainModel(t, baseModel, size.width, size.height)
			assertManifestoMainView(t, model.View(), size.width, size.height, "q Quit")
		})

		t.Run(strings.Join([]string{"applying", sizeLabel(size.width, size.height)}, "_"), func(t *testing.T) {
			model := resizedMainModel(t, baseModel, size.width, size.height)
			model.applyingProfile = "Production profile with a very long display name"
			expectedAction := "Ctrl+C Exit immediately"
			if size.width < minimumTerminalWidth || size.height < minimumTerminalHeight {
				expectedAction = "q Quit"
			}
			assertManifestoMainView(t, model.View(), size.width, size.height, expectedAction)
		})

		if size.width < minimumTerminalWidth || size.height < minimumTerminalHeight {
			continue
		}

		t.Run(strings.Join([]string{"inspection", sizeLabel(size.width, size.height)}, "_"), func(t *testing.T) {
			model := resizedMainModel(t, baseModel, size.width, size.height)
			updatedModel, command := model.Update(runeKey('i'))
			if command != nil {
				t.Fatal("command is not nil, want no command when opening inspection")
			}
			model = updatedModel.(Model)
			view := model.View()
			assertManifestoMainView(t, view, size.width, size.height, "Return")
			if strings.Contains(view, "super-secret") {
				t.Fatalf("inspection View() = %q, must not contain unmasked environment value", view)
			}
		})

		t.Run(strings.Join([]string{"confirmation", sizeLabel(size.width, size.height)}, "_"), func(t *testing.T) {
			model := resizedMainModel(t, baseModel, size.width, size.height)
			updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if command != nil {
				t.Fatal("command is not nil, want confirmation before apply")
			}
			model = updatedModel.(Model)
			view := model.View()
			assertManifestoMainView(t, view, size.width, size.height, "Cancel")
			for _, forbidden := range []string{"super-secret", "Password=****", "https://queue.production.example.test/with/a/long/path", "https://api.production.example.test/with/a/long/path"} {
				if strings.Contains(view, forbidden) {
					t.Fatalf("confirmation View() = %q, must not contain resolved value %q", view, forbidden)
				}
			}
		})

		t.Run(strings.Join([]string{"error", sizeLabel(size.width, size.height)}, "_"), func(t *testing.T) {
			model := resizedMainModel(t, baseModel, size.width, size.height)
			model.state = errorState
			model.recoverableError = RecoverableError{
				Problem: "Action could not continue.",
				Context: []string{
					"Profile: Production profile with a very long display name",
					"Target: frontend-api-with-a-very-long-target-name [dotenv]",
					"Selector: VITE_APPLICATION_BACKEND_API_URL",
				},
				Reason:   strings.Repeat("target validation could not complete safely for the selected configuration value ", 8),
				Recovery: "Return to the profile list, inspect the target context, and try again.",
			}
			assertManifestoMainView(t, model.View(), size.width, size.height, "q Quit")
		})

		t.Run(strings.Join([]string{"success", sizeLabel(size.width, size.height)}, "_"), func(t *testing.T) {
			model := resizedMainModel(t, baseModel, size.width, size.height)
			model.state = successState
			model.successResult = &app.Result{
				ProfileName: "Production profile with a very long display name",
				Changes: []app.PlannedChange{
					{TargetName: "database-with-a-very-long-target-name", TargetFile: "/very/long/project/path/backend/appsettings.Development.json", TargetType: config.TargetTypeJSON, SelectorName: "jsonPath", Selector: "services.database.primary.connectionStrings.defaultConnection.value"},
					{TargetName: "worker-queue-with-a-very-long-target-name", TargetFile: "/very/long/project/path/worker/configuration/config.yaml", TargetType: config.TargetTypeYAML, SelectorName: "yamlPath", Selector: "services.worker.queue.primary.endpoint.with.many.segments"},
					{TargetName: "frontend-api-with-a-very-long-target-name", TargetFile: "/very/long/project/path/frontend/.env.local", TargetType: config.TargetTypeDotenv, SelectorName: "key", Selector: "VITE_APPLICATION_BACKEND_API_URL"},
				},
			}
			view := model.View()
			assertVisibleWidth(t, view, size.width)
			assertVisibleHeight(t, view, size.height)
			for _, forbidden := range []string{"super-secret", "https://queue.production.example.test/with/a/long/path", "https://api.production.example.test/with/a/long/path"} {
				if strings.Contains(view, forbidden) || strings.Contains(model.FinalMessage(), forbidden) {
					t.Fatalf("success output must not contain resolved value %q\nview: %q\nfinal: %q", forbidden, view, model.FinalMessage())
				}
			}
		})
	}
}

func TestUpdate_CommandBarsMatchAdvertisedReturnAndCancelKeys(t *testing.T) {
	for _, key := range []tea.KeyMsg{runeKey('q'), tea.KeyMsg{Type: tea.KeyEsc}, runeKey('n')} {
		model := New(app.New(
			config.Target{},
			[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
		))
		updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model = updatedModel.(Model)
		if command != nil {
			t.Fatal("command is not nil, want confirmation before apply")
		}
		if !strings.Contains(model.View(), "n/Esc/q Cancel") {
			t.Fatalf("confirmation View() = %q, want advertised cancel keys", model.View())
		}

		updatedModel, command = model.Update(key)
		model = updatedModel.(Model)
		if command != nil {
			t.Fatalf("command is not nil after %q, want return to list without quitting", key.String())
		}
		if model.state != listState {
			t.Fatalf("state after %q = %d, want listState", key.String(), model.state)
		}
	}

	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", ValueFromEnv: stringPointer("MISSING_CONNECTION_STRING")}},
	))
	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no apply command for unavailable profile")
	}
	if !strings.Contains(model.View(), "Any key Return") || !strings.Contains(model.View(), "q Quit") {
		t.Fatalf("error View() = %q, want advertised return and quit keys", model.View())
	}

	updatedModel, command = model.Update(runeKey('x'))
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want any non-q key to return without quitting")
	}
	if model.state != listState || !model.recoverableError.IsZero() {
		t.Fatalf("model after error return = %#v, want list with cleared error", model)
	}

	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)
	updatedModel, command = model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want q to quit from advertised error view")
	}
}

func TestRecoverableErrorLines_WrapsLongFallbackErrors(t *testing.T) {
	model := New(app.New(
		config.Target{File: "/very/long/project/path/backend/appsettings.Development.json", JSONPath: "services.database.primary.connectionStrings.defaultConnection.value"},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)
	model.state = errorState
	model.recoverableError = model.genericRecoverableError(errors.New(strings.Repeat("unexpected target preparation failure ", 12)))

	view := model.View()
	assertVisibleWidth(t, view, 80)
	assertVisibleHeight(t, view, 24)
	assertMainCommandBarAtBottom(t, view, "q Quit")
}

func resizedMainModel(t *testing.T, model Model, width int, height int) Model {
	t.Helper()

	updatedModel, command := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	if command != nil {
		t.Fatal("command is not nil after window resize")
	}

	return updatedModel.(Model)
}

func assertManifestoMainView(t *testing.T, view string, width int, height int, expectedBottomAction string) {
	t.Helper()

	assertVisibleWidth(t, view, width)
	assertVisibleHeight(t, view, height)
	if expectedBottomAction != "" {
		assertMainCommandBarAtBottom(t, view, expectedBottomAction)
	}
}

func assertMainCommandBarAtBottom(t *testing.T, view string, expectedAction string) {
	t.Helper()

	lines := visibleLines(view)
	if len(lines) == 0 {
		t.Fatal("View() rendered no visible lines")
	}
	if !strings.Contains(lines[len(lines)-1], expectedAction) {
		t.Fatalf("last line = %q, want command bar action %q", lines[len(lines)-1], expectedAction)
	}
}

func sizeLabel(width int, height int) string {
	return fmt.Sprintf("%dx%d", width, height)
}
