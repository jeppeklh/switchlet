package configeditor

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type profileImportState struct {
	mappings []app.ConfigEditImportMapping
	prepared app.ConfigEditProfileImport
	selected map[string]bool
	cursor   int
	// A fresh, non-zero-sized token also distinguishes requests across editor sessions.
	request *byte
}

type profileImportedMsg struct {
	request  *byte
	prepared app.ConfigEditProfileImport
	err      error
}

func (model *Model) beginImportProfile(profileName string) {
	model.beginDuplicateProfile(profileName)
	if model.state != editorStateProfileNameInput {
		return
	}
	model.profileForm.mode = profileDraftImport
	model.profileForm.nameNext = profileNameNextImport
	model.profileForm.importForm = profileImportState{mappings: model.workflow.ProfileImportMappings(model.document)}
	model.resetScrollOffset()
}

func (model *Model) discardProfileImport() {
	form := &model.profileForm.importForm
	form.prepared = app.ConfigEditProfileImport{}
	form.selected = nil
	form.request = nil
	form.cursor = 0
	model.profileForm.errorMessage = ""
	model.resetScrollOffset()
}

func (model *Model) readProfileImport() tea.Cmd {
	model.discardProfileImport()
	form := &model.profileForm.importForm
	mapped := false
	for _, mapping := range form.mappings {
		mapped = mapped || strings.TrimSpace(mapping.SourceFile) != ""
	}
	if !mapped {
		model.state = editorStateProfileImportMapping
		model.profileForm.errorMessage = "Map at least one source file."
		return nil
	}
	form.request = new(byte)
	request := form.request
	mappings := slices.Clone(form.mappings)
	workflow, document, baseName := model.workflow, model.document, model.profileForm.originalName
	model.state = editorStateProfileImportLoading
	return func() tea.Msg {
		prepared, err := workflow.PrepareProfileImport(document, baseName, mappings)
		return profileImportedMsg{request: request, prepared: prepared, err: err}
	}
}

func (model Model) handleProfileImported(message profileImportedMsg) (tea.Model, tea.Cmd) {
	if model.result != nil || model.state != editorStateProfileImportLoading || message.request == nil || message.request != model.profileForm.importForm.request {
		return model, nil
	}
	model.profileForm.importForm.request = nil
	if message.err != nil {
		model.profileForm.errorMessage = message.err.Error()
		model.state = editorStateProfileImportMapping
		return model, nil
	}
	model.profileForm.importForm.prepared = message.prepared
	model.profileForm.importForm.selected = make(map[string]bool)
	model.state = editorStateProfileImportSelection
	return model, nil
}

func (model Model) handleProfileImportKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := &model.profileForm.importForm
	if model.state == editorStateProfileImportSourceInput {
		switch message.Type {
		case tea.KeyEsc:
			model.state = editorStateProfileImportMapping
		case tea.KeyEnter:
			form.mappings[form.cursor].SourceFile = strings.TrimSpace(model.inputValue)
			model.state = editorStateProfileImportMapping
		default:
			model.handleInputEditKey(message)
			return model, nil
		}
		model.inputValue, model.inputCursor = "", 0
		return model, nil
	}
	if isQuitKey(message) {
		return model.beginQuit()
	}
	if model.state == editorStateProfileImportLoading {
		if isBackKey(message) {
			model.discardProfileImport()
			model.state = editorStateProfileImportMapping
		}
		return model, nil
	}
	if updated, handled := model.handleScrollablePanelKey(message); handled {
		return updated, nil
	}
	count := len(form.mappings)
	if model.state == editorStateProfileImportSelection {
		count = len(form.prepared.Values)
	}
	switch {
	case isMoveUpKey(message):
		if count > 0 {
			form.cursor = (form.cursor + count - 1) % count
		}
	case isMoveDownKey(message):
		if count > 0 {
			form.cursor = (form.cursor + 1) % count
		}
	case isRuneKey(message, 'g'):
		form.cursor = 0
	case isRuneKey(message, 'G'):
		if count > 0 {
			form.cursor = count - 1
		}
	case isBackKey(message):
		if model.state == editorStateProfileImportMapping {
			model.cancelProfileForm()
		} else {
			model.discardProfileImport()
			model.state = editorStateProfileImportMapping
		}
	case model.state == editorStateProfileImportMapping:
		switch {
		case isRuneKey(message, 'e') && count > 0:
			model.inputValue = form.mappings[form.cursor].SourceFile
			model.inputCursor = len([]rune(model.inputValue))
			model.state = editorStateProfileImportSourceInput
		case isRuneKey(message, 'x') && count > 0:
			form.mappings[form.cursor].SourceFile = ""
		case isOpenKey(message):
			return model, model.readProfileImport()
		}
	case model.state == editorStateProfileImportSelection:
		switch {
		case message.Type == tea.KeySpace && count > 0:
			row := form.prepared.Values[form.cursor]
			if row.Available {
				form.selected[row.TargetName] = !form.selected[row.TargetName]
			}
		case isRuneKey(message, 'r'):
			return model, model.readProfileImport()
		case isRuneKey(message, 'm'):
			model.discardProfileImport()
			model.state = editorStateProfileImportMapping
		case isOpenKey(message):
			var selected []string
			for _, row := range form.prepared.Values {
				if form.selected[row.TargetName] {
					selected = append(selected, row.TargetName)
				}
			}
			draft, err := model.workflow.ProfileDraftFromImport(model.document, form.prepared, model.profileForm.draft.Name, selected)
			if err != nil {
				model.profileForm.errorMessage = err.Error()
				return model, nil
			}
			draft.Protected = model.profileForm.draft.Protected
			model.profileForm.draft = draft
			model.profileForm.errorMessage = ""
			model.state = editorStateProfileReview
		}
	}
	model.resetScrollOffset()
	return model, nil
}
