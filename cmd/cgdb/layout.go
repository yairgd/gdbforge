package main

import (
	"strings"

	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// OnLayout applies a named workspace layout (:layout default).
func (a *DebuggerApp) OnLayout(args ...any) {
	name := platform.LayoutDefault
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

// ApplyLayout rebuilds the active tab tree for a registered layout name.
func (a *DebuggerApp) ApplyLayout(name string) {
	if a.tab == nil || !a.State().HasLayout(name) {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("layout").Error("unknown layout: " + name)
		}
		return
	}
	switch name {
	case platform.LayoutDefault:
		a.applyDefaultLayout()
	default:
		if a.ctx.Log != nil {
			a.ctx.Log.Named("layout").Error("layout not implemented: " + name)
		}
		return
	}
	a.State().SetCurrentLayout(name)
	a.RequestFrame()
}

func (a *DebuggerApp) applyDefaultLayout() {
	code := a.layoutCodeWidget()
	tw := termui.NewTabDefaultDebugLayout(
		"basic debugger",
		code,
		a.gdbWidget,
		a.outputWidget,
		a.bpWidget,
		a.threadWidget,
		a.callstackWidget,
		a.State().DefaultLayoutRatios(),
	)
	if tree := tw.ActiveTree(); tree != nil {
		a.tab.SetActiveTree(tree)
	}
	a.State().SetEqualAlways(true)
	a.tab.SetEqualAlways(true)
	a.tab.FocusWidget(a.gdbWidget)
	a.EnterInsertMode()
}

func (a *DebuggerApp) layoutCodeWidget() *widgets.CodeWidget {
	if path := a.State().CurrentFile(); path != "" {
		if w := a.ensureCodeBuffer(path); w != nil {
			return w
		}
	}
	if a.primaryCode != nil {
		return a.primaryCode
	}
	w := widgets.NewCodeWidget()
	w.PaneName = "[No Name]"
	w.SetClipboard(a.ClipboardIO())
	if a.gdbWidget != nil {
		w.SetPTY(a.gdbWidget.Session(), a.State())
	}
	a.primaryCode = w
	return w
}
