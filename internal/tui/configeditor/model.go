package configeditor

import (
	"strings"

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

	state       editorState
	width       int
	height      int
	cursor      int
	filter      string
	inputValue  string
	inputCursor int
	saveError   string
	activeTab   overviewTab
	embedded    bool
	profileForm profileDraftState
	managedForm managedValueDraftState
	requestID   int

	savedConfigPath string
	savedChanges    []app.ConfigEditChange
	result          *Result
	returnToPicker  bool
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
}

func (model *Model) applyFilter() {
	model.filter = strings.TrimSpace(model.inputValue)
	model.inputValue = ""
	model.inputCursor = 0
	model.cursor = 0
	model.state = editorStateOverview
}

func (model *Model) cancelFilter() {
	model.inputValue = ""
	model.inputCursor = 0
	model.state = editorStateOverview
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
	filter := strings.ToLower(model.activeFilter())
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
			if filter != "" && !strings.Contains(strings.ToLower(profile.Name), filter) {
				continue
			}
			rows = append(rows, navigationRow{Kind: navigationRowProfile, Label: profile.Name, ProfileIndex: index})
		}
	}

	return rows
}

func (model *Model) selectOverviewTab(tab overviewTab) {
	model.activeTab = tab
	model.cursor = 0
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
	for _, value := range []string{managedValue.TargetName, managedValue.TargetFile, string(managedValue.TargetType), managedValue.Selector} {
		if strings.Contains(strings.ToLower(value), filter) {
			return true
		}
	}

	return false
}
