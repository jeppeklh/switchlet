package main

import (
	"fmt"
	"io"
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

	if err := runCommand(os.Args[1:], workingDirectory, startProgram, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, workingDirectory string, runProgram func(tea.Model) error, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return run(workingDirectory, runProgram)
	}

	switch args[0] {
	case "help", "-h", "--help":
		_, err := io.WriteString(output, usageText())
		return err
	case "init":
		if len(args) != 1 {
			return fmt.Errorf("init does not accept additional arguments\n\n%s", usageText())
		}

		return runInit(workingDirectory, input, output, defaultInitDependencies())
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText())
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

func usageText() string {
	return `Usage:
  switchlet        Launch the profile switcher
  switchlet init   Create a new .switchlet.yaml in the current directory
  switchlet help   Show this help text
`
}
