package configexport

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

func (model Model) View() string {
	if model.tooSmall() {
		return ui.RenderShell(ui.Shell{Title: "Export example", Panels: []ui.Panel{{Title: "Resize required", Lines: []string{"Minimum size: 80x24", "Resize to continue."}}}, Actions: []ui.Action{{Key: "q", Label: "Cancel"}}, Width: model.width, Height: model.height})
	}
	shell := model.shell()
	if model.help {
		shell.Panels = []ui.Panel{{Title: "Export help", Lines: ui.HelpLines(append(shell.Actions, ui.PanelScrollActions()...), ui.PrimaryPanelWidth(model.width, 1)), Focused: true, FillHeight: true}}
		shell.Actions = ui.HelpReturnActions("Cancel")
		return ui.RenderShell(shell)
	}
	if model.state != nameInput && model.state != preparing && model.state != writing {
		shell.Actions = ui.AppendHelpAction(shell.Actions)
		metrics := model.metrics()
		shell.Panels[0].Scroll = &ui.PanelScroll{Offset: metrics.ClampOffset(model.scroll)}
		if metrics.CanScroll() {
			shell.Actions = append(shell.Actions, ui.PanelScrollActions()...)
		}
	}
	return ui.RenderShell(shell)
}

func (model Model) shell() ui.Shell {
	title := "Export example"
	lines := []string{"Export current draft to .switchlet.example.yaml.", "Replace every literal with an environment name you choose.", "Existing references are preserved. No environment values are exported.", ""}
	actions := []ui.Action{{Key: "Esc/q", Label: "Cancel"}}
	switch model.state {
	case editing:
		lines = append(lines, fmt.Sprintf("Entry %d of %d", min(model.cursor+1, len(model.draft.Entries)), len(model.draft.Entries)))
		start := max(0, model.cursor-4)
		for index := start; index < min(len(model.draft.Entries), start+5); index++ {
			entry := model.draft.Entries[index]
			name := entry.EnvironmentName
			if name == "" {
				name = "name required"
			}
			marker := "  "
			if index == model.cursor {
				marker = "> "
			}
			if entry.ExistingReference {
				name += " (keep)"
			}
			lines = append(lines, ansi.Truncate(marker+entry.ProfileName+" / "+entry.TargetName+" -> "+name, ui.PrimaryPanelWidth(model.width, 1), "…"))
		}
		if len(model.draft.Entries) > 0 {
			lines = append(lines, " ")
			lines = append(lines, model.entryDetails(model.draft.Entries[model.cursor])...)
		}
		actions = append([]ui.Action{{Key: "e", Label: "Environment name"}, {Key: "Enter", Label: "Review export"}, {Key: "j/k", Label: "Move"}, {Key: "g/G", Label: "First/Last"}}, actions...)
	case nameInput:
		title = "Environment reference"
		entry := model.draft.Entries[model.cursor]
		lines = []string{ui.RenderInputWithinWidth("Name", string(model.input), model.inputCursor, ui.PrimaryPanelWidth(model.width, 1)), "Enter a variable NAME, never its value. q is literal text.", "Names use letters, digits, and underscores; no leading digit.", ""}
		lines = append(lines, model.entryDetails(entry)...)
		actions = []ui.Action{{Key: "Enter", Label: "Use name"}, {Key: "Esc", Label: "Back"}, {Key: "Ctrl+U", Label: "Clear"}, {Key: "Left/Right", Label: "Move"}}
	case preparing:
		lines = []string{"Preparing sanitized export and checking destination..."}
	case review:
		title = "Review example export"
		action := "Create example"
		if model.prepared.ReplacesExisting {
			lines = []string{"Replace the existing example with this reviewed configuration."}
			action = "Replace example"
		} else {
			lines = []string{"Create a new example configuration."}
		}
		lines = append(lines, "Output: "+model.prepared.Path, "Local configuration and managed files remain unchanged.")
		if len(model.prepared.SharedVariables) > 0 {
			approval := "NOT approved (Space to approve)"
			if model.sharedApproved {
				approval = "approved (Space to revoke)"
			}
			lines = append(lines, "Shared environment sources: "+strings.Join(model.prepared.SharedVariables, ", "), "Shared source approval: "+approval)
			actions = append([]ui.Action{{Key: "Space", Label: "Approve shared sources"}}, actions...)
		}
		lines = append(lines, "", "Managed targets (preserved)")
		for _, target := range model.draft.Targets {
			lines = append(lines, target.TargetName+" ["+string(target.TargetType)+"]", "File: "+app.DisplayInitTargetPath(model.document.ProjectRoot, target.TargetFile), target.SelectorName+": "+target.Selector)
		}
		lines = append(lines, "", "Profile environment sources")
		for _, entry := range model.draft.Entries {
			lines = append(lines, model.entryDetails(entry)...)
			lines = append(lines, "")
		}
		actions = append([]ui.Action{{Key: "Enter", Label: action}}, actions...)
	case writing:
		lines = []string{"Writing the reviewed example..."}
		actions = nil
	case completed:
		title = "Example exported"
		lines = []string{"Created: " + model.prepared.Path, "Share this example, then copy it to .switchlet.yaml in the receiving project.", "Run switchlet doctor for the environment readiness checklist.", "Supply values through your shell or secret tooling.", "Your active configuration draft is unchanged."}
		actions = []ui.Action{{Key: "Enter/Esc/q", Label: "Back to editor"}}
	case failed:
		title = "Export needs attention"
		lines = []string{"Review export again before retrying."}
		actions = []ui.Action{{Key: "Enter/Esc", Label: "Back to names"}, {Key: "q", Label: "Cancel"}}
	}
	if model.err != "" {
		lines = append(lines, "", model.err)
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, ui.PrimaryPanelWidth(model.width, 1), true), "\n")...)
	}
	return ui.Shell{Title: "Switchlet config", Headerless: true, Panels: []ui.Panel{{Title: title, Lines: wrapped, Focused: true, FillHeight: true}}, Actions: actions, Width: model.width, Height: model.height}
}

func (model Model) entryDetails(entry app.ExampleExportEntry) []string {
	source := entry.EnvironmentName
	if source == "" {
		source = "name required"
	}
	return []string{fmt.Sprintf("%s / %s -> %s", entry.ProfileName, entry.TargetName, source),
		fmt.Sprintf("Protected: %t   Type: %s", entry.Protected, entry.ValueType),
		"File: " + app.DisplayInitTargetPath(model.document.ProjectRoot, entry.TargetFile),
		fmt.Sprintf("%s: %s (%s)", entry.SelectorName, entry.Selector, entry.TargetType)}
}

func (model Model) metrics() ui.PanelScrollMetrics {
	return ui.PanelScrollMetricsForShell(model.shell(), 0)
}
func (model *Model) scrollKey(key tea.KeyMsg) bool {
	if model.state == preparing || model.state == writing {
		return false
	}
	metrics := model.metrics()
	if !metrics.CanScroll() {
		return false
	}
	switch key.Type {
	case tea.KeyPgDown:
		model.scroll += metrics.PageStep()
	case tea.KeyPgUp:
		model.scroll -= metrics.PageStep()
	case tea.KeyHome:
		model.scroll = 0
	case tea.KeyEnd:
		model.scroll = metrics.MaxOffset
	default:
		return false
	}
	model.scroll = metrics.ClampOffset(model.scroll)
	return true
}
