package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) handleInsertMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

	if msg.String() == "esc" {
		m.state.Mode = 1
		m.input.Blur()
		return m, nil
	}

	if msg.String() == "ctrl+s" {
		text := m.input.Value()
		m.state.SubmitText(text)
		m.input.Reset()
		m.refreshViewport()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
