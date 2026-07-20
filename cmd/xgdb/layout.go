package main

import (
	"strings"

	"github.com/yairgd/cgdb-go/internal/xgdb/layout"
	"github.com/yairgd/cgdb-go/internal/xgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// OnLayout applies a named workspace layout (:layout panels|default|classic).
// With no name, re-applies the panels (startup) layout.
func (a *DebuggerApp) OnLayout(args ...any) {
	name := layout.Panels
	if len(args) > 0 {
		if s, ok := args[0].(string); ok && strings.TrimSpace(s) != "" {
			name = strings.TrimSpace(s)
		}
	}
	a.ApplyLayout(name)
}

func (a *DebuggerApp) layoutCompletions(prefix string) []string {
	var out []string
	for _, name := range a.State().Layouts() {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	return out
}

func (a *DebuggerApp) debugPanes(code termui.Widget) layout.Panes {
	return layout.Panes{
		Code:        code,
		GDB:         a.gdbWidget,
		Output:      a.outputWidget,
		Breakpoints: a.bpWidget,
		Threads:     a.threadWidget,
		Callstack:   a.callstackWidget,
	}
}

// ApplyLayout rebuilds the active tab tree for a registered layout name.
func (a *DebuggerApp) ApplyLayout(name string) {
	if a.tab == nil || !a.State().HasLayout(name) {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("layout").Error("unknown layout: " + name)
		}
		return
	}
	tw := a.buildLayoutTab(name)
	if tw == nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("layout").Error("layout not implemented: " + name)
		}
		return
	}
	if tree := tw.ActiveTree(); tree != nil {
		a.tab.SetActiveTree(tree)
	}
	a.finishLayoutApply(name)
}

func (a *DebuggerApp) buildLayoutTab(name string) *termui.TabWidget {
	code := a.layoutCodePane()
	panes := a.debugPanes(code)
	switch name {
	case layout.Default:
		return layout.BuildDefault("basic debugger", panes, a.State().DefaultLayoutRatios())
	case layout.Panels:
		return layout.BuildPanels("panels", panes)
	case layout.Classic:
		return layout.BuildClassic("classic", panes)
	default:
		return nil
	}
}

func (a *DebuggerApp) finishLayoutApply(name string) {
	a.State().SetCurrentLayout(name)
	a.State().SetEqualAlways(true)
	a.tab.SetEqualAlways(true)
	a.tab.FocusWidget(a.gdbWidget)
	a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
	a.tab.SetLeafMark(leafMarkGDB, a.tab.FindLeaf(func(w termui.Widget) bool { return w == a.gdbWidget }))
	a.EnterInsertMode()
	a.RequestFrame()
}

// layoutCodePane returns the widget for the code leaf (source buffer or logo splash).
func (a *DebuggerApp) layoutCodePane() termui.Widget {
	if w := a.layoutCodeWidget(); w != nil {
		return w
	}
	if a.logoWidget != nil {
		return a.logoWidget
	}
	a.logoWidget = widgets.NewLogoWidget()
	return a.logoWidget
}

func (a *DebuggerApp) layoutCodeWidget() *widgets.CodeWidget {
	if path := a.State().CurrentFile(); path != "" {
		if w, _ := a.ensureCodeBuffer(path); w != nil {
			return w
		}
	}
	if a.primaryCode != nil {
		return a.primaryCode
	}
	return nil
}

// registerLayouts registers named workspace layouts on AppState.
// Startup / current layout is panels.
func (a *DebuggerApp) registerLayouts() {
	for _, name := range []string{layout.Panels, layout.Default, layout.Classic} {
		a.State().RegisterLayout(name)
	}
	a.State().SetCurrentLayout(layout.Panels)
}

// newStartupTab builds the initial panels workspace tab.
func (a *DebuggerApp) newStartupTab(code termui.Widget) *termui.TabWidget {
	return layout.BuildPanels("panels", a.debugPanes(code))
}
