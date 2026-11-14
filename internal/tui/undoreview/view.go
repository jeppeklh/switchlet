package undoreview

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

// View renders value-safe metadata and explicit actions within the terminal size.
func (model Model) View() string {
	if model.tooSmall() {
		actions := []ui.Action{{Key: "Esc", Label: "Back"}, {Key: "q", Label: "Quit"}}
		if model.state == committing {
			actions = []ui.Action{{Key: "q", Label: "Quit"}}
		}
		return ui.RenderShell(ui.Shell{Headerless: true, Width: model.width, Height: model.height,
			Panels:  []ui.Panel{{Title: "Resize required", Lines: []string{"Terminal too small.", "Minimum size: 60x16"}}},
			Actions: actions,
		})
	}
	shell := model.shell()
	metrics := ui.PanelScrollMetricsForShell(shell, 0)
	shell.Panels[0].Scroll = &ui.PanelScroll{Offset: metrics.ClampOffset(model.scrollOffset)}
	if metrics.CanScroll() {
		shell.Actions = append(shell.Actions, ui.PanelScrollActions()...)
	}
	return ui.RenderShell(shell)
}

func (model Model) shell() ui.Shell {
	title, lines := model.content()
	// Wrap complete paths and reasons so scrolling can expose every affected file.
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, ui.PrimaryPanelWidth(model.width, 1), true), "\n")...)
	}
	return ui.Shell{Headerless: true, Width: model.width, Height: model.height,
		Panels: []ui.Panel{{Title: title, Lines: wrapped, Focused: true}}, Actions: model.actions()}
}

func (model Model) content() (string, []string) {
	switch model.state {
	case loadingStatus:
		return "Undo and recovery", []string{"Checking recording and recovery status..."}
	case loadingReview:
		return "Review undo action", []string{"Checking the current record and affected files..."}
	case committing:
		return "Undo and recovery", []string{"Completing the confirmed action...", "Wait for the result before applying another profile."}
	case confirmation:
		if model.review != nil {
			return model.reviewContent(*model.review)
		}
	case completed:
		lines := []string{undoCompletionMessage(model.result)}
		if model.statusErr != nil {
			lines = append(lines, "Undo status could not be refreshed. Press r to retry.")
		}
		return "Undo result", append(lines, model.statusLines()...)
	case failed:
		lines := []string{"Action could not complete."}
		if model.err != nil {
			lines = append(lines, model.err.Error())
		}
		lines = append(lines, "", "Press r to refresh and review the current record.")
		if model.status == nil || model.statusErr != nil {
			lines = append(lines, "CLI fallback: switchlet undo --status", "If the record is corrupt, inspect files before using", "switchlet undo --discard; discard does not restore files.")
		}
		return "Undo needs attention", append(lines, model.statusLines()...)
	}
	lines := model.statusLines()
	if action := model.restoreAction(); action != "" {
		lines = append([]string{"Restoration has not been checked. Press Enter to review."}, lines...)
	} else if model.status != nil && !model.status.Enabled {
		lines = append(lines, "", "Enable recording before a changed apply to save a snapshot.")
	} else {
		lines = append(lines, "", "No recorded apply is available to restore.")
	}
	return "Undo and recovery", lines
}

func (model Model) statusLines() []string {
	if model.status == nil || model.statusErr != nil {
		return nil
	}
	return model.metadata(*model.status)
}

func (model Model) metadata(result app.UndoResult) []string {
	recording := "Disabled"
	if result.Enabled {
		recording = "Enabled"
	}
	lines := []string{ui.RenderKeyValue("Recording", recording), ui.RenderKeyValue("Snapshot", snapshotLabel(result.Status))}
	if result.ProfileName != "" {
		lines = append(lines, ui.RenderKeyValue("Recorded apply", fmt.Sprintf("%q", result.ProfileName)), ui.RenderKeyValue("Recorded at", result.CreatedAt.Format(time.RFC3339)))
	}
	if len(result.Files) > 0 {
		lines = append(lines, "", fmt.Sprintf("Affected files (%d)", len(result.Files)))
		for _, path := range result.Files {
			if relative, err := filepath.Rel(model.projectRoot, path); model.projectRoot != "" && err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				path = relative
			}
			lines = append(lines, fmt.Sprintf("  %q", filepath.ToSlash(path)))
		}
	}
	return lines
}

func (model Model) reviewContent(review app.UndoResult) (string, []string) {
	var title string
	var lines []string
	switch review.Action {
	case "enable":
		title = "Enable undo recording?"
		lines = []string{"Whole-file snapshots can contain secrets, including", "unmanaged values. They are private but NOT encrypted.", "Only the last changed apply is retained.", "Confirm to enable recording for this configuration."}
	case "discard":
		title = "Discard the saved record?"
		lines = []string{"This deletes the undo/recovery record only.", "It does NOT restore or change any target file.", "Recovery from this record will no longer be available.", "Recording remains enabled."}
	default:
		title = "Confirm undo"
		if review.Action == "recover" {
			title = "Confirm interrupted-operation recovery"
		}
		lines = []string{"Restoration is available for the reviewed record.", "Restore exact recorded file state, including unmanaged", "contents. This is not a switch to a previous profile.", "Files and the record will be checked again on confirm."}
	}
	return title, append(append(lines, ""), model.metadata(review)...)
}

func (model Model) actions() []ui.Action {
	actions := []ui.Action{}
	switch model.state {
	case overview:
		if action := model.restoreAction(); action != "" {
			label := "Review undo"
			if action == "recover" {
				label = "Review recovery"
			}
			actions = append(actions, ui.Action{Key: "Enter", Label: label, Priority: ui.ActionPriorityPrimary}, ui.Action{Key: "x", Label: "Discard", Priority: ui.ActionPriorityNormal})
		} else if model.status != nil && !model.status.Enabled {
			actions = append(actions, ui.Action{Key: "e", Label: "Enable recording", Priority: ui.ActionPriorityPrimary})
		}
	case confirmation:
		actions = append(actions, ui.Action{Key: "Enter/y", Label: "Confirm", Priority: ui.ActionPriorityPrimary}, ui.Action{Key: "n/Esc", Label: "Cancel", Priority: ui.ActionPriorityPrimary})
	case completed:
		actions = append(actions, ui.Action{Key: "Enter", Label: "Return", Priority: ui.ActionPriorityPrimary})
	}
	if model.state == overview || model.state == failed || model.state == completed {
		actions = append(actions, ui.Action{Key: "r", Label: "Refresh", Priority: ui.ActionPrioritySecondary})
	}
	if model.state != committing && model.state != confirmation {
		actions = append(actions, ui.Action{Key: "Esc", Label: "Back", Priority: ui.ActionPriorityPrimary})
	}
	return append(actions, ui.Action{Key: "q", Label: "Quit", Priority: ui.ActionPriorityCritical})
}

func snapshotLabel(status string) string {
	switch status {
	case "ready":
		return "Recorded apply"
	case "pending":
		return "Interrupted apply — needs recovery"
	case "undoing":
		return "Interrupted restoration — needs recovery"
	case "empty":
		return "No recorded apply"
	case "disabled":
		return "Recording disabled"
	default:
		return "Unavailable"
	}
}

func undoCompletionMessage(result *app.UndoResult) string {
	if result == nil {
		return "Action completed."
	}
	switch result.Action {
	case "enable":
		return "Undo recording enabled."
	case "discard":
		return "Saved record discarded. Target files were not changed."
	default:
		return "Recorded files restored. The snapshot was consumed."
	}
}
