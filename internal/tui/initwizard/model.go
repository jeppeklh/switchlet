package initwizard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	ui "github.com/jeppeklh/switchlet/internal/tui"
)

const (
	initWizardMinimumTerminalWidth  = 80
	initWizardMinimumTerminalHeight = 24
)

type initWizardStep int

const (
	initWizardStepFileSelect initWizardStep = iota
	initWizardStepFileFilter
	initWizardStepManualFile
	initWizardStepTypeSelect
	initWizardStepPathBrowse
	initWizardStepPathSearch
	initWizardStepManualPath
	initWizardStepDotenvKeySelect
	initWizardStepManualDotenvKey
	initWizardStepManagedValueName
	initWizardStepManagedValueCheckpoint
	initWizardStepProfileName
	initWizardStepProfileTargetInclude
	initWizardStepProfileValueSource
	initWizardStepProfileValue
	initWizardStepProfileSummary
	initWizardStepReview
)

type initWizardProfileDraft struct {
	Name           string
	Values         []app.InitProfileValue
	TargetIndex    int
	UseEnvironment bool
	Value          string
	Protected      bool
}

type initWizardEffectKind int

const (
	initWizardEffectFileInspection initWizardEffectKind = iota + 1
	initWizardEffectJSONSelectorValidation
	initWizardEffectYAMLSelectorValidation
	initWizardEffectTOMLSelectorValidation
	initWizardEffectDotenvKeyValidation
)

type initWizardPendingEffect struct {
	RequestID         int
	Kind              initWizardEffectKind
	StepNumber        int
	Title             string
	Message           string
	ReturnStep        initWizardStep
	ReturnCursor      int
	ReturnInputValue  string
	ReturnInputCursor int
	TargetPath        string
	DisplayPath       string
	TargetType        app.InitTargetType
	Selector          string
	FileFilter        string
}

// Options configures init wizard presentation for command-level init decisions.
type Options struct {
	OverwriteExistingConfig bool
}

// Result describes the completed interactive init wizard outcome.
type Result struct {
	Targets            []app.InitTarget
	Profiles           []app.InitProfile
	ShouldIgnoreConfig bool
	Cancelled          bool
}

type initWizardModel struct {
	workingDirectory        string
	workflow                app.InitWorkflow
	step                    initWizardStep
	width                   int
	height                  int
	cursor                  int
	errorDetail             ui.RecoverableError
	inputValue              string
	inputCursor             int
	fileFilter              string
	fileCandidates          []app.InitTargetFileCandidate
	selectedFile            app.InitTargetFileSelection
	browseNodes             []targetSelectorNode
	browseAncestors         []targetBrowseLevel
	selectedJSONPath        string
	selectedYAMLPath        string
	selectedTOMLPath        string
	selectedDotenvKey       string
	targets                 []app.InitTarget
	profiles                []app.InitProfile
	draftProfile            initWizardProfileDraft
	shouldIgnoreConfig      bool
	shouldIgnoreConfigSet   bool
	overwriteExistingConfig bool
	effectRequestID         int
	pendingEffect           *initWizardPendingEffect
	result                  *Result
}

// NewModel creates the Bubble Tea model for the interactive init wizard.
func NewModel(workingDirectory string, workflow app.InitWorkflow) (tea.Model, error) {
	return NewModelWithOptions(workingDirectory, workflow, Options{})
}

// NewModelWithOptions creates the Bubble Tea model for init wizard command options.
func NewModelWithOptions(workingDirectory string, workflow app.InitWorkflow, options Options) (tea.Model, error) {
	return newInitWizardModelWithOptions(workingDirectory, workflow, options)
}

func newInitWizardModel(workingDirectory string, workflow app.InitWorkflow) (initWizardModel, error) {
	return newInitWizardModelWithOptions(workingDirectory, workflow, Options{})
}

func newInitWizardModelWithOptions(workingDirectory string, workflow app.InitWorkflow, options Options) (initWizardModel, error) {
	fileCandidates, err := workflow.DiscoverTargetFileCandidates(workingDirectory)
	if err != nil {
		return initWizardModel{}, err
	}

	return initWizardModel{
		workingDirectory:        workingDirectory,
		workflow:                workflow,
		step:                    initWizardStepFileSelect,
		fileCandidates:          fileCandidates,
		overwriteExistingConfig: options.OverwriteExistingConfig,
	}, nil
}

