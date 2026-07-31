package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (c *searchCtl) onCmdChange(text string) {
	a := c.app
	if a.cmdWidget == nil || a.cmdWidget.Kind() != termui.CmdKindSearch {
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
	host.SetSearchColor(a.State().SearchColor())
	host.SetSearchPattern(pat)
}

func (c *searchCtl) onCmdSubmit(pattern string) {
	host := c.target
	if host == nil {
		host = c.resolveHost()
	}
	if host == nil {
		return
	}
	host.SetSearchColor(c.app.State().SearchColor())
	host.CommitSearch(pattern)
	c.target = host // keep for n/N and */# on same pane
}

func (c *searchCtl) nextMatch() {
	host := c.target
	if host == nil {
		host = c.resolveHost()
	}
	if host == nil {
		return
	}
	host.SetSearchColor(c.app.State().SearchColor())
	if host.SearchNext() {
		c.app.RequestFrame()
	}
}

func (c *searchCtl) prevMatch() {
	host := c.target
	if host == nil {
		host = c.resolveHost()
	}
	if host == nil {
		return
	}
	host.SetSearchColor(c.app.State().SearchColor())
	if host.SearchPrev() {
		c.app.RequestFrame()
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
	a := c.app
	// Always use the focused pane so */# follow the caret the user sees.
	host := c.resolveHost()
	if host == nil {
		return
	}
	host.SetSearchColor(a.State().SearchColor())

	if host.SearchPattern() != "" && c.cursorInMatch(host) {
		c.target = host
		if dir < 0 {
			_ = host.SearchPrev()
		} else {
			_ = host.SearchNext()
		}
		a.RequestFrame()
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
		a.RequestFrame()
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
	a := c.app
	if host := c.hostOf(a.focusedWidget()); host != nil {
		return host
	}
	if cw := a.activeCodeWidget(); cw != nil {
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
	c.target = c.resolveHost()
	if c.target != nil {
		c.target.SetSearchColor(c.app.State().SearchColor())
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
