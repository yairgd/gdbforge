package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	xterm "github.com/gitpod-io/xterm-go"
)

func (c *CompositeTerminal) handleScrollKey(ev *tcell.EventKey) bool {
	if c == nil || ev == nil {
		return false
	}
	switch ev.Key() {
	case tcell.KeyPgUp:
		c.scrollPageUp()
		return true
	case tcell.KeyPgDn:
		c.scrollPageDown()
		return true
	case tcell.KeyHome:
		c.scrollHome()
		return true
	case tcell.KeyEnd:
		c.scrollEnd()
		return true
	default:
		return false
	}
}

func (c *CompositeTerminal) scrollPageUp() {
	if c == nil || c.ctl == nil {
		return
	}
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		term.ScrollPages(-1)
	})
}

func (c *CompositeTerminal) scrollPageDown() {
	if c == nil || c.ctl == nil {
		return
	}
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		term.ScrollPages(1)
	})
}

func (c *CompositeTerminal) scrollHome() {
	if c == nil || c.ctl == nil {
		return
	}
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		term.ScrollToTop()
	})
}

func (c *CompositeTerminal) scrollEnd() {
	if c == nil || c.ctl == nil {
		return
	}
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		term.ScrollToBottom()
	})
}

// AtBottom reports whether the viewport is pinned to the live tail.
func (c *CompositeTerminal) AtBottom() bool {
	return c.atBottom()
}

// ScrollToBottom jumps the viewport to the live tail.
func (c *CompositeTerminal) ScrollToBottom() {
	c.scrollEnd()
}

func (c *CompositeTerminal) atBottom() bool {
	if c == nil || c.ctl == nil {
		return true
	}
	atBottom := true
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		buf := term.Buffer()
		atBottom = buf.YDisp >= buf.YBase
	})
	return atBottom
}

// handleEnterScrollSnap scrolls to the live tail when Enter is pressed while
// the user has scrolled up into scrollback (does not send Enter to the PTY).
func (c *CompositeTerminal) handleEnterScrollSnap(ev *tcell.EventKey) bool {
	if c == nil || ev == nil {
		return false
	}
	switch ev.Key() {
	case tcell.KeyEnter, tcell.KeyCtrlM, tcell.KeyCtrlJ:
	default:
		return false
	}
	if c.atBottom() {
		return false
	}
	c.scrollEnd()
	return true
}
