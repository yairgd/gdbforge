package widgets

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// FileListHost receives file-list intents from FileListWidget.
type FileListHost interface {
	OpenSourcePath(path string)
}

// FileListWidget shows GDB project source files (j/k selection; Enter opens).
// Mouse: first click selects (blue mark); second click on the marked row opens.
type FileListWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer
	state    *debugstate.State

	paths    []string
	selected int

	host FileListHost
}

func NewFileListWidget(host FileListHost) *FileListWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)

	w := &FileListWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Files"},
		viewport:   vp,
		buf:        buf,
		host:       host,
	}
	vp.RowStyle = w.rowStyle
	vp.SetOnSearchJump(func(lineIdx int) {
		w.viewport.CursorLine = lineIdx
		w.syncSelectedFromViewport()
	})
	w.initKeyBindings()
	w.rebuild()
	return w
}

// SetHost replaces the file-list host (tests).
func (w *FileListWidget) SetHost(host FileListHost) {
	w.host = host
}

func (w *FileListWidget) SetAppState(st *debugstate.State) {
	w.state = st
}

func (w *FileListWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1) }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.viewport.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.viewport.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("open", func(args ...any) { w.openSelected() }, "<Enter>")
}

func (w *FileListWidget) markColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkColor()
	}
	return platform.DefaultMarkColor
}

func (w *FileListWidget) markDimColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkDimColor()
	}
	return platform.DefaultMarkDimColor
}

func (w *FileListWidget) mutedColor() tcell.Color {
	if w.state != nil {
		return w.state.MutedColor()
	}
	return platform.DefaultMutedColor
}

func (w *FileListWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.paths) == 0 {
		return st.Foreground(w.mutedColor())
	}
	if lineIdx == w.selected {
		bg := w.markDimColor()
		if w.Focused() {
			bg = w.markColor()
		}
		_ = line
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	_ = line
	return st
}

func (w *FileListWidget) move(delta int) {
	n := len(w.paths)
	if n == 0 {
		return
	}
	w.selected = (w.selected + delta%n + n) % n
	w.viewport.CursorLine = w.selected
	w.viewport.CursorCol = 0
	w.viewport.EnsureLineVisible()
}

func (w *FileListWidget) clampLine(line int) int {
	n := len(w.paths)
	if n == 0 {
		return 0
	}
	if line < 0 {
		return 0
	}
	if line >= n {
		return n - 1
	}
	return line
}

func (w *FileListWidget) syncSelectedFromViewport() {
	if len(w.paths) == 0 {
		return
	}
	line := w.clampLine(w.viewport.CursorLine)
	w.selected = line
	w.viewport.CursorLine = line
}

func (w *FileListWidget) openSelected() {
	if len(w.paths) == 0 || w.host == nil {
		return
	}
	if w.selected < 0 || w.selected >= len(w.paths) {
		return
	}
	w.host.OpenSourcePath(w.paths[w.selected])
}

// SetItems replaces the file list and rebuilds the viewport.
func (w *FileListWidget) SetItems(paths []string) {
	w.paths = append([]string(nil), paths...)
	if w.selected >= len(w.paths) {
		w.selected = len(w.paths) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
	w.rebuild()
}

func (w *FileListWidget) rebuild() {
	w.buf.Clear()
	w.viewport.Left = 0
	if len(w.paths) == 0 {
		w.buf.AppendLine("no files")
		w.viewport.CursorLine = 0
		return
	}
	for _, p := range w.paths {
		w.buf.AppendLine(p)
	}
	w.viewport.CursorLine = w.selected
	w.viewport.CursorCol = 0
	w.viewport.EnsureCursorVisible()
}

func (w *FileListWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *FileListWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if e.Buttons()&tcell.ButtonPrimary == 0 {
			w.viewport.HandleEvent(e)
			return
		}
		prev := w.selected
		w.viewport.HandleEvent(e)
		line := w.clampLine(w.viewport.CursorLine)
		w.selected = line
		w.viewport.CursorLine = line
		// Open only on a second click when the blue mark is already on this row.
		if line == prev {
			w.openSelected()
		}
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.viewport.HandleEvent(e)
	}
}

func (w *FileListWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(false)
}

func (w *FileListWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

func (w *FileListWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *FileListWidget) Paths() []string {
	return append([]string(nil), w.paths...)
}

func (w *FileListWidget) Viewport() *termui.Viewport {
	return w.viewport
}

func (w *FileListWidget) Selected() int {
	return w.selected
}

func (w *FileListWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}
