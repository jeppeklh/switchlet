package configeditor

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

const (
	configEditorMinimumTerminalWidth  = 80
	configEditorMinimumTerminalHeight = 24
)

type editorState int

const (
	editorStateOverview editorState = iota
	editorStateFilter
	editorStateProfileNameInput
	editorStateProfileIncludeValues
	editorStateProfileValueSource
	editorStateProfileValueInput
	editorStateProfileReview
	editorStateProfileRemoveConfirm
	editorStateManagedValueFileLoading
	editorStateManagedValueFileSelect
	editorStateManagedValueFileFilter
	editorStateManagedValueManualFileInput
	editorStateManagedValueTypeSelect
	editorStateManagedValueSelectorLoading
	editorStateManagedValueSelectorSelect
	editorStateManagedValueSelectorFilter
	editorStateManagedValueManualSelectorInput
	editorStateManagedValueSelectorValidating
	editorStateManagedValueNameInput
	editorStateManagedValueReview
	editorStateManagedValueRemoveConfirm
	editorStateDirtyQuitConfirm
	editorStateSaving
	editorStateSaveSuccess
)

type navigationRowKind int

const (
	navigationRowProfilesSection navigationRowKind = iota
	navigationRowProfile
	navigationRowManagedValuesSection
	navigationRowManagedValue
	navigationRowReview
)

type overviewTab int

const (
	overviewTabProfiles overviewTab = iota
	overviewTabTargets
	overviewTabReview
)

type navigationRow struct {
	Kind              navigationRowKind
	Label             string
	ProfileIndex      int
	ManagedValueIndex int
}

// Options configures the config editor model.
type Options struct {
	Embedded bool
}

// Result describes how the config editor exited.
type Result struct {
	Saved      bool
	Cancelled  bool
	Quit       bool
	ConfigPath string
	Changes    []app.ConfigEditChange
}

// Model is the Bubble Tea model for the interactive config editor.
type Model struct {
	workflow       app.ConfigEditWorkflow
	targetWorkflow app.InitWorkflow
	document       app.ConfigEditDocument

	state        editorState
	width        int
	height       int
	cursor       int
	filter       string
	inputValue   string
	inputCursor  int
	scrollOffset int
	saveError    string
	activeTab    overviewTab
	embedded     bool
	profileForm  profileDraftState
	managedForm  managedValueDraftState
	requestID    int

	savedConfigPath string
	savedChanges    []app.ConfigEditChange
	result          *Result
	returnToPicker  bool
	helpOpen        bool
}

// NewModel creates the Bubble Tea model for the interactive config editor.
func NewModel(document app.ConfigEditDocument, workflow app.ConfigEditWorkflow) Model {
	return NewModelWithOptions(document, workflow, Options{})
}

// NewModelWithOptions creates the config editor model with explicit options.
func NewModelWithOptions(document app.ConfigEditDocument, workflow app.ConfigEditWorkflow, options Options) Model {
	return Model{
		workflow:       workflow,
		targetWorkflow: app.DefaultInitWorkflow(),
		document:       document,
		state:          editorStateOverview,
		embedded:       options.Embedded,
	}
}

func (model Model) Init() tea.Cmd {
	return nil
}

// Result returns the editor result after the model reaches a terminal state.
func (model Model) Result() (Result, bool) {
	if model.result == nil {
		return Result{}, false
	}

	return *model.result, true
}

func (model Model) overview() app.ConfigEditOverview {
	return model.workflow.BuildConfigEditOverview(model.document)
}

func (model Model) isTerminalTooSmall() bool {
	if model.width == 0 || model.height == 0 {
		return false
	}

	return model.width < configEditorMinimumTerminalWidth || model.height < configEditorMinimumTerminalHeight
}

func (model *Model) closeWithoutSaving() {
	model.result = &Result{}
}

func (model *Model) quitWithoutSaving() {
	model.result = &Result{Quit: true}
}

func (model *Model) cancel() {
	model.result = &Result{Cancelled: true, Quit: true}
}

func (model *Model) completeSaved() {
	model.result = &Result{
		Saved:      true,
		ConfigPath: model.savedConfigPath,
		Changes:    append([]app.ConfigEditChange(nil), model.savedChanges...),
	}
}

func (model *Model) beginFilter() {
	model.state = editorStateFilter
	model.inputValue = model.filter
	model.inputCursor = len([]rune(model.inputValue))
	model.saveError = ""
	model.resetScrollOffset()
}

