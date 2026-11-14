package configeditor

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type profileDraftMode int

const (
	profileDraftAdd profileDraftMode = iota
	profileDraftUpdate
	profileDraftDuplicate
	profileDraftCapture
	profileDraftImport
)

type profileNameNextState int

const (
	profileNameNextIncludeValues profileNameNextState = iota
	profileNameNextReview
	profileNameNextImport
)

type profileDraftState struct {
	importForm    profileImportState
	mode          profileDraftMode
	originalName  string
	draft         app.ConfigEditProfileDraft
	includeCursor int
	valueIndex    int
	sourceCursor  int
	nameNext      profileNameNextState
	errorMessage  string
}

func (model *Model) beginAddProfile() {
	draft := model.workflow.NewProfileDraft(model.document)
	model.profileForm = profileDraftState{
		mode:     profileDraftAdd,
		draft:    draft,
		nameNext: profileNameNextIncludeValues,
	}
	model.state = editorStateProfileNameInput
	model.inputValue = ""
	model.inputCursor = 0
	model.saveError = ""
}

func (model *Model) beginCaptureProfile() tea.Cmd {
	requestID := model.nextRequestID()
	model.profileForm = profileDraftState{
		mode:     profileDraftCapture,
		nameNext: profileNameNextReview,
	}
	model.state = editorStateProfileCaptureLoading
	model.inputValue = ""
	model.inputCursor = 0
	model.saveError = ""

	return captureCurrentProfile(model.workflow, requestID, model.document)
}

func (model *Model) beginEditProfile(profileName string) {
	draft, err := model.workflow.ProfileDraft(model.document, profileName)
	if err != nil {
		model.saveError = err.Error()
		model.selectReviewOverview()
		return
	}

	model.profileForm = profileDraftState{
		mode:         profileDraftUpdate,
		originalName: profileName,
		draft:        draft,
	}
	model.state = editorStateProfileIncludeValues
	model.inputValue = ""
	model.inputCursor = 0
	model.saveError = ""
}

func (model *Model) beginDuplicateProfile(profileName string) {
	draft, err := model.workflow.ProfileDraft(model.document, profileName)
	if err != nil {
		model.saveError = err.Error()
		model.selectReviewOverview()
		return
	}

	draft.Name = ""
	model.profileForm = profileDraftState{
		mode:         profileDraftDuplicate,
		originalName: profileName,
		draft:        draft,
		nameNext:     profileNameNextReview,
	}
	model.state = editorStateProfileNameInput
	model.inputValue = ""
	model.inputCursor = 0
	model.saveError = ""
}

func (model *Model) beginRenameProfile(profileName string) {
	draft, err := model.workflow.ProfileDraft(model.document, profileName)
	if err != nil {
		model.saveError = err.Error()
		model.selectReviewOverview()
		return
	}

	model.profileForm = profileDraftState{
		mode:         profileDraftUpdate,
		originalName: profileName,
		draft:        draft,
		nameNext:     profileNameNextReview,
	}
	model.state = editorStateProfileNameInput
	model.inputValue = draft.Name
	model.inputCursor = len([]rune(model.inputValue))
	model.saveError = ""
}

func (model *Model) beginToggleProfileProtected(profileName string) {
	draft, err := model.workflow.ProfileDraft(model.document, profileName)
	if err != nil {
		model.saveError = err.Error()
		model.selectReviewOverview()
		return
	}

	draft.Protected = !draft.Protected
	model.profileForm = profileDraftState{
		mode:         profileDraftUpdate,
		originalName: profileName,
		draft:        draft,
	}
	model.state = editorStateProfileReview
	model.saveError = ""
}

func (model *Model) beginRemoveProfile(profileName string) {
	draft, err := model.workflow.ProfileDraft(model.document, profileName)
	if err != nil {
		model.saveError = err.Error()
		model.selectReviewOverview()
		return
	}

	model.profileForm = profileDraftState{
		mode:         profileDraftUpdate,
		originalName: profileName,
		draft:        draft,
	}
	model.state = editorStateProfileRemoveConfirm
	model.saveError = ""
}

func (model *Model) cancelProfileForm() {
	model.profileForm = profileDraftState{}
	model.inputValue = ""
	model.inputCursor = 0
	model.state = editorStateOverview
}

func (model Model) handleProfileCaptured(message profileCapturedMsg) (tea.Model, tea.Cmd) {
	if message.requestID != model.requestID || model.state != editorStateProfileCaptureLoading {
		return model, nil
	}
	if message.err != nil {
		model.profileForm.errorMessage = message.err.Error()
		return model, nil
	}

	model.profileForm.draft = message.draft
	model.profileForm.errorMessage = ""
	model.state = editorStateProfileNameInput
	model.inputValue = ""
	model.inputCursor = 0
	return model, nil
}

func (model *Model) applyProfileNameInput() {
	profileName := strings.TrimSpace(model.inputValue)
	if profileName == "" {
		model.profileForm.errorMessage = "Profile name must be set."
		return
	}

	if model.profileForm.mode == profileDraftDuplicate || model.profileForm.mode == profileDraftImport {
		draft, err := model.workflow.DuplicateProfileDraft(model.document, model.profileForm.originalName, profileName)
		if err != nil {
			model.profileForm.errorMessage = err.Error()
			return
		}
		model.profileForm.draft = draft
	} else {
		model.profileForm.draft.Name = profileName
	}
	model.profileForm.errorMessage = ""
	model.inputValue = ""
	model.inputCursor = 0
	if model.profileForm.nameNext == profileNameNextImport {
		model.state = editorStateProfileImportMapping
		return
	}
	if model.profileForm.nameNext == profileNameNextReview {
		model.state = editorStateProfileReview
		return
	}

	if len(model.profileForm.draft.Values) == 1 {
		model.profileForm.draft.Values[0].Included = true
		model.beginProfileValueSource(0)
		return
	}

	model.state = editorStateProfileIncludeValues
}

