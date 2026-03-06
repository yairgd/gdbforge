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
	input    InputBox
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
		input:    NewInputBox(),
		viewport: vp,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state.Mode {
	case app.InsertMode:
		return m.handleInsertMode(msg)
	case app.NormalMode:
		return m.handleNormalMode(msg)
	case app.CommandMode:
		return m.handleCommandMode(msg)
	}
	return m, nil
}

func (m *Model) handleInsertMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

	switch msg.String() {

	case "esc":
		m.state.Mode = app.NormalMode
		m.input.Blur()
		return m, nil
	}

	cmd := m.input.Update(msg)
	return m, cmd
}

func (m *Model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

	switch msg.Type {

	case tea.KeyEnter:
		text := m.input.Value()

		return m, func() tea.Msg {
			m.input.Reset()
			return SubmitMsg{Text: text}
		}

	case tea.KeyRunes:
		if msg.String() == ":" {
			m.state.Mode = app.CommandMode
			return m, nil
		}
	}

	// any other key returns to insert
	m.state.Mode = app.InsertMode
	m.input.Focus()

	return m, nil
}

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
		return m.handleKey(msg)

	case SubmitMsg:
		m.state.SubmitText(msg.Text)
		m.refreshViewport()
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {

	top := topStyle.Render("Vim-Style TUI")

	vpBox := paneStyle.Width(m.state.Width).Render(m.viewport.View())
	inputBox := inputStyle.Width(m.state.Width).Render(m.input.View())

	var cmdLine string

	switch m.state.Mode {
	case app.CommandMode:
		cmdLine = cmdStyle.Width(m.state.Width).
			Render(":" + m.state.CommandInput)
	case app.NormalMode:
		cmdLine = cmdStyle.Width(m.state.Width).
			Render("-- NORMAL --")
	case app.InsertMode:
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
