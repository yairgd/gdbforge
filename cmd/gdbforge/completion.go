package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// completionHost is the narrow surface completionCtl needs from the composition
// root. DebuggerApp implements it; completionCtl must not depend on *DebuggerApp.
type completionHost interface {
	GDBWidget() *widgets.GDBWidget
	LuaConsoleWidget() *widgets.LuaConsoleWidget
	LuaGdbforgeComplete(text string) (newLine string, matches []string)
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
	// forLua is true while ModeCompletion is driven by Lua REPL Tab.
	forLua bool
}

// attach takes ownership of the wildmenu model and its chrome widget.
func (c *completionCtl) attach(menu *termui.CompletionMenu, bar *termui.CompletionBarWidget) {
	c.menu = menu
	c.bar = bar
	c.view = bar
}

func (c *completionCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onMsg)
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

func (c *completionCtl) isForLua() bool { return c != nil && c.forLua }

func (c *completionCtl) setForGDB(v bool) { c.forGDB = v }

func (c *completionCtl) setForLua(v bool) { c.forLua = v }

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

func (c *completionCtl) useLuaInput() bool {
	h := c.host
	if h == nil || !c.forLua || h.LuaConsoleWidget() == nil {
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
		h.GDBWidget().ApplyCompletion(gdb.WithCompletionSpace(gdb.ApplyMenuChoice("", name)))
		return
	}
	if c.useLuaInput() {
		cur := h.LuaConsoleWidget().InputText()
		h.LuaConsoleWidget().ApplyCompletion(gdb.WithCompletionSpace(luahost.ApplyGdbforgeChoice(cur, name)))
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
		c.forLua = false
		return
	}
	if c.forGDB || c.forLua {
		c.forGDB = false
		c.forLua = false
		h.SetMode(platform.ModeInsert)
		return
	}
	c.forGDB = false
	c.forLua = false
	h.SetMode(platform.ModeCommand)
}

// luaTabComplete runs gdbforge.* completion for the Lua REPL input line.
func (c *completionCtl) luaTabComplete() {
	h := c.host
	if h == nil || h.LuaConsoleWidget() == nil {
		return
	}
	text := h.LuaConsoleWidget().InputText()
	newLine, names := h.LuaGdbforgeComplete(text)
	if newLine != "" && newLine != text {
		h.LuaConsoleWidget().ApplyCompletion(newLine)
		text = newLine
	}
	c.publishLuaMenu(text, names)
	switch len(names) {
	case 0:
		// nothing
	case 1:
		if names[0] == "." {
			h.LuaConsoleWidget().ApplyCompletion(text)
		} else {
			h.LuaConsoleWidget().ApplyCompletion(gdb.WithCompletionSpace(luahost.ApplyGdbforgeChoice(text, names[0])))
		}
		c.clear()
	default:
		c.forLua = true
		c.forGDB = false
		h.SetMode(platform.ModeCompletion)
	}
	h.RequestFrame()
}

// refreshLuaMenu re-queries gdbforge.* completions for the Lua REPL line.
func (c *completionCtl) refreshLuaMenu() {
	h := c.host
	if h == nil || h.LuaConsoleWidget() == nil {
		c.leaveMode()
		return
	}
	text := h.LuaConsoleWidget().InputText()
	if strings.TrimSpace(text) == "" {
		c.leaveMode()
		return
	}
	_, names := h.LuaGdbforgeComplete(text)
	c.publishLuaMenu(text, names)
	c.forLua = true
	c.forGDB = false
	if h.Mode() != platform.ModeCompletion {
		h.SetMode(platform.ModeCompletion)
	}
}

func (c *completionCtl) publishLuaMenu(text string, names []string) {
	h := c.host
	if h == nil {
		return
	}
	h.PublishCompletion(termui.CompletionMsg{
		Input: text,
		Token: text,
		Names: names,
	})
}

// gdbTabComplete runs MI tab completion for the GDB console line.
// The GDB pane is an xterm emulator; CLI line capture is not wired yet.
func (c *completionCtl) gdbTabComplete() {
	_ = c
}

// refreshGDBMenu re-runs -complete for the current GDB input and replaces the
// wildmenu. Does not apply LCP or unique matches (typing owns the line).
// Tab/arrows only move selection and must not call this.
//
// Stay in ModeCompletion across 0/1-match re-queries so further typing and
// backspace keep refreshing. Leaving on ≤1 made small candidate sets die as
// soon as the list narrowed (or -complete briefly returned empty).
func (c *completionCtl) refreshGDBMenu() {
	c.leaveMode()
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
