package termui

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
)

// ClipboardIO is the shared copy/paste bridge used by Viewport-backed widgets
// and single-line editors (CmdWidget, ConsolePane input).
type ClipboardIO struct {
	Copy func(text string)
	// Paste reads CLIPBOARD (Ctrl+V).
	Paste func() string
	// PastePrimary reads X11 PRIMARY (middle-click). Optional; falls back to Paste.
	PastePrimary func() string
}

func (c *ClipboardIO) copyText(text string) {
	if c == nil || c.Copy == nil || text == "" {
		return
	}
	c.Copy(text)
}

func (c *ClipboardIO) pasteText() string {
	if c == nil || c.Paste == nil {
		return ""
	}
	return c.Paste()
}

// pastePrimaryText prefers PRIMARY (middle-click), then CLIPBOARD.
func (c *ClipboardIO) pastePrimaryText() string {
	if c != nil && c.PastePrimary != nil {
		if t := c.PastePrimary(); t != "" {
			return t
		}
	}
	return c.pasteText()
}

// isPasteKey reports Ctrl+V (including terminal variants that send Ctrl+rune).
func isPasteKey(e *tcell.EventKey) bool {
	if e.Key() == tcell.KeyCtrlV {
		return true
	}
	if e.Modifiers()&tcell.ModCtrl == 0 || e.Key() != tcell.KeyRune {
		return false
	}
	return e.Rune() == 'v' || e.Rune() == 'V'
}

// isCopyKey reports Ctrl+C / Ctrl+X (including Ctrl+rune variants).
func isCopyCutKey(e *tcell.EventKey) bool {
	if e.Key() == tcell.KeyCtrlC || e.Key() == tcell.KeyCtrlX {
		return true
	}
	if e.Modifiers()&tcell.ModCtrl == 0 || e.Key() != tcell.KeyRune {
		return false
	}
	switch e.Rune() {
	case 'c', 'C', 'x', 'X':
		return true
	}
	return false
}

// isConsoleClipboardKey is Ctrl+C/X/V for routing to the scrollback viewport.
func isConsoleClipboardKey(e *tcell.EventKey) bool {
	return isPasteKey(e) || isCopyCutKey(e)
}

// firstLinePaste keeps a single-line paste (cmdline / console input).
func firstLinePaste(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	return text
}

// isMiddlePaste reports a middle-button press (Linux primary-selection paste).
func isMiddlePaste(e *tcell.EventMouse) bool {
	return e != nil && e.Buttons()&tcell.ButtonMiddle != 0
}
