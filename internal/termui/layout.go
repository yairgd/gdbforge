package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Layout struct {
	tree WidgetTree
}

func (l *Layout) NewSplit(dir SplitDir, w Widget) {
	l.tree.Split(dir, w)
}

func (l *Layout) HandleEvent(ev tcell.Event) {
	l.tree.HandleEvent(ev)
}

func (l *Layout) FocusLeft() {
	l.tree.FocusLeft()
}
func (l *Layout) FocusRight() {
	l.tree.FocusRight()
}

func (l *Layout) FocusUp() {
	l.tree.FocusUp()
}
func (l *Layout) FocusDown() {
	l.tree.FocusDown()
}

func (l *Layout) Draw(c Canvas) {
	l.tree.BuildLayout(c)
	l.tree.Draw(c)
}

func (l *Layout) BuildLayout(c Canvas) {

	l.tree.BuildLayout(c)
}

func NewLayout(widget Widget) *Layout {
	layout := &Layout{
		tree: NewWidgetTree(widget),
	}
	return layout
}
