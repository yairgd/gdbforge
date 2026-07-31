package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// completionHost is the narrow surface completionCtl needs from the composition
// root. DebuggerApp implements it; completionCtl must not depend on *DebuggerApp.
type completionHost interface {
	GDBWidget() *widgets.GDBWidget
	CmdWidget() *termui.CmdWidget
	Backend() backend.Backend
	Session() core.Session
	State() *platform.AppState
	Mode() platform.Mode
	SetMode(mode platform.Mode)
	IsDLVConfirming() bool
	PublishCompletion(msg termui.CompletionMsg)
	RequestFrame()
}

// completionCtl owns the wildmenu domain: candidate menu, its chrome view, and
// the GDB / cmdline completion queries. Mode entry decisions stay on
// DebuggerApp (handleCompletionKey); the ctl owns the domain.
type completionCtl struct {
	host completionHost
	menu *termui.CompletionMenu
	view termui.CompletionView
	bar  *termui.CompletionBarWidget // concrete chrome; also CompletionView
	// forGDB is true while ModeCompletion is driven by GDB Tab
	// (apply/cancel return to insert mode instead of command mode).
	forGDB bool
}

// attach takes ownership of the wildmenu model and its chrome widget.
func (c *completionCtl) attach(menu *termui.CompletionMenu, bar *termui.CompletionBarWidget) {
	c.menu = menu
	c.bar = bar
	c.view = bar
}

// onMsg applies Tab results to the CompletionMenu and syncs the view.
func (c *completionCtl) onMsg(msg termui.CompletionMsg) {
	if c.menu == nil {
		return
	}
	c.menu.Set(msg.Names)
	c.syncView()
}

func (c *completionCtl) syncView() {
	if c.view == nil {
		return
	}
	if c.menu == nil || !c.menu.Active() {
		c.view.Clear()
		return
	}
	names, sel := c.menu.Snapshot()
	c.view.SetItems(names, sel)
}

func (c *completionCtl) clear() {
	if c.menu != nil {
		c.menu.Clear()
	}
	if c.view != nil {
		c.view.Clear()
	}
}

func (c *completionCtl) active() bool {
	return c.menu != nil && c.menu.Active()
}

// hasMenu reports whether a wildmenu model exists (nil before InitB).
func (c *completionCtl) hasMenu() bool { return c != nil && c.menu != nil }

// isForGDB reports whether the wildmenu is driven by GDB Tab.
func (c *completionCtl) isForGDB() bool { return c != nil && c.forGDB }

func (c *completionCtl) setForGDB(v bool) { c.forGDB = v }

// useGDBInput reports whether wildmenu edits should drive the GDB input line.
// Prefer cmdline when it is active so a stuck forGDB flag cannot route :b
// editing into MI -complete.
func (c *completionCtl) useGDBInput() bool {
	h := c.host
	if h == nil || !c.forGDB || h.GDBWidget() == nil {
		return false
	}
	cmd := h.CmdWidget()
	return cmd == nil || !cmd.Active()
}

// move steps the wildmenu selection and repaints the chrome.
func (c *completionCtl) move(delta int) {
	if c.menu == nil {
		return
	}
	c.menu.Move(delta)
	c.syncView()
}

// applySelected inserts the highlighted candidate into the GDB or cmdline input.
func (c *completionCtl) applySelected() {
	h := c.host
	if h == nil || c.menu == nil {
		return
	}
	name := c.menu.Selected()
	if name == "" {
		return
	}
	if c.useGDBInput() {
		cur := h.GDBWidget().InputText()
		h.GDBWidget().ApplyCompletion(gdb.WithCompletionSpace(gdb.ApplyMenuChoice(cur, name)))
		return
	}
	if cmd := h.CmdWidget(); cmd != nil {
		cmd.ApplyCompletion(name)
	}
}

