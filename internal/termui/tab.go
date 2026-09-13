package termui

import (
	tcell "github.com/gdamore/tcell/v2"
)

//
// Tab model
//

// Tab is chrome for one tab entry: a title plus a Layout.
//
// Tab is a generic container. It hosts any Layout — the split tree today, any
// future layout tomorrow — and deliberately exposes no operation of its own on
// that content. Callers needing layout-specific behaviour take the Layout (or
// the concrete layout type they built) and work on it directly.
type Tab struct {
	Title   string
	Content Layout
}

//
// TabWidget
//

// TabWidget manages a list of Tabs and forwards Draw/HandleEvent to the active
// tab's Layout.
//
// Current implementation is intentionally degenerate:
//   - Always exactly one tab
//   - No tab switching
//   - No tab header rendering
//
// Do not add methods here that forward into the Layout. Hand out the Layout
// and let the caller drive it.
type TabWidget struct {
	Widget

	tabs   []Tab
	active int
}

//
// Constructors
//

// NewTabWidget creates a TabWidget with a single tab showing content.
func NewTabWidget(title string, content Layout) *TabWidget {
	return &TabWidget{
		tabs: []Tab{
			{
				Title:   title,
				Content: content,
			},
		},
		active: 0,
	}
}

// NewTabTwoHozSplitWins creates a tab with a horizontal split: top over bottom.
func NewTabTwoHozSplitWins(title string, top Widget, bottom Widget) *TabWidget {
	lay := NewSplitLayout(top)
	lay.Split(Horizontal, bottom)
	return NewTabWidget(title, lay)
}

//
// Content access
//

// Layout returns the active tab's content. Callers that need arrangement
// specific operations keep the concrete layout they built, or type-assert.
func (t *TabWidget) Layout() Layout {
	if t == nil || len(t.tabs) == 0 {
		return nil
	}
	return t.tabs[t.active].Content
}

// SetLayout replaces the active tab's content (layout apply / remount).
func (t *TabWidget) SetLayout(content Layout) {
	if t == nil || len(t.tabs) == 0 || content == nil {
		return
	}
	t.tabs[t.active].Content = content
}

//
// Widget
//

// HandleEvent forwards the event to the active tab's layout.
func (t *TabWidget) HandleEvent(ev tcell.Event) {
	if l := t.Layout(); l != nil {
		l.HandleEvent(ev)
	}
}

// Draw lays out and paints the active tab's layout using the full assigned
// rect. Apps own chrome banding (cmdline / wildmenu) via HandleResize.
func (t *TabWidget) Draw(c Canvas) {
	l := t.Layout()
	if l == nil {
		return
	}
	l.BuildLayout(c)
	l.Draw(c)
}

func (t *TabWidget) DrawStatusLine(c Canvas, active bool) {}
