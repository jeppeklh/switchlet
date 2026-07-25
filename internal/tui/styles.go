package tui

import "github.com/charmbracelet/lipgloss"

type styleSet struct {
	title        lipgloss.Style
	subtitle     lipgloss.Style
	muted        lipgloss.Style
	panel        lipgloss.Style
	focusedPanel lipgloss.Style
	panelTitle   lipgloss.Style
	focusedTitle lipgloss.Style
	selectedRow  lipgloss.Style
	inactiveRow  lipgloss.Style
	disabledRow  lipgloss.Style
	badge        lipgloss.Style
	commandBar   lipgloss.Style
	commandKey   lipgloss.Style
	input        lipgloss.Style
	success      lipgloss.Style
	warning      lipgloss.Style
	error        lipgloss.Style
}

func defaultStyles() styleSet {
	return styleSet{
		title:        lipgloss.NewStyle().Bold(true),
		subtitle:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		muted:        lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		panel:        lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1),
		focusedPanel: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("12")).Padding(0, 1),
		panelTitle:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		focusedTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		selectedRow:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		inactiveRow:  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		disabledRow:  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		badge:        lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		commandBar:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		commandKey:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		input:        lipgloss.NewStyle().Bold(true),
		success:      lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		warning:      lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		error:        lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
}
