package configeditor

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type managedValueDraftMode int

const (
	managedValueDraftAdd managedValueDraftMode = iota
	managedValueDraftRename
	managedValueDraftLocationUpdate
	managedValueDraftRemove
)

type managedValueRequestKind int

const (
	managedValueRequestNone managedValueRequestKind = iota
	managedValueRequestDiscoverFiles
	managedValueRequestInspectFile
	managedValueRequestValidateSelector
)

type managedValueSelectorOption struct {
	Label    string
	Selector string
}

type managedValueDraftState struct {
	mode            managedValueDraftMode
	originalName    string
	target          app.InitTarget
	fileCandidates  []app.InitTargetFileCandidate
	fileCursor      int
	fileFilter      string
	typeCursor      int
	selectorOptions []managedValueSelectorOption
	selectorCursor  int
	selectorFilter  string
	requestID       int
	requestKind     managedValueRequestKind
	pendingDocument app.ConfigEditDocument
	renameResult    app.ConfigEditRenameManagedValueResult
	removeResult    app.ConfigEditRemoveManagedValueResult
	errorMessage    string
}

func (model *Model) beginAddManagedValue() tea.Cmd {
	model.managedForm = managedValueDraftState{mode: managedValueDraftAdd}
	model.inputValue = ""
	model.inputCursor = 0
	model.saveError = ""
	model.state = editorStateManagedValueFileLoading

	return model.startManagedValueFileDiscovery()
}

func (model *Model) beginRenameManagedValue(targetName string) {
	target, ok := model.managedValueTarget(targetName)
	if !ok {
		model.saveError = fmt.Sprintf("target %q was not found", targetName)
		model.selectReviewOverview()
		return
	}

	model.managedForm = managedValueDraftState{
		mode:         managedValueDraftRename,
		originalName: targetName,
		target:       target,
	}
	model.state = editorStateManagedValueNameInput
	model.inputValue = targetName
	model.inputCursor = len([]rune(model.inputValue))
	model.saveError = ""
}

func (model *Model) beginEditManagedValueLocation(targetName string) tea.Cmd {
	target, ok := model.managedValueTarget(targetName)
	if !ok {
		model.saveError = fmt.Sprintf("target %q was not found", targetName)
		model.selectReviewOverview()
		return nil
	}

	model.managedForm = managedValueDraftState{
		mode:         managedValueDraftLocationUpdate,
		originalName: targetName,
		target:       target,
	}
	model.inputValue = ""
	model.inputCursor = 0
	model.saveError = ""
	model.state = editorStateManagedValueFileLoading

	return model.startManagedValueFileDiscovery()
}

func (model *Model) beginRemoveManagedValue(targetName string) {
	target, ok := model.managedValueTarget(targetName)
	if !ok {
		model.saveError = fmt.Sprintf("target %q was not found", targetName)
		model.selectReviewOverview()
		return
	}

	updatedDocument, result, err := model.workflow.RemoveManagedValue(model.document, targetName)
	if err != nil {
		model.saveError = err.Error()
		model.selectReviewOverview()
		return
	}

	model.managedForm = managedValueDraftState{
		mode:            managedValueDraftRemove,
		originalName:    targetName,
		target:          target,
		pendingDocument: updatedDocument,
		removeResult:    result,
	}
	model.state = editorStateManagedValueRemoveConfirm
	model.saveError = ""
}

func (model *Model) startManagedValueFileDiscovery() tea.Cmd {
	requestID := model.startManagedValueRequest(managedValueRequestDiscoverFiles)
	return discoverManagedValueFiles(model.targetWorkflow, requestID, model.document.ProjectRoot)
}

func (model *Model) startManagedValueFileInspection(candidate app.InitTargetFileCandidate) tea.Cmd {
	requestID := model.startManagedValueRequest(managedValueRequestInspectFile)
	model.state = editorStateManagedValueSelectorLoading
	return inspectManagedValueFileCandidate(model.targetWorkflow, requestID, candidate)
}

func (model *Model) startManagedValueExplicitFileInspection(targetPath string, displayPath string, targetType app.InitTargetType) tea.Cmd {
	requestID := model.startManagedValueRequest(managedValueRequestInspectFile)
	model.state = editorStateManagedValueSelectorLoading
	return inspectManagedValueFile(model.targetWorkflow, requestID, targetPath, displayPath, targetType)
}

