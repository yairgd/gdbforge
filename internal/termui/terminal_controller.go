package termui

import (
	"errors"
	"sync"

	tcell "github.com/gdamore/tcell/v2"
	xterm "github.com/gitpod-io/xterm-go"
)

type Color struct {
	R uint8
	G uint8
	B uint8
}
type TerminalCell struct {
	Rune  rune
	Style tcell.Style
}

func convertFgColor(cell *xterm.CellData) tcell.Color {
	switch {
	case cell.IsFgRGB():
		rgb := xterm.ToColorRGB(cell.Fg)

		return tcell.NewRGBColor(
			int32(rgb[0]),
			int32(rgb[1]),
			int32(rgb[2]),
		)

	case cell.IsFgPalette():
		return tcell.PaletteColor(cell.GetFgColor())

	default:
		return tcell.ColorDefault
	}
}

func convertBgColor(cell *xterm.CellData) tcell.Color {
	switch {
	case cell.IsBgRGB():
		rgb := xterm.ToColorRGB(cell.Bg)

		return tcell.NewRGBColor(
			int32(rgb[0]),
			int32(rgb[1]),
			int32(rgb[2]),
		)

	case cell.IsBgPalette():
		return tcell.PaletteColor(cell.GetBgColor())

	default:
		return tcell.ColorDefault
	}
}

func (c *TerminalController) Cell(x, y int) TerminalCell {
	c.mu.RLock()
	defer c.mu.RUnlock()

	empty := TerminalCell{
		Rune:  ' ',
		Style: tcell.StyleDefault,
	}

	if c.term == nil {
		return empty
	}

	if x < 0 || x >= c.term.Cols() ||
		y < 0 || y >= c.term.Rows() {
		return empty
	}

	buf := c.term.Buffer()

	line := buf.Lines.Get(buf.YDisp + y)
	if line == nil {
		return empty
	}

	cell := xterm.NewCellData()
	line.LoadCell(x, cell)

	r := ' '

	if chars := cell.GetChars(); chars != "" {
		for _, ch := range chars {
			r = ch
			break
		}
	}

	style := tcell.StyleDefault.
		Foreground(convertFgColor(cell)).
		Background(convertBgColor(cell)).
		Bold(cell.IsBold() != 0).
		Underline(cell.IsUnderline() != 0)

	if cell.IsItalic() != 0 {
		style = style.Italic(true)
	}

	if cell.IsStrikethrough() != 0 {
		style = style.StrikeThrough(true)
	}

	if cell.IsInverse() != 0 {
		style = style.Reverse(true)
	}

	return TerminalCell{
		Rune:  r,
		Style: style,
	}
}

type InputHandler func([]byte) error

type TerminalController struct {
	mu sync.RWMutex

	term *xterm.Terminal

	inputHandler InputHandler

	closed bool
}

func NewTerminalController(
	cols int,
	rows int,
	scrollback int,
) *TerminalController {

	term := xterm.New(
		xterm.WithCols(cols),
		xterm.WithRows(rows),
		xterm.WithScrollback(scrollback),
	)

	return &TerminalController{
		term: term,
	}
}

// Write feeds raw terminal bytes into the emulator.
//
// Examples:
//
//	"hello\n"
//	"\x1b[31mred\x1b[0m"
//	"\x1b[2J"
//	"\x1b[10;20H"
func (c *TerminalController) Write(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("terminal controller is closed")
	}

	c.term.Write(data)

	return nil
}

func (c *TerminalController) WriteString(s string) error {
	return c.Write([]byte(s))
}

// SetInputHandler connects terminal keyboard output
// to some external destination.
//
// Examples:
//
//	PTY
//	UART
//	socket
//	nil for read-only terminal
func (c *TerminalController) SetInputHandler(handler InputHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.inputHandler = handler
}

// SendInput sends user-generated input to the external side.
//
// This does NOT write characters directly to the terminal screen.
// Normally the remote application echoes them back if terminal echo
// is enabled.
func (c *TerminalController) SendInput(data []byte) error {
	c.mu.RLock()

	if c.closed {
		c.mu.RUnlock()
		return errors.New("terminal controller is closed")
	}

	handler := c.inputHandler

	c.mu.RUnlock()

	if handler == nil {
		return nil
	}

	return handler(data)
}

func (c *TerminalController) SendString(s string) error {
	return c.SendInput([]byte(s))
}

func (c *TerminalController) Resize(cols, rows int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("terminal controller is closed")
	}

	c.term.Resize(cols, rows)

	return nil
}

func (c *TerminalController) Size() (cols, rows int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, 0
	}

	return c.term.Cols(), c.term.Rows()
}

func (c *TerminalController) Cursor() (x, y int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, 0
	}

	return c.term.CursorX(), c.term.CursorY()
}

// WithTerminal gives the viewer temporary read access
// to the xterm state.
//
// The callback runs while the read lock is held.
func (c *TerminalController) WithTerminal(
	fn func(*xterm.Terminal),
) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return
	}

	fn(c.term)
}

func (c *TerminalController) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true

	c.term.Dispose()
	c.term = nil
	c.inputHandler = nil
}
