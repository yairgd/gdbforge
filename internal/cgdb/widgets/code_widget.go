package widgets

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

const (
	pcMarker    = "━━▶"
	pcGutterPad = "   "
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
	hiLines  []string // chroma ANSI lines (same length as rawLines)
}

func NewCodeWidget() *CodeWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)
	vp.ANSI = true

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
	plain := termui.StripANSI(line)
	if strings.HasPrefix(strings.TrimLeft(plain, " "), pcMarker) || strings.HasPrefix(plain, pcMarker) {
		return tcell.StyleDefault.Background(tcell.ColorDarkSlateGray)
	}
	return tcell.StyleDefault
}

// ShowLocation loads path from disk (if needed), marks line with ━━▶, and scrolls to it.
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
		w.hiLines = highlightLines(path, lines)
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
		markANSI := pcGutterPad
		if ln == w.pcLine {
			markANSI = "\x1b[1;38;5;226m" + pcMarker + "\x1b[0m"
		}
		src := text
		if i < len(w.hiLines) {
			src = w.hiLines[i]
		}
		gutter := fmt.Sprintf("%s \x1b[38;5;244m%4d\x1b[0m\x1b[38;5;240m│\x1b[0m ", markANSI, ln)
		w.buf.AppendLine(gutter + src)
	}
	if len(w.rawLines) == 0 {
		w.buf.AppendLine(fmt.Sprintf("%s (empty file)", pcGutterPad))
	}
}

func highlightLines(path string, lines []string) []string {
	src := strings.Join(lines, "\n")
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Get(strings.TrimPrefix(filepath.Ext(path), "."))
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		return append([]string(nil), lines...)
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, it); err != nil {
		return append([]string(nil), lines...)
	}
	out := strings.Split(buf.String(), "\n")
	// Preserve trailing empty line count from Split.
	for len(out) < len(lines) {
		out = append(out, "")
	}
	if len(out) > len(lines) {
		out = out[:len(lines)]
	}
	return out
}

func readSourceLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
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

func (w *CodeWidget) Path() string { return w.path }
func (w *CodeWidget) PCLine() int  { return w.pcLine }
func (w *CodeWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}
