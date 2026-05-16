package termui

import (
	tcell "github.com/gdamore/tcell/v2"
)

//
// Tab model
//

// Tab represents a single tab entry.
// For now, we always have exactly one tab.
type Tab struct {
	Title  string
	Layout *Layout
}

//
// TabWidget
//

// TabWidget is a simple container that forwards
// events and drawing to the active tab widget.
//
// Current implementation is intentionally degenerate:
//   - Always exactly one tab
//   - No tab switching
//   - No tab header rendering
//
// This keeps the architecture ready for future
// multi-tab support without adding complexity now.
type TabWidget struct {
	BaseWidget

	tabs   []Tab
	active int
}

//
// Constructor
//

// NewTabWidget creates a TabWidget with a single tab.
// This is the default behavior for now.
func NewTabWidget(
	title string,
	layout *Layout,
	uiContext UIContext,
) *TabWidget {
	return &TabWidget{
		BaseWidget: NewBaseWidget(uiContext),
		tabs: []Tab{
			{
				Title:  title,
				Layout: layout,
			},
		},
		active: 0,
	}
}

//
// Event handling
//

// HandleEvent forwards the event to the active tab widget.
func (t *TabWidget) HandleEvent(ev tcell.Event) {
	if len(t.tabs) == 0 {
		return
	}

	active := t.tabs[t.active].Layout

	if active != nil {
		active.HandleEvent(ev)
	}
}

//
// Size management
//

// SetSize updates the container size
// and forwards the size to the active widget.
func (t *TabWidget) SetSize(w, h int) {
	t.BaseWidget.SetSize(w, h)

	if len(t.tabs) == 0 {
		return
	}

	active := t.tabs[t.active].Layout

	if active != nil {
		active.SetSize(w, h)
	}
}

//
// Drawing
//

// Draw forwards the draw call to the active tab widget.
// The screen is owned by the application and passed in.
func (t *TabWidget) Draw(r Rect) {
	if len(t.tabs) == 0 {
		return
	}

	layout := t.tabs[t.active].Layout

	if layout != nil {
		layout.Draw(r)
	}
}

//
// Helper
//

// ActiveWidget returns the currently active widget.
// Useful for future features like focus or tab switching.
func (t *TabWidget) ActiveLayout() *Layout {
	if len(t.tabs) == 0 {
		return nil
	}

	return t.tabs[t.active].Layout
}

// NewTab creates a new tab with an initial horizontal split layout.
// The layout contains two widgets: top and bottom.
func NewTabTwoHozSplitWins(uiContext UIContext, title string, top Widget, bottom Widget) *TabWidget {

	node := &Node{
		Type:   NodeLeaf,
		Widget: top,
	}
	// place the node insde layout
	layout := NewLayout(node, uiContext)
	// split it to another node with bottom widget
	layout.NewSplit(Horizontal, bottom)

	return &TabWidget{
		tabs: []Tab{
			{
				Title:  title,
				Layout: layout,
			},
		},
		active: 0,
	}

}