func (model *Model) startManagedValueSelectorValidation(selector string) tea.Cmd {
	requestID := model.startManagedValueRequest(managedValueRequestValidateSelector)
	model.state = editorStateManagedValueSelectorValidating
	return validateManagedValueSelector(model.targetWorkflow, requestID, model.managedForm.target.File, model.managedForm.target.Type, selector)
}

func (model *Model) startManagedValueRequest(kind managedValueRequestKind) int {
	requestID := model.nextRequestID()
	model.managedForm.requestID = requestID
	model.managedForm.requestKind = kind
	model.managedForm.errorMessage = ""
	return requestID
}

func (model Model) staleManagedValueRequest(requestID int, kind managedValueRequestKind) bool {
	return model.managedForm.requestID != requestID || model.managedForm.requestKind != kind
}

func (model *Model) clearManagedValueRequest() {
	model.managedForm.requestID = 0
	model.managedForm.requestKind = managedValueRequestNone
}

func (model *Model) cancelManagedValueForm() {
	model.managedForm = managedValueDraftState{}
	model.inputValue = ""
	model.inputCursor = 0
	model.state = editorStateOverview
}

func (model Model) managedValueTarget(targetName string) (app.InitTarget, bool) {
	for _, target := range model.document.Targets {
		if target.Name == targetName {
			return target, true
		}
	}

	return app.InitTarget{}, false
}

func (model *Model) applyManagedValueManualFile() tea.Cmd {
	targetPath := strings.TrimSpace(model.inputValue)
	if targetPath == "" {
		model.managedForm.errorMessage = "File path must be set."
		return nil
	}

	resolvedPath := app.ResolveInitTargetPath(model.document.ProjectRoot, targetPath)
	displayPath := app.DisplayInitTargetPath(model.document.ProjectRoot, resolvedPath)
	model.managedForm.target.File = resolvedPath
	model.inputValue = ""
	model.inputCursor = 0

	targetType, ok := app.InferInitTargetType(resolvedPath)
	if !ok {
		model.managedForm.typeCursor = 0
		model.state = editorStateManagedValueTypeSelect
		return nil
	}

	model.managedForm.target.Type = targetType
	return model.startManagedValueExplicitFileInspection(resolvedPath, displayPath, targetType)
}

func (model *Model) chooseManagedValueTargetType() tea.Cmd {
	targetTypes := managedValueTargetTypeChoices()
	model.clampManagedValueTypeCursor()
	if len(targetTypes) == 0 {
		return nil
	}

	targetType := targetTypes[model.managedForm.typeCursor]
	model.managedForm.target.Type = targetType
	displayPath := app.DisplayInitTargetPath(model.document.ProjectRoot, model.managedForm.target.File)
	return model.startManagedValueExplicitFileInspection(model.managedForm.target.File, displayPath, targetType)
}

func (model *Model) chooseManagedValueFile() tea.Cmd {
	candidates := model.filteredManagedValueFileCandidates()
	model.clampManagedValueFileCursor(len(candidates))
	if len(candidates) == 0 {
		return nil
	}

	return model.startManagedValueFileInspection(candidates[model.managedForm.fileCursor])
}

func (model *Model) chooseManagedValueSelector() {
	options := model.filteredManagedValueSelectorOptions()
	model.clampManagedValueSelectorCursor(len(options))
	if len(options) == 0 {
		return
	}

	model.setManagedValueSelector(options[model.managedForm.selectorCursor].Selector)
	model.advanceAfterManagedValueSelector()
}

func (model *Model) setManagedValueSelector(selector string) {
	model.managedForm.target.JSONPath = ""
	model.managedForm.target.Key = ""
	model.managedForm.target.YAMLPath = ""
	model.managedForm.target.TOMLPath = ""

	switch model.managedForm.target.Type {
	case app.InitTargetTypeDotenv:
		model.managedForm.target.Key = selector
	case app.InitTargetTypeYAML:
		model.managedForm.target.YAMLPath = selector
	case app.InitTargetTypeTOML:
		model.managedForm.target.TOMLPath = selector
	default:
		model.managedForm.target.JSONPath = selector
	}
}

func (model *Model) advanceAfterManagedValueSelector() {
	model.inputValue = ""
	model.inputCursor = 0
	model.managedForm.errorMessage = ""
	if model.managedForm.mode == managedValueDraftAdd {
		model.state = editorStateManagedValueNameInput
		return
	}

	model.state = editorStateManagedValueReview
}

