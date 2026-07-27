package initwizard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	ui "github.com/jeppeklh/switchlet/internal/tui"
)

const (
	overwriteConfirmationChoiceCount = 2
	overwriteConfirmationPanelWidth  = 84
)

// OverwriteConfirmationResult describes the completed overwrite confirmation outcome.
type OverwriteConfirmationResult struct {
	Replace   bool
	Cancelled bool
}

type overwriteConfirmationModel struct {
	configPath string
	width      int
	height     int
	cursor     int
	result     *OverwriteConfirmationResult
}

// NewOverwriteConfirmationModel creates the Bubble Tea model for init overwrite confirmation.
func NewOverwriteConfirmationModel(configPath string) tea.Model {
	return overwriteConfirmationModel{
		configPath: configPath,
		cursor:     0,
	}
}

func (model overwriteConfirmationModel) Init() tea.Cmd {
	return nil
}

func (model overwriteConfirmationModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case tea.KeyMsg:
		if message.Type == tea.KeyCtrlC || message.Type == tea.KeyEsc || isQuitKey(message) {
			model.cancel()
			return model, tea.Quit
		}
		if model.isTerminalTooSmall() {
			return model, nil
		}

		switch {
		case isMoveUpKey(message):
			model.cursor--
			if model.cursor < 0 {
				model.cursor = overwriteConfirmationChoiceCount - 1
			}
		case isMoveDownKey(message):
			model.cursor++
			if model.cursor >= overwriteConfirmationChoiceCount {
				model.cursor = 0
			}
		case isRuneKey(message, 'y'):
			model.replace()
			return model, tea.Quit
		case isRuneKey(message, 'n'):
			model.keepExisting()
			return model, tea.Quit
		case message.Type == tea.KeyEnter:
			if model.cursor == 1 {
				model.replace()
			} else {
				model.keepExisting()
			}
			return model, tea.Quit
		}
	}

	return model, nil
}

func (model overwriteConfirmationModel) View() string {
	if model.isTerminalTooSmall() {
		return ui.RenderShell(ui.Shell{
			Title:    "Switchlet init",
			Subtitle: "Terminal too small.",
			Panels: []ui.Panel{{Title: "Resize required", Lines: []string{
				fmt.Sprintf("Minimum size: %dx%d", initWizardMinimumTerminalWidth, initWizardMinimumTerminalHeight),
				fmt.Sprintf("Current size: %dx%d", model.width, model.height),
				"Resize the terminal to continue.",
			}}},
			Actions: []ui.Action{{Key: "q", Label: "Cancel"}, {Key: "Ctrl+C", Label: "Cancel immediately"}},
			Width:   model.width,
			Height:  model.height,
		})
	}

	choices := []ui.ListRow{
		{Label: "Keep existing configuration", State: ui.RowNormal},
		{Label: "Replace .switchlet.yaml", State: ui.RowNormal},
	}
	choices[model.cursor].State = ui.RowSelected

	decisionLines := []string{
		ui.RenderKeyValue("File", model.configPath),
		"",
		"Replacing it will write a new validated setup after review.",
		"",
	}
	decisionLines = append(decisionLines, ui.RenderListRows(choices)...)

	return ui.RenderShell(ui.Shell{
		Title:    "Switchlet init",
		Subtitle: "Existing configuration",
		Metadata: []string{"Choose whether setup should continue."},
		Panels: []ui.Panel{{
			Title:   "Switchlet is already configured in this directory.",
			Lines:   decisionLines,
			Focused: true,
			Width:   overwriteConfirmationPanelWidth,
		}},
		Actions: []ui.Action{
			{Key: "Enter", Label: "Select"},
			{Key: "↑/↓ or j/k", Label: "Move"},
			{Key: "y", Label: "Replace"},
			{Key: "n", Label: "Keep"},
			{Key: "q", Label: "Cancel"},
			{Key: "Ctrl+C", Label: "Cancel immediately"},
		},
		Width:  model.width,
		Height: model.height,
	})
}

// Result returns the completed overwrite confirmation result, if available.
func (model overwriteConfirmationModel) Result() (OverwriteConfirmationResult, bool) {
	if model.result == nil {
		return OverwriteConfirmationResult{}, false
	}

	return *model.result, true
}

func (model overwriteConfirmationModel) isTerminalTooSmall() bool {
	if model.width == 0 || model.height == 0 {
		return false
	}

	return model.width < initWizardMinimumTerminalWidth || model.height < initWizardMinimumTerminalHeight
}

func (model *overwriteConfirmationModel) keepExisting() {
	model.result = &OverwriteConfirmationResult{}
}

func (model *overwriteConfirmationModel) replace() {
	model.result = &OverwriteConfirmationResult{Replace: true}
}

func (model *overwriteConfirmationModel) cancel() {
	model.result = &OverwriteConfirmationResult{Cancelled: true}
}
