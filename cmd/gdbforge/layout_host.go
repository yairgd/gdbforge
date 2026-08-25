package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/layout"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// layoutHost is the narrow surface LayoutShell needs from the composition root.
type layoutHost interface {
	State() *platform.AppState
	LogNamed(name string) *platform.NamedLogger
	RequestFrame()
	RequestRedraw()
	SetMode(mode platform.Mode)
	EnterInsertMode(args ...any)
	GDBWidget() *widgets.GDBWidget
	LogoWidget() *widgets.LogoWidget
	SetLogoWidget(w *widgets.LogoWidget)
	FocusedWidget() termui.Widget
	focusedCode() *widgets.CodeWidget
	ActiveCodeWidget() *widgets.CodeWidget
	LayoutCodePane() termui.Widget
	DebugPanes(code termui.Widget) layout.Panes
	AsmPreferAsm() bool
	AsmHasSplit() bool
	AsmWidget() *widgets.AssemblyWidget
	BufsSetPrimary(w *widgets.CodeWidget)
}

func (a *DebuggerApp) LogNamed(name string) *platform.NamedLogger {
	if a == nil || a.ctx.Log == nil {
		return nil
	}
	return a.ctx.Log.Named(name)
}

func (a *DebuggerApp) LayoutCodePane() termui.Widget  { return a.layoutCodePane() }
func (a *DebuggerApp) DebugPanes(code termui.Widget) layout.Panes { return a.debugPanes(code) }
func (a *DebuggerApp) AsmPreferAsm() bool             { return a.asm.PreferAsm() }
func (a *DebuggerApp) AsmHasSplit() bool              { return a.asm.hasSplit() }
func (a *DebuggerApp) AsmWidget() *widgets.AssemblyWidget { return a.asm.Widget() }
func (a *DebuggerApp) BufsSetPrimary(w *widgets.CodeWidget) { a.bufs.setPrimary(w) }
func (a *DebuggerApp) SetLogoWidget(w *widgets.LogoWidget)  { a.logoWidget = w }
