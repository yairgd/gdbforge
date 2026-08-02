package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// CompletionView paints a CompletionMenu snapshot. CompletionBarWidget is the
// default chrome row; a list window could implement the same interface later.
type CompletionView interface {
	SetItems(names []string, selected int)
	Clear()
	Active() bool
}

// CompletionBarWidget is chrome (not a WidgetTree leaf): one row above the
// command line. It only paints items + selection from CompletionMenu via
// SetItems — no selection ownership.
type CompletionBarWidget struct {
	BaseWidget
	names    []string
	selected int
	start    int // first visible candidate (horizontal roll)
}

var _ CompletionView = (*CompletionBarWidget)(nil)

func NewCompletionBarWidget(ctx platform.AppContext) *CompletionBarWidget {
	return &CompletionBarWidget{
		BaseWidget: NewBaseWidget(ctx),
	}
}

// SetItems replaces the painted candidate row. selected is clamped.
func (w *CompletionBarWidget) SetItems(names []string, selected int) {
	w.names = append([]string(nil), names...)
	w.selected = selected
	if w.selected < 0 {
		w.selected = 0
	}
	if n := len(w.names); n > 0 && w.selected >= n {
		w.selected = n - 1
	}
	w.start = 0
}

// Active reports whether the bar has candidates to paint.
func (w *CompletionBarWidget) Active() bool {
	return len(w.names) > 0
}

// Clear hides the wildmenu row.
func (w *CompletionBarWidget) Clear() {
	w.names = nil
	w.selected = 0
	w.start = 0
}

func (w *CompletionBarWidget) HandleEvent(ev tcell.Event) {
	// Navigation is owned by CompletionMenu + ModeCompletion keys.
}

func (w *CompletionBarWidget) DrawStatusLine(c Canvas, active bool) {}

func nameWidth(name string) int {
	return len([]rune(name))
}

func (w *CompletionBarWidget) endCol(start, idx int) int {
	if idx < start || idx >= len(w.names) {
		return 0
	}
	x := 0
	for i := start; i <= idx; i++ {
		if i > start {
			x++ // spacer
		}
		x += nameWidth(w.names[i])
	}
	return x
}

func (w *CompletionBarWidget) ensureSelectedVisible(width int) {
	n := len(w.names)
	if n == 0 || width <= 0 {
		w.start = 0
		return
	}
	if w.selected < 0 {
		w.selected = 0
	}
	if w.selected >= n {
		w.selected = n - 1
	}
	if w.start < 0 || w.start >= n {
		w.start = 0
	}
	if w.selected < w.start {
		w.start = w.selected
	}
	for w.start < w.selected && w.endCol(w.start, w.selected) > width {
		w.start++
	}
}

func (w *CompletionBarWidget) Draw(c Canvas) {
	if !w.Active() {
		return
	}

	bg := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	sel := tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack).Bold(true)
	c.ClearLine(0, bg)

	width := c.W()
	w.ensureSelectedVisible(width)

	x := 0
	for i := w.start; i < len(w.names); i++ {
		if x >= width {
			break
		}
		st := bg
		if i == w.selected {
			st = sel
		}
		if i > w.start {
			if x+1 >= width {
				break
			}
			c.Print(x, 0, bg, " ")
			x++
		}
		runes := []rune(w.names[i])
		remain := width - x
		if remain <= 0 {
			break
		}
		chunk := w.names[i]
		truncated := false
		if len(runes) > remain {
			truncated = true
			if remain <= 1 {
				c.Print(x, 0, st, "…")
				break
			}
			chunk = string(runes[:remain-1]) + "…"
			runes = []rune(chunk)
		}
		c.Print(x, 0, st, chunk)
		x += len(runes)
		if truncated {
			break
		}
	}
}
