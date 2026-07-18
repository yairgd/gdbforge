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
	Title string
	tree  *WidgetTree
}

//
// TabWidget
//

// TabWidget is a simple container that forwards
// events and drawing to the active tab's WidgetTree.
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
	tree *WidgetTree,
) *TabWidget {
	return &TabWidget{
		tabs: []Tab{
			{
				Title: title,
				tree:  tree,
			},
		},
		active: 0,
	}
}

// HandleEvent forwards the event to the active tab tree.
func (t *TabWidget) HandleEvent(ev tcell.Event) {
	if tree := t.ActiveTree(); tree != nil {
		tree.HandleEvent(ev)
	}
}

func (t *TabWidget) FocusAt(x, y int) bool {
	if tree := t.ActiveTree(); tree != nil {
		return tree.FocusAt(x, y)
	}
	return false
}

func (t *TabWidget) IsSeparatorAt(x, y int) bool {
	if tree := t.ActiveTree(); tree != nil {
		return tree.findSeparator(x, y) != nil
	}
	return false
}

// Draw forwards the draw call to the active tab tree.
func (t *TabWidget) Draw(c Canvas) {
	tree := t.ActiveTree()
	if tree == nil {
		return
	}
	r := c.Rect()
	inner := c.WithRect(NewRect(r.X(), r.Y(), r.W(), r.H()-2))
	tree.BuildLayout(inner)
	tree.Draw(inner)
}

func (t *TabWidget) DrawStatusLine(c Canvas, active bool) {}

//
// Helper
//

// ActiveTree returns the WidgetTree of the currently selected tab.
func (t *TabWidget) ActiveTree() *WidgetTree {
	if len(t.tabs) == 0 {
		return nil
	}
	return t.tabs[t.active].tree
}

// FocusedWidget returns the focused leaf widget in the active tab.
func (t *TabWidget) FocusedWidget() Widget {
	if tree := t.ActiveTree(); tree != nil {
		return tree.FocusedWidget()
	}
	return nil
}

// FocusWidget focuses the leaf showing w (safe before first layout).
func (t *TabWidget) FocusWidget(w Widget) bool {
	if tree := t.ActiveTree(); tree != nil {
		return tree.FocusWidget(w)
	}
	return false
}

// ReplaceFocusedWidget replaces the widget shown in the focused window.
// Does not split, create panes, or change tree geometry.
func (t *TabWidget) ReplaceFocusedWidget(w Widget) bool {
	if tree := t.ActiveTree(); tree != nil {
		return tree.ReplaceFocusedWidget(w)
	}
	return false
}

// ReplaceMatchingLeafWidget replaces a non-focused leaf matching match (see WidgetTree).
func (t *TabWidget) ReplaceMatchingLeafWidget(w Widget, match func(Widget) bool) bool {
	if tree := t.ActiveTree(); tree != nil {
		return tree.ReplaceMatchingLeafWidget(w, match)
	}
	return false
}

func (t *TabWidget) SetEqualAlways(v bool) {
	if tree := t.ActiveTree(); tree != nil {
		tree.SetEqualAlways(v)
	}
}

func (t *TabWidget) EqualAlways() bool {
	if tree := t.ActiveTree(); tree != nil {
		return tree.EqualAlways()
	}
	return false
}

func (t *TabWidget) SetOnResize(fn func()) {
	if tree := t.ActiveTree(); tree != nil {
		tree.SetOnResize(fn)
	}
}

func (t *TabWidget) SetInsertActive(active bool) {
	if tree := t.ActiveTree(); tree != nil {
		tree.SetInsertActive(active)
	}
}

func (t *TabWidget) VerticalSplit(w Widget) {
	if tree := t.ActiveTree(); tree != nil {
		tree.Split(Vertical, w)
	}
}

func (t *TabWidget) HorizontalSplit(w Widget) {
	if tree := t.ActiveTree(); tree != nil {
		tree.Split(Horizontal, w)
	}
}

func (t *TabWidget) FocusLeft() {
	if tree := t.ActiveTree(); tree != nil {
		tree.FocusLeft()
	}
}

func (t *TabWidget) FocusRight() {
	if tree := t.ActiveTree(); tree != nil {
		tree.FocusRight()
	}
}

func (t *TabWidget) FocusUp() {
	if tree := t.ActiveTree(); tree != nil {
		tree.FocusUp()
	}
}

func (t *TabWidget) FocusDown() {
	if tree := t.ActiveTree(); tree != nil {
		tree.FocusDown()
	}
}

func (t *TabWidget) DeleteFocus() bool {
	if tree := t.ActiveTree(); tree != nil {
		return tree.DeleteFocus()
	}
	return false
}

// NewTabTwoHozSplitWins creates a tab with a horizontal split: top over bottom.
func NewTabTwoHozSplitWins(title string, top Widget, bottom Widget) *TabWidget {
	tree := NewWidgetTree(top)
	tree.Split(Horizontal, bottom)
	return &TabWidget{
		tabs: []Tab{
			{
				Title: title,
				tree:  tree,
			},
		},
		active: 0,
	}
}
