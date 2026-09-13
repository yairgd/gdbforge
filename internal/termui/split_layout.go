package termui

//
// Layout
//

// Layout is tab content that owns its own window arrangement.
//
// A Tab is a container for layouts of any kind and knows nothing beyond this
// interface: it calls BuildLayout to hand down the assigned canvas, then Draw,
// and routes events through HandleEvent. SplitLayout is the Vim-style tiling
// implementation; a future layout (a different window arrangement, a single
// full-tab form, a whole embedded app) implements the same interface.
//
// Do not add Tab methods that forward into a Layout. Code needing
// arrangement-specific operations holds the concrete layout instead.
type Layout interface {
	Widget // HandleEvent, Draw, DrawStatusLine

	// BuildLayout computes geometry for the assigned canvas. Called before
	// Draw on every frame.
	BuildLayout(c Canvas)
}

//
// SplitLayout
//

// SplitLayout is the tiling Layout: panes separated by draggable splits, with
// Vim-style focus navigation and named leaf marks.
//
// It embeds *WidgetTree rather than wrapping it, so every tree operation stays
// reachable on the layout with no forwarding code (lay.FocusLeft(),
// lay.SetLeafMark(...), lay.Split(...)).
type SplitLayout struct {
	*WidgetTree
}

// NewSplitLayout creates a single-pane tiling layout showing root.
func NewSplitLayout(root Widget) *SplitLayout {
	return &SplitLayout{WidgetTree: NewWidgetTree(root)}
}

// DrawStatusLine is a no-op: every pane paints its own status row inside Draw.
// Present only to satisfy Widget, which WidgetTree does not implement.
func (l *SplitLayout) DrawStatusLine(c Canvas, active bool) {}
