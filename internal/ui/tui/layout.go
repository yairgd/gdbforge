package tui

import "github.com/charmbracelet/lipgloss"

var paneStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(0, 1)

var inputStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("205"))

var cmdStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("0")).
	Background(lipgloss.Color("10")).
	Bold(true)

var cmdInputBoxStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("255")). // black text
	Background(lipgloss.Color("0")).   // green background
	Bold(true)

var topStyle = lipgloss.NewStyle().
	Foreground(lipplaygloss.Color("0")).
	Background(lipgloss.Color("0")).
	Bold(true)
