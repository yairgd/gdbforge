package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/layout"
	"github.com/yairgd/gdbforge/internal/termui"
)

// ApplyLayout rebuilds the active tab tree for a registered layout name.
func (w *LayoutShell) ApplyLayout(name string) {
	h := w.host
	if w == nil || h == nil || w.Tab() == nil || !h.State().HasLayout(name) {
		if w != nil && h != nil {
			if log := h.LogNamed("layout"); log != nil {
				log.Error("unknown layout: " + name)
			}
		}
		return
	}
	tree := w.buildLayoutTree(name)
	if tree == nil {
		if log := h.LogNamed("layout"); log != nil {
			log.Error("layout not implemented: " + name)
		}
		return
	}
	w.Tab().SetActiveTree(tree)
	w.finishLayoutApply(name)
}

func (w *LayoutShell) buildLayoutTree(name string) *termui.WidgetTree {
	h := w.host
	code := h.LayoutCodePane()
	panes := h.DebugPanes(code)
	switch name {
	case layout.Default:
		return layout.BuildDefault(panes, h.State().DefaultLayoutRatios())
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

func (w *LayoutShell) finishLayoutApply(name string) {
	h := w.host
	tab := w.Tab()
	h.State().SetCurrentLayout(name)
	h.State().SetEqualAlways(true)
	tab.SetEqualAlways(true)
	tab.FocusWidget(h.GDBWidget())
	tab.SetLeafMark(leafMarkCode, tab.FindLeaf(isCodeSlot))
	tab.SetLeafMark(leafMarkGDB, tab.FindLeaf(func(wid termui.Widget) bool { return wid == h.GDBWidget() }))
	h.EnterInsertMode()
	h.RequestFrame()
}
