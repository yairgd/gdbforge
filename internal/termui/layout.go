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

func (l *Layout) FocusAt(x, y int) bool {
	return l.tree.FocusAt(x, y)
}

func (l *Layout) IsSeparatorAt(x, y int) bool {
	return l.tree.findSeparator(x, y) != nil
}

func (l *Layout) IsDraggingSeparator() bool {
	return l.tree.IsDraggingSeparator()
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

func (l *Layout) DeleteFocus() bool {
	return l.tree.DeleteFocus()
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

func (l *Layout) SetOnResize(fn func()) {
	l.tree.SetOnResize(fn)
}

func (l *Layout) SetInsertActive(active bool) {
	l.tree.SetInsertActive(active)
}

func (l *Layout) FocusedWidget() Widget {
	return l.tree.FocusedWidget()
}
