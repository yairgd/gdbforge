package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (c *searchCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onSearchTextChanged)
	platform.Subscribe(bus, c.onSearchSubmitted)
}

func (c *searchCtl) onSearchTextChanged(msg termui.SearchTextChangedMsg) {
	c.onCmdChange(msg.Text)
}

func (c *searchCtl) onSearchSubmitted(msg termui.SearchSubmittedMsg) {
	c.onCmdSubmit(msg.Pattern)
}

func (c *searchCtl) onCmdChange(text string) {
	h := c.host
	if h == nil || h.CmdWidget() == nil || h.CmdWidget().Kind() != termui.CmdKindSearch {
		return
	}
	host := c.target
	if host == nil {
		host = c.resolveHost()
		c.target = host
	}
	if host == nil {
		return
	}
	pat := ""
	runes := []rune(text)
	if len(runes) > 1 {
		pat = string(runes[1:])
	}
	host.SetSearchColor(h.State().SearchColor())
	host.SetSearchPattern(pat)
}

func (c *searchCtl) onCmdSubmit(pattern string) {
	h := c.host
	host := c.target
	if host == nil {
		host = c.resolveHost()
	}
	if host == nil || h == nil {
		return
	}
	host.SetSearchColor(h.State().SearchColor())
	host.CommitSearch(pattern)
	c.target = host // keep for n/N and */# on same pane
}

func (c *searchCtl) nextMatch() {
	h := c.host
	host := c.target
	if host == nil {
		host = c.resolveHost()
	}
	if host == nil || h == nil {
		return
	}
	host.SetSearchColor(h.State().SearchColor())
	if host.SearchNext() {
		h.RequestFrame()
	}
}

func (c *searchCtl) prevMatch() {
	h := c.host
	host := c.target
	if host == nil {
		host = c.resolveHost()
	}
	if host == nil || h == nil {
		return
	}
	host.SetSearchColor(h.State().SearchColor())
	if host.SearchPrev() {
		h.RequestFrame()
	}
}

// wordMatch implements vim * (dir>0) / # (dir<0): set pattern from the
// word under the caret (prefer caret over a stale selection), then jump
// next/prev. With no word, fall back to selection, then existing pattern.
//
// If the caret already sits on a highlight of the active pattern (e.g. after
// /46 landed on "46" inside "1052946"), only navigate — do not expand to the
// enclosing identifier.
func (c *searchCtl) wordMatch(dir int) {
	h := c.host
	if h == nil {
		return
	}
	// Always use the focused pane so */# follow the caret the user sees.
	host := c.resolveHost()
	if host == nil {
		return
	}
	host.SetSearchColor(h.State().SearchColor())

	if host.SearchPattern() != "" && c.cursorInMatch(host) {
		c.target = host
		if dir < 0 {
			_ = host.SearchPrev()
		} else {
			_ = host.SearchNext()
		}
		h.RequestFrame()
		return
	}

	// Prefer caret word so a leftover mouse/select mark does not lock */# onto
	// an old token when the user moves to a new word.
	pattern := c.wordAt(host)
	if pattern == "" {
		pattern = c.selectionAt(host)
	}
	if pattern != "" {
		host.CommitSearch(pattern)
		c.target = host
		c.clearSelection(host)
		if dir < 0 {
			_ = host.SearchPrev()
		} else {
			_ = host.SearchNext()
		}
		h.RequestFrame()
		return
	}
	if dir < 0 {
		c.prevMatch()
		return
	}
	c.nextMatch()
}

func (c *searchCtl) cursorInMatch(host termui.SearchHost) bool {
	if host == nil {
		return false
	}
	if x, ok := host.(interface{ CursorInSearchMatch() bool }); ok {
		return x.CursorInSearchMatch()
	}
	if vp := c.viewportOf(host); vp != nil {
		return vp.CursorInSearchMatch()
	}
	return false
}

func (c *searchCtl) hasActivePattern() bool {
	host := c.target
	if host == nil {
		host = c.resolveHost()
	}
	if host == nil {
		return false
	}
	return host.SearchPattern() != ""
}

func (c *searchCtl) wordAt(host termui.SearchHost) string {
	if host == nil {
		return ""
	}
	if w, ok := host.(interface{ WordAtCursor() string }); ok {
		return w.WordAtCursor()
	}
	return ""
}

func (c *searchCtl) selectionAt(host termui.SearchHost) string {
	if host == nil {
		return ""
	}
	vp := c.viewportOf(host)
	if vp == nil || !vp.HasSelection() {
		return ""
	}
	return strings.TrimSpace(vp.SelectedText())
}

func (c *searchCtl) clearSelection(host termui.SearchHost) {
	if vp := c.viewportOf(host); vp != nil {
		vp.ClearSelection()
	}
}

func (c *searchCtl) viewportOf(host termui.SearchHost) *termui.Viewport {
	switch t := host.(type) {
	case *termui.Viewport:
		return t
	case interface{ Viewport() *termui.Viewport }:
		return t.Viewport()
	default:
		return nil
	}
}

// resolveHost returns the /search target for the last active (focused)
// pane — the one with the green/blue status bar. Falls back to the active
// CodeWidget when the focused pane has no viewport.
func (c *searchCtl) resolveHost() termui.SearchHost {
	h := c.host
	if h == nil {
		return nil
	}
	if host := c.hostOf(h.FocusedWidget()); host != nil {
		return host
	}
	if cw := h.ActiveCodeWidget(); cw != nil {
		return cw
	}
	return nil
}

func (c *searchCtl) hostOf(w termui.Widget) termui.SearchHost {
	if w == nil {
		return nil
	}
	if host, ok := w.(termui.SearchHost); ok {
		return host
	}
	switch t := w.(type) {
	case *widgets.CodeWidget:
		return t
	case *widgets.AssemblyWidget:
		return t
	case *widgets.BreakpointWidget:
		return t.Viewport()
	case *widgets.ThreadWidget:
		return t.Viewport()
	case *widgets.CallStackWidget:
		return t.Viewport()
	case *widgets.FileListWidget:
		return t.Viewport()
	case *widgets.HelpWidget:
		return t.Viewport()
	case *widgets.GDBWidget:
		return t.Viewport()
	case *widgets.OutputWidget:
		return t.Viewport()
	case *termui.LoggerWidget:
		return t.Viewport()
	case interface{ Viewport() *termui.Viewport }:
		return t.Viewport()
	}
	return nil
}

// captureFocused stores the focused pane as the search target and applies
// the current search color. Used when entering '/' mode.
func (c *searchCtl) captureFocused() {
	h := c.host
	c.target = c.resolveHost()
	if c.target != nil && h != nil {
		c.target.SetSearchColor(h.State().SearchColor())
	}
}

// clearTarget drops the remembered search pane (e.g. entering ':' command mode).
func (c *searchCtl) clearTarget() {
	c.target = nil
}

// revertPreview undoes an uncommitted '/' preview while keeping the target
// so n/N and */# still work on a previously committed pattern.
func (c *searchCtl) revertPreview() {
	if c.target != nil {
		c.target.RevertSearch()
	}
}
