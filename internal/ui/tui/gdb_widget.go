package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yairgd/promptcore/internal/core"
)

type GDBWidget struct {
	buffer   *core.Buffer
	viewport core.Viewport

	inputBuf string
	cursor   int

	sendFunc func(string)
	sendRaw  func(string)

	width  int
	height int
}

func NewGDBWidget(sendFunc func(string), sendRawFunc func(string)) *GDBWidget {
	buf := core.NewBuffer()

	return &GDBWidget{
		buffer:   buf,
		viewport: core.Viewport{Height: 10},

		inputBuf: "",
		sendFunc: sendFunc,
		sendRaw:  sendRawFunc,

		width:  80,
		height: 20,
	}
}

func (m GDBWidget) Init() tea.Cmd {
	return nil
}

func (m *GDBWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	// --- GDB OUTPUT ---
	case core.GdbOutputMsg:
		m.buffer.AppendText(msg.Data)
		m.viewport.FollowBottom(m.buffer)

	// --- RESIZE ---
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// keep space for input line
		m.viewport.Height = m.height - 1

	// --- INPUT ---
	case tea.KeyMsg:
		switch msg.Type {

		case tea.KeyEnter:
			m.sendFunc(m.inputBuf)

			m.buffer.AppendText(m.inputBuf)
			m.viewport.FollowBottom(m.buffer)

			m.inputBuf = ""
			m.cursor = 0

		case tea.KeyBackspace:
			if m.cursor > 0 {
				m.inputBuf =
					m.inputBuf[:m.cursor-1] +
						m.inputBuf[m.cursor:]
				m.cursor--
			}

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyRight:
			if m.cursor < len(m.inputBuf) {
				m.cursor++
			}

		case tea.KeyUp:
			m.sendRaw("\x1b[A")

		case tea.KeyDown:
			m.sendRaw("\x1b[B")

		case tea.KeySpace:
			m.inputBuf =
				m.inputBuf[:m.cursor] +
					" " +
					m.inputBuf[m.cursor:]
			m.cursor++

		case tea.KeyRunes:
			r := string(msg.Runes)
			m.inputBuf =
				m.inputBuf[:m.cursor] +
					r +
					m.inputBuf[m.cursor:]
			m.cursor += len(r)
		}
	}

	return m, nil
}

func (m GDBWidget) View() string {

	// --- visible lines from buffer ---
	lines := m.viewport.VisibleLines(m.buffer)
	output := strings.Join(lines, "\n")

	// --- input with cursor ---
	left := m.inputBuf[:m.cursor]
	right := ""

	cursorChar := " "

	if m.cursor < len(m.inputBuf) {
		cursorChar = string(m.inputBuf[m.cursor])
		right = m.inputBuf[m.cursor+1:]
	}

	cursor := "\x1b[47m\x1b[30m" + cursorChar + "\x1b[0m"

	return output + left + cursor + right
}