func (model *Model) applyManagedValueManualSelector() tea.Cmd {
	selector := strings.TrimSpace(model.inputValue)
	if selector == "" {
		model.managedForm.errorMessage = managedValueSelectorInputLabel(model.managedForm.target.Type) + " must be set."
		return nil
	}

	return model.startManagedValueSelectorValidation(selector)
}

func (model *Model) applyManagedValueNameInput() {
	managedValueName := strings.TrimSpace(model.inputValue)
	if managedValueName == "" {
		model.managedForm.errorMessage = "Target name must be set."
		return
	}

	model.inputValue = ""
	model.inputCursor = 0
	model.managedForm.errorMessage = ""
	if model.managedForm.mode == managedValueDraftRename {
		updatedDocument, result, err := model.workflow.RenameManagedValue(model.document, model.managedForm.originalName, managedValueName)
		if err != nil {
			model.inputValue = managedValueName
			model.inputCursor = len([]rune(model.inputValue))
			model.managedForm.errorMessage = err.Error()
			return
		}

		model.managedForm.pendingDocument = updatedDocument
		model.managedForm.renameResult = result
		model.managedForm.target.Name = managedValueName
		model.state = editorStateManagedValueReview
		return
	}

	model.managedForm.target.Name = managedValueName
	model.state = editorStateManagedValueReview
}

func (model *Model) completeManagedValueDraft() {
	var (
		updatedDocument app.ConfigEditDocument
		targetName      string
		err             error
	)
	removedManagedValue := model.managedForm.mode == managedValueDraftRemove

	switch model.managedForm.mode {
	case managedValueDraftRename, managedValueDraftRemove:
		updatedDocument = model.managedForm.pendingDocument
		targetName = model.managedForm.target.Name
	case managedValueDraftLocationUpdate:
		updatedDocument, err = model.workflow.UpdateManagedValueLocation(model.document, model.managedForm.originalName, model.managedForm.target)
		targetName = model.managedForm.originalName
	default:
		updatedDocument, err = model.workflow.AddManagedValue(model.document, model.managedForm.target)
		targetName = model.managedForm.target.Name
	}
	if err != nil {
		model.managedForm.errorMessage = err.Error()
		return
	}

	model.document = updatedDocument
	model.managedForm = managedValueDraftState{}
	model.inputValue = ""
	model.inputCursor = 0
	model.state = editorStateOverview
	model.saveError = ""
	if targetName == "" || removedManagedValue {
		model.selectReviewOverview()
		return
	}
	model.selectOverviewTab(overviewTabTargets)
	if rowIndex := model.managedValueRowIndex(targetName); rowIndex >= 0 {
		model.cursor = rowIndex
		return
	}
	model.selectReviewOverview()
}

func (model Model) managedValueRowIndex(targetName string) int {
	rows := model.navigationRows(model.overview())
	for index, row := range rows {
		if row.Kind == navigationRowManagedValue && row.Label == targetName {
			return index
		}
	}

	return -1
}

func (model Model) filteredManagedValueFileCandidates() []app.InitTargetFileCandidate {
	filter := normalizeManagedValueFilter(model.managedForm.fileFilter)
	if filter == "" {
		return model.managedForm.fileCandidates
	}

	filtered := make([]app.InitTargetFileCandidate, 0)
	for _, candidate := range model.managedForm.fileCandidates {
		relativePath := strings.ToLower(filepath.ToSlash(candidate.RelativePath))
		baseName := strings.ToLower(filepath.Base(candidate.RelativePath))
		if strings.Contains(baseName, filter) || strings.Contains(relativePath, filter) || strings.Contains(strings.ToLower(string(candidate.Type)), filter) {
			filtered = append(filtered, candidate)
		}
	}

	return filtered
}

func (model Model) filteredManagedValueSelectorOptions() []managedValueSelectorOption {
	filter := normalizeManagedValueFilter(model.managedForm.selectorFilter)
	if filter == "" {
		return model.managedForm.selectorOptions
	}

	filtered := make([]managedValueSelectorOption, 0)
	for _, option := range model.managedForm.selectorOptions {
		if strings.Contains(strings.ToLower(option.Label), filter) || strings.Contains(strings.ToLower(option.Selector), filter) {
			filtered = append(filtered, option)
		}
	}

	return filtered
}

func normalizeManagedValueFilter(filterValue string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(filterValue), "\\", "/"))
}

func (model *Model) clampManagedValueFileCursor(candidateCount int) {
	if candidateCount <= 0 {
		model.managedForm.fileCursor = 0
		return
	}
	if model.managedForm.fileCursor < 0 {
		model.managedForm.fileCursor = 0
	}
	if model.managedForm.fileCursor >= candidateCount {
		model.managedForm.fileCursor = candidateCount - 1
	}
}

