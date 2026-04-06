package uitcell

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"

	//	"github.com/muesli/ansi"
	"strings"
)

//////////////////////////
// GDB WIDGET
//////////////////////////

type GDBWidget struct {
	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf string
	Cursor   int

	Debugger core.Debugger
	Width    int
	Height   int
}

func NewGDBWidget(debugger core.Debugger) *GDBWidget {
	buf := core.NewBuffer()

	return &GDBWidget{
		Buffer:   buf,
		Viewport: core.Viewport{Height: 10},
		Debugger: debugger,
		Cursor:   0,
	}
}

//////////////////////////
// PUBLIC API
//////////////////////////

func (m *GDBWidget) SetSize(w, h int) {
	m.Width = w
	m.Height = h
	m.Viewport.Height = h - 1
}

func (m *GDBWidget) OnGDBOutput(data string) {
	m.Buffer.AppendText(data)
	m.Viewport.FollowBottom(m.Buffer)
}

func DrawANSIText(screen tcell.Screen, x, y int, text string, baseStyle tcell.Style, maxWidth int) {

	style := baseStyle
	posX := x

	i := 0
	for i < len(text) {

		// --- ESC sequence ---
		if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '[' {

			j := i + 2
			for j < len(text) && text[j] < '@' || text[j] > '~' {
				j++
			}

			if j < len(text) {
				finalChar := text[j]

				// 🎯 רק SGR (m) מטופל
				if finalChar == 'm' {
					seq := text[i+2 : j]
					codes := strings.Split(seq, ";")

					for _, c := range codes {
						switch c {

						case "0":
							style = baseStyle

						case "1":
							style = style.Bold(true)

						case "22":
							style = style.Bold(false)

						case "30":
							style = style.Foreground(tcell.ColorBlack)
						case "31":
							style = style.Foreground(tcell.ColorMaroon)
						case "32":
							style = style.Foreground(tcell.ColorGreen)
						case "33":
							style = style.Foreground(tcell.ColorYellow)
						case "34":
							style = style.Foreground(tcell.ColorBlue)
						case "35":
							style = style.Foreground(tcell.ColorPurple)
						case "36":
							style = style.Foreground(tcell.ColorTeal)
						case "37":
							style = style.Foreground(tcell.ColorWhite)
						}
					}
				}

				// ❗ כל השאר — מתעלמים
				i = j + 1
				continue
			}
		}
		// --- normal char ---
		ch := rune(text[i])

		if posX < maxWidth {
			screen.SetContent(posX, y, ch, nil, style)
			posX++
		}

		i++
	}
}

//////////////////////////
// EVENTS
//////////////////////////

func (m *GDBWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventResize:
		m.SetSize(e.Size())

	case *tcell.EventKey:

		switch e.Key() {

		case tcell.KeyCtrlC:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x03") // SIGINT
			}
			return
		case tcell.KeyCtrlD:
			if m.Debugger.Send != nil {
				m.Debugger.Send("q\n") // SIGINT
			}
		//	return

		case tcell.KeyEnter:
			if m.Debugger.Send != nil {
				m.Debugger.Send(m.InputBuf)
			}

			m.Buffer.AppendText(m.InputBuf)
			m.Viewport.FollowBottom(m.Buffer)

			m.InputBuf = ""
			m.Cursor = 0

		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if m.Cursor > 0 {
				m.InputBuf = m.InputBuf[:m.Cursor-0-1] + m.InputBuf[m.Cursor-0:]
				m.Cursor--
			}

		case tcell.KeyLeft:
			if m.Cursor > 0 {
				m.Cursor--
			}

		case tcell.KeyRight:
			if m.Cursor < len(m.InputBuf) {
				m.Cursor++
			}

		case tcell.KeyUp:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x1b[A")
			}

		case tcell.KeyDown:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x1b[B")
			}

		case tcell.KeyRune:
			r := string(e.Rune())
			m.InputBuf =
				m.InputBuf[:m.Cursor-0] +
					r +
					m.InputBuf[m.Cursor-0:]
			m.Cursor += len(r)
		}
	}
}

//////////////////////////
// DRAW
//////////////////////////

func (m *GDBWidget) Draw(screen tcell.Screen) {

	screen.Clear()
	style := tcell.StyleDefault

	// --- OUTPUT ---
	lines := m.Viewport.VisibleLines(m.Buffer)

	for y, line := range lines {
		for x, _ := range line {
			if x >= m.Width {
				break
			}
			DrawANSIText(screen, 0, y, line, style, m.Width)
			//	screen.SetContent(x, y, ch, nil, style)
		}
	}

	// --- INPUT LINE ---
	inputY := m.Height - 2

	for x, ch := range m.InputBuf {
		if x >= m.Width {
			break
		}
		screen.SetContent(x+6, inputY, ch, nil, style)
	}

	// --- CURSOR ---
	screen.ShowCursor(m.Cursor+6, inputY)

	screen.Show()
}
