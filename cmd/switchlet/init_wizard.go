package main

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/editor"
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
	initWizardStepTargetName
	initWizardStepTargetSummary
	initWizardStepProfileName
	initWizardStepProfileTargetInclude
	initWizardStepProfileSource
	initWizardStepProfileValue
	initWizardStepProfileProtected
	initWizardStepProfileSummary
	initWizardStepReview
)

type initWizardProfileDraft struct {
	Name           string
	Values         []config.ProfileValue
	TargetIndex    int
	UseEnvironment bool
	Value          string
	Protected      bool
}

type initWizardResult struct {
	Targets            []config.Target
	Profiles           []config.Profile
	ShouldIgnoreConfig bool
	Cancelled          bool
}

type initWizardModel struct {
	workingDirectory      string
	dependencies          initDependencies
	step                  initWizardStep
	width                 int
	height                int
	cursor                int
	errorMessage          string
	inputValue            string
	inputCursor           int
	fileFilter            string
	fileCandidates        []editor.TargetFileCandidate
	selectedFile          targetFileSelection
	browseNodes           []editor.StringTargetNode
	browseAncestors       []targetBrowseLevel
	selectedJSONPath      string
	selectedDotenvKey     string
	targets               []config.Target
	profiles              []config.Profile
	draftProfile          initWizardProfileDraft
	shouldIgnoreConfig    bool
	shouldIgnoreConfigSet bool
	result                *initWizardResult
}

func newInitWizardModel(workingDirectory string, dependencies initDependencies) (initWizardModel, error) {
	fileCandidates, err := dependencies.discoverTargetFileCandidates(workingDirectory)
	if err != nil {
		return initWizardModel{}, err
	}

	return initWizardModel{
		workingDirectory: workingDirectory,
		dependencies:     dependencies,
		step:             initWizardStepFileSelect,
		fileCandidates:   fileCandidates,
	}, nil
}

func runInitWizard(workingDirectory string, input io.Reader, output io.Writer, dependencies initDependencies) (initWizardResult, error) {
	model, err := newInitWizardModel(workingDirectory, dependencies)
	if err != nil {
		return initWizardResult{}, err
	}

	finalModel, err := runFullScreenTerminalProgram(model, tea.WithInput(input), tea.WithOutput(output))
	if err != nil {
		return initWizardResult{}, fmt.Errorf("run init wizard: %w", err)
	}

	completedModel, ok := finalModel.(initWizardModel)
	if !ok {
		return initWizardResult{}, fmt.Errorf("run init wizard: unexpected final model type %T", finalModel)
	}
	if completedModel.result == nil {
		return initWizardResult{}, fmt.Errorf("run init wizard: finished without a result")
	}

	return *completedModel.result, nil
}

func shouldUseInitWizard(input io.Reader, output io.Writer) bool {
	inputFile, ok := input.(*os.File)
	if !ok {
		return false
	}

	outputFile, ok := output.(*os.File)
	if !ok {
		return false
	}

	return isCharacterDevice(inputFile) && isCharacterDevice(outputFile)
}

func isCharacterDevice(file *os.File) bool {
	fileInfo, err := file.Stat()
	if err != nil {
		return false
	}

	return fileInfo.Mode()&os.ModeCharDevice != 0
}

func (model initWizardModel) Init() tea.Cmd {
	return nil
}

func (model initWizardModel) isTerminalTooSmall() bool {
	if model.width == 0 || model.height == 0 {
		return false
	}

	return model.width < initWizardMinimumTerminalWidth || model.height < initWizardMinimumTerminalHeight
}

func (model *initWizardModel) cancel() {
	model.result = &initWizardResult{Cancelled: true}
}

func (model *initWizardModel) complete() {
	targets := make([]config.Target, len(model.targets))
	copy(targets, model.targets)

	profiles := make([]config.Profile, len(model.profiles))
	copy(profiles, model.profiles)

	model.result = &initWizardResult{
		Targets:            targets,
		Profiles:           profiles,
		ShouldIgnoreConfig: model.shouldIgnoreConfig,
	}
}

func (model *initWizardModel) beginPathBrowse() {
	model.step = initWizardStepPathBrowse
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
	model.browseNodes = model.selectedFile.nodes
	model.browseAncestors = nil
	model.selectedJSONPath = ""
	model.selectedDotenvKey = ""
}