func (model initWizardModel) Init() tea.Cmd {
	return nil
}

// Result returns the completed wizard result, if the model reached a terminal state.
func (model initWizardModel) Result() (Result, bool) {
	if model.result == nil {
		return Result{}, false
	}

	return *model.result, true
}

func (model initWizardModel) isTerminalTooSmall() bool {
	if model.width == 0 || model.height == 0 {
		return false
	}

	return model.width < initWizardMinimumTerminalWidth || model.height < initWizardMinimumTerminalHeight
}

func (model *initWizardModel) cancel() {
	model.pendingEffect = nil
	model.result = &Result{Cancelled: true}
}

func (model *initWizardModel) complete() {
	targets := make([]app.InitTarget, len(model.targets))
	copy(targets, model.targets)

	profiles := make([]app.InitProfile, len(model.profiles))
	copy(profiles, model.profiles)

	model.result = &Result{
		Targets:            targets,
		Profiles:           profiles,
		ShouldIgnoreConfig: model.shouldIgnoreConfig,
	}
}

func (model *initWizardModel) beginPathBrowse() {
	model.step = initWizardStepPathBrowse
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
	model.browseNodes = targetSelectorNodesForSelection(model.selectedFile)
	model.browseAncestors = nil
	model.selectedJSONPath = ""
	model.selectedYAMLPath = ""
	model.selectedTOMLPath = ""
	model.selectedDotenvKey = ""
}

func (model *initWizardModel) beginDotenvKeySelect() {
	model.step = initWizardStepDotenvKeySelect
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
	model.selectedJSONPath = ""
	model.selectedYAMLPath = ""
	model.selectedTOMLPath = ""
	model.selectedDotenvKey = ""
}

func (model *initWizardModel) beginManagedValueName() {
	model.step = initWizardStepManagedValueName
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
}

func (model *initWizardModel) beginManagedValueCheckpoint() {
	model.step = initWizardStepManagedValueCheckpoint
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
}

func (model *initWizardModel) beginProfileEntry() {
	model.step = initWizardStepProfileName
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
	model.draftProfile = initWizardProfileDraft{Values: make([]app.InitProfileValue, 0, len(model.targets))}
}

func (model *initWizardModel) beginReview() {
	model.syncIgnorePreference()
	model.step = initWizardStepReview
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
}

func (model *initWizardModel) returnToProfilesFromReview() {
	if len(model.profiles) == 0 {
		model.beginProfileEntry()
		return
	}

	model.beginProfileAdded()
}

func (model *initWizardModel) beginProfileValue() {
	model.step = initWizardStepProfileValue
	model.cursor = 0
	model.clearError()
	model.setInputValue(model.draftProfile.Value)
}

func (model *initWizardModel) beginProfileValueSource() {
	model.step = initWizardStepProfileValueSource
	if model.draftProfile.UseEnvironment {
		model.cursor = 1
	} else {
		model.cursor = 0
	}
	model.clearError()
}

func (model *initWizardModel) beginProfileAdded() {
	model.step = initWizardStepProfileSummary
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
}

func (model *initWizardModel) syncIgnorePreference() {
	if !app.InitProfilesHaveLiteralValues(model.profiles) {
		model.shouldIgnoreConfig = false
		model.shouldIgnoreConfigSet = false
		return
	}

	if !model.shouldIgnoreConfigSet {
		model.shouldIgnoreConfig = true
		model.shouldIgnoreConfigSet = true
	}
}

func (model *initWizardModel) appendTarget(name string) {
	target := app.InitTarget{
		Name: name,
		File: model.selectedFile.Path,
		Type: model.selectedFile.TargetType,
	}
	switch model.selectedFile.TargetType {
	case app.InitTargetTypeDotenv:
		target.Key = model.selectedDotenvKey
	case app.InitTargetTypeYAML:
		target.YAMLPath = model.selectedYAMLPath
	case app.InitTargetTypeTOML:
		target.TOMLPath = model.selectedTOMLPath
	default:
		target.JSONPath = model.selectedJSONPath
	}

	model.targets = append(model.targets, target)
	model.beginManagedValueCheckpoint()
}

func (model *initWizardModel) removeLastTarget() {
	if len(model.targets) == 0 {
		return
	}

	removedTarget := model.targets[len(model.targets)-1]
	model.targets = model.targets[:len(model.targets)-1]
	model.removeProfileValuesForTarget(removedTarget.Name)
	if len(model.targets) == 0 {
		model.step = initWizardStepFileSelect
		model.cursor = 0
		model.clearError()
		return
	}

	model.beginManagedValueCheckpoint()
}

