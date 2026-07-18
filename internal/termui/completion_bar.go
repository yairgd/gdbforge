package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
)

// CompletionBarWidget is chrome (not a WidgetTree leaf): one row above the
// command line, fed by CompletionMsg, drawn after TabWidget to overlay that row.
type CompletionBarWidget struct {
	BaseWidget
	names    []string
	selected int
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
}

func (w *CompletionBarWidget) HandleEvent(ev tcell.Event) {
	if e, ok := ev.(*tcell.EventKey); ok {
		_ = w.HandleBoundKey(e)
	}
}

func (w *CompletionBarWidget) DrawStatusLine(c Canvas, active bool) {}

func (w *CompletionBarWidget) Draw(c Canvas) {
	// Only paint while wildmenu is active; otherwise leave the pane status line alone.
	if !w.Active() {
		return
	}

	bg := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	sel := tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack).Bold(true)
	c.ClearLine(0, bg)

	x := 0
	width := c.W()
	for i, name := range w.names {
		if x >= width {
			break
		}
		st := bg
		if i == w.selected {
			st = sel
		}
		if i > 0 {
			if x+1 >= width {
				break
			}
			c.Print(x, 0, bg, " ")
			x++
		}
		runes := []rune(name)
		remain := width - x
		if remain <= 0 {
			break
		}
		chunk := name
		if len(runes) > remain {
			if remain <= 1 {
				c.Print(x, 0, st, "…")
				break
			}
			chunk = string(runes[:remain-1]) + "…"
			runes = []rune(chunk)
		}
		c.Print(x, 0, st, chunk)
		x += len(runes)
	}
}