// leaveMode closes the wildmenu and returns to the mode that opened it.
func (c *completionCtl) leaveMode() {
	h := c.host
	c.clear()
	if h == nil {
		c.forGDB = false
		return
	}
	if c.forGDB {
		c.forGDB = false
		h.SetMode(platform.ModeInsert)
		return
	}
	c.forGDB = false
	h.SetMode(platform.ModeCommand)
}

// gdbTabComplete runs completion for the GDB/Delve input line and feeds the same
// CompletionMsg / wildmenu path used by cmdline trie Completer.
func (c *completionCtl) gdbTabComplete() {
	h := c.host
	if h == nil || h.GDBWidget() == nil {
		return
	}
	text := h.GDBWidget().InputText()
	if h.Backend() == nil {
		return
	}
	if h.IsDLVConfirming() {
		return
	}
	res := h.Backend().Complete(h.Session(), h.State(), text)

	// Expand to longest common prefix when it grows the line.
	if res.Completion != "" && res.Completion != text {
		h.GDBWidget().ApplyCompletion(res.Completion)
		text = res.Completion
	}

	names := res.Matches
	if len(names) == 0 && res.Completion != "" {
		names = []string{res.Completion}
	}
	c.publishGDBMenu(text, names)

	switch len(names) {
	case 0:
		// nothing
	case 1:
		// Unique match — no further completions for this word; add a trailing space.
		h.GDBWidget().ApplyCompletion(gdb.WithCompletionSpace(names[0]))
		c.clear()
	default:
		c.forGDB = true
		h.SetMode(platform.ModeCompletion)
	}
	h.RequestFrame()
}

// refreshGDBMenu re-runs -complete for the current GDB input and replaces the
// wildmenu. Does not apply LCP or unique matches (typing owns the line).
// Tab/arrows only move selection and must not call this.
//
// Stay in ModeCompletion across 0/1-match re-queries so further typing and
// backspace keep refreshing. Leaving on ≤1 made small candidate sets die as
// soon as the list narrowed (or -complete briefly returned empty).
func (c *completionCtl) refreshGDBMenu() {
	h := c.host
	if h == nil || h.GDBWidget() == nil {
		c.leaveMode()
		return
	}
	text := h.GDBWidget().InputText()
	if strings.TrimSpace(text) == "" {
		c.leaveMode()
		return
	}
	if h.Backend() == nil {
		c.leaveMode()
		return
	}
	if h.IsDLVConfirming() {
		c.leaveMode()
		return
	}
	res := h.Backend().Complete(h.Session(), h.State(), text)
	names := res.Matches
	if len(names) == 0 && res.Completion != "" && res.Completion != text {
		names = []string{res.Completion}
	}
	c.publishGDBMenu(text, names)
	c.forGDB = true
	if h.Mode() != platform.ModeCompletion {
		h.SetMode(platform.ModeCompletion)
	}
}

func (c *completionCtl) publishGDBMenu(text string, names []string) {
	h := c.host
	if h == nil {
		return
	}
	menu := gdb.MenuNames(text, names)
	// After file:, attach signatures from -symbol-info-functions
	// ("foo" → "foo(int, char *)"); apply still inserts bare name.
	if h.Backend() != nil {
		menu = h.Backend().EnrichLinespecMenu(text, menu, h.Session(), h.State())
	}
	h.PublishCompletion(termui.CompletionMsg{
		Input: text,
		Token: text,
		Names: menu,
	})
}

// refreshCmdMenu re-syncs the cmdline parser and replaces the wildmenu.
func (c *completionCtl) refreshCmdMenu() {
	h := c.host
	if h == nil {
		return
	}
	cmd := h.CmdWidget()
	if cmd == nil || !cmd.Active() {
		c.leaveMode()
		return
	}
	names := cmd.CompletionNames()
	h.PublishCompletion(termui.CompletionMsg{
		Input: cmd.Text(),
		Token: cmd.Text(),
		Names: names,
	})
	if len(names) <= 1 {
		c.leaveMode()
		return
	}
	if h.Mode() != platform.ModeCompletion {
		h.SetMode(platform.ModeCompletion)
	}
}
