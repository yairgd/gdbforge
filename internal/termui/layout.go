package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Layout struct {
	BaseWidget
	grid *Grid
	rect Rect
	tree WidgetTree
}

func (l *Layout) NewSplit(dir SplitDir, w Widget) {
	l.tree.Split(dir, w)
}

func (l *Layout) HandleEvent(ev tcell.Event) {
	l.tree.HandleEvent(ev)
}

func (l *Layout) SetRect(r Rect) {
	l.rect = r
	l.grid = NewGrid(r.w, r.h)

}

func (l *Layout) Draw() {
	// draw outer frane
	l.grid.DrawHorizontal(0, 0, l.rect.w-0)
	l.grid.DrawHorizontal(l.rect.h-1, 0, l.rect.w-0)
	l.grid.DrawVertical(0, 0, l.rect.h-0)
	l.grid.DrawVertical(l.rect.w-1, 0, l.rect.h-0)

	// draw inner frame
	l.grid.Draw(l.uiContext.Screen(), tcell.StyleDefault)

	// l.tree.Draw()
}

func (l *Layout) BuildLayout(rect Rect) {
	l.tree.BuildLayout(rect, l.grid)

	// l.tree.Draw()
}

func NewLayout(widget Widget, rect Rect, uiContext UIContext) *Layout {
	layout := &Layout{
		BaseWidget: NewBaseWidget(uiContext),
		tree:       NewWidgetTree(widget),
	}
	layout.SetRect(rect)
	layout.grid = NewGrid(rect.w, rect.h)

	return layout

}
