package widgets

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

const (
	pcMarker     = "-->"
	pcGutterPad  = "   " // same width as -->
	codeLineFmt  = "%s %4d| %s"
)

// CodeWidget is a passive scrollable source view (About-style Viewport).
// It does not talk to GDB; the app calls ShowLocation on stops / :e.
// Each open file gets its own instance (PaneName = basename).
type CodeWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	path     string
	pcLine   int // 1-based
	rawLines []string
}

func NewCodeWidget() *CodeWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)

	w := &CodeWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Code"},
		viewport:   vp,
		buf:        buf,
	}
	vp.LineStyle = w.lineStyle
	w.initKeyBindings()
	return w
}

func (w *CodeWidget) initKeyBindings() {
	w.BindKeyFunc("scroll-up", func(args ...any) { w.viewport.ScrollLineUp() }, "<Up>", "k")
	w.BindKeyFunc("scroll-down", func(args ...any) { w.viewport.ScrollLineDown() }, "<Down>", "j")
	w.BindKeyFunc("page-up", func(args ...any) { w.viewport.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.viewport.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.viewport.ScrollHome() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.viewport.ScrollEnd() }, "<End>", "G")
}

func (w *CodeWidget) lineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if strings.HasPrefix(line, pcMarker) {
		return st.Foreground(tcell.ColorYellow).Bold(true)
	}
	return st
}

// ShowLocation loads path from disk (if needed), marks line with -->, and scrolls to it.
// line is 1-based.
func (w *CodeWidget) ShowLocation(path string, line int) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	if line < 1 {
		line = 1
	}

	if path != w.path {
		lines, err := readSourceLines(path)
		if err != nil {
			return err
		}
		w.path = path
		w.rawLines = lines
	}
	w.pcLine = line
	w.rebuildBuffer()

	idx := line - 1
	if idx < 0 {
		idx = 0
	}
	if n := len(w.rawLines); n > 0 && idx >= n {
		idx = n - 1
	}
	w.viewport.Center(idx, 20)
	return nil
}

func (w *CodeWidget) rebuildBuffer() {
	w.buf.Clear()
	for i, text := range w.rawLines {
		ln := i + 1
		mark := pcGutterPad
		if ln == w.pcLine {
			mark = pcMarker
		}
		w.buf.AppendLine(fmt.Sprintf(codeLineFmt, mark, ln, text))
	}
	if len(w.rawLines) == 0 {
		w.buf.AppendLine(fmt.Sprintf("%s (empty file)", pcGutterPad))
	}
}

func readSourceLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	// Allow long source lines.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func (w *CodeWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *CodeWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		w.viewport.HandleEvent(e)
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.viewport.HandleEvent(e)
	}
}

func (w *CodeWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(focused)
}

func (w *CodeWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

func (w *CodeWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *CodeWidget) Viewport() *termui.Viewport {
	return w.viewport
}

// Path / PCLine expose current location for tests.
func (w *CodeWidget) Path() string  { return w.path }
func (w *CodeWidget) PCLine() int   { return w.pcLine }
func (w *CodeWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}
