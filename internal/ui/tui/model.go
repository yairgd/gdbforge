package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yairgd/cgdb-go/internal/app"
	"github.com/yairgd/cgdb-go/internal/core"
)

type Model struct {
	state       app.State
	input       InputBox
	cmdInputBox *CmdInputBox
	app         *app.App

	viewport viewport.Model
}

func NewModel() Model {
	vp := viewport.New(80, 20)

	return Model{
		state:       app.NewState(),
		input:       NewInputBox(),
		cmdInputBox: NewCmdInputBox(core.NewMemoryHistory(), nil),
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
			return core.SubmitMessage{Text: text}
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

func (m *Model) refreshViewport() {
	content := ""
	for i, l := range m.state.Lines {
		content += fmt.Sprintf("[%d]\n%s\n\n", i+1, l)
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.state.Width = msg.Width
		m.state.Height = msg.Height
		m.input.SetWidth(m.state.Width - 2)
	//	m.input.SetHeight(6)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+d":
			return m, tea.Quit

		default:
			return m.handleKey(msg)
		}
	case CancelCmdMode:
		m.state.Mode = app.NormalMode

	case core.SubmitMessage:
		m.state.SubmitText(msg.Text)
		m.refreshViewport()
		return m, nil

	case core.Event:
		switch msg.(type) {
		case core.Quit:
			return m, tea.Quit
		}
		nextEvents, _ := m.app.HandleEvent(msg)

		// chain core
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
	m.input.SetWidth(m.state.Width - 2)
	m.input.SetHeight(6)
	vpBox := paneStyle.Width(m.state.Width - 2).Height(m.state.Height - 12).Render(m.viewport.View())
	inputBox := inputStyle.Width(m.state.Width - 2).Height(6).Render(m.input.View())
	cmdInputBox := cmdInputBoxStyle.Width(m.state.Width).Height(1).Render(m.cmdInputBox.View())

	return fmt.Sprintf("%s\n\n%s\n%s",
		vpBox,
		inputBox,
		cmdInputBox,
	)
}
