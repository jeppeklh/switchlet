package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestCompactPicker_PreservesSelectionAndActionsAcrossResize(t *testing.T) {
	for _, count := range []int{1, 8, 100} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			model := New(app.New(config.Target{}, numberedProfiles(count)))
			model.cursor = count - 1
			selected := model.profiles[model.cursor].Name
			model.currentProfiles = map[string]struct{}{selected: {}}
			for _, size := range [][2]int{{120, 32}, {60, 16}, {60, 20}, {80, 24}, {99, 16}, {100, 16}, {120, 32}, {60, 16}} {
				model = resizedMainModel(t, model, size[0], size[1])
				view := model.View()
				for _, want := range []string{"> " + selected + " [current]", "Enter Apply+Exit", "Space Apply", "q Quit"} {
					if !strings.Contains(view, want) {
						t.Fatalf("%dx%d view missing %q:\n%s", size[0], size[1], want, view)
					}
				}
				assertVisibleWidth(t, view, size[0])
				assertVisibleHeight(t, view, size[1])
				if model.cursor != count-1 {
					t.Fatal("resize changed selected profile")
				}
				if size[0] == 60 && size[1] == 16 {
					start, end := model.visibleProfileRange()
					if end-start != min(count, 7) {
						t.Fatalf("visible rows = %d, want %d", end-start, min(count, 7))
					}
				}
			}
			updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
			model = updated.(Model)
			if model.cursor != max(0, count-1-7) {
				t.Fatalf("page-up cursor = %d, want one visible page", model.cursor)
			}
			updated, _ = model.Update(runeKey('/'))
			model = updated.(Model)
			if !strings.Contains(model.View(), "Enter Apply filter") || strings.Contains(model.View(), "i Inspect") {
				t.Fatalf("search should show input actions:\n%s", model.View())
			}
			updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
			model = updated.(Model)
			model = acceptProfileSearch(t, model, "no match exists")
			if !strings.Contains(model.View(), "No profiles match") || strings.Contains(model.View(), "Enter Apply") {
				t.Fatalf("empty filter should not advertise apply:\n%s", model.View())
			}
			updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
			model = updated.(Model)
			if !strings.Contains(model.View(), "Enter Apply+Exit") {
				t.Fatal("clearing filter did not restore apply")
			}
		})
	}
}

