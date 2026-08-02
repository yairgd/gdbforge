// Package layout builds named debugger workspace trees (geometry only).
// Layout-specific key policy lives in cmd/gdbforge, not here.
package layout

import (
	"github.com/yairgd/gdbforge/internal/termui"
)

// Named layouts registered with AppState / :layout.
const (
	Default = "default"
	Panels  = "panels"
	Classic = "classic"
	Wide    = "wide"
)

// AsmSplitRight reports whether :layout <name> asm should place Assembly to
// the right of Code (vs below Code).
func AsmSplitRight(name string) bool {
	switch name {
	case Classic, Wide, Panels:
		return true
	default:
		return false
	}
}

// Panes are the singleton widgets a layout may place. Unused fields may be nil
// (e.g. Classic ignores Output / list panes).
type Panes struct {
	Code        termui.Widget
	GDB         termui.Widget
	Output      termui.Widget
	Breakpoints termui.Widget
	Threads     termui.Widget
	Callstack   termui.Widget
}

// Spec builds a WidgetTree for one named layout.
type Spec interface {
	Name() string
	Build(panes Panes) *termui.WidgetTree
}

func clampRatio(r float64) float64 {
	if r < 0.1 {
		return 0.1
	}
	if r > 0.9 {
		return 0.9
	}
	return r
}
