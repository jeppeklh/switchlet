package main

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/tui"
	"github.com/jeppeklh/switchlet/internal/tui/configeditor"
)

type interactiveSessionMode int

const (
	interactiveSessionMain interactiveSessionMode = iota
	interactiveSessionConfig
)

type interactiveSessionModel struct {
	workingDirectory string
	application      app.Application
	mainModel        tui.Model
	configModel      configeditor.Model
	mode             interactiveSessionMode
	width            int
	height           int
}

func newInteractiveSessionModel(workingDirectory string, application app.Application) interactiveSessionModel {
	return interactiveSessionModel{
		workingDirectory: workingDirectory,
		application:      application,
		mainModel:        tui.New(application),
		mode:             interactiveSessionMain,
	}
}

func (model interactiveSessionModel) Init() tea.Cmd {
	return model.mainModel.Init()
}

func (model interactiveSessionModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := message.(tea.WindowSizeMsg); ok {
		model.width = size.Width
		model.height = size.Height
	}

	if model.mode == interactiveSessionConfig {
		return model.updateConfig(message)
	}

	return model.updateMain(message)
}

func (model interactiveSessionModel) View() string {
	if model.mode == interactiveSessionConfig {
		return model.configModel.View()
	}

	return model.mainModel.View()
}

func (model interactiveSessionModel) FinalMessage() string {
	if model.mode == interactiveSessionMain {
		return model.mainModel.FinalMessage()
	}

	return ""
}

func (model interactiveSessionModel) updateMain(message tea.Msg) (tea.Model, tea.Cmd) {
	updatedModel, command := model.mainModel.Update(message)
	mainModel, ok := updatedModel.(tui.Model)
	if !ok {
		return model, command
	}
	model.mainModel = mainModel

	if !model.mainModel.ConfigRequested() {
		return model, command
	}

	configModel, err := configEditorModelForWorkingDirectory(model.workingDirectory, configeditor.Options{Embedded: true})
	if err != nil {
		model.mainModel = tui.NewConfigOpenError(err)
		return model, model.mainModel.Init()
	}

	model.configModel = model.resizeConfigModel(configModel)
	model.mode = interactiveSessionConfig
	return model, model.configModel.Init()
}

func (model interactiveSessionModel) updateConfig(message tea.Msg) (tea.Model, tea.Cmd) {
	updatedModel, command := model.configModel.Update(message)
	configModel, ok := updatedModel.(configeditor.Model)
	if !ok {
		return model, command
	}
	model.configModel = configModel

	result, ok := model.configModel.Result()
	if !ok {
		return model, command
	}
	if result.Quit {
		return model, tea.Quit
	}

	return model.handleConfigResult(result)
}

func (model interactiveSessionModel) handleConfigResult(result configeditor.Result) (tea.Model, tea.Cmd) {
	if result.Saved {
		application, err := loadApplication(model.workingDirectory)
		if err != nil {
			model.mainModel = model.resizeMainModel(tui.NewReloadError(err))
			model.mode = interactiveSessionMain
			return model, model.mainModel.Init()
		}

		model.application = application
	}

	model.mainModel = model.resizeMainModel(tui.New(model.application))
	model.mode = interactiveSessionMain
	return model, model.mainModel.Init()
}

func (model interactiveSessionModel) resizeMainModel(mainModel tui.Model) tui.Model {
	if model.width <= 0 && model.height <= 0 {
		return mainModel
	}

	updatedModel, _ := mainModel.Update(tea.WindowSizeMsg{Width: model.width, Height: model.height})
	if resizedModel, ok := updatedModel.(tui.Model); ok {
		return resizedModel
	}

	return mainModel
}

func (model interactiveSessionModel) resizeConfigModel(configModel configeditor.Model) configeditor.Model {
	if model.width <= 0 && model.height <= 0 {
		return configModel
	}

	updatedModel, _ := configModel.Update(tea.WindowSizeMsg{Width: model.width, Height: model.height})
	if resizedModel, ok := updatedModel.(configeditor.Model); ok {
		return resizedModel
	}

	return configModel
}