func TestCompactPicker_ProtectedUnavailableAndUnicodeContext(t *testing.T) {
	t.Setenv("SWITCHLET_COMPACT_MISSING", "")
	model := New(app.New(config.Target{File: "設定/サービス.json", JSONPath: "service.url"}, []config.Profile{
		{Name: "本番 🔒 " + strings.Repeat("長い名前", 20), Protected: true, Value: stringPointer("private-value")},
		{Name: "Unavailable", ValueFromEnv: stringPointer("SWITCHLET_COMPACT_MISSING")},
	}))
	model = resizedMainModel(t, model, 60, 16)
	view := model.View()
	for _, want := range []string{"> 本番", "protected", "x Unavailable", "i Inspect", "? Help"} {
		if !strings.Contains(view, want) {
			t.Fatalf("compact view missing %q:\n%s", want, view)
		}
	}
	assertVisibleWidth(t, view, 60)
	updated, _ := model.Update(runeKey('j'))
	model = updated.(Model)
	if !strings.Contains(model.View(), "Unavailable · 1 target") || !strings.Contains(model.View(), "Enter Details") {
		t.Fatalf("unavailable selection lost its state:\n%s", model.View())
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil || model.state != errorState || !strings.Contains(model.View(), "SWITCHLET_COMPACT_MISSING") {
		t.Fatalf("unavailable profile should show recovery details:\n%s", model.View())
	}
}

func TestCompactDetails_RevealHideAndHelpScroll(t *testing.T) {
	model := New(app.New(config.Target{File: "設定/サービス.json", JSONPath: "service.url"}, []config.Profile{
		{Name: "Local", Value: stringPointer("private-value")},
	}))
	model = resizedMainModel(t, model, 60, 16)
	updated, _ := model.Update(runeKey('i'))
	model = updated.(Model)
	if strings.Contains(model.View(), "Profiles") || strings.Contains(model.View(), "private-value") {
		t.Fatalf("inspection should focus on hidden detail:\n%s", model.View())
	}
	updated, _ = model.Update(runeKey('v'))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(Model)
	if !strings.Contains(model.View(), "private-value") || !strings.Contains(model.View(), "サービス.json") {
		t.Fatalf("revealed contents should be reachable by scrolling:\n%s", model.View())
	}
	detailOffset := model.scrollOffset
	updated, _ = model.Update(runeKey('?'))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(Model)
	if !model.helpOpen || model.scrollOffset != detailOffset {
		t.Fatal("help changed the detail viewport")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	updated, _ = model.Update(runeKey('v'))
	model = updated.(Model)
	if strings.Contains(model.View(), "private-value") || model.valuesVisible {
		t.Fatal("hiding values left revealed data visible")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	updated, _ = model.Update(runeKey('?'))
	model = updated.(Model)
	if model.helpScrollOffset != 0 {
		t.Fatal("opening help did not reset its viewport")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(Model)
	if model.helpScrollOffset == 0 || !strings.Contains(model.View(), "Refresh") || !strings.Contains(model.View(), "Quit") {
		t.Fatalf("help's remaining actions are unreachable:\n%s", model.View())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyHome})
	model = updated.(Model)
	if model.helpScrollOffset != 0 || !strings.Contains(model.View(), "Available actions") {
		t.Fatal("help did not return to the top")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	updated, _ = model.Update(runeKey('v'))
	model = updated.(Model)
	if !strings.Contains(model.View(), "v Hide values") || strings.Contains(model.View(), "private-value") {
		t.Fatalf("compact picker should indicate reveal without displaying values:\n%s", model.View())
	}
	updated, _ = model.Update(runeKey('i'))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(Model)
	if !strings.Contains(model.View(), "private-value") {
		t.Fatal("inspection did not retain the picker's reveal setting")
	}
}

func TestCompactPicker_RejectsSizesBelowSupportedBoundary(t *testing.T) {
	base := New(app.New(config.Target{}, numberedProfiles(1)))
	for _, size := range [][2]int{{59, 16}, {60, 15}} {
		model := resizedMainModel(t, base, size[0], size[1])
		if !strings.Contains(model.View(), "Minimum size: 60x16") {
			t.Fatalf("%dx%d should request resize:\n%s", size[0], size[1], model.View())
		}
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Fatal("undersized terminal accepted apply")
		}
	}
}

func TestView_ShowsTooSmallTerminalMessage(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 59, Height: 19})
	model = updatedModel.(Model)

	view := model.View()
	if !strings.Contains(view, "Terminal too small.") {
		t.Fatalf("View() = %q, want too-small message", view)
	}
	if !strings.Contains(view, "Minimum size: 60x16") {
		t.Fatalf("View() = %q, want minimum size guidance", view)
	}
	if !strings.Contains(view, "Current size: 59x19") {
		t.Fatalf("View() = %q, want current size guidance", view)
	}
	if !strings.Contains(view, "q Quit") {
		t.Fatalf("View() = %q, want quit guidance", view)
	}
}

func TestView_MainPickerUsesCompactNarrowTerminalLayout(t *testing.T) {
	model := New(app.New(
		config.Target{File: "config/development.json", JSONPath: "database.primary.url"},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	model = updatedModel.(Model)

	view := model.View()
	if strings.Contains(view, "Terminal too small") {
		t.Fatalf("View() = %q, want compact main picker instead of too-small state", view)
	}
	for _, expected := range []string{"* Profiles", "> Local", "i Inspect", "? Help", "q Quit"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("View() = %q, want compact main picker content %q", view, expected)
		}
	}
	assertVisibleWidth(t, view, 60)
	assertVisibleHeight(t, view, 20)
	assertMainCommandBarAtBottom(t, view, "q Quit")
}

func TestView_TooSmallTerminalMessageStaysWithinReportedWidth(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	model = updatedModel.(Model)

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 20 {
			t.Fatalf("line %q has width %d, want at most 20", line, lipgloss.Width(line))
		}
	}
	assertVisibleHeight(t, view, 10)
}

func TestView_CommandBarUsesReportedTerminalHeight(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 200, height: 60},
		{width: 120, height: 40},
		{width: 80, height: 24},
		{width: 60, height: 20},
		{width: 60, height: 16},
	} {
		model := New(app.New(
			config.Target{},
			[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
		))

		updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		model = updatedModel.(Model)

		lines := visibleLines(model.View())
		if len(lines) != size.height {
			t.Fatalf("View() at %dx%d rendered %d lines, want %d", size.width, size.height, len(lines), size.height)
		}
		if !strings.Contains(lines[len(lines)-1], "q Quit") {
			t.Fatalf("last line at %dx%d = %q, want command bar at bottom", size.width, size.height, lines[len(lines)-1])
		}
	}
}

