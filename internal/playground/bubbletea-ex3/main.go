package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item string

func (i item) FilterValue() string { return string(i) }

type mainPane struct {
	list      list.Model
	table     table.Model
	progress  progress.Model
	search    textinput.Model
	searching bool
	percent   float64
	width     int
	height    int
}

func newMainPane() *mainPane {

	items := []list.Item{
		item("Server A"),
		item("Server B"),
		item("Server C"),
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Servers"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true) // ✅ FIXED

	columns := []table.Column{
		{Title: "Name", Width: 15},
		{Title: "Status", Width: 12},
	}

	rows := []table.Row{
		{"Server A", "Running"},
		{"Server B", "Stopped"},
		{"Server C", "Running"},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	p := progress.New(progress.WithDefaultGradient())

	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 64
	ti.Width = 30

	return &mainPane{
		list:     l,
		table:    t,
		progress: p,
		search:   ti,
	}
}

func (m *mainPane) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return t
	})
}

func (m *mainPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		contentHeight := msg.Height - 6
		m.list.SetSize(msg.Width/2-3, contentHeight)
		m.table.SetWidth(msg.Width/2 - 3)
		m.table.SetHeight(contentHeight)

	case time.Time:
		m.percent += 0.01
		if m.percent > 1.0 {
			m.percent = 0
		}
		return m, tick()

	case tea.KeyMsg:

		// Activate search
		if msg.String() == "/" && !m.searching {
			m.searching = true
			m.search.Focus()
			return m, nil
		}

		// Handle search input
		if m.searching {

			switch msg.String() {

			case "esc":
				m.searching = false
				m.search.SetValue("")
				m.list.SetFilterText("")
				m.search.Blur()
				return m, nil

			case "enter":
				m.searching = false
				m.search.Blur()
				return m, nil
			}

			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.list.SetFilterText(m.search.Value())
			return m, cmd
		}

		if msg.String() == "q" {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.table, _ = m.table.Update(msg)

	return m, cmd
}

func (m *mainPane) View() string {

	var left string

	searchStyle := lipgloss.NewStyle()

	if !m.searching {
		searchStyle = searchStyle.
			Foreground(lipgloss.Color("8")) // gray when inactive
	}

	searchLine := searchStyle.Render(m.search.View())

	left = lipgloss.JoinVertical(
		lipgloss.Left,
		searchLine,
		m.list.View(),
	)

	row := lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		lipgloss.NewStyle().Width(m.width/2-2).Render(left),
		lipgloss.NewStyle().Width(m.width/2-2).Render(m.table.View()),
	)

	progressBar := lipgloss.NewStyle().
		Width(m.width - 4).
		Render(m.progress.ViewAs(m.percent))

	layout := lipgloss.JoinVertical(
		lipgloss.Center,
		row,

		progressBar,
	)

	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("2")). // green
		Padding(1, 1)

	return frameStyle.Render(layout)
}

func main() {
	p := tea.NewProgram(newMainPane(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
