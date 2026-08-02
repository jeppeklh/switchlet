package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type commandOutputStyles struct {
	styled  bool
	title   lipgloss.Style
	muted   lipgloss.Style
	heading lipgloss.Style
	label   lipgloss.Style
	marker  lipgloss.Style
	badge   lipgloss.Style
	success lipgloss.Style
	warning lipgloss.Style
	error   lipgloss.Style
}

func defaultCommandOutputStyles(outputOptions commandOutputOptions) commandOutputStyles {
	if outputOptions.NoColor {
		return plainCommandOutputStyles()
	}

	return commandOutputStyles{
		styled:  true,
		title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		heading: lipgloss.NewStyle().Bold(true),
		label:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		marker:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		badge:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		success: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		warning: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		error:   lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
}

func plainCommandOutputStyles() commandOutputStyles {
	style := lipgloss.NewStyle()
	return commandOutputStyles{
		styled:  false,
		title:   style,
		muted:   style,
		heading: style,
		label:   style,
		marker:  style,
		badge:   style,
		success: style,
		warning: style,
		error:   style,
	}
}

func renderCommandErrorText(message string, outputOptions commandOutputOptions) string {
	styles := defaultCommandOutputStyles(outputOptions)
	lines := strings.Split(message, "\n")
	for index, line := range lines {
		switch {
		case index == 0 && strings.TrimSpace(line) != "":
			lines[index] = styles.error.Render(line)
		case isCommandErrorSection(line):
			lines[index] = styles.heading.Render(line)
		}
	}

	return strings.Join(lines, "\n")
}

func isCommandErrorSection(line string) bool {
	switch strings.TrimSpace(line) {
	case "Available profiles:", "Try:", "Usage:", "Flags:", "Examples:", "Exit codes:":
		return true
	default:
		return false
	}
}
