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
	initWizardStepPathBrowse
	initWizardStepPathSearch
	initWizardStepManualPath
	initWizardStepProfileName
	initWizardStepProfileSource
	initWizardStepProfileValue
	initWizardStepProfileProtected
	initWizardStepProfileSummary
	initWizardStepReview
)

type initWizardProfileDraft struct {
	Name           string
	UseEnvironment bool
	Value          string
	Protected      bool
}

type initWizardResult struct {
	Target             config.Target
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
	fileFilter            string
	fileCandidates        []editor.TargetFileCandidate
	selectedFile          targetFileSelection
	browseNodes           []editor.StringTargetNode
	browseAncestors       []targetBrowseLevel
	selectedJSONPath      string
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

	finalModel, err := tea.NewProgram(model, tea.WithInput(input), tea.WithOutput(output)).Run()
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
	profiles := make([]config.Profile, len(model.profiles))
	copy(profiles, model.profiles)

	model.result = &initWizardResult{
		Target: config.Target{
			File:     model.selectedFile.path,
			JSONPath: model.selectedJSONPath,
		},
		Profiles:           profiles,
		ShouldIgnoreConfig: model.shouldIgnoreConfig,
	}
}

func (model *initWizardModel) beginPathBrowse() {
	model.step = initWizardStepPathBrowse
	model.cursor = 0
	model.errorMessage = ""
	model.inputValue = ""
	model.browseNodes = model.selectedFile.nodes
	model.browseAncestors = nil
	model.selectedJSONPath = ""
}

func (model *initWizardModel) beginProfileEntry() {
	model.step = initWizardStepProfileName
	model.cursor = 0
	model.errorMessage = ""
	model.inputValue = ""
	model.draftProfile = initWizardProfileDraft{}
	if len(model.profiles) > 0 {
		model.draftProfile.Protected = false
	}
}

func (model *initWizardModel) beginReview() {
	model.syncIgnorePreference()
	model.step = initWizardStepReview
	model.cursor = 0
	model.errorMessage = ""
	model.inputValue = ""
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

func (model *initWizardModel) appendDraftProfile() {
	profile := config.Profile{
		Name:      model.draftProfile.Name,
		Protected: model.draftProfile.Protected,
	}
	if model.draftProfile.UseEnvironment {
		profile.ValueFromEnv = stringValuePointer(model.draftProfile.Value)
	} else {
		profile.Value = stringValuePointer(model.draftProfile.Value)
	}

	model.profiles = append(model.profiles, profile)
	model.draftProfile = initWizardProfileDraft{}
	model.step = initWizardStepProfileSummary
	model.cursor = 0
	model.errorMessage = ""
	model.inputValue = ""
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
		initWizardStepProfileName,
		initWizardStepProfileValue:
		return true
	default:
		return false
	}
}
