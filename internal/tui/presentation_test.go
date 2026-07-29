package tui

import (
	"fmt"
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

func TestRenderShell_PrioritizesEssentialCommandBarActionsAtConstrainedWidth(t *testing.T) {
	got := RenderShell(Shell{
		Title: "Switchlet",
		Panels: []Panel{{
			Title:   "Profiles",
			Lines:   []string{"> Local"},
			Focused: true,
		}},
		Actions: []Action{
			{Key: "↑/↓ or j/k", Label: "Move", Priority: ActionPrioritySecondary},
			{Key: "PgUp/PgDn", Label: "Page", Priority: ActionPrioritySecondary},
			{Key: "Home/End", Label: "Jump", Priority: ActionPrioritySecondary},
			{Key: "Enter", Label: "Apply", Priority: ActionPriorityPrimary},
			{Key: "i", Label: "Inspect", Priority: ActionPrioritySecondary},
			{Key: "q", Label: "Quit", Priority: ActionPriorityCritical},
		},
		Width: 28,
	})

	commandLine := visibleLines(got)[len(visibleLines(got))-1]
	for _, expected := range []string{"Enter Apply", "q Quit"} {
		if !strings.Contains(commandLine, expected) {
			t.Fatalf("command line = %q, want essential action %q", commandLine, expected)
		}
	}
	for _, omitted := range []string{"Move", "Page", "Jump", "Inspect"} {
		if strings.Contains(commandLine, omitted) {
			t.Fatalf("command line = %q, want low-priority action %q omitted first", commandLine, omitted)
		}
	}
	if lipgloss.Width(commandLine) > 28 {
		t.Fatalf("command line %q has width %d, want at most 28", commandLine, lipgloss.Width(commandLine))
	}
}

func TestRenderShell_PrioritizesModeSpecificReturnActionsAtConstrainedWidth(t *testing.T) {
	got := RenderShell(Shell{
		Title: "Switchlet init",
		Panels: []Panel{{
			Title:   "Value",
			Lines:   []string{"Literal value: local_"},
			Focused: true,
		}},
		Actions: []Action{
			{Key: "Enter", Label: "Save"},
			{Key: "←/→", Label: "Move"},
			{Key: "Bksp/Del", Label: "Edit"},
			{Key: "Esc", Label: "Source"},
			{Key: "Ctrl+C", Label: "Cancel"},
		},
		Width: 40,
	})

	commandLine := visibleLines(got)[len(visibleLines(got))-1]
	for _, expected := range []string{"Enter Save", "Esc Source", "Ctrl+C Cancel"} {
		if !strings.Contains(commandLine, expected) {
			t.Fatalf("command line = %q, want essential action %q", commandLine, expected)
		}
	}
	for _, omitted := range []string{"Move", "Edit"} {
		if strings.Contains(commandLine, omitted) {
			t.Fatalf("command line = %q, want editing action %q omitted first", commandLine, omitted)
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
	if got := PrimaryPanelWidth(120, 2); got != 60 {
		t.Fatalf("PrimaryPanelWidth(120, 2) = %d, want 60", got)
	}
	if got := PrimaryPanelWidth(80, 2); got != 74 {
		t.Fatalf("PrimaryPanelWidth(80, 2) = %d, want 74", got)
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

func TestRenderShell_KnownHeightSplitPanelsKeepTopBorder(t *testing.T) {
	got := RenderShell(Shell{
		Headerless: true,
		Panels: []Panel{
			{Title: "Profiles", Lines: []string{"> Local"}, Focused: true, FillHeight: true},
			{Title: "Profile contents", Lines: []string{"Local"}, FillHeight: true},
		},
		Actions: []Action{{Key: "q", Label: "Quit"}},
		Width:   120,
		Height:  20,
	})

	if strings.HasSuffix(got, "\n") {
		t.Fatalf("RenderShell() = %q, want no trailing newline for known-height shell", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 20 {
		t.Fatalf("RenderShell() rendered %d lines, want 20", len(lines))
	}
	if !strings.Contains(lines[0], "┌") || !strings.Contains(lines[0], "─") || strings.Contains(lines[0], "Profiles") {
		t.Fatalf("first line = %q, want pane top border before title content", lines[0])
	}
	if !strings.Contains(lines[1], "* Profiles") || !strings.Contains(lines[1], "Profile contents") {
		t.Fatalf("second line = %q, want panel titles below top border", lines[1])
	}
}

func TestRenderShell_KnownHeightStackedPanelsKeepTopBorders(t *testing.T) {
	got := RenderShell(Shell{
		Headerless: true,
		Panels: []Panel{
			{Title: "Profiles", Lines: []string{"> Local"}, Focused: true},
			{Title: "Profile contents", Lines: []string{"Local"}},
		},
		Actions: []Action{{Key: "q", Label: "Quit"}},
		Width:   80,
		Height:  20,
	})

	if strings.HasSuffix(got, "\n") {
		t.Fatalf("RenderShell() = %q, want no trailing newline for known-height shell", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 20 {
		t.Fatalf("RenderShell() rendered %d lines, want 20", len(lines))
	}
	if !strings.Contains(lines[0], "┌") || !strings.Contains(lines[0], "─") || strings.Contains(lines[0], "Profiles") {
		t.Fatalf("first line = %q, want first pane top border before title content", lines[0])
	}
	secondPanelTitleIndex := lineIndexContaining(got, "Profile contents")
	if secondPanelTitleIndex <= 0 {
		t.Fatalf("RenderShell() = %q, want second panel title after a top border", got)
	}
	if !strings.Contains(lines[secondPanelTitleIndex-1], "┌") || !strings.Contains(lines[secondPanelTitleIndex-1], "─") {
		t.Fatalf("line before second panel title = %q, want second pane top border", lines[secondPanelTitleIndex-1])
	}
}

func TestRenderShell_ClipsPanelOverflowWithinKnownHeight(t *testing.T) {
	panelLines := make([]string, 0, 20)
	for index := 1; index <= 20; index++ {
		panelLines = append(panelLines, fmt.Sprintf("line %02d", index))
	}

	got := RenderShell(Shell{
		Title: "Switchlet",
		Panels: []Panel{{
			Title:   "Long content",
			Lines:   panelLines,
			Focused: true,
		}},
		Actions: []Action{{Key: "Enter", Label: "Apply"}, {Key: "q", Label: "Quit"}},
		Width:   80,
		Height:  10,
	})

	lines := visibleLines(got)
	if len(lines) != 10 {
		t.Fatalf("RenderShell() rendered %d lines, want 10", len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "q Quit") {
		t.Fatalf("last line = %q, want command bar at bottom", lines[len(lines)-1])
	}
	if !strings.Contains(got, "... ") {
		t.Fatalf("RenderShell() = %q, want intentional overflow marker", got)
	}
	if strings.Contains(got, "line 20") {
		t.Fatalf("RenderShell() = %q, want overflowing content clipped before command bar", got)
	}
}

func TestRenderShell_WindowedPanelOverflowKeepsSelectedRowVisible(t *testing.T) {
	rows := make([]ListRow, 0, 20)
	for index := 1; index <= 20; index++ {
		state := RowNormal
		if index == 18 {
			state = RowSelected
		}
		rows = append(rows, ListRow{Label: fmt.Sprintf("Profile %02d", index), State: state})
	}

	got := RenderShell(Shell{
		Title: "Switchlet",
		Panels: []Panel{{
			Title:   "Profiles",
			Lines:   RenderListRows(rows),
			Focused: true,
		}},
		Actions: []Action{{Key: "Enter", Label: "Apply"}, {Key: "q", Label: "Quit"}},
		Width:   80,
		Height:  10,
	})

	if !strings.Contains(got, "> Profile 18") {
		t.Fatalf("RenderShell() = %q, want selected row to remain visible in clipped panel", got)
	}
	if !strings.Contains(got, "earlier") || !strings.Contains(got, "more") {
		t.Fatalf("RenderShell() = %q, want overflow marker to describe hidden content around selected row", got)
	}
}

func TestRenderShell_RendersSplitLayoutAtComfortableWidth(t *testing.T) {
	got := RenderShell(Shell{
		Title: "Switchlet",
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
		Title: "Switchlet",
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
		Subtitle: "Profile details",
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
