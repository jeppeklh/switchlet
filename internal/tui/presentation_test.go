package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderListRow_StatesRemainDistinctWithoutColor(t *testing.T) {
	tests := []struct {
		name string
		row  ListRow
		want string
	}{
		{name: "normal", row: ListRow{Label: "Local", State: RowNormal}, want: "  Local"},
		{name: "selected", row: ListRow{Label: "Local", State: RowSelected}, want: "> Local"},
		{name: "inactive selected", row: ListRow{Label: "Local", State: RowInactiveSelected}, want: "~ Local"},
		{name: "disabled", row: ListRow{Label: "Local", State: RowDisabled}, want: "x Local"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := RenderListRow(testCase.row)
			if !strings.Contains(got, testCase.want) {
				t.Fatalf("RenderListRow() = %q, want visible row %q", got, testCase.want)
			}
		})
	}
}

func TestRenderListRow_AppendsCompactBadges(t *testing.T) {
	row := ListRow{
		Label:  "Production",
		State:  RowSelected,
		Badges: []Badge{{Label: "protected"}, {Label: "unavailable"}, {Label: "env"}},
	}

	got := RenderListRow(row)
	for _, expected := range []string{"> Production", "[protected]", "[unavailable]", "[env]"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("RenderListRow() = %q, want visible %q", got, expected)
		}
	}
}

func TestRenderCommandBar_GroupsKeyboardActions(t *testing.T) {
	got := RenderCommandBar([]Action{
		{Key: "↑/↓ or j/k", Label: "Move"},
		{Key: "Enter", Label: "Apply"},
		{Key: "i", Label: "Inspect"},
		{Key: "q", Label: "Quit"},
	})

	for _, expected := range []string{"↑/↓ or j/k", "Move", "Enter", "Apply", "i", "Inspect", "q", "Quit"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("RenderCommandBar() = %q, want visible %q", got, expected)
		}
	}
}

func TestRenderInput_ClampsCursorAndShowsInsertionPoint(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		want   string
	}{
		{name: "negative", cursor: -1, want: "Profile name: _Local"},
		{name: "middle", cursor: 2, want: "Profile name: Lo_cal"},
		{name: "past end", cursor: 99, want: "Profile name: Local_"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := RenderInput("Profile name", "Local", testCase.cursor)
			if !strings.Contains(got, testCase.want) {
				t.Fatalf("RenderInput() = %q, want visible input %q", got, testCase.want)
			}
		})
	}
}

func TestRenderInputWithinWidth_KeepsCursorVisibleForLongValues(t *testing.T) {
	got := RenderInputWithinWidth("Literal value", "postgres://very-long-host-name.example.test/database", 51, 32)

	if lipgloss.Width(got) > 32 {
		t.Fatalf("RenderInputWithinWidth() = %q with width %d, want at most 32", got, lipgloss.Width(got))
	}
	if !strings.Contains(got, "_") {
		t.Fatalf("RenderInputWithinWidth() = %q, want visible cursor", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("RenderInputWithinWidth() = %q, want truncated long value marker", got)
	}
}

func TestPrimaryPanelWidth_MatchesShellSplitBehavior(t *testing.T) {
	if got := PrimaryPanelWidth(120, 2); got != 62 {
		t.Fatalf("PrimaryPanelWidth(120, 2) = %d, want 62", got)
	}
	if got := PrimaryPanelWidth(80, 2); got != 76 {
		t.Fatalf("PrimaryPanelWidth(80, 2) = %d, want 76", got)
	}
}

func TestRenderStepProgress_MarksCurrentStep(t *testing.T) {
	got := RenderStepProgress(2, []string{"Target", "Path", "Profiles", "Review"})
	want := "1 Target  [2 Path]  3 Profiles  4 Review"
	if got != want {
		t.Fatalf("RenderStepProgress() = %q, want %q", got, want)
	}
}

func TestRenderShell_RendersPanelsAndCommandBar(t *testing.T) {
	got := RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Select a profile",
		Panels: []Panel{
			{Title: "Profiles", Lines: []string{"> Local"}},
			{Title: "Selected", Lines: []string{"Status: ready"}},
		},
		Actions: []Action{{Key: "Enter", Label: "Apply"}, {Key: "q", Label: "Quit"}},
	})

	for _, expected := range []string{"Switchlet", "Profiles", "> Local", "Selected", "Status: ready", "Enter", "Apply", "q", "Quit"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("RenderShell() = %q, want %q", got, expected)
		}
	}
	if strings.Contains(got, "---") {
		t.Fatalf("RenderShell() = %q, want no raw dashed section separators", got)
	}
}

func TestRenderShell_AnchorsCommandBarWhenHeightIsKnown(t *testing.T) {
	got := RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Select a profile",
		Panels: []Panel{
			{Title: "Profiles", Lines: []string{"> Local"}, Focused: true},
			{Title: "Selected", Lines: []string{"Ready"}},
		},
		Actions: []Action{{Key: "Enter", Label: "Apply"}, {Key: "q", Label: "Quit"}},
		Width:   120,
		Height:  20,
	})

	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 20 {
		t.Fatalf("RenderShell() rendered %d lines, want 20", len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "q Quit") {
		t.Fatalf("last line = %q, want command bar at bottom", lines[len(lines)-1])
	}
	if lines[len(lines)-3] != "" {
		t.Fatalf("line before command separator = %q, want vertical whitespace before bottom command bar", lines[len(lines)-3])
	}
}

func TestRenderShell_RendersSplitLayoutAtComfortableWidth(t *testing.T) {
	got := RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Switch a named profile safely",
		Panels: []Panel{
			{Title: "Profiles", Lines: []string{"> Local"}, Focused: true},
			{Title: "Selected", Lines: []string{"Local"}},
		},
		Width: 120,
	})

	if !lineContains(got, "* Profiles", "Selected") {
		t.Fatalf("RenderShell() = %q, want split panel titles on one line", got)
	}
}

func TestRenderShell_RendersStackedLayoutAtMinimumWidth(t *testing.T) {
	got := RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Switch a named profile safely",
		Panels: []Panel{
			{Title: "Profiles", Lines: []string{"> Local"}, Focused: true},
			{Title: "Selected", Lines: []string{"Local"}},
		},
		Width: 80,
	})

	if lineContains(got, "* Profiles", "Selected") {
		t.Fatalf("RenderShell() = %q, want stacked panel titles at minimum width", got)
	}
}

func TestRenderShell_TruncatesLongLinesToShellWidth(t *testing.T) {
	got := RenderShell(Shell{
		Title:    "Switchlet",
		Subtitle: "Switch a named profile safely",
		Metadata: []string{"/very/long/path/to/appsettings.Development.json"},
		Panels: []Panel{{Title: "Selected", Lines: []string{
			"Target file: /very/long/path/to/appsettings.Development.json",
			"Target JSON path: services.database.primary.connectionStrings.defaultConnection.value",
		}}},
		Actions: []Action{{Key: "Enter", Label: "Apply selected profile with a deliberately long action label"}},
		Width:   40,
	})

	for _, line := range strings.Split(got, "\n") {
		if lipgloss.Width(line) > 40 {
			t.Fatalf("line %q has width %d, want at most 40", line, lipgloss.Width(line))
		}
	}
}
