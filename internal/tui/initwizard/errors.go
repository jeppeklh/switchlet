package initwizard

import (
	"fmt"

	ui "github.com/jeppeklh/switchlet/internal/tui"
)

func (model *initWizardModel) clearError() {
	model.errorDetail = ui.RecoverableError{}
}

func (model *initWizardModel) setStepError(problem string, reason string, recovery string) {
	model.errorDetail = ui.RecoverableError{
		Problem:  problem,
		Context:  model.stepErrorContextLines(),
		Reason:   reason,
		Recovery: recovery,
	}
}

func (model *initWizardModel) setInputRequiredError() {
	model.setStepError("Input is required.", "Value must not be empty.", "Enter a value or press Esc to go back.")
}

func (model initWizardModel) stepErrorContextLines() []string {
	lines := make([]string, 0, 4)

	switch model.step {
	case initWizardStepFileFilter:
		if model.inputValue != "" {
			lines = append(lines, ui.RenderKeyValue("Filter", model.inputValue))
		}
	case initWizardStepManualFile:
		if model.inputValue != "" {
			lines = append(lines, ui.RenderKeyValue("File", model.inputValue))
		}
	case initWizardStepPathSearch, initWizardStepManualPath:
		lines = model.appendSelectedFileContext(lines)
		if model.inputValue != "" {
			lines = append(lines, ui.RenderKeyValue("Selector", model.inputValue))
		}
	case initWizardStepManualDotenvKey:
		lines = model.appendSelectedFileContext(lines)
		if model.inputValue != "" {
			lines = append(lines, ui.RenderKeyValue("Key", model.inputValue))
		}
	case initWizardStepManagedValueName:
		lines = model.appendSelectedFileContext(lines)
		if selector := model.selectedValueSelector(); selector != "" {
			lines = append(lines, ui.RenderKeyValue("Selected value", selector))
		}
		if model.inputValue != "" {
			lines = append(lines, ui.RenderKeyValue("Managed value name", model.inputValue))
		}
	case initWizardStepProfileName:
		if model.inputValue != "" {
			lines = append(lines, ui.RenderKeyValue("Profile name", model.inputValue))
		}
	case initWizardStepProfileTargetInclude, initWizardStepProfileValueSource, initWizardStepProfileValue:
		if model.draftProfile.Name != "" {
			lines = append(lines, ui.RenderKeyValue("Profile", model.draftProfile.Name))
		}
		if target := model.currentDraftTarget(); target.Name != "" {
			lines = append(lines, ui.RenderKeyValue("Managed value", target.Name))
		}
	}

	return lines
}

func (model initWizardModel) appendSelectedFileContext(lines []string) []string {
	if model.selectedFile.DisplayPath != "" {
		lines = append(lines, ui.RenderKeyValue("File", model.selectedFile.DisplayPath))
	}
	if model.selectedFile.TargetType != "" {
		lines = append(lines, ui.RenderKeyValue("Type", initTargetTypeDisplayName(model.selectedFile.TargetType)))
	}

	return lines
}

func (model initWizardModel) selectedValueSelector() string {
	if model.selectedDotenvKey != "" {
		return model.selectedDotenvKey
	}
	if model.selectedYAMLPath != "" {
		return model.selectedYAMLPath
	}
	if model.selectedTOMLPath != "" {
		return model.selectedTOMLPath
	}

	return model.selectedJSONPath
}

func fileInspectionError(pendingEffect initWizardPendingEffect, err error) ui.RecoverableError {
	return ui.RecoverableError{
		Problem:  "Could not inspect configuration file.",
		Context:  pendingErrorContextLines(pendingEffect),
		Reason:   errorReason(err),
		Recovery: "Choose another supported file or enter a different path.",
		Cause:    err,
	}
}

func jsonSelectorValidationError(pendingEffect initWizardPendingEffect, err error) ui.RecoverableError {
	return ui.RecoverableError{
		Problem:  "Could not use this JSON value.",
		Context:  pendingErrorContextLines(pendingEffect),
		Reason:   errorReason(err),
		Recovery: "Choose an existing string value or enter another path.",
		Cause:    err,
	}
}

func yamlSelectorValidationError(pendingEffect initWizardPendingEffect, err error) ui.RecoverableError {
	return ui.RecoverableError{
		Problem:  "Could not use this YAML value.",
		Context:  pendingErrorContextLines(pendingEffect),
		Reason:   errorReason(err),
		Recovery: "Choose an existing string value or enter another path.",
		Cause:    err,
	}
}

func tomlSelectorValidationError(pendingEffect initWizardPendingEffect, err error) ui.RecoverableError {
	return ui.RecoverableError{
		Problem:  "Could not use this TOML value.",
		Context:  pendingErrorContextLines(pendingEffect),
		Reason:   errorReason(err),
		Recovery: "Choose an existing string value or enter another path.",
		Cause:    err,
	}
}

func dotenvKeyValidationError(pendingEffect initWizardPendingEffect, err error) ui.RecoverableError {
	return ui.RecoverableError{
		Problem:  "Could not use this dotenv key.",
		Context:  pendingErrorContextLines(pendingEffect),
		Reason:   errorReason(err),
		Recovery: "Choose an existing unique key or enter another key.",
		Cause:    err,
	}
}

func pendingErrorContextLines(pendingEffect initWizardPendingEffect) []string {
	lines := make([]string, 0, 4)
	if pendingEffect.DisplayPath != "" {
		lines = append(lines, ui.RenderKeyValue("File", pendingEffect.DisplayPath))
	}
	if pendingEffect.TargetType != "" {
		lines = append(lines, ui.RenderKeyValue("Type", initTargetTypeDisplayName(pendingEffect.TargetType)))
	}
	if pendingEffect.Selector != "" {
		lines = append(lines, ui.RenderKeyValue("Selector", pendingEffect.Selector))
	}

	return lines
}

func errorReason(err error) string {
	if err == nil {
		return "Unknown error."
	}

	return fmt.Sprint(err)
}
