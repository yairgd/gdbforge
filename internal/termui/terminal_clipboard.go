package termui

// TerminalClipboard remembers clipboard callbacks across CompositeTerminal
// recreation (e.g. OutputWidget.Clear replaces the inner term).
type TerminalClipboard struct {
	io ClipboardIO
}

func (t *TerminalClipboard) Set(io ClipboardIO) {
	if t == nil {
		return
	}
	t.io = io
}

func (t *TerminalClipboard) Apply(c *CompositeTerminal) {
	if t == nil || c == nil {
		return
	}
	c.SetClipboard(t.io)
}