func (model *Model) beginProfileValueSource(valueIndex int) {
	if valueIndex < 0 || valueIndex >= len(model.profileForm.draft.Values) {
		model.state = editorStateProfileReview
		return
	}

	model.profileForm.valueIndex = valueIndex
	model.profileForm.sourceCursor = 0
	if model.profileForm.draft.Values[valueIndex].Source == app.ProfileSourceEnvironment {
		model.profileForm.sourceCursor = 1
	}
	model.profileForm.errorMessage = ""
	model.state = editorStateProfileValueSource
}

func (model *Model) beginProfileValueInput(valueIndex int) {
	if valueIndex < 0 || valueIndex >= len(model.profileForm.draft.Values) {
		model.state = editorStateProfileReview
		return
	}

	model.profileForm.valueIndex = valueIndex
	value := model.profileForm.draft.Values[valueIndex]
	if value.Source == app.ProfileSourceEnvironment {
		model.inputValue = value.EnvironmentVariableName
	} else {
		model.inputValue = value.LiteralValue
	}
	model.inputCursor = len([]rune(model.inputValue))
	model.profileForm.errorMessage = ""
	model.state = editorStateProfileValueInput
}

func (model *Model) applyProfileValueInput() {
	valueIndex := model.profileForm.valueIndex
	if valueIndex < 0 || valueIndex >= len(model.profileForm.draft.Values) {
		model.state = editorStateProfileReview
		return
	}
	if strings.TrimSpace(model.inputValue) == "" {
		model.profileForm.errorMessage = "Profile value must be set."
		return
	}

	if model.profileForm.draft.Values[valueIndex].Source == app.ProfileSourceEnvironment {
		model.profileForm.draft.Values[valueIndex].EnvironmentVariableName = strings.TrimSpace(model.inputValue)
	} else {
		value, err := app.NormalizeManagedValue(model.profileForm.draft.Values[valueIndex].ValueType, model.inputValue)
		if err != nil {
			model.profileForm.errorMessage = err.Error()
			return
		}
		model.profileForm.draft.Values[valueIndex].LiteralValue = value
	}
	model.profileForm.errorMessage = ""
	model.inputValue = ""
	model.inputCursor = 0

	if nextIndex, ok := model.nextIncompleteIncludedProfileValue(valueIndex + 1); ok {
		model.beginProfileValueSource(nextIndex)
		return
	}

	model.state = editorStateProfileReview
}

func (model Model) nextIncompleteIncludedProfileValue(startIndex int) (int, bool) {
	values := model.profileForm.draft.Values
	for offset := 0; offset < len(values); offset++ {
		index := startIndex + offset
		if index >= len(values) {
			index -= len(values)
		}
		if !values[index].Included {
			continue
		}
		if values[index].Source == app.ProfileSourceEnvironment {
			if strings.TrimSpace(values[index].EnvironmentVariableName) == "" {
				return index, true
			}
			continue
		}
		if strings.TrimSpace(values[index].LiteralValue) == "" {
			return index, true
		}
	}

	return 0, false
}

func (model Model) includedProfileValueCount() int {
	count := 0
	for _, value := range model.profileForm.draft.Values {
		if value.Included {
			count++
		}
	}

	return count
}

func (model *Model) clampProfileIncludeCursor() {
	if len(model.profileForm.draft.Values) == 0 {
		model.profileForm.includeCursor = 0
		return
	}
	if model.profileForm.includeCursor < 0 {
		model.profileForm.includeCursor = 0
	}
	if model.profileForm.includeCursor >= len(model.profileForm.draft.Values) {
		model.profileForm.includeCursor = len(model.profileForm.draft.Values) - 1
	}
}

func (model *Model) completeProfileDraft() {
	var (
		updatedDocument app.ConfigEditDocument
		err             error
	)
	if model.profileForm.mode == profileDraftAdd {
		updatedDocument, err = model.workflow.AddProfileDraft(model.document, model.profileForm.draft)
	} else if model.profileForm.mode == profileDraftUpdate {
		updatedDocument, err = model.workflow.UpdateProfileDraft(model.document, model.profileForm.originalName, model.profileForm.draft)
	} else {
		updatedDocument, err = model.workflow.AddProfileDraft(model.document, model.profileForm.draft)
	}
	if err != nil {
		model.profileForm.errorMessage = err.Error()
		return
	}

	model.document = updatedDocument
	profileName := model.profileForm.draft.Name
	model.profileForm = profileDraftState{}
	model.inputValue = ""
	model.inputCursor = 0
	model.state = editorStateOverview
	model.saveError = ""
	model.selectOverviewTab(overviewTabProfiles)
	if rowIndex := model.profileRowIndex(profileName); rowIndex >= 0 {
		model.cursor = rowIndex
		return
	}
	model.selectReviewOverview()
}

func (model *Model) removeProfileDraft() {
	updatedDocument, err := model.workflow.RemoveProfile(model.document, model.profileForm.originalName)
	if err != nil {
		model.profileForm.errorMessage = err.Error()
		return
	}

	model.document = updatedDocument
	model.profileForm = profileDraftState{}
	model.inputValue = ""
	model.inputCursor = 0
	model.state = editorStateOverview
	model.saveError = ""
	model.selectReviewOverview()
}

func (model Model) profileRowIndex(profileName string) int {
	rows := model.navigationRows(model.overview())
	for index, row := range rows {
		if row.Kind == navigationRowProfile && row.Label == profileName {
			return index
		}
	}

	return -1
}