func (model *initWizardModel) removeProfileValuesForTarget(targetName string) {
	profiles := make([]app.InitProfile, 0, len(model.profiles))
	for _, profile := range model.profiles {
		values := make([]app.InitProfileValue, 0, len(profile.Values))
		for _, value := range profile.Values {
			if value.Target != targetName {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			continue
		}

		profile.Values = values
		profiles = append(profiles, profile)
	}

	model.profiles = profiles
	model.syncIgnorePreference()
}

func (model *initWizardModel) appendDraftProfile() {
	profile := app.InitProfile{
		Name:      model.draftProfile.Name,
		Values:    append([]app.InitProfileValue(nil), model.draftProfile.Values...),
		Protected: model.draftProfile.Protected,
	}

	model.profiles = append(model.profiles, profile)
	model.draftProfile = initWizardProfileDraft{}
	model.syncIgnorePreference()
	model.beginProfileAdded()
}

func (model *initWizardModel) removeLastProfile() {
	if len(model.profiles) == 0 {
		return
	}

	model.profiles = model.profiles[:len(model.profiles)-1]
	model.syncIgnorePreference()
	if len(model.profiles) == 0 {
		model.beginProfileEntry()
		return
	}

	model.step = initWizardStepProfileSummary
	model.cursor = 0
	model.clearError()
	model.inputValue = ""
}

func (model initWizardModel) filteredFileCandidates(filterValue string) []app.InitTargetFileCandidate {
	return filterTargetFileCandidates(model.fileCandidates, filterValue)
}

func (model initWizardModel) selectableStructuredPaths() []string {
	return flattenSelectableTargetPaths(targetSelectorNodesForSelection(model.selectedFile))
}

func (model initWizardModel) filteredSelectableStructuredPaths(filterValue string) []string {
	return filterSelectableTargetPaths(model.selectableStructuredPaths(), filterValue)
}

func (model initWizardModel) filteredDotenvKeys(filterValue string) []string {
	return filterDotenvKeys(model.selectedFile.DotenvKeys, filterValue)
}

func (model *initWizardModel) clampCursor(total int) {
	if total <= 0 {
		model.cursor = 0
		return
	}
	if model.cursor < 0 {
		model.cursor = 0
	}
	if model.cursor >= total {
		model.cursor = total - 1
	}
}

func (model initWizardModel) profileNameExists(name string) bool {
	for _, profile := range model.profiles {
		if profile.Name == name {
			return true
		}
	}

	return false
}

func (model initWizardModel) targetNameExists(name string) bool {
	for _, target := range model.targets {
		if target.Name == name {
			return true
		}
	}

	return false
}

func (model initWizardModel) currentDraftTarget() app.InitTarget {
	if model.draftProfile.TargetIndex < 0 || model.draftProfile.TargetIndex >= len(model.targets) {
		return app.InitTarget{}
	}

	return model.targets[model.draftProfile.TargetIndex]
}

func (model *initWizardModel) beginProfileTargetInclude() {
	model.step = initWizardStepProfileTargetInclude
	model.cursor = 0
	model.clearError()
	model.clearInputValue()
}

func (model *initWizardModel) appendDraftProfileValue() {
	target := model.currentDraftTarget()
	value := app.InitProfileValue{Target: target.Name}
	if model.draftProfile.UseEnvironment {
		value.ValueFromEnv = stringValuePointer(model.draftProfile.Value)
	} else {
		value.Value = stringValuePointer(model.draftProfile.Value)
	}

	model.draftProfile.Values = append(model.draftProfile.Values, value)
	model.advanceDraftProfileTarget()
}

func (model *initWizardModel) advanceDraftProfileTarget() {
	model.draftProfile.TargetIndex++
	model.draftProfile.UseEnvironment = false
	model.draftProfile.Value = ""
	model.cursor = 0
	model.clearError()
	model.clearInputValue()

	if model.draftProfile.TargetIndex >= len(model.targets) {
		if len(model.draftProfile.Values) == 0 {
			model.draftProfile.TargetIndex = 0
			model.beginProfileTargetInclude()
			model.setStepError("Profile scope is empty.", "A profile must include at least one managed value.", "Set at least one managed value before continuing.")
			return
		}

		model.appendDraftProfile()
		return
	}

	model.beginProfileTargetInclude()
}

func (model *initWizardModel) trimDraftProfileValuesFromTargetIndex(targetIndex int) {
	if targetIndex < 0 {
		targetIndex = 0
	}
	if targetIndex >= len(model.targets) || len(model.draftProfile.Values) == 0 {
		return
	}

	revisitedTargets := make(map[string]struct{}, len(model.targets)-targetIndex)
	for _, target := range model.targets[targetIndex:] {
		revisitedTargets[target.Name] = struct{}{}
	}

	values := model.draftProfile.Values[:0]
	for _, value := range model.draftProfile.Values {
		if _, revisited := revisitedTargets[value.Target]; revisited {
			continue
		}

		values = append(values, value)
	}

	model.draftProfile.Values = values
}

func windowRange(cursor int, total int, windowSize int) (int, int) {
	if total <= windowSize {
		return 0, total
	}

	if cursor < windowSize {
		return 0, windowSize
	}

	start := cursor - windowSize + 1
	if start+windowSize > total {
		start = total - windowSize
	}

	return start, start + windowSize
}

func isTextEntryStep(step initWizardStep) bool {
	switch step {
	case initWizardStepFileFilter,
		initWizardStepManualFile,
		initWizardStepPathSearch,
		initWizardStepManualPath,
		initWizardStepManualDotenvKey,
		initWizardStepManagedValueName,
		initWizardStepProfileName,
		initWizardStepProfileValue:
		return true
	default:
		return false
	}
}

func (model *initWizardModel) setInputValue(value string) {
	model.inputValue = value
	model.inputCursor = len([]rune(value))
}

func (model *initWizardModel) clearInputValue() {
	model.inputValue = ""
	model.inputCursor = 0
}

func (model *initWizardModel) clampInputCursor() {
	inputLength := len([]rune(model.inputValue))
	if model.inputCursor < 0 {
		model.inputCursor = 0
	}
	if model.inputCursor > inputLength {
		model.inputCursor = inputLength
	}
}

func (model *initWizardModel) moveInputCursorLeft() {
	model.clampInputCursor()
	if model.inputCursor > 0 {
		model.inputCursor--
	}
}

func (model *initWizardModel) moveInputCursorRight() {
	model.clampInputCursor()
	if model.inputCursor < len([]rune(model.inputValue)) {
		model.inputCursor++
	}
}

func (model *initWizardModel) moveInputCursorToStart() {
	model.inputCursor = 0
}

func (model *initWizardModel) moveInputCursorToEnd() {
	model.inputCursor = len([]rune(model.inputValue))
}

func (model *initWizardModel) insertInputValue(value string) {
	model.clampInputCursor()
	currentRunes := []rune(model.inputValue)
	insertedRunes := []rune(value)

	updatedRunes := make([]rune, 0, len(currentRunes)+len(insertedRunes))
	updatedRunes = append(updatedRunes, currentRunes[:model.inputCursor]...)
	updatedRunes = append(updatedRunes, insertedRunes...)
	updatedRunes = append(updatedRunes, currentRunes[model.inputCursor:]...)

	model.inputValue = string(updatedRunes)
	model.inputCursor += len(insertedRunes)
}

func (model *initWizardModel) deleteInputRuneBeforeCursor() {
	model.clampInputCursor()
	if model.inputCursor == 0 {
		return
	}

	currentRunes := []rune(model.inputValue)
	updatedRunes := make([]rune, 0, len(currentRunes)-1)
	updatedRunes = append(updatedRunes, currentRunes[:model.inputCursor-1]...)
	updatedRunes = append(updatedRunes, currentRunes[model.inputCursor:]...)

	model.inputValue = string(updatedRunes)
	model.inputCursor--
}

func (model *initWizardModel) deleteInputRuneAtCursor() {
	model.clampInputCursor()
	currentRunes := []rune(model.inputValue)
	if model.inputCursor >= len(currentRunes) {
		return
	}

	updatedRunes := make([]rune, 0, len(currentRunes)-1)
	updatedRunes = append(updatedRunes, currentRunes[:model.inputCursor]...)
	updatedRunes = append(updatedRunes, currentRunes[model.inputCursor+1:]...)

	model.inputValue = string(updatedRunes)
}

func stringValuePointer(value string) *string {
	return &value
}
