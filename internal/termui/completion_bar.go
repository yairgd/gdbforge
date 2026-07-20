package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbx/internal/platform"
)

// CompletionBarWidget is chrome (not a WidgetTree leaf): one row above the
// command line, fed by CompletionMsg, drawn after TabWidget to overlay that row.
type CompletionBarWidget struct {
	BaseWidget
	names    []string
	selected int
	start    int // first visible candidate (horizontal roll)
}

func NewCompletionBarWidget(ctx platform.AppContext) *CompletionBarWidget {
	w := &CompletionBarWidget{
		BaseWidget: NewBaseWidget(ctx),
	}
	if ctx.Bus != nil {
		platform.Subscribe(ctx.Bus, w.onCompletion)
	}
	w.initKeyBindings()
	return w
}

func (w *CompletionBarWidget) initKeyBindings() {
	w.BindKeyFunc("prev", func(args ...any) { w.move(-1) }, "<Left>", "<Up>")
	w.BindKeyFunc("next", func(args ...any) { w.move(1) }, "<Right>", "<Down>")
}

func (w *CompletionBarWidget) onCompletion(msg CompletionMsg) {
	if len(msg.Names) <= 1 {
		w.Clear()
		return
	}
	w.names = append([]string(nil), msg.Names...)
	w.selected = 0
	w.start = 0
}

// MoveSelection steps the wildmenu highlight by delta (wraps).
func (w *CompletionBarWidget) MoveSelection(delta int) {
	w.move(delta)
}

func (w *CompletionBarWidget) move(delta int) {
	n := len(w.names)
	if n == 0 {
		return
	}
	w.selected = (w.selected + delta%n + n) % n
}

// Active reports whether the wildmenu has candidates.
func (w *CompletionBarWidget) Active() bool {
	return len(w.names) > 0
}

// Selected returns the highlighted completion name, or "".
func (w *CompletionBarWidget) Selected() string {
	if w.selected < 0 || w.selected >= len(w.names) {
		return ""
	}
	return w.names[w.selected]
}

// Clear hides the wildmenu.
func (w *CompletionBarWidget) Clear() {
	w.names = nil
	w.selected = 0
	w.start = 0
}

func (w *CompletionBarWidget) HandleEvent(ev tcell.Event) {
	if e, ok := ev.(*tcell.EventKey); ok {
		_ = w.HandleBoundKey(e)
	}
}

func (w *CompletionBarWidget) DrawStatusLine(c Canvas, active bool) {}

// nameWidth is the display width of a completion token (runes).
func nameWidth(name string) int {
	return len([]rune(name))
}

// endCol returns the exclusive column of names[idx] when drawing from start.
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

// ensureSelectedVisible rolls start left/right so the selected item fits in width.
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
	// Roll left (increase start) until selected ends within width, or start
	// catches up to selected (single overlong token).
	for w.start < w.selected && w.endCol(w.start, w.selected) > width {
		w.start++
	}
}

func (w *CompletionBarWidget) Draw(c Canvas) {
	// Only paint while wildmenu is active; otherwise leave the pane status line alone.
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
