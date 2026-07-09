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
	layout *Layout
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
	Widget

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
) *TabWidget {
	return &TabWidget{
		tabs: []Tab{
			{
				Title:  title,
				layout: layout,
			},
		},
		active: 0,
	}
}

// HandleEvent forwards the event to the active tab widget.
func (t *TabWidget) HandleEvent(ev tcell.Event) {
	if len(t.tabs) == 0 {
		return
	}

	active := t.tabs[t.active].layout

	if active != nil {
		active.HandleEvent(ev)
	}
}

func (t *TabWidget) FocusAt(x, y int) bool {
	if len(t.tabs) == 0 {
		return false
	}

	layout := t.tabs[t.active].layout
	if layout == nil {
		return false
	}

	return layout.FocusAt(x, y)
}

func (t *TabWidget) IsSeparatorAt(x, y int) bool {
	if len(t.tabs) == 0 {
		return false
	}
	layout := t.tabs[t.active].layout
	if layout == nil {
		return false
	}
	return layout.IsSeparatorAt(x, y)
}

// Draw forwards the draw call to the active tab widget.
// The screen is owned by the application and passed in.
func (t *TabWidget) Draw(c Canvas) {
	if len(t.tabs) == 0 {
		return
	}

	layout := t.tabs[t.active].layout

	if layout != nil {
		r := c.Rect()
		layout.Draw(c.WithRect(NewRect(r.X(), r.Y(), r.W(), r.H()-2)))
	}
}

func (t *TabWidget) DrawStatusLine(c Canvas, active bool) {}

//
// Helper
//

// ActiveWidget returns the currently active widget.
// Useful for future features like focus or tab switching.
func (t *TabWidget) ActiveLayout() *Layout {
	if len(t.tabs) == 0 {
		return nil
	}

	return t.tabs[t.active].layout
}

func (t *TabWidget) SetOnResize(fn func()) {
	if layout := t.ActiveLayout(); layout != nil {
		layout.SetOnResize(fn)
	}
}

func (t *TabWidget) VerticalSplit(w Widget) {
	t.tabs[t.active].layout.NewSplit(Vertical, w)

}

func (t *TabWidget) HorizontalSplit(w Widget) {
	t.tabs[t.active].layout.NewSplit(Horizontal, w)

}

func (t *TabWidget) FocusLeft() {
	t.tabs[t.active].layout.FocusLeft()

}

func (t *TabWidget) FocusRight() {
	t.tabs[t.active].layout.FocusRight()

}

func (t *TabWidget) FocusUp() {
	t.tabs[t.active].layout.FocusUp()

}

func (t *TabWidget) FocusDown() {
	t.tabs[t.active].layout.FocusDown()

}

func (t *TabWidget) DeleteFocus() bool {
	return t.tabs[t.active].layout.DeleteFocus()

}

// NewTab creates a new tab with an initial horizontal split layout.
// The layout contains two widgets: top and bottom.
func NewTabTwoHozSplitWins(title string, top Widget, bottom Widget) *TabWidget {

	//w, h := canvas.grid.W, canvas.grid.H
	//	rect := Rect{0, 0, w, h - 2}
	// place the node insde layout
	layout := NewLayout(top) //, rect, canvas)
	//	layout.SetRect(rect)
	// split it to another node with bottom widget

	//	layout.NewSplit(Horizontal, top)
	//	layout.NewSplit(Vertical, top)
	//	layout.NewSplit(Vertical, top)
	//	layout.NewSplit(Horizontal, top)
	//	layout.NewSplit(Vertical, bottom)

	//	layout.NewSplit(Horizontal, bottom)
	//layout.NewSplit(Vertical, bottom)

	//layout.NewSplit(Vertical, bottom)
	//layout.NewSplit(Horizontal, bottom)
	//layout.NewSplit(Vertical, bottom)
	//	layout.NewSplit(Horizontal, bottom)
	//	layout.NewSplit(Vertical, top)
	//	layout.NewSplit(Horizontal, bottom)

	//layout.BuildLayout(canvas)

	return &TabWidget{
		tabs: []Tab{
			{
				Title:  title,
				layout: layout,
			},
		},
		active: 0,
	}

}