func TestView_LongRecoverableErrorKeepsCommandBarVisible(t *testing.T) {
	model := New(app.New(
		config.Target{File: "/very/long/project/path/backend/appsettings.Development.json", JSONPath: "services.database.primary.connectionStrings.defaultConnection.value"},
		[]config.Profile{{Name: "Local", Value: stringPointer("Server=localhost;Database=App;")}},
	))
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updatedModel.(Model)
	model.state = errorState
	model.recoverableError = RecoverableError{
		Problem:  "Action could not continue.",
		Reason:   strings.Repeat("target preparation failed because the configured path could not be validated safely ", 12),
		Recovery: "Fix the selected profile or target, then try again.",
	}

	view := model.View()
	assertVisibleWidth(t, view, 80)
	assertVisibleHeight(t, view, 24)
	lines := visibleLines(view)
	if !strings.Contains(lines[len(lines)-1], "q Quit") {
		t.Fatalf("last line = %q, want command bar at bottom", lines[len(lines)-1])
	}
}

func TestUpdate_DoesNotApplyHiddenActionsWhenTerminalIsTooSmall(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", strings.TrimSpace(`
{
	  "service": {
	    "baseUrl": "https://old.example.test"
	  }
}
`)+"\n")

	model := New(app.New(
		config.Target{File: targetPath, JSONPath: "service.baseUrl"},
		[]config.Profile{{Name: "Local", Value: stringPointer("https://new.example.test")}},
	))

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 59, Height: 19})
	model = updatedModel.(Model)

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updatedModel.(Model)

	if command != nil {
		t.Fatal("command is not nil, want no hidden apply command")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}

	updatedContents := readFile(t, targetPath)
	if strings.Contains(string(updatedContents), "https://new.example.test") {
		t.Fatalf("updated target = %q, want no file modification while terminal is too small", string(updatedContents))
	}
}

func TestUpdate_TooSmallTerminalStillAllowsDocumentedQuitAndCancelKeys(t *testing.T) {
	model := New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
	))

	updatedModel, _ := model.Update(runeKey('i'))
	model = updatedModel.(Model)
	if model.state != inspectState {
		t.Fatalf("state = %d, want inspectState", model.state)
	}

	updatedModel, _ = model.Update(tea.WindowSizeMsg{Width: 59, Height: 19})
	model = updatedModel.(Model)
	if !strings.Contains(model.View(), "q Quit") {
		t.Fatalf("View() = %q, want q quit command-bar copy", model.View())
	}

	updatedModel, command := model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want q to quit from too-small terminal state")
	}

	model = New(app.New(
		config.Target{},
		[]config.Profile{{Name: "Production", Value: stringPointer("Server=prod;Database=App;"), Protected: true}},
	))
	updatedModel, _ = model.Update(runeKey('i'))
	model = updatedModel.(Model)
	updatedModel, _ = model.Update(tea.WindowSizeMsg{Width: 59, Height: 19})
	model = updatedModel.(Model)

	updatedModel, command = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updatedModel.(Model)
	if command != nil {
		t.Fatal("command is not nil, want no command when cancelling inspection from too-small terminal")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState", model.state)
	}

	updatedModel, command = model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if command == nil {
		t.Fatal("command is nil, want quit command from too-small list view")
	}
}
