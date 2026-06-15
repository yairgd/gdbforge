package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yairgd/promptcore/internal/app"
	"github.com/yairgd/promptcore/internal/core"
)

type GdbModel struct {
	state       app.State
	gdbWidget   *GDBWidget
	cmdInputBox *CmdInputBox
	app         *app.App

	viewport viewport.Model
}

func NewGdbModel(sendFunc func(string), sendRawFunc func(string)) GdbModel {
	vp := viewport.New(80, 20)

	history := core.NewMemoryHistory()

	completer := core.NewSimpleCompleter([]string{
		"break", "continue", "next", "step",
		"print", "bt", "info", "run", "quit",
	})

	cmdBox := NewCmdInputBox(history, completer)

	return GdbModel{
		state:       app.NewState(),
		gdbWidget:   NewGDBWidget(sendFunc, sendRawFunc),
		cmdInputBox: cmdBox,
		viewport:    vp,
		app:         app.New(),
	}
}

func (m GdbModel) Init() tea.Cmd {
	return nil
}

func (m *GdbModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m *GdbModel) handleInsertMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

	switch msg.String() {

	case "esc":
		m.state.Mode = app.NormalMode
		//	m.input.Blur()
		return m, nil
	}

	update, cmd := m.gdbWidget.Update(msg)
	m.gdbWidget = update.(*GDBWidget)
	return m, cmd
}

func (m *GdbModel) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

	switch msg.Type {

	case tea.KeyEnter:
		//		text := m.input.Value()

		//		return m, func() tea.Msg {
		//			m.input.Reset()
		//			return core.SubmitMsg{Text: text}
		//		}

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
	//	m.input.Focus()

	return m, nil
}

func (m GdbModel) View() string {

	//	top := topStyle.Render("Vim-Style TUI")
	//	m.gdbWidget.SetWidth(m.state.Width - 4)
	//	m.gdbWidget.SetHeight(6)
	vpBox := paneStyle.Width(m.state.Width - 2).Height(m.state.Height - 12).Render(m.viewport.View())
	gdbWidget := paneStyle.Width(m.state.Width - 2).Height(6).Render(m.gdbWidget.View())
	cmdInputBox := cmdInputBoxStyle.Width(m.state.Width).Height(1).Render(m.cmdInputBox.View())

	return fmt.Sprintf("%s\n\n%s\n%s",
		vpBox,
		gdbWidget,
		cmdInputBox,
	)
}

func (m GdbModel) Update111(msg tea.Msg) (tea.Model, tea.Cmd) {

	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// --- Window size ---
	case tea.WindowSizeMsg:
		m.state.Width = msg.Width
		m.state.Height = msg.Height

	// --- Keyboard ---
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+d":
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			//	var updated tea.Model
			_, cmd = m.handleKey(msg)
			//	m = updated.(GdbModel) // ✔ נכון

			cmds = append(cmds, cmd)

			return m, tea.Batch(cmds...)
		}

	// --- Mode change ---
	case CancelCmdMode:
		m.state.Mode = app.NormalMode

	// --- Submit ---
	//case core.SubmitMsg:
	//	m.state.SubmitText(msg.Text)
	//	m.refreshViewport()

	// --- Core events ---
	case core.Event:
		switch msg.(type) {
		case core.Quit:
			return m, tea.Quit
		}

		nextEvents, _ := m.app.HandleEvent(msg)

		for _, e := range nextEvents {
			cmds = append(cmds, emitEvent(e))
		}

	}
	// =========================
	// --- CHILDREN UPDATES ---
	// =========================

	// --- GDB widget ---
	{
		//		var cmd tea.Cmd
		//	var updated tea.Model

		_, _ = m.gdbWidget.Update(msg)
		//	m.gdbWidget = updated.(*GDBWidget)

		//		cmds = append(cmds, cmd)
	}

	// --- viewport ---
	//	{
	//		var cmd tea.Cmd
	//		m.viewport, cmd = m.viewport.Update(msg)
	//		cmds = append(cmds, cmd)
	//	}

	return m, tea.Batch(cmds...)
}

func (m GdbModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.state.Width = msg.Width
		m.state.Height = msg.Height
	//	m.input.SetWidth(m.state.Width - 2)
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
		//	m.refreshViewport()
		return m, nil

	case core.GdbOutputMsg:
		update, _ := m.gdbWidget.Update(msg)
		m.gdbWidget = update.(*GDBWidget)

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
	//	var cmd tea.Cmd
	//	m.gdbWidget.Update(msg)

	//	m.viewport, cmd = m.viewport.Update(msg)
	return m, nil //cmd
}
