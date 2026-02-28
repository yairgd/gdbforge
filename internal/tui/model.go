package tui

import (
	"fmt"

	"github.com/yairgd/promptcore/internal/app"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	state    app.State
	input    textarea.Model
	viewport viewport.Model
}

func NewModel() Model {
	ta := textarea.New()
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(6)
	ta.ShowLineNumbers = true

	vp := viewport.New(80, 20)

	return Model{
		state:    app.NewState(),
		input:    ta,
		viewport: vp,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.state.Width = msg.Width
		m.state.Height = msg.Height

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 12

		m.input.SetWidth(msg.Width)
		m.input.SetHeight(6)

	case tea.KeyMsg:

		switch m.state.Mode {

		case 0: // InsertMode
			return m.handleInsertMode(msg)

		case 1: // NormalMode
			if msg.String() == ":" {
				m.state.Mode = 2
				return m, nil
			}
			m.state.Mode = 0
			m.input.Focus()
			return m, nil

		case 2: // CommandMode
			return m.handleCommandMode(msg)
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {

	top := "Vim-Style TUI"

	vpBox := paneStyle.Width(m.state.Width).Render(m.viewport.View())
	inputBox := inputStyle.Width(m.state.Width).Render(m.input.View())

	var cmdLine string

	switch m.state.Mode {
	case 2:
		cmdLine = cmdStyle.Width(m.state.Width).
			Render(":" + m.state.CommandInput)
	case 1:
		cmdLine = cmdStyle.Width(m.state.Width).
			Render("-- NORMAL --")
	default:
		cmdLine = cmdStyle.Width(m.state.Width).
			Render("-- INSERT --")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s",
		top,
		vpBox,
		inputBox,
		cmdLine,
	)
}
