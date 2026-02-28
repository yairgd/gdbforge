package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type state int

const (
	stateHome state = iota
	stateHelp
)

type model struct {
	w, h    int
	state   state
	cursor  int
	choices []string
}

func initialModel() model {
	return model{
		state:   stateHome,
		cursor:  0,
		choices: []string{"Start", "Help", "Quit"},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		k := msg.String()

		// Quit always
		if k == "ctrl+c" || k == "q" {
			return m, tea.Quit
		}

		// Simple state machine
		switch m.state {

		case stateHome:
			switch k {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}
			case "enter":
				switch m.choices[m.cursor] {
				case "Start":
					// TODO: do something (set state, start a command, etc.)
				case "Help":
					m.state = stateHelp
				case "Quit":
					return m, tea.Quit
				}
			}

		case stateHelp:
			switch k {
			case "esc", "backspace", "h":
				m.state = stateHome
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateHelp:
		return m.viewHelp()
	default:
		return m.viewHome()
	}
}

func (m model) viewHome() string {
	var b strings.Builder
	b.WriteString("My Bubble Tea App\n")
	b.WriteString("-----------------\n\n")

	for i, c := range m.choices {
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor
		}
		fmt.Fprintf(&b, "%s %s\n", cursor, c)
	}

	b.WriteString("\n")
	b.WriteString("Keys: ↑/↓ (or j/k), Enter, q to quit\n")
	b.WriteString(fmt.Sprintf("Size: %dx%d\n", m.w, m.h))
	return b.String()
}

func (m model) viewHelp() string {
	return strings.Join([]string{
		"Help",
		"----",
		"",
		"- Up/Down or j/k: move",
		"- Enter: select",
		"- h / Esc: back",
		"- q: quit",
		"",
	}, "\n")
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
