package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/layout"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

// OnLayout applies a named workspace layout (:layout panels|default|classic|wide).
// With no name, re-applies the wide (startup).
//
// :layout <name> asm attaches Assembly to that workspace:
//   - horizontal-based layouts (classic, wide) → asm to the right of Code
//   - vertical-based layouts (panels, default) → asm below Code
func (a *DebuggerApp) OnLayout(args ...any) {
	name := layout.Wide
	withAsm := false
	if len(args) > 0 {
		if s, ok := args[0].(string); ok && strings.TrimSpace(s) != "" {
			name = strings.TrimSpace(s)
		}
	}
	if len(args) > 1 {
		if s, ok := args[1].(string); ok && isAsmLayoutType(s) {
			withAsm = true
		}
	}
	a.ApplyLayout(name)
	if withAsm {
		a.attachAsmForLayout(name)
	}
}

func isAsmLayoutType(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "asm" || s == "assembly"
}

// attachAsmForLayout places the dedicated asm leaf according to the named
// layout: classic/wide/panels put asm right of Code; default puts it below.
func (a *DebuggerApp) attachAsmForLayout(name string) {
	if a.layoutPrefersAsmRight(name) {
		a.SplitAsmRight()
		return
	}
	a.SplitAsmBelow()
}

// layoutPrefersAsmRight is true for layouts whose asm view should open to the
// right of Code rather than below it.
func (a *DebuggerApp) layoutPrefersAsmRight(name string) bool {
	return layout.AsmSplitRight(name)
}

func (a *DebuggerApp) layoutCompletions(prefix string) []string {
	fields := strings.Fields(prefix)
	trailingSpace := len(prefix) > 0 && (prefix[len(prefix)-1] == ' ' || prefix[len(prefix)-1] == '\t')

	// :layout <name> <asm|assembly>
	if len(fields) >= 1 && a.State().HasLayout(fields[0]) {
		switch {
		case len(fields) == 1 && trailingSpace:
			return []string{"asm", "assembly"}
		case len(fields) >= 2 && !trailingSpace:
			last := fields[len(fields)-1]
			var out []string
			for _, n := range []string{"asm", "assembly"} {
				if strings.HasPrefix(n, last) {
					out = append(out, n)
				}
			}
			return out
		case len(fields) >= 2 && trailingSpace:
			return nil
		}
	}

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
	case layout.Wide:
		return layout.BuildWide("wide", panes)
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
	if path := a.Debug().CurrentFile(); path != "" {
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
// Startup / current layout is wide.
func (a *DebuggerApp) registerLayouts() {
	for _, name := range []string{layout.Wide, layout.Panels, layout.Default, layout.Classic} {
		a.State().RegisterLayout(name)
	}
	a.State().SetCurrentLayout(layout.Wide)
}

// newStartupTab builds the initial wide workspace tab.
func (a *DebuggerApp) newStartupTab(code termui.Widget) *termui.TabWidget {
	return layout.BuildWide("wide", a.debugPanes(code))
}
