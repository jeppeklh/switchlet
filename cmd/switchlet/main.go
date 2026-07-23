package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
	"github.com/jeppeklh/switchlet/internal/config"
	"github.com/jeppeklh/switchlet/internal/tui"
)

func main() {
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get working directory: %v\n", err)
		os.Exit(1)
	}

	if err := run(workingDirectory, startProgram); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(workingDirectory string, runProgram func(tea.Model) error) error {
	configPath, err := config.Discover(workingDirectory)
	if err != nil {
		return err
	}

	loadedConfig, err := config.Load(configPath)
	if err != nil {
		return err
	}

	application := app.New(loadedConfig.Target, loadedConfig.Profiles)
	if err := application.ValidateStartup(); err != nil {
		return err
	}
	if err := runProgram(tui.New(application)); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}

	return nil
}

func startProgram(model tea.Model) error {
	_, err := tea.NewProgram(model).Run()
	return err
}
