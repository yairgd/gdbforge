package tui

import (
	"fmt"

	"github.com/yairgd/promptcore/internal/app"
	"github.com/yairgd/promptcore/internal/events"

	//"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	state       app.State
	input       InputBox
	cmdInputBox *CmdInputBox
	app         *app.App

	viewport viewport.Model
}

// emit event as tea.Cmd
func emitEvent(e events.Event) tea.Cmd {
	return func() tea.Msg {
		return e
	}
}

// simulate async work
func sendMessageCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return events.MessageSent{Text: text}
	}
}

func NewModel() Model {
	//	ta := textarea.New()
	//	ta.Focus()
	//	ta.CharLimit = 0
	//	ta.SetHeight(6)
	//	ta.ShowLineNumbers = true

	vp := viewport.New(80, 20)

	return Model{
		state:       app.NewState(),
		input:       NewInputBox(),
		cmdInputBox: NewCmdInputBox(),
		viewport:    vp,
		app:         app.New(),
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
		return m, m.cmdInputBox.Update(msg)
		//return m.handleCommandMode(msg)
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
			return events.SubmitMsg{Text: text}
		}

	// Transition from normal mode to command mode.
	// Command mode is activated when ':' is pressed (similar to Vim).
	case tea.KeyRunes:
		if msg.String() == ":" {
			m.state.Mode = app.CommandMode
			m.cmdInputBox.SetActive()
			return m, nil
		}
	}

	// any other key returns to insert
	m.state.Mode = app.InsertMode
	m.input.Focus()

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

		m.cmdInputBox.SetWidth(msg.Width)
		m.cmdInputBox.SetHeight(1)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+d":
			return m, tea.Quit

		default:
			return m.handleKey(msg)
		}
	case CancelCmdMode:
		m.state.Mode = app.NormalMode

	case events.SubmitMsg:
		m.state.SubmitText(msg.Text)
		m.refreshViewport()
		return m, nil

	case events.Event:
		switch msg.(type) {
		case events.Quit:
			return m, tea.Quit
		}
		nextEvents, _ := m.app.HandleEvent(msg)

		// chain events
		var cmds []tea.Cmd
		for _, e := range nextEvents {
			cmds = append(cmds, emitEvent(e))
		}

		return m, tea.Batch(cmds...)

	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {

	//	top := topStyle.Render("Vim-Style TUI")

	vpBox := paneStyle.Width(m.state.Width).Render(m.viewport.View())
	inputBox := inputStyle.Width(m.state.Width).Render(m.input.View())
	cmdInputBox := cmdInputBoxStyle.Width(m.state.Width).Height(1).Render(m.cmdInputBox.View())

	return fmt.Sprintf("%s\n\n%s\n%s",
		//	top,
		vpBox,
		inputBox,
		cmdInputBox,
		//		cmdLine,
	)
}
