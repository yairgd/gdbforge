package termui

import (
	"strings"
	"time"

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

// Middle-click paste edge detection (UI thread only).
var (
	middleButtonHeld bool
	middlePasteAt    time.Time
)

const middlePasteDebounce = 120 * time.Millisecond

// isMiddlePaste reports the rising edge of a middle-button click.
// Motion while held must not paste again — terminals with mouse reporting
// send many ButtonMiddle events per physical click.
func isMiddlePaste(e *tcell.EventMouse) bool {
	if e == nil {
		return false
	}
	down := e.Buttons()&tcell.ButtonMiddle != 0
	if !down {
		middleButtonHeld = false
		return false
	}
	if middleButtonHeld {
		return false
	}
	now := e.When()
	if now.IsZero() {
		now = time.Now()
	}
	// Some terminals emit two press events without a clean release between.
	if !middlePasteAt.IsZero() && now.Sub(middlePasteAt) < middlePasteDebounce {
		middleButtonHeld = true
		return false
	}
	middleButtonHeld = true
	middlePasteAt = now
	return true
}

// resetMiddlePasteState is for tests.
func resetMiddlePasteState() {
	middleButtonHeld = false
	middlePasteAt = time.Time{}
}
