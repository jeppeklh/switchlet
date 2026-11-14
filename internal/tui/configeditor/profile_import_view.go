package configeditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

func (model Model) isProfileImportReview() bool {
	return model.state == editorStateProfileImportMapping || model.state == editorStateProfileImportSelection ||
		(model.state == editorStateProfileReview && model.profileForm.mode == profileDraftImport)
}

func (model Model) profileImportView() string {
	shell := model.profileImportShell()
	scroll := -1
	if model.isProfileImportReview() {
		scroll = 0
	}
	return model.configEditorShellWithScroll(shell.Subtitle, shell.Panels, shell.Actions, scroll)
}

func (model Model) profileImportShell() ui.Shell {
	form := model.profileForm.importForm
	title := "Map source files"
	identity := "Base: " + model.profileForm.originalName + "  New: " + model.profileForm.draft.Name
	if model.state != editorStateProfileReview {
		identity = ansi.Truncate(identity, ui.PrimaryPanelWidth(model.width, 1), "…")
	}
	lines := []string{identity}
	actions := []ui.Action{{Key: "Esc/h", Label: "Back"}, {Key: "q", Label: "Quit"}}
	switch model.state {
	case editorStateProfileImportSourceInput:
		title = "Source file"
		mapping := form.mappings[form.cursor]
		lines = append(lines, ui.RenderInputWithinWidth("Source", model.inputValue, model.inputCursor, ui.PrimaryPanelWidth(model.width, 1)),
			"Destination: "+mapping.TargetFile, "Format: "+string(mapping.TargetType),
			"Relative paths use the config directory. Empty means keep base values.",
			"The source must use the displayed format. q is literal text.")
		actions = []ui.Action{{Key: "Enter", Label: "Map source"}, {Key: "Esc", Label: "Cancel"}}
	case editorStateProfileImportLoading:
		title = "Reading source files"
		lines = append(lines, "Reading configured selectors. No files are changed.")
	case editorStateProfileImportMapping:
		lines = append(lines, "Map each destination to a source of the same format.", "Unmapped files keep the base profile's values.", "")
		labels := make([]string, len(form.mappings))
		for index, mapping := range form.mappings {
			source := "keep base"
			if mapping.SourceFile != "" {
				source = mapping.SourceFile
			}
			labels[index] = fmt.Sprintf("%s (%s) <- %s", app.DisplayInitTargetPath(model.document.ProjectRoot, mapping.TargetFile), mapping.TargetType, source)
		}
		lines = append(lines, model.importListLines(labels)...)
		if len(form.mappings) > 0 {
			mapping := form.mappings[form.cursor]
			lines = append(lines, "", "Destination: "+mapping.TargetFile, "Source: "+mapping.SourceFile)
		}
		actions = append([]ui.Action{{Key: "Enter", Label: "Read sources"}, {Key: "e", Label: "Map source"}, {Key: "x", Label: "Clear source"}, {Key: "j/k", Label: "Move"}}, actions...)
	case editorStateProfileImportSelection:
		title = "Review imported values"
		lines = append(lines, "Select replacements. Unselected entries keep their base source.", "Captured literal values are hidden.", "")
		labels := make([]string, len(form.prepared.Values))
		for index, row := range form.prepared.Values {
			mark := "[ ]"
			if form.selected[row.TargetName] {
				mark = "[x]"
			}
			status := "available"
			if row.Error != "" {
				mark, status = "[-]", "read failed"
			} else if !row.Available {
				mark, status = "[-]", "keep base"
			}
			labels[index] = fmt.Sprintf("%s %s (%s)", mark, row.TargetName, status)
		}
		lines = append(lines, model.importListLines(labels)...)
		if len(form.prepared.Values) > 0 {
			row := form.prepared.Values[form.cursor]
			lines = append(lines, "", "Target: "+row.TargetName, "Destination: "+row.TargetFile, "Source: "+row.SourceFile,
				fmt.Sprintf("%s: %s (%s, %s)", row.SelectorName, row.Selector, row.TargetType, row.ValueType))
			if row.Error != "" {
				lines = append(lines, "Read failed: "+row.Error)
			}
		}
		actions = append([]ui.Action{{Key: "Enter", Label: "Review profile"}, {Key: "Space", Label: "Select"}, {Key: "j/k", Label: "Move"}, {Key: "r", Label: "Refresh sources"}, {Key: "m", Label: "Change mappings"}}, actions...)
	case editorStateProfileReview:
		title = "Review imported profile"
		lines = append(lines, fmt.Sprintf("Protected: %t", model.profileForm.draft.Protected),
			"Selected literals use the reviewed snapshot. Sources are not reread on save.",
			"Save draft, then review and save the configuration.", "")
		for index, value := range model.profileForm.draft.Values {
			row := form.prepared.Values[index]
			status := "keep base"
			if form.selected[value.TargetName] {
				status = "import hidden literal"
			}
			lines = append(lines, value.TargetName+": "+status, "Destination: "+value.TargetFile)
			if form.selected[value.TargetName] {
				lines = append(lines, "Source: "+row.SourceFile)
			} else if value.Source == app.ProfileSourceEnvironment && value.Included {
				lines = append(lines, "Environment reference: "+value.EnvironmentVariableName)
			} else if !value.Included {
				lines = append(lines, "Not included in base profile")
			}
			lines = append(lines, "")
		}
		actions = append([]ui.Action{{Key: "Enter/l", Label: "Save draft"}, {Key: "Space/p", Label: "Toggle protected"}}, actions...)
	}
	if model.profileForm.errorMessage != "" {
		lines = append(lines, "", model.profileForm.errorMessage)
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, ui.PrimaryPanelWidth(model.width, 1), true), "\n")...)
	}
	lines = wrapped
	return ui.Shell{Title: "Switchlet config", Subtitle: title, Panels: []ui.Panel{{Title: title, Lines: lines, Focused: true, FillHeight: model.height > 0}}, Actions: actions, Width: model.width, Height: model.height, Headerless: true}
}

// Keep navigation visible at the minimum terminal size; details use panel scroll.
func (model Model) importListLines(labels []string) []string {
	const visibleRows = 5
	cursor := model.profileForm.importForm.cursor
	start := max(0, cursor-visibleRows+1)
	end := min(len(labels), start+visibleRows)
	lines := []string{fmt.Sprintf("%d of %d", min(cursor+1, len(labels)), len(labels))}
	for index := start; index < end; index++ {
		prefix := "  "
		if index == cursor {
			prefix = "> "
		}
		lines = append(lines, ansi.Truncate(prefix+labels[index], ui.PrimaryPanelWidth(model.width, 1), "…"))
	}
	return lines
}
