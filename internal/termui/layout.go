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

func (l *Layout) Draw(c Canvas) {
	// draw outer frane
	//l.grid.DrawHorizontal(0, 0, l.rect.w-0, false)
	//	l.grid.DrawHorizontal(l.rect.h-1, 0, l.rect.w, false)
	//	l.grid.DrawVertical(0, 0, l.rect.h-0, false)
	//	l.grid.DrawVertical(l.rect.w-1, 0, l.rect.h-0, false)

	// draw inner frame
	//	l.grid.Draw(c.screen, tcell.StyleDefault)
	l.tree.Draw(c)

	//l.uiContext.Screen().Show()
}

func (l *Layout) BuildLayout(c Canvas) {

	l.tree.BuildLayout(c)
}

func NewLayout(widget Widget) *Layout {
	layout := &Layout{
		tree: NewWidgetTree(widget),
	}
	//	layout.SetRect(rect)
	//	layout.grid = NewGrid(rect.w, rect.h)

	return layout

}
