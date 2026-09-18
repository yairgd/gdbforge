package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/layout"
	"github.com/yairgd/termforge"
)

// ApplyLayout remounts the active tab with a freshly built named layout.
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
	lay := w.buildLayout(name)
	if lay == nil {
		if log := h.LogNamed("layout"); log != nil {
			log.Error("layout not implemented: " + name)
		}
		return
	}
	w.Tab().SetLayout(lay)
	w.finishLayoutApply(name)
}

func (w *LayoutShell) buildLayout(name string) *termforge.WidgetTree {
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
	lay := w.Layout()
	h.State().SetCurrentLayout(name)
	h.State().SetEqualAlways(true)
	// A freshly built layout carries none of the wiring done at startup, so
	// re-apply it here. Missing the resize hook left separator drags unable to
	// request a frame after the first :layout switch.
	lay.SetStatusClipboard(h.ClipboardIO())
	lay.SetOnResize(h.RequestFrame)
	lay.SetEqualAlways(true)
	lay.PinBottom(h.CmdWidget(), 1)
	lay.FocusWidget(h.GDBWidget())
	lay.SetLeafMark(leafMarkCode, lay.FindLeaf(isCodeSlot))
	lay.SetLeafMark(leafMarkGDB, lay.FindLeaf(func(wid termforge.Widget) bool { return wid == h.GDBWidget() }))
	h.EnterInsertMode()
	h.RequestFrame()
}