func (model *initWizardModel) beginDotenvKeySelect() {
	model.step = initWizardStepDotenvKeySelect
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
	model.selectedJSONPath = ""
	model.selectedDotenvKey = ""
}

func (model *initWizardModel) beginTargetName() {
	model.step = initWizardStepTargetName
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
}

func (model *initWizardModel) beginTargetSummary() {
	model.step = initWizardStepTargetSummary
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
}

func (model *initWizardModel) beginProfileEntry() {
	model.step = initWizardStepProfileName
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
	model.draftProfile = initWizardProfileDraft{Values: make([]config.ProfileValue, 0, len(model.targets))}
}

func (model *initWizardModel) beginReview() {
	model.syncIgnorePreference()
	model.step = initWizardStepReview
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
}

func (model *initWizardModel) syncIgnorePreference() {
	if !hasLiteralProfiles(model.profiles) {
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
	target := config.Target{
		Name: name,
		File: model.selectedFile.path,
		Type: model.selectedFile.targetType,
	}
	if model.selectedFile.targetType == config.TargetTypeDotenv {
		target.Key = model.selectedDotenvKey
	} else {
		target.JSONPath = model.selectedJSONPath
	}

	model.targets = append(model.targets, target)
	model.beginTargetSummary()
}

func (model *initWizardModel) removeLastTarget() {
	if len(model.targets) == 0 {
		return
	}

	model.targets = model.targets[:len(model.targets)-1]
	if len(model.targets) == 0 {
		model.step = initWizardStepFileSelect
		model.cursor = 0
		model.errorMessage = ""
		return
	}

	model.beginTargetSummary()
}

func (model *initWizardModel) appendDraftProfile() {
	profile := config.Profile{
		Name:      model.draftProfile.Name,
		Values:    append([]config.ProfileValue(nil), model.draftProfile.Values...),
		Protected: model.draftProfile.Protected,
	}

	model.profiles = append(model.profiles, profile)
	model.draftProfile = initWizardProfileDraft{}
	model.step = initWizardStepProfileSummary
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
	model.syncIgnorePreference()
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
	model.errorMessage = ""
	model.inputValue = ""
}

func (model initWizardModel) filteredFileCandidates(filterValue string) []editor.TargetFileCandidate {
	return filterTargetFileCandidates(model.fileCandidates, filterValue)
}

func (model initWizardModel) selectableJSONPaths() []string {
	return flattenSelectableJSONPaths(model.selectedFile.nodes)
}

func (model initWizardModel) filteredSelectableJSONPaths(filterValue string) []string {
	return filterSelectableJSONPaths(model.selectableJSONPaths(), filterValue)
}

func (model initWizardModel) filteredDotenvKeys(filterValue string) []string {
	return filterDotenvKeys(model.selectedFile.dotenvKeys, filterValue)
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

func (model initWizardModel) currentDraftTarget() config.Target {
	if model.draftProfile.TargetIndex < 0 || model.draftProfile.TargetIndex >= len(model.targets) {
		return config.Target{}
	}

	return model.targets[model.draftProfile.TargetIndex]
}

func (model *initWizardModel) beginProfileTargetInclude() {
	model.step = initWizardStepProfileTargetInclude
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
}

func (model *initWizardModel) beginProfileSource() {
	model.step = initWizardStepProfileSource
	model.cursor = 0
	model.errorMessage = ""
	model.clearInputValue()
}

func (model *initWizardModel) appendDraftProfileValue() {
	target := model.currentDraftTarget()
	value := config.ProfileValue{Target: target.Name}
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
	model.errorMessage = ""
	model.clearInputValue()

	if model.draftProfile.TargetIndex >= len(model.targets) {
		if len(model.draftProfile.Values) == 0 {
			model.errorMessage = "a profile must include at least one target value"
			model.draftProfile.TargetIndex = 0
			model.beginProfileTargetInclude()
			return
		}

		model.step = initWizardStepProfileProtected
		return
	}

	if len(model.targets) == 1 {
		model.beginProfileSource()
		return
	}

	model.beginProfileTargetInclude()
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
		initWizardStepTargetName,
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
