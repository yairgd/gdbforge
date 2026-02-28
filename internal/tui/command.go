package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

	switch msg.Type {

	case tea.KeyEsc:
		m.state.Mode = 1 // NormalMode
		m.state.CommandInput = ""
		return m, nil

	case tea.KeyEnter:
		quit := m.state.ExecuteCommand()
		m.refreshViewport()
		m.state.Mode = 1
		if quit {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.state.CommandInput) > 0 {
			m.state.CommandInput = m.state.CommandInput[:len(m.state.CommandInput)-1]
		}

	case tea.KeyRunes:
		m.state.CommandInput += string(msg.Runes)
	}

	return m, nil
}
