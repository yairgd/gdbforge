package app

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/platform"
)

func (a *DebuggerApp) InitB() error {
	a.ctx = platform.NewAppContext()
	a.debug = debugstate.New(a.State())

	if path := a.cfg.LogFile; path != "" {
		if err := a.enableFileLog(path); err != nil {
			return err
		}
	}
	a.miLog = a.ctx.Log.Named("gdb-mi")

	if err := a.initBuiltins(); err != nil {
		return err
	}

	logo := a.logoWidget
	if logo == nil {
		logo = widgets.NewLogoWidget()
		a.logoWidget = logo
	}
	initLayoutShell(a, a.newStartupTab(logo))

	lay := a.Layout()
	lay.SetStatusClipboard(a.ClipboardIO())
	lay.FocusWidget(a.gdbWidget)
	lay.SetLeafMark(leafMarkCode, lay.FindLeaf(isCodeSlot))
	lay.SetLeafMark(leafMarkGDB, lay.FindLeaf(func(w termforge.Widget) bool { return w == a.gdbWidget }))
	a.EnterInsertMode()
	lay.SetOnResize(a.RequestFrame)
	a.State().SetEqualAlways(true)
	lay.SetEqualAlways(true)
	// The workspace fills the screen; the cmdline is a pinned 1-row leaf at the
	// bottom of the tree, so the line above it is that split's own separator.
	a.AddWidget(a.Widget())

	a.cmdWidget = termforge.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.SetPostInterrupt(a.PostInterrupt)
	a.cmdWidget.SetClipboard(a.ClipboardIO())
	a.restoreCmdlineHistory()
	a.SetCmdline(a.cmdWidget)
	lay.PinBottom(a.cmdWidget, 1)

	// Registered last so it paints over the workspace. Either view is floating,
	// so neither reserves layout space and opening the wildmenu never reshapes
	// the splits.
	a.comp.attach(&termforge.CompletionMenu{}, a.addCompletionView())

	a.registerUIComponents()

	a.InitKeyBindings()
	a.ExapData()

	a.RegisterModeHandler(platform.ModeNormal, a.withGlobalKeys(a.handleNormalKey))
	a.RegisterModeHandler(platform.ModeInsert, a.withGlobalKeys(a.handleInsertKey))
	a.RegisterModeHandler(platform.ModeCommand, a.withGlobalKeys(a.handleCommandKey))
	a.RegisterModeHandler(platform.ModeSearch, a.withGlobalKeys(a.handleSearchKey))
	a.RegisterModeHandler(platform.ModeCompletion, a.withGlobalKeys(a.handleCompletionKey))
	a.RegisterModeHandler(platform.ModeLua, a.withGlobalKeys(a.lua.handleKey))
	return nil
}

// completionAsWindow picks which CompletionView paints the wildmenu: the
// centered list window, or the classic one-row bar on the separator above the
// cmdline. Both satisfy CompletionView and both are floating widgets, so this
// is the only line that has to change.
const completionAsWindow = false

// addCompletionView builds the chosen view and registers it.
func (a *DebuggerApp) addCompletionView() termforge.CompletionView {
	if completionAsWindow {
		a.compPopup = termforge.NewCompletionPopupWidget(a.ctx)
		a.AddFloatingWidget(a.compPopup, a.completionPopupRect)
		return a.compPopup
	}
	bar := termforge.NewCompletionBarWidget(a.ctx)
	a.AddFloatingWidget(bar, completionBarRect)
	return bar
}

// completionBarRect is the row above the cmdline — the workspace separator the
// bar has always overlaid. It is floating rather than a chrome row because the
// cmdline now lives in the tree, so a row would land below it, not above.
func completionBarRect(c termforge.Canvas) termforge.Rect {
	if c.H() < 2 || c.W() < 1 {
		return termforge.Rect{}
	}
	return c.ChildRect(0, c.H()-2, c.W(), 1)
}

// completionPopupRect centers the wildmenu window over the workspace, sized to
// its candidates. The zero Rect (too small to frame, or no candidates) hides
// it, which is also what its Draw checks.
func (a *DebuggerApp) completionPopupRect(c termforge.Canvas) termforge.Rect {
	if a.compPopup == nil {
		return termforge.Rect{}
	}
	width, height := a.compPopup.PreferredSize()
	// The cmdline row is not workspace, so center over what is above it.
	avail := c.H() - 1
	if width > c.W() {
		width = c.W()
	}
	if height > avail {
		height = avail
	}
	if width < 4 || height < 3 {
		return termforge.Rect{}
	}
	return c.ChildRect((c.W()-width)/2, (avail-height)/2, width, height)
}
