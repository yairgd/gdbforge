package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/layout"
	"github.com/yairgd/gdbforge/internal/termui"
)

// ApplyLayout rebuilds the active tab tree for a registered layout name.
func (w *Workspace) ApplyLayout(name string) {
	if w == nil || w.app == nil || w.Tab() == nil || !w.app.State().HasLayout(name) {
		if w != nil && w.app != nil && w.app.ctx.Log != nil {
			w.app.ctx.Log.Named("layout").Error("unknown layout: " + name)
		}
		return
	}
	tree := w.buildLayoutTree(name)
	if tree == nil {
		if w.app.ctx.Log != nil {
			w.app.ctx.Log.Named("layout").Error("layout not implemented: " + name)
		}
		return
	}
	w.Tab().SetActiveTree(tree)
	w.finishLayoutApply(name)
}

func (w *Workspace) buildLayoutTree(name string) *termui.WidgetTree {
	a := w.app
	code := a.layoutCodePane()
	panes := a.debugPanes(code)
	switch name {
	case layout.Default:
		return layout.BuildDefault(panes, a.State().DefaultLayoutRatios())
	case layout.Panels:
		return layout.BuildPanels(panes)
	case layout.Classic:
		return layout.BuildClassic(panes)
	case layout.Wide:
		return layout.BuildWide(panes)
	default:
		return nil
	}
}

func (w *Workspace) finishLayoutApply(name string) {
	a := w.app
	tab := w.Tab()
	a.State().SetCurrentLayout(name)
	a.State().SetEqualAlways(true)
	tab.SetEqualAlways(true)
	tab.FocusWidget(a.gdbWidget)
	tab.SetLeafMark(leafMarkCode, tab.FindLeaf(isCodeSlot))
	tab.SetLeafMark(leafMarkGDB, tab.FindLeaf(func(wid termui.Widget) bool { return wid == a.gdbWidget }))
	a.EnterInsertMode()
	a.RequestFrame()
}
