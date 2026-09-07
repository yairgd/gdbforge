package demo

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// ScrollPane is a read-only scrollable text pane for cmd/demo.
type ScrollPane struct {
	termui.BaseWidget
	doc *termui.ScrollDocument
	buf *platform.Buffer
}

// NewScrollPane builds a pane with optional initial lines.
func NewScrollPane(name string, lines ...string) *ScrollPane {
	buf := platform.NewBuffer()
	doc := termui.NewScrollDocument(buf)
	doc.SetFollowTail(true)
	doc.SetReadOnly(true)
	doc.SetCursorVisible(false)
	for _, line := range lines {
		buf.AppendLine(line)
	}
	return &ScrollPane{
		BaseWidget: termui.BaseWidget{PaneName: name},
		doc:        doc,
		buf:        buf,
	}
}

func (p *ScrollPane) Buffer() *platform.Buffer {
	if p == nil {
		return nil
	}
	return p.buf
}

func (p *ScrollPane) SetClipboard(io termui.ClipboardIO) {
	if p != nil && p.doc != nil {
		p.doc.SetClipboard(io)
	}
}

func (p *ScrollPane) SetMouseOrigin(screenX, screenY int) {
	if p != nil && p.doc != nil {
		p.doc.SetMouseOrigin(screenX, screenY)
	}
}

func (p *ScrollPane) Clear() {
	if p == nil || p.buf == nil {
		return
	}
	p.buf.Clear()
	if p.doc != nil {
		p.doc.Home()
		p.doc.SetFollowTail(true)
	}
}

func (p *ScrollPane) SetFocused(focused bool) {
	p.BaseWidget.SetFocused(focused)
	if p.doc != nil {
		p.doc.SetCursorVisible(false)
	}
}

func (p *ScrollPane) Draw(c termui.Canvas) {
	if p == nil || p.doc == nil {
		return
	}
	p.doc.SetMouseOrigin(c.ScreenX(0), c.ScreenY(0))
	p.doc.Draw(c)
}

func (p *ScrollPane) DrawStatusLine(c termui.Canvas, active bool) {
	p.BaseWidget.DrawStatusLine(c, active)
}

func (p *ScrollPane) HandleEvent(ev tcell.Event) {
	if p == nil || p.doc == nil {
		return
	}
	switch e := ev.(type) {
	case *tcell.EventMouse:
		p.doc.HandleEvent(e)
	case *tcell.EventKey:
		p.doc.HandleEvent(e)
	}
}

func (p *ScrollPane) HandleFocusKey(ev *tcell.EventKey) bool {
	if p == nil || p.doc == nil || ev == nil {
		return false
	}
	p.doc.HandleEvent(ev)
	return true
}