func (model *Model) clampManagedValueSelectorCursor(optionCount int) {
	if optionCount <= 0 {
		model.managedForm.selectorCursor = 0
		return
	}
	if model.managedForm.selectorCursor < 0 {
		model.managedForm.selectorCursor = 0
	}
	if model.managedForm.selectorCursor >= optionCount {
		model.managedForm.selectorCursor = optionCount - 1
	}
}

func (model *Model) clampManagedValueTypeCursor() {
	targetTypes := managedValueTargetTypeChoices()
	if len(targetTypes) == 0 {
		model.managedForm.typeCursor = 0
		return
	}
	if model.managedForm.typeCursor < 0 {
		model.managedForm.typeCursor = 0
	}
	if model.managedForm.typeCursor >= len(targetTypes) {
		model.managedForm.typeCursor = len(targetTypes) - 1
	}
}

func managedValueTargetTypeChoices() []app.InitTargetType {
	return []app.InitTargetType{app.InitTargetTypeJSON, app.InitTargetTypeYAML, app.InitTargetTypeTOML, app.InitTargetTypeDotenv}
}

func managedValueSelectorOptions(selection app.InitTargetFileSelection) []managedValueSelectorOption {
	switch selection.TargetType {
	case app.InitTargetTypeDotenv:
		options := make([]managedValueSelectorOption, 0, len(selection.DotenvKeys))
		for _, key := range selection.DotenvKeys {
			options = append(options, managedValueSelectorOption{Label: key, Selector: key})
		}
		return options
	case app.InitTargetTypeYAML:
		return managedValueYAMLSelectorOptions(selection.YAMLNodes)
	case app.InitTargetTypeTOML:
		return managedValueTOMLSelectorOptions(selection.TOMLNodes)
	default:
		return managedValueJSONSelectorOptions(selection.Nodes)
	}
}

func managedValueJSONSelectorOptions(nodes []app.InitStringTargetNode) []managedValueSelectorOption {
	options := make([]managedValueSelectorOption, 0)
	for _, node := range nodes {
		if node.Selectable {
			options = append(options, managedValueSelectorOption{Label: node.JSONPath, Selector: node.JSONPath})
		}
		options = append(options, managedValueJSONSelectorOptions(node.Children)...)
	}

	return options
}

func managedValueYAMLSelectorOptions(nodes []app.InitYAMLStringTargetNode) []managedValueSelectorOption {
	options := make([]managedValueSelectorOption, 0)
	for _, node := range nodes {
		if node.Selectable {
			options = append(options, managedValueSelectorOption{Label: node.YAMLPath, Selector: node.YAMLPath})
		}
		options = append(options, managedValueYAMLSelectorOptions(node.Children)...)
	}

	return options
}

func managedValueTOMLSelectorOptions(nodes []app.InitTOMLStringTargetNode) []managedValueSelectorOption {
	options := make([]managedValueSelectorOption, 0)
	for _, node := range nodes {
		if node.Selectable {
			options = append(options, managedValueSelectorOption{Label: node.TOMLPath, Selector: node.TOMLPath})
		}
		options = append(options, managedValueTOMLSelectorOptions(node.Children)...)
	}

	return options
}

func managedValueSelectorInputLabel(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeDotenv:
		return "Dotenv key"
	case app.InitTargetTypeYAML:
		return "YAML value path"
	case app.InitTargetTypeTOML:
		return "TOML value path"
	default:
		return "JSON value path"
	}
}

func managedValueSelectorFieldName(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeDotenv:
		return "key"
	case app.InitTargetTypeYAML:
		return "yamlPath"
	case app.InitTargetTypeTOML:
		return "tomlPath"
	default:
		return "jsonPath"
	}
}

func managedValueTypeDisplayName(targetType app.InitTargetType) string {
	switch targetType {
	case app.InitTargetTypeDotenv:
		return "dotenv"
	case app.InitTargetTypeYAML:
		return "yaml"
	case app.InitTargetTypeTOML:
		return "toml"
	default:
		return "json"
	}
}

func managedValueTargetSelector(target app.InitTarget) string {
	switch target.Type {
	case app.InitTargetTypeDotenv:
		return target.Key
	case app.InitTargetTypeYAML:
		return target.YAMLPath
	case app.InitTargetTypeTOML:
		return target.TOMLPath
	default:
		return target.JSONPath
	}
}
