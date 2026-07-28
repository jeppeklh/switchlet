package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
)

func TestUpdate_StatusActionRequestsComparisonRefreshesAndIgnoresStaleResults(t *testing.T) {
	projectRoot := t.TempDir()
	targetPath := writeTargetFile(t, projectRoot, "config.json", `{"database":{"url":"postgres://local"}}`)
	originalContents := readFile(t, targetPath)
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
	if !strings.Contains(model.View(), "Status comparison complete") {
		t.Fatalf("View() = %q, want ready status feedback", model.View())
	}
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during status comparison")
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
	originalContents := readFile(t, targetPath)
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

	updatedModel, returnCommand := model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if returnCommand != nil {
		t.Fatal("returnCommand is not nil, want q to return from diff loading")
	}
	if model.state != listState || model.cursor != 1 {
		t.Fatalf("state/cursor = %d/%d, want list with Staging still selected", model.state, model.cursor)
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
	if model.state != diffLoadingState || model.diffComparison != nil || model.comparisonProfileName != "Local" {
		t.Fatalf("model after stale diff result = %#v, want Local loading unchanged", model)
	}

	message := currentCommand()
	updatedModel, readyCommand := model.Update(message)
	model = updatedModel.(Model)
	if readyCommand != nil {
		t.Fatal("readyCommand is not nil, want no command after diff result")
	}
	if model.state != diffReadyState || model.diffComparison == nil || model.diffComparison.ProfileName != "Local" {
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
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during diff comparison")
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
		{name: "diff ready esc", state: diffReadyState, key: tea.KeyMsg{Type: tea.KeyEsc}},
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
			}

			updatedModel, command := model.Update(testCase.key)
			model = updatedModel.(Model)
			if command != nil {
				t.Fatal("command is not nil, want return without command")
			}
			if model.state != listState || model.cursor != 1 {
				t.Fatalf("state/cursor = %d/%d, want list with preserved selection", model.state, model.cursor)
			}
			if model.statusComparison != nil || model.diffComparison != nil || !model.comparisonError.IsZero() || model.comparisonRequestKind != comparisonRequestNone {
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
	originalContents := readFile(t, targetPath)
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
	if !bytes.Equal(readFile(t, targetPath), originalContents) {
		t.Fatal("target file changed during failed status comparison")
	}

	updatedModel, returnCommand := model.Update(runeKey('q'))
	model = updatedModel.(Model)
	if returnCommand != nil {
		t.Fatal("returnCommand is not nil, want q to return from comparison error")
	}
	if model.state != listState {
		t.Fatalf("state = %d, want listState after comparison error return", model.state)
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
