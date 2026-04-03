package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Mode int

const (
	InsertMode Mode = iota
	NormalMode
	CommandMode
)

var paneStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(0, 1)

var inputStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("205"))

var cmdStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("0")).  // black text
	Background(lipgloss.Color("10")). // green background
	Bold(true)

type model struct {
	mode         Mode
	input        textarea.Model
	viewport     viewport.Model
	lines        []string
	commandInput string
	width        int
	height       int
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Write multi-line text here..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(6)
	ta.ShowLineNumbers = true

	vp := viewport.New(80, 20)

	return model{
		mode:     InsertMode,
		input:    ta,
		viewport: vp,
		lines:    []string{},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 12

		m.input.SetWidth(msg.Width)
		m.input.SetHeight(6)

	case tea.KeyMsg:

		switch m.mode {

		// ================= INSERT MODE =================
		case InsertMode:

			if msg.String() == "esc" {
				m.mode = NormalMode
				m.input.Blur()
				return m, nil
			}

			if msg.String() == "ctrl+s" {
				text := m.input.Value()
				if text != "" {
					m.lines = append(m.lines, text)
					m.input.Reset()
					m.refreshViewport_1()
				}
				return m, nil
			}

			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		// ================= NORMAL MODE =================
		case NormalMode:

			switch msg.String() {

			case ":":
				m.mode = CommandMode
				m.commandInput = ""
				return m, nil

			case "enter":
				text := m.input.Value()
				if strings.TrimSpace(text) != "" {
					m.lines = append(m.lines, text)
					m.refreshViewport_1()
					m.input.Reset()
				}
				return m, nil

			default:
				// any other key returns to insert mode
				m.mode = InsertMode
				m.input.Focus()
				return m, nil
			}

		// ================= COMMAND MODE =================
		case CommandMode:

			switch msg.Type {

			case tea.KeyEsc:
				m.mode = NormalMode
				m.commandInput = ""
				return m, nil

			case tea.KeyEnter:
				cmd := m.executeCommand()
				m.mode = NormalMode
				m.commandInput = ""
				return m, cmd

			case tea.KeyBackspace:
				if len(m.commandInput) > 0 {
					m.commandInput = m.commandInput[:len(m.commandInput)-1]
				}
				return m, nil

			case tea.KeyRunes:
				m.commandInput += string(msg.Runes)
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) executeCommand() tea.Cmd {
	cmd := strings.TrimSpace(m.commandInput)

	switch cmd {

	case "q":
		return tea.Quit

	case "hello":
		m.lines = append(m.lines, "🤖 hi this is hello command")
		m.refreshViewport_1()
		return nil

	case "clear":
		m.lines = []string{}
		m.refreshViewport_1()
		return nil

	default:
		m.lines = append(m.lines, "Unknown command: "+cmd)
		m.refreshViewport_1()
		return nil
	}
}

func (m *model) refreshViewport_1() {
	content := ""
	for i, l := range m.lines {
		content += fmt.Sprintf("[%d]\n%s\n\n", i+1, l)
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m model) View() string {

	top := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10")).
		Render("Vim-Style TUI | ESC → NORMAL | : → COMMAND | Ctrl+S send")

	vpBox := paneStyle.
		Width(m.width).
		Render(m.viewport.View())

	inputBox := inputStyle.
		Width(m.width).
		Render(m.input.View())

	// Command line always visible
	var cmdLine string

	switch m.mode {

	case CommandMode:
		cmdLine = cmdStyle.
			Width(m.width).
			Render(":" + m.commandInput)

	case NormalMode:
		cmdLine = cmdStyle.
			Width(m.width).
			Render("-- NORMAL --")

	case InsertMode:
		cmdLine = cmdStyle.
			Width(m.width).
			Render("-- INSERT --")
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n%s",
		top,
		vpBox,
		inputBox,
		cmdLine,
	)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
