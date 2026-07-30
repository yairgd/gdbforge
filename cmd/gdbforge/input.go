package main

import (
	"errors"
	"strconv"
	"strings"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// withGlobalKeys runs mode-independent shortcuts (Ctrl-Z, Ctrl-C, Ctrl-D) before a mode handler.
func (a *DebuggerApp) withGlobalKeys(h termui.KeyHandler) termui.KeyHandler {
	return func(ev *tcell.EventKey) bool {
		if a.tryGlobalSuspend(ev) {
			return true
		}
		if a.tryGlobalInterrupt(ev) {
			return true
		}
		if a.tryGlobalEOF(ev) {
			return true
		}
		return h(ev)
	}
}

// tryGlobalSuspend handles Ctrl-Z in any mode/focus: SIGTSTP inferior if
// running, otherwise suspend gdbforge (same as GDB job control).
func (a *DebuggerApp) tryGlobalSuspend(ev *tcell.EventKey) bool {
	if !isCtrlZ(ev) {
		return false
	}
	a.onGdbConsoleSuspend()
	return true
}

// tryGlobalInterrupt handles Ctrl-C in any mode/focus.
// If a :lua worker job is running, cancel it (unblocks sleep/wait_port).
// Otherwise interrupt the debugger session (GDB/dlv PTY ^C).
func (a *DebuggerApp) tryGlobalInterrupt(ev *tcell.EventKey) bool {
	if !isCtrlC(ev) {
		return false
	}
	if a.cancelLuaJob() {
		if a.outputWidget != nil {
			a.outputWidget.AppendHostLine("cancelled (Ctrl-C)")
		}
		a.RequestFrame()
		return true
	}
	a.onGdbConsoleInterrupt()
	a.RequestFrame()
	return true
}

// tryGlobalEOF handles Ctrl-D in any mode/focus: same as GDB-console EOF
// (send q / quit; confirm if inferior alive).
func (a *DebuggerApp) tryGlobalEOF(ev *tcell.EventKey) bool {
	if !isCtrlD(ev) {
		return false
	}
	a.onGdbConsoleEOF()
	a.RequestFrame()
	return true
}

func isCtrlZ(ev *tcell.EventKey) bool {
	if ev == nil {
		return false
	}
	if ev.Key() == tcell.KeyCtrlZ {
		return true
	}
	// ASCII SUB (0x1A): some NewEventKey paths use Key(26) instead of KeyCtrlZ.
	if ev.Key() == tcell.Key(0x1a) {
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == 0x1a {
		return true
	}
	// KeyCtrlZ events also carry Rune 'z' + ModCtrl; KeyRune+ModCtrl variants too.
	if (ev.Rune() == 'z' || ev.Rune() == 'Z') && ev.Modifiers()&tcell.ModCtrl != 0 {
		return true
	}
	return false
}

func isCtrlC(ev *tcell.EventKey) bool {
	if ev == nil {
		return false
	}
	if ev.Key() == tcell.KeyCtrlC {
		return true
	}
	if ev.Key() == tcell.KeyRune && (ev.Rune() == 'c' || ev.Rune() == 'C') &&
		ev.Modifiers()&tcell.ModCtrl != 0 {
		return true
	}
	return false
}

func isCtrlD(ev *tcell.EventKey) bool {
	if ev == nil {
		return false
	}
	if ev.Key() == tcell.KeyCtrlD {
		return true
	}
	if ev.Key() == tcell.KeyRune && (ev.Rune() == 'd' || ev.Rune() == 'D') &&
		ev.Modifiers()&tcell.ModCtrl != 0 {
		return true
	}
	return false
}

func (a *DebuggerApp) handleInsertKey(ev *tcell.EventKey) bool {
	// GDB console insert: pass all keys through so typing is native (Space, n,
	// etc.). Only Esc leaves insert mode; Tab runs completion + wildmenu
	// (MI -complete for GDB, command-name list for Delve).
	if a.focusedIsGdb() {
		if key, ok := platform.KeyFromEvent(ev); ok {
			if key.Key == tcell.KeyEscape {
				a.onEscape()
				return true
			}
			if key.Key == tcell.KeyTAB {
				a.gdbTabComplete()
				return true
			}
		}
		a.tab.HandleEvent(ev)
		return true
	}
	if a.tryKeyBindings(a.insertKeys, ev) {
		return true
	}
	a.tab.HandleEvent(ev)
	return true
}

func (a *DebuggerApp) handleNormalKey(ev *tcell.EventKey) bool {
	// Layout hook reserved for future layout-specific binds (currently no-op).
	if a.currentLayoutBehavior().HandleNormalKey(a, ev) {
		return true
	}
	if a.tryKeyBindings(a.keyBindings, ev) {
		return true
	}
	if isCopyKey(ev) {
		a.tab.HandleEvent(ev)
		return true
	}
	// Focused scrollable panes (e.g. Log) handle their bindings without insert mode.
	if w := a.focusedWidget(); w != nil {
		if h, ok := w.(termui.FocusKeyHandler); ok && h.HandleFocusKey(ev) {
			return true
		}
	}
	return true
}

// toggleCodeBreakEnable toggles enable/disable at the active CodeWidget cursor
// (same as BreakpointWidget e). Disabled marks stay in the BP list and show yellow.
func (a *DebuggerApp) toggleCodeBreakEnable() {
	cw := a.activeCodeWidget()
	if focused := a.focusedCode(); focused != nil {
		cw = focused
	}
	a.breaks.toggleCodeBreakEnableOn(cw)
}

func (a *DebuggerApp) handleCommandKey(ev *tcell.EventKey) bool {
	// Cmdline owns completion — never keep a prior GDB wildmenu session.
	a.completionForGDB = false
	a.cmdWidget.HandleEvent(ev)
	if ev.Key() == tcell.KeyTAB && a.completionActive() {
		a.completionForGDB = false
		a.SetMode(platform.ModeCompletion)
		a.RequestFrame()
		return true
	}
	if ev.Key() == tcell.KeyEnter {
		a.clearCompletion()
		a.cmdWidget.Deativate()
		if a.Mode() == platform.ModeCommand {
			a.SetMode(platform.ModeNormal)
		}
	}
	return true
}

// handleSearchKey owns keys while ModeSearch ('/' cmdline) is active.
// Same edit line as command mode, but no tab-completion / command parse —
// edits live-preview matches on searchTarget; Enter commits.
func (a *DebuggerApp) handleSearchKey(ev *tcell.EventKey) bool {
	a.completionForGDB = false
	a.clearCompletion()
	a.cmdWidget.HandleEvent(ev)
	if !a.cmdWidget.Active() && a.Mode() == platform.ModeSearch {
		a.SetMode(platform.ModeNormal)
	}
	a.RequestFrame()
	return true
}

func (a *DebuggerApp) onSearchCmdChange(text string) {
	if a.cmdWidget == nil || a.cmdWidget.Kind() != termui.CmdKindSearch {
		return
	}
	host := a.searchTarget
	if host == nil {
		host = a.resolveSearchHost()
		a.searchTarget = host
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

func (a *DebuggerApp) onSearchCmdSubmit(pattern string) {
	host := a.searchTarget
	if host == nil {
		host = a.resolveSearchHost()
	}
	if host == nil {
		return
	}
	host.SetSearchColor(a.State().SearchColor())
	host.CommitSearch(pattern)
	a.searchTarget = host // keep for n/N and */# on same pane
}

func (a *DebuggerApp) searchNextMatch() {
	host := a.searchTarget
	if host == nil {
		host = a.resolveSearchHost()
	}
	if host == nil {
		return
	}
	host.SetSearchColor(a.State().SearchColor())
	if host.SearchNext() {
		a.RequestFrame()
	}
}

func (a *DebuggerApp) searchPrevMatch() {
	host := a.searchTarget
	if host == nil {
		host = a.resolveSearchHost()
	}
	if host == nil {
		return
	}
	host.SetSearchColor(a.State().SearchColor())
	if host.SearchPrev() {
		a.RequestFrame()
	}
}

// searchWordMatch implements vim * (dir>0) / # (dir<0): set pattern from the
// word under the caret (prefer caret over a stale selection), then jump
// next/prev. With no word, fall back to selection, then existing pattern.
//
// If the caret already sits on a highlight of the active pattern (e.g. after
// /46 landed on "46" inside "1052946"), only navigate — do not expand to the
// enclosing identifier.
func (a *DebuggerApp) searchWordMatch(dir int) {
	// Always use the focused pane so */# follow the caret the user sees.
	host := a.resolveSearchHost()
	if host == nil {
		return
	}
	host.SetSearchColor(a.State().SearchColor())

	if host.SearchPattern() != "" && a.cursorInSearchMatch(host) {
		a.searchTarget = host
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
	pattern := a.wordAtSearchHost(host)
	if pattern == "" {
		pattern = a.selectionAtSearchHost(host)
	}
	if pattern != "" {
		host.CommitSearch(pattern)
		a.searchTarget = host
		a.clearSearchHostSelection(host)
		if dir < 0 {
			_ = host.SearchPrev()
		} else {
			_ = host.SearchNext()
		}
		a.RequestFrame()
		return
	}
	if dir < 0 {
		a.searchPrevMatch()
		return
	}
	a.searchNextMatch()
}

func (a *DebuggerApp) cursorInSearchMatch(host termui.SearchHost) bool {
	if host == nil {
		return false
	}
	if c, ok := host.(interface{ CursorInSearchMatch() bool }); ok {
		return c.CursorInSearchMatch()
	}
	if vp := a.viewportOfSearchHost(host); vp != nil {
		return vp.CursorInSearchMatch()
	}
	return false
}

// trySearchOrGdbNext is normal-mode n: on Code always GDB next (like s/c);
// elsewhere search-next when a pattern is active, else GDB next.
func (a *DebuggerApp) trySearchOrGdbNext() bool {
	if a.focusedIsCode() {
		a.sendGdbExec("next")
		return true
	}
	if a.hasActiveSearchPattern() {
		a.searchNextMatch()
		return true
	}
	a.sendGdbExec("next")
	return true
}

func (a *DebuggerApp) hasActiveSearchPattern() bool {
	host := a.searchTarget
	if host == nil {
		host = a.resolveSearchHost()
	}
	if host == nil {
		return false
	}
	return host.SearchPattern() != ""
}

func (a *DebuggerApp) wordAtSearchHost(host termui.SearchHost) string {
	if host == nil {
		return ""
	}
	if w, ok := host.(interface{ WordAtCursor() string }); ok {
		return w.WordAtCursor()
	}
	return ""
}

func (a *DebuggerApp) selectionAtSearchHost(host termui.SearchHost) string {
	if host == nil {
		return ""
	}
	vp := a.viewportOfSearchHost(host)
	if vp == nil || !vp.HasSelection() {
		return ""
	}
	return strings.TrimSpace(vp.SelectedText())
}

func (a *DebuggerApp) clearSearchHostSelection(host termui.SearchHost) {
	if vp := a.viewportOfSearchHost(host); vp != nil {
		vp.ClearSelection()
	}
}

func (a *DebuggerApp) viewportOfSearchHost(host termui.SearchHost) *termui.Viewport {
	switch t := host.(type) {
	case *termui.Viewport:
		return t
	case interface{ Viewport() *termui.Viewport }:
		return t.Viewport()
	default:
		return nil
	}
}

// resolveSearchHost returns the /search target for the last active (focused)
// pane — the one with the green/blue status bar. Falls back to the active
// CodeWidget when the focused pane has no viewport.
func (a *DebuggerApp) resolveSearchHost() termui.SearchHost {
	if host := a.searchHostOf(a.focusedWidget()); host != nil {
		return host
	}
	if cw := a.activeCodeWidget(); cw != nil {
		return cw
	}
	return nil
}

func (a *DebuggerApp) searchHostOf(w termui.Widget) termui.SearchHost {
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

// handleCompletionKey owns keys while the wildmenu is open (ModeCompletion).
// Tab/arrows only move selection. Letters/backspace edit the source line
// (GDB console or cmdline) and re-query completions into the menu — no local filter.
func (a *DebuggerApp) handleCompletionKey(ev *tcell.EventKey) bool {
	if a.completionMenu == nil {
		a.leaveCompletionMode()
		return true
	}
	if a.tryKeyBindings(a.completionKeys, ev) {
		return true
	}
	isType := ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0
	isBS := ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2
	if isType || isBS {
		// Prefer cmdline when it is active so a stuck completionForGDB flag
		// cannot route :b editing into MI -complete.
		useGDB := a.completionForGDB && a.gdbWidget != nil &&
			(a.cmdWidget == nil || !a.cmdWidget.Active())
		if useGDB {
			if isType {
				a.gdbWidget.InsertInputRune(ev.Rune())
			} else {
				a.gdbWidget.BackspaceInput()
			}
			a.refreshGDBCompletionMenu()
			a.RequestFrame()
			return true
		}
		if a.cmdWidget != nil {
			a.completionForGDB = false
			a.cmdWidget.HandleEvent(ev)
			if !a.cmdWidget.Active() {
				a.clearCompletion()
				a.SetMode(platform.ModeNormal)
				a.RequestFrame()
				return true
			}
			a.refreshCmdCompletionMenu()
			a.RequestFrame()
			return true
		}
	}
	// Other keys: leave wildmenu and continue editing.
	a.clearCompletion()
	if a.completionForGDB {
		a.completionForGDB = false
		a.SetMode(platform.ModeInsert)
		a.tab.HandleEvent(ev)
	} else {
		a.SetMode(platform.ModeCommand)
		a.cmdWidget.HandleEvent(ev)
	}
	a.RequestFrame()
	return true
}

func (a *DebuggerApp) leaveCompletionMode() {
	a.clearCompletion()
	if a.completionForGDB {
		a.completionForGDB = false
		a.SetMode(platform.ModeInsert)
		return
	}
	a.completionForGDB = false
	a.SetMode(platform.ModeCommand)
}

// gdbTabComplete runs completion for the GDB/Delve input line and feeds the same
// CompletionMsg / wildmenu path used by cmdline trie Completer.
func (a *DebuggerApp) gdbTabComplete() {
	if a.gdbWidget == nil {
		return
	}
	text := a.gdbWidget.InputText()
	if a.backend == nil {
		return
	}
	if a.isDLV() && a.dlvConfirm.Confirming() {
		return
	}
	res := a.backend.Complete(a.GDB(), a.State(), text)

	// Expand to longest common prefix when it grows the line.
	if res.Completion != "" && res.Completion != text {
		a.gdbWidget.ApplyCompletion(res.Completion)
		text = res.Completion
	}

	names := res.Matches
	if len(names) == 0 && res.Completion != "" {
		names = []string{res.Completion}
	}
	a.publishGDBCompletionMenu(text, names)

	switch len(names) {
	case 0:
		// nothing
	case 1:
		// Unique match — no further completions for this word; add a trailing space.
		a.gdbWidget.ApplyCompletion(gdb.WithCompletionSpace(names[0]))
		a.clearCompletion()
	default:
		a.completionForGDB = true
		a.SetMode(platform.ModeCompletion)
	}
	a.RequestFrame()
}

// refreshGDBCompletionMenu re-runs -complete for the current GDB input and
// replaces the wildmenu. Does not apply LCP or unique matches (typing owns the
// line). Tab/arrows only move selection and must not call this.
//
// Stay in ModeCompletion across 0/1-match re-queries so further typing and
// backspace keep refreshing. Leaving on ≤1 made small candidate sets die as
// soon as the list narrowed (or -complete briefly returned empty).
func (a *DebuggerApp) refreshGDBCompletionMenu() {
	if a.gdbWidget == nil {
		a.leaveCompletionMode()
		return
	}
	text := a.gdbWidget.InputText()
	if strings.TrimSpace(text) == "" {
		a.leaveCompletionMode()
		return
	}
	if a.backend == nil {
		a.leaveCompletionMode()
		return
	}
	if a.isDLV() && a.dlvConfirm.Confirming() {
		a.leaveCompletionMode()
		return
	}
	res := a.backend.Complete(a.GDB(), a.State(), text)
	names := res.Matches
	if len(names) == 0 && res.Completion != "" && res.Completion != text {
		names = []string{res.Completion}
	}
	a.publishGDBCompletionMenu(text, names)
	a.completionForGDB = true
	if a.Mode() != platform.ModeCompletion {
		a.SetMode(platform.ModeCompletion)
	}
}

func (a *DebuggerApp) publishGDBCompletionMenu(text string, names []string) {
	menu := gdb.MenuNames(text, names)
	// After file:, attach signatures from -symbol-info-functions
	// ("foo" → "foo(int, char *)"); apply still inserts bare name.
	if a.backend != nil {
		menu = a.backend.EnrichLinespecMenu(text, menu, a.GDB(), a.State())
	}
	if a.ctx.Bus != nil {
		platform.Publish(a.ctx.Bus, termui.CompletionMsg{
			Input: text,
			Token: text,
			Names: menu,
		})
	}
}

// refreshCmdCompletionMenu re-syncs the cmdline parser and replaces the wildmenu.
func (a *DebuggerApp) refreshCmdCompletionMenu() {
	if a.cmdWidget == nil || !a.cmdWidget.Active() {
		a.leaveCompletionMode()
		return
	}
	names := a.cmdWidget.CompletionNames()
	if a.ctx.Bus != nil {
		platform.Publish(a.ctx.Bus, termui.CompletionMsg{
			Input: a.cmdWidget.Text(),
			Token: a.cmdWidget.Text(),
			Names: names,
		})
	}
	if len(names) <= 1 {
		a.leaveCompletionMode()
		return
	}
	if a.Mode() != platform.ModeCompletion {
		a.SetMode(platform.ModeCompletion)
	}
}

func isCopyKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlC || ev.Key() == tcell.KeyCtrlX || ev.Key() == tcell.KeyCtrlV {
		return true
	}
	if ev.Modifiers()&tcell.ModCtrl == 0 || ev.Key() != tcell.KeyRune {
		return false
	}
	switch ev.Rune() {
	case 'c', 'C', 'x', 'X', 'v', 'V':
		return true
	}
	return false
}

func (a *DebuggerApp) HandleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	primary := ev.Buttons()&tcell.ButtonPrimary != 0
	wheel := ev.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0
	inCmd := a.cmdLineContains(x, y)

	if a.Mode() == platform.ModeCommand || a.Mode() == platform.ModeSearch || a.Mode() == platform.ModeCompletion {
		// Middle-click paste into the cmdline (Linux terminal convention).
		if a.cmdWidget != nil && ev.Buttons()&tcell.ButtonMiddle != 0 {
			a.cmdWidget.HandleEvent(ev)
			a.RequestFrame()
			return
		}
		if primary && inCmd {
			a.clickCmdLine(x)
			return
		}
		if (primary || wheel) && !inCmd {
			// Click/wheel outside the cmdline: leave command/search mode (like Esc),
			// then fall through so the pane under the pointer can take focus.
			a.leaveCommandMode()
		} else {
			return
		}
	}

	if primary && inCmd {
		a.enterCommandMode()
		a.clickCmdLine(x)
		return
	}

	if primary {
		if !a.tab.IsSeparatorAt(x, y) && a.tab.FocusAt(x, y) {
			a.rememberCodeLeafFromFocus()
			if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
				a.enterLuaMode(lw)
			} else {
				a.EnterInsertMode()
			}
		}
	}

	if wheel {
		if a.tab.FocusAt(x, y) {
			a.rememberCodeLeafFromFocus()
			if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
				a.enterLuaMode(lw)
			} else {
				a.EnterInsertMode()
			}
		}
	}

	a.tab.HandleEvent(ev)
	// Always repaint so green stop marks / selection update after clicks.
	a.RequestFrame()
}

// enterCommandMode activates the ':' cmdline (same as pressing ':').
// Leaves insert-active so the focused pane status is blue, matching Esc then ':'.
func (a *DebuggerApp) enterCommandMode() {
	a.leaveLuaMode()
	a.completionForGDB = false
	a.clearCompletion()
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.searchTarget = nil
	if a.cmdWidget != nil && !a.cmdWidget.Active() {
		a.cmdWidget.Activate()
	}
	a.SetMode(platform.ModeCommand)
	a.RequestFrame()
}

// enterSearchMode activates the '/' cmdline (same as pressing '/').
// Captures the last active (focused) pane — green/blue status — as search target.
func (a *DebuggerApp) enterSearchMode() {
	a.leaveLuaMode()
	a.completionForGDB = false
	a.clearCompletion()
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.searchTarget = a.resolveSearchHost()
	if a.searchTarget != nil {
		a.searchTarget.SetSearchColor(a.State().SearchColor())
	}
	if a.cmdWidget != nil {
		a.cmdWidget.ActivateSearch()
	}
	a.SetMode(platform.ModeSearch)
	a.RequestFrame()
}

// leaveCommandMode exits ':' / '/' / wildmenu (same as Esc).
func (a *DebuggerApp) leaveCommandMode() {
	wasSearch := a.Mode() == platform.ModeSearch ||
		(a.cmdWidget != nil && a.cmdWidget.Kind() == termui.CmdKindSearch)
	a.completionForGDB = false
	a.clearCompletion()
	if wasSearch && a.searchTarget != nil {
		a.searchTarget.RevertSearch()
		// Keep searchTarget so n/N and */# still work on the committed pattern.
	}
	if a.cmdWidget != nil {
		a.cmdWidget.Deativate()
	}
	a.SetMode(platform.ModeNormal)
	a.RequestFrame()
}

func (a *DebuggerApp) cmdLineContains(x, y int) bool {
	if a.cmdWidget == nil {
		return false
	}
	for _, n := range a.Widgets() {
		if n.Widget() == a.cmdWidget {
			return n.Rect().Contains(x, y)
		}
	}
	return false
}

func (a *DebuggerApp) clickCmdLine(screenX int) {
	if a.cmdWidget == nil {
		return
	}
	originX := 0
	for _, n := range a.Widgets() {
		if n.Widget() == a.cmdWidget {
			originX = n.Rect().X()
			break
		}
	}
	if a.Mode() == platform.ModeCompletion {
		a.completionForGDB = false
		a.clearCompletion()
		if a.cmdWidget != nil && a.cmdWidget.Kind() == termui.CmdKindSearch {
			a.SetMode(platform.ModeSearch)
			if !a.cmdWidget.Active() {
				a.cmdWidget.ActivateSearch()
			}
		} else {
			a.SetMode(platform.ModeCommand)
			if a.cmdWidget != nil && !a.cmdWidget.Active() {
				a.cmdWidget.Activate()
			}
		}
	}
	a.cmdWidget.SetCursorAtLocalX(screenX - originX)
	a.RequestFrame()
}

func (a *DebuggerApp) HandleResize() {
	c := a.UpdateCanvas()

	w := a.Widgets()
	if len(w) < 3 {
		return
	}
	// Tab keeps full height (it insets H-2 internally). Bar overlays H-2; cmd at H-1.
	w[0].SetRect(c.ChildRect(0, 0, c.W(), c.H()))
	w[1].SetRect(c.ChildRect(0, c.H()-2, c.W(), 1))
	w[2].SetRect(c.ChildRect(0, c.H()-1, c.W(), 1))
}

func (app *DebuggerApp) handleUnknownCommand(ev termui.CommandEvent) bool {
	if msg, ok := ev.(termui.SubmitMsg); ok && app.tryGotoLineCmd(msg.Text) {
		return true
	}
	// TODO: show unknown command feedback in the UI
	return true
}

// tryGotoLineCmd handles Vim-style :N / :0 — jump browse cursor to line N
// in the active Code buffer (blue line). :0 goes to line 1.
func (a *DebuggerApp) tryGotoLineCmd(text string) bool {
	line, ok := parseGotoLineCmd(text)
	if !ok {
		return false
	}
	cw := a.activeCodeWidget()
	if focused := a.focusedCode(); focused != nil {
		cw = focused
	}
	if cw == nil {
		return true // consumed as goto, but no buffer
	}
	cw.GotoLine(line)
	a.RequestFrame()
	return true
}

// parseGotoLineCmd accepts ":42", "42", ":0" (→ line 1). Rejects non-numeric.
func parseGotoLineCmd(text string) (line int, ok bool) {
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, ":") {
		s = strings.TrimSpace(s[1:])
	}
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	if n < 1 {
		n = 1
	}
	return n, true
}

func (app *DebuggerApp) handleExitMode(_ termui.CommandEvent) bool {
	app.leaveCommandMode()
	return true
}

func (a *DebuggerApp) HandleInterrupt(ev *tcell.EventInterrupt) {
	switch data := ev.Data().(type) {
	case events.GdbOutputMsg:
		// Avoid per-line file logging during free-run floods (major TUI lag).
		if a.miLog != nil && !a.Debug().InferiorRunning() {
			for _, line := range strings.Split(data.Data, "\n") {
				a.miLog.Info(line)
			}
			if data.Err != nil {
				a.miLog.Error(data.Err.Error())
			}
		} else if a.miLog != nil && data.Err != nil {
			a.miLog.Error(data.Err.Error())
		}
		if a.gdbWidget != nil {
			a.handleGdbOutputMsg(data)
		}
		if a.outputWidget != nil && data.Data != "" {
			a.outputWidget.AppendPty(data.Data)
		}
		// No RequestFrame: Run() already redraws after this interrupt.
	case events.InferiorOutputMsg:
		if data.Data == "" {
			break
		}
		if a.outputWidget != nil {
			a.outputWidget.AppendInferior(data.Data)
		}
		// Mirror into the debugger console when enabled (default on for Delve).
		if a.gdbWidget != nil && a.Debug().GdbTargetPrint() {
			a.gdbWidget.AppendTargetText(data.Data)
		}
		a.RequestFrame()
	case core.ExecOutputMsg:
		if a.execWidget != nil {
			a.execWidget.HandleEvent(ev)
		}
	case aiReplyMsg:
		if a.gdbWidget != nil {
			a.gdbWidget.AppendLines(data.lines)
			a.RequestFrame()
		}
	case codeRefreshMsg:
		if data.fromStop {
			// Late stop paint lost the race with call-stack / frame browse.
			if data.stopGen != a.codeNavGen {
				a.RequestFrame()
				break
			}
			if a.callstackWidget != nil {
				a.callstackWidget.SelectLevel(0)
			}
			if data.stop != nil {
				w := a.updateCodeAfterStop(data.stop)
				a.applyCodeStop(w)
				// Repaint all Code gutters from the model (fresh from the
				// pre-stop -break-list query, or whatever Merge already holds).
				if a.breakpoints != nil {
					a.paintCodeBreakmarks(a.breakpoints.Items())
				}
				a.RequestFrame()
				break
			}
		}
		a.applyCodeStop(data.widget)
		a.RequestFrame()
	case asmRefreshMsg:
		a.applyAsmRefresh(data)
		a.RequestFrame()
	case breakpointsUIMsg:
		// refreshBreakpoints may have applied off-thread; push gutters again
		// on the UI thread so a late Code buffer still gets marks.
		if a.breakpoints != nil {
			a.breaks.syncBreakpointViews()
		}
		a.RequestFrame()
	case debugInfoUIMsg:
		// Models were updated off-thread; push to views on the UI thread.
		// Do not force frame 0 or re-drive Code here — that races with call-stack browse.
		a.syncThreadViews()
		a.syncCallStackViews()
		a.syncCodeFromCallstack()
		a.RequestFrame()
	case luaUIMsg:
		func() {
			defer func() {
				if data.done != nil {
					close(data.done)
				}
			}()
			if data.fn != nil {
				data.fn()
			}
		}()
	case luaJobDoneMsg:
		if data.err != nil {
			msg := data.err.Error()
			// Ctrl-C already printed "cancelled (Ctrl-C)"; skip duplicate.
			if !errors.Is(data.err, luahost.ErrJobCancelled) &&
				!strings.Contains(msg, "cancelled") &&
				!strings.Contains(msg, "context canceled") {
				if a.outputWidget != nil {
					a.outputWidget.AppendHostLine(data.name + ": " + msg)
				}
				if a.ctx.Log != nil {
					a.ctx.Log.Named("lua").Error(data.name + ": " + msg)
				}
			}
		}
		a.RequestFrame()
	case string:
		// GDB PTY closed (q / quit / -gdb-exit) — leave the app.
		if data == "gdb-exit" {
			a.Exit()
			return
		}
		if a.gdbWidget != nil {
			a.gdbWidget.HandleEvent(ev)
		}
		if a.execWidget != nil {
			a.execWidget.HandleEvent(ev)
		}
	default:
		if a.gdbWidget != nil {
			a.gdbWidget.HandleEvent(ev)
		}
		if a.execWidget != nil {
			a.execWidget.HandleEvent(ev)
		}
	}
}
