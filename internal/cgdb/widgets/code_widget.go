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
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

const (
	pcMarker    = "━━▶"
	pcGutterPad = "   "
)

// CodeWidget is a scrollable source view. The app calls ShowLocation on stops / :e.
// When focused: Up/Down move a bold cursor line; Space toggles a breakpoint
// (insert if none, -break-delete if present) via the shared GDB PTY.
// Red marks and the Breakpoint list refresh from MI =breakpoint-* notifies.
type CodeWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	sess  core.Session
	state *platform.AppState

	path     string
	pcLine   int // 1-based program counter
	selLine  int // 1-based cursor / bold line
	rawLines []string
	hiLines  []string // chroma ANSI lines (same length as rawLines)
	bpLines  map[int]struct{} // enabled breakpoints → red line numbers
	bpNums   map[int][]int    // line → GDB breakpoint numbers (any state)
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
	vp.RowStyle = w.rowStyle
	w.initKeyBindings()
	return w
}

// SetPTY wires the shared GDB session for Space → -break-insert / clear.
func (w *CodeWidget) SetPTY(sess core.Session, state *platform.AppState) {
	w.sess = sess
	w.state = state
}

func (w *CodeWidget) initKeyBindings() {
	w.BindKeyFunc("sel-up", func(args ...any) { w.moveSel(-1) }, "<Up>", "k")
	w.BindKeyFunc("sel-down", func(args ...any) { w.moveSel(1) }, "<Down>", "j")
	w.BindKeyFunc("page-up", func(args ...any) { w.moveSel(-10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.moveSel(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.moveSelTo(1) }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.moveSelTo(len(w.rawLines)) }, "<End>", "G")
	w.BindKeyFunc("break-toggle", func(args ...any) { w.breakAtSel() }, " ")
}

func (w *CodeWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	ln := lineIdx + 1
	if ln == w.pcLine {
		st = st.Background(tcell.ColorDarkSlateGray)
	}
	if ln == w.selLine && w.Focused() {
		st = st.Bold(true)
		if ln != w.pcLine {
			st = st.Background(tcell.ColorDarkBlue)
		}
	}
	_ = line
	return st
}

func (w *CodeWidget) moveSel(delta int) {
	n := len(w.rawLines)
	if n == 0 {
		return
	}
	if w.selLine < 1 {
		w.selLine = 1
	}
	w.moveSelTo(w.selLine + delta)
}

func (w *CodeWidget) moveSelTo(line int) {
	n := len(w.rawLines)
	if n == 0 {
		return
	}
	if line < 1 {
		line = 1
	}
	if line > n {
		line = n
	}
	w.selLine = line
	w.viewport.CursorLine = line - 1
	w.viewport.CursorCol = 0
	w.viewport.Left = 0
	w.viewport.EnsureCursorVisible()
}

func (w *CodeWidget) breakAtSel() {
	if w.path == "" || len(w.rawLines) == 0 {
		return
	}
	if w.selLine < 1 {
		w.selLine = 1
	}
	if w.selLine > len(w.rawLines) {
		w.selLine = len(w.rawLines)
	}
	// Basename matches how GDB resolves source (and pending locations).
	loc := fmt.Sprintf("%s:%d", filepath.Base(w.path), w.selLine)
	if w.lineHasBreak(w.selLine) {
		// clear removes all breakpoints at this source location.
		w.sendMI("clear " + loc)
		w.clearLocalBreak(w.selLine)
		return
	}
	w.sendMI("break " + loc)
	w.addLocalBreak(w.selLine)
}

func (w *CodeWidget) lineHasBreak(line int) bool {
	if w.hasBreakpoint(line) {
		return true
	}
	return len(w.bpNums[line]) > 0
}

func (w *CodeWidget) addLocalBreak(line int) {
	if w.bpLines == nil {
		w.bpLines = make(map[int]struct{})
	}
	w.bpLines[line] = struct{}{}
	w.rebuildBuffer()
}

func (w *CodeWidget) clearLocalBreak(line int) {
	delete(w.bpLines, line)
	delete(w.bpNums, line)
	w.rebuildBuffer()
}

func (w *CodeWidget) sendMI(cmd string) {
	sendGdbCmd(w.sess, w.state, cmd)
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
	w.selLine = line
	w.rebuildBuffer()

	idx := line - 1
	if idx < 0 {
		idx = 0
	}
	if n := len(w.rawLines); n > 0 && idx >= n {
		idx = n - 1
	}
	w.viewport.Left = 0
	w.viewport.CursorCol = 0
	pageH := w.viewport.Height()
	if pageH <= 0 {
		pageH = 20
	}
	w.viewport.Center(idx, pageH)
	return nil
}

// SetBreakInfos updates gutter state from -break-list rows for this file.
// Enabled breakpoints get a red line number; any number on a line is used by
// Space to delete (toggle off).
//
// A nil slice means "no update" (failed refresh) so existing red marks stay.
// A non-nil empty slice clears marks (GDB truly has no breakpoints for this file).
func (w *CodeWidget) SetBreakInfos(items []mcp.BreakInfo) {
	if items == nil {
		return
	}
	w.bpLines = make(map[int]struct{})
	w.bpNums = make(map[int][]int)
	for _, it := range items {
		if it.Line < 1 {
			continue
		}
		if it.Number > 0 {
			w.bpNums[it.Line] = append(w.bpNums[it.Line], it.Number)
		}
		if it.Enabled {
			w.bpLines[it.Line] = struct{}{}
		}
	}
	if w.path != "" || len(w.rawLines) > 0 {
		w.rebuildBuffer()
	}
}

// SetBreakpointLines marks enabled breakpoint lines (tests / simple callers).
func (w *CodeWidget) SetBreakpointLines(lines []int) {
	items := make([]mcp.BreakInfo, 0, len(lines))
	for i, ln := range lines {
		if ln > 0 {
			items = append(items, mcp.BreakInfo{
				Number:  i + 1,
				Line:    ln,
				Enabled: true,
				File:    w.path,
			})
		}
	}
	w.SetBreakInfos(items)
}

func (w *CodeWidget) hasBreakpoint(line int) bool {
	_, ok := w.bpLines[line]
	return ok
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
		num := fmt.Sprintf("%4d", ln)
		var numANSI string
		if w.hasBreakpoint(ln) {
			// Red background, white bold digits.
			numANSI = "\x1b[48;5;196;38;5;231;1m" + num + "\x1b[0m"
		} else {
			numANSI = "\x1b[38;5;244m" + num + "\x1b[0m"
		}
		gutter := fmt.Sprintf("%s %s\x1b[38;5;240m│\x1b[0m ", markANSI, numANSI)
		w.buf.AppendLine(gutter + src)
	}
	if len(w.rawLines) == 0 {
		w.buf.AppendLine(fmt.Sprintf("%s (empty file)", pcGutterPad))
	}
	// ANSI lines are long in bytes; never keep a stale horizontal skip.
	w.viewport.Left = 0
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

func (w *CodeWidget) syncSelFromViewport() {
	n := len(w.rawLines)
	if n == 0 {
		return
	}
	line := w.viewport.CursorLine + 1
	if line < 1 {
		line = 1
	}
	if line > n {
		line = n
	}
	w.selLine = line
}

func (w *CodeWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		w.viewport.HandleEvent(e)
		// Keep the bold blue cursor line in sync with mouse click / drag.
		w.syncSelFromViewport()
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.viewport.HandleEvent(e)
	}
}

func (w *CodeWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(false)
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

func (w *CodeWidget) Path() string    { return w.path }
func (w *CodeWidget) PCLine() int     { return w.pcLine }
func (w *CodeWidget) SelLine() int    { return w.selLine }
func (w *CodeWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}
