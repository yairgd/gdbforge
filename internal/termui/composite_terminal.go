package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

const defaultHostPrefix = "[lua] "

// CompositeTerminal wraps one xterm buffer with one bidirectional PTY and
// optional inject-only host lines (e.g. lua.print on the IO pane).
type CompositeTerminal struct {
	ctl        *TerminalController
	tty        *ptyx.TTY
	cancelPump func()
	hostPrefix string
	keys       *commands.KeyBindingRegistry
}

// NewCompositeTerminal creates an empty terminal emulator.
func NewCompositeTerminal(cols, rows, scrollback int) *CompositeTerminal {
	return newCompositeTerminal(cols, rows, scrollback, defaultHostPrefix)
}

// NewCompositeTerminalWithPrefix is like NewCompositeTerminal with a custom host prefix.
func NewCompositeTerminalWithPrefix(cols, rows, scrollback int, hostPrefix string) *CompositeTerminal {
	return newCompositeTerminal(cols, rows, scrollback, hostPrefix)
}

func newCompositeTerminal(cols, rows, scrollback int, hostPrefix string) *CompositeTerminal {
	c := &CompositeTerminal{
		ctl:        NewTerminalController(cols, rows, scrollback),
		hostPrefix: hostPrefix,
	}
	c.initKeyBindings()
	return c
}

func (c *CompositeTerminal) Controller() *TerminalController {
	if c == nil {
		return nil
	}
	return c.ctl
}

// AttachTTY wires PTY bytes in and keyboard out. nil detaches.
func (c *CompositeTerminal) AttachTTY(tty *ptyx.TTY, opts WireTTYOpts) {
	if c == nil {
		return
	}
	c.detach()
	if tty == nil {
		WireTTYInput(nil, c.ctl)
		return
	}
	c.tty = tty
	c.cancelPump = WireTTY(tty, c.ctl, opts)
	WireTTYInput(tty, c.ctl)
}

// WriteHostLine injects a host line with the configured prefix.
func (c *CompositeTerminal) WriteHostLine(s string) {
	if c == nil || s == "" {
		return
	}
	prefix := c.hostPrefix
	if prefix == "" {
		prefix = defaultHostPrefix
	}
	_ = c.ctl.WriteString(prefix + s + "\r\n")
}

// WriteRaw injects raw terminal bytes (e.g. startup boot capture).
func (c *CompositeTerminal) WriteRaw(data string) {
	if c == nil || data == "" {
		return
	}
	_ = c.ctl.WriteString(data)
}

// Resize resizes the emulator and attached PTY winsize.
func (c *CompositeTerminal) Resize(cols, rows int) error {
	if c == nil {
		return nil
	}
	if err := c.ctl.Resize(cols, rows); err != nil {
		return err
	}
	if c.tty != nil {
		return c.tty.SetSize(uint16(rows), uint16(cols))
	}
	return nil
}

func (c *CompositeTerminal) Detach() {
	if c == nil {
		return
	}
	c.detach()
}

func (c *CompositeTerminal) detach() {
	if c.cancelPump != nil {
		c.cancelPump()
		c.cancelPump = nil
	}
	c.tty = nil
	WireTTYInput(nil, c.ctl)
}

// Close detaches and disposes the emulator.
func (c *CompositeTerminal) Close() {
	if c == nil {
		return
	}
	c.detach()
	c.ctl.Close()
}

// Paint draws the terminal onto a canvas. When paintCursor is true, an inverse
// block is drawn at the xterm cursor (focused pane caret).
func (c *CompositeTerminal) Paint(cv Canvas, paintCursor bool) {
	if c == nil || c.ctl == nil {
		return
	}
	_ = c.Resize(cv.W(), cv.H())
	cols, rows := c.ctl.Size()
	if cols > cv.W() {
		cols = cv.W()
	}
	if rows > cv.H() {
		rows = cv.H()
	}
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			cell := c.ctl.Cell(x, y)
			cv.SetContent(x, y, cell.Rune, cell.Style)
		}
	}
	if !paintCursor {
		return
	}
	cx, cy := c.ctl.Cursor()
	if cx < 0 || cy < 0 || cx >= cols || cy >= rows {
		return
	}
	under := c.ctl.Cell(cx, cy).Rune
	if under == 0 {
		under = ' '
	}
	NewInverseCursor().Paint(cv, cx, cy, under)
}

func (c *CompositeTerminal) initKeyBindings() {
	c.keys = commands.NewKeyBindingRegistry()
	send := func(seq string) func(...any) {
		return func(...any) {
			if c.ctl != nil {
				_ = c.ctl.SendInput([]byte(seq))
			}
		}
	}
	bind := func(name, seq string, bindings ...string) {
		c.keys.Bind(commands.NewCommand(name, send(seq)), bindings...)
	}

	bind("enter", "\r", "<Enter>", "<C-m>")
	bind("backspace", "\x7f", "<Backspace>")
	bind("tab", "\t", "<Tab>", "<C-i>")
	bind("esc", "\x1b", "<Esc>")
	bind("up", "\x1b[A", "<Up>")
	bind("down", "\x1b[B", "<Down>")
	bind("left", "\x1b[D", "<Left>")
	bind("right", "\x1b[C", "<Right>")
	bind("home", "\x1b[H", "<Home>")
	bind("end", "\x1b[F", "<End>")
	bind("delete", "\x1b[3~", "<Delete>")
	bind("page-up", "\x1b[5~", "<PgUp>", "<C-b>")
	bind("page-down", "\x1b[6~", "<PgDn>", "<C-f>")
	bind("interrupt", "\x03", "<C-c>")
	bind("eof", "\x04", "<C-d>")
	bind("suspend", "\x1a", "<C-z>")
	bind("refresh", "\x0c", "<C-l>")
}

// HandleKey forwards a key event to the PTY via the key-binding trie; plain
// runes fall through when no chord matched.
func (c *CompositeTerminal) HandleKey(ev *tcell.EventKey) bool {
	if c == nil || c.ctl == nil || ev == nil {
		return false
	}
	key, ok := platform.KeyFromEvent(ev)
	if !ok {
		return false
	}
	if c.keys != nil {
		completed, handled := c.keys.HandleKey(key)
		if handled {
			if !completed {
				return c.keys.InPartial()
			}
			return true
		}
	}
	if ev.Key() == tcell.KeyRune {
		return c.ctl.SendInput([]byte(string(ev.Rune()))) == nil
	}
	// Linux often emits KeyBackspace distinct from KeyBackspace2.
	if ev.Key() == tcell.KeyBackspace {
		return c.ctl.SendInput([]byte("\x7f")) == nil
	}
	return false
}

func (c *CompositeTerminal) ResetKeyPartial() {
	if c != nil && c.keys != nil {
		c.keys.ResetPartial()
	}
}