func (model *Model) applyFilter() {
	model.filter = strings.TrimSpace(model.inputValue)
	model.inputValue = ""
	model.inputCursor = 0
	model.cursor = 0
	model.state = editorStateOverview
	model.resetScrollOffset()
}

func (model *Model) cancelFilter() {
	model.inputValue = ""
	model.inputCursor = 0
	model.state = editorStateOverview
	model.resetScrollOffset()
}

func (model *Model) clampCursor(rowCount int) {
	if rowCount <= 0 {
		model.cursor = 0
		return
	}
	if model.cursor < 0 {
		model.cursor = 0
	}
	if model.cursor >= rowCount {
		model.cursor = rowCount - 1
	}
}

func (model Model) activeFilter() string {
	if model.state == editorStateFilter {
		return strings.TrimSpace(model.inputValue)
	}

	return model.filter
}

func (model *Model) nextRequestID() int {
	model.requestID++
	return model.requestID
}

func (model Model) navigationRows(overview app.ConfigEditOverview) []navigationRow {
	filter := normalizeOverviewFilter(model.activeFilter())
	rows := make([]navigationRow, 0)

	switch model.activeTab {
	case overviewTabTargets:
		for index, managedValue := range overview.ManagedValues {
			if filter != "" && !managedValueMatchesFilter(managedValue, filter) {
				continue
			}
			rows = append(rows, navigationRow{Kind: navigationRowManagedValue, Label: managedValue.TargetName, ManagedValueIndex: index})
		}
	case overviewTabReview:
	default:
		for index, profile := range overview.Profiles {
			if filter != "" && !profileMatchesOverviewFilter(profile, filter) {
				continue
			}
			rows = append(rows, navigationRow{Kind: navigationRowProfile, Label: profile.Name, ProfileIndex: index})
		}
	}

	return rows
}

func normalizeOverviewFilter(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
}

func profileMatchesOverviewFilter(profile app.ConfigEditProfileItem, filter string) bool {
	if strings.Contains(normalizeOverviewFilter(profile.Name), filter) {
		return true
	}

	fields := make([]string, 0, len(profile.Values)*6)
	for _, value := range profile.Values {
		for _, field := range []string{
			value.TargetName,
			value.TargetFile,
			string(value.TargetType),
			value.SelectorName,
			value.Selector,
			value.EnvironmentVariableName,
		} {
			if field != "" {
				fields = append(fields, normalizeOverviewFilter(field))
			}
		}
	}

	return overviewFilterTermsMatch(fields, filter)
}

func (model *Model) selectOverviewTab(tab overviewTab) {
	model.activeTab = tab
	model.cursor = 0
	model.resetScrollOffset()
}

func (model *Model) selectPreviousOverviewTab() {
	switch model.activeTab {
	case overviewTabTargets:
		model.selectOverviewTab(overviewTabProfiles)
	case overviewTabReview:
		model.selectOverviewTab(overviewTabTargets)
	default:
		model.selectOverviewTab(overviewTabReview)
	}
}

func (model *Model) selectNextOverviewTab() {
	switch model.activeTab {
	case overviewTabProfiles:
		model.selectOverviewTab(overviewTabTargets)
	case overviewTabTargets:
		model.selectOverviewTab(overviewTabReview)
	default:
		model.selectOverviewTab(overviewTabProfiles)
	}
}

func managedValueMatchesFilter(managedValue app.ConfigEditManagedValueItem, filter string) bool {
	fields := []string{managedValue.TargetName, managedValue.TargetFile, string(managedValue.TargetType), managedValue.SelectorName, managedValue.Selector}
	for _, value := range fields {
		if strings.Contains(normalizeOverviewFilter(value), filter) {
			return true
		}
	}

	normalizedFields := make([]string, 0, len(fields))
	for _, field := range fields {
		if field != "" {
			normalizedFields = append(normalizedFields, normalizeOverviewFilter(field))
		}
	}

	return overviewFilterTermsMatch(normalizedFields, filter)
}

func overviewFilterTermsMatch(fields []string, filter string) bool {
	terms := strings.FieldsFunc(filter, func(value rune) bool {
		return unicode.IsSpace(value) || value == '/' || value == '\\'
	})
	if len(terms) == 0 {
		return false
	}

	for _, term := range terms {
		matchedTerm := false
		for _, field := range fields {
			if strings.Contains(field, term) {
				matchedTerm = true
				break
			}
		}
		if !matchedTerm {
			return false
		}
	}

	return true
}
