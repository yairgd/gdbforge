package widgets

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

const (
	pcMarker    = "━━▶"
	pcGutterPad = "   "
	// codeGutterCols is the visible width of "━━▶ ####│ " (mark + space + 4-digit
	// line + │ + space). Must match rebuildBuffer gutter layout.
	codeGutterCols = 10
)

// CodeWidget is a scrollable source view. The app calls ShowLocation on stops / :edit.
// When focused: Up/Down move a bold cursor line; Space / e fire intents for the
// shared breakpoint model (app owns GDB sends).
type CodeWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	state *debugstate.State

	// onBreakToggle is Space — insert/clear breakpoint at cursor (app sends GDB).
	onBreakToggle func(path string, line int)
	// onToggleEnable is "e" — enable/disable at cursor.
	onToggleEnable func()

	path       string
	pcLine     int // 1-based program counter
	selLine    int // 1-based cursor / bold line
	preferCol  int // preferred source column (0-based, past gutter)
	rawLines   []string
	hiLines    []string // chroma ANSI lines (same length as rawLines)
	bpByLine   map[int]models.BreakGutter

	// unavailable: source path cannot be shown (missing file, .so without sources).
	unavailable      bool
	unavailablePath  string
	unavailableExtra string // optional func / line hint under the path
}

func NewCodeWidget() *CodeWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursor(termui.NewInverseCursor())
	vp.SetCursorVisible(false)
	vp.ANSI = true

	w := &CodeWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Code"},
		viewport:   vp,
		buf:        buf,
	}
	vp.RowStyle = w.rowStyle
	vp.SetSearchContentOffset(codeGutterCols)
	vp.SetOnSearchJump(func(lineIdx int) {
		w.selLine = lineIdx + 1
		w.preferCol = w.contentCol()
	})
	w.initKeyBindings()
	return w
}

// SetAppState wires break/mark colors for gutters.
func (w *CodeWidget) SetAppState(state *debugstate.State) {
	w.state = state
}

// SetOnBreakToggle registers Space → insert/clear at path:line (app owns GDB).
func (w *CodeWidget) SetOnBreakToggle(fn func(path string, line int)) {
	w.onBreakToggle = fn
}

// SetOnToggleEnable registers a callback for "e" (enable/disable at cursor).
func (w *CodeWidget) SetOnToggleEnable(fn func()) {
	w.onToggleEnable = fn
}

func (w *CodeWidget) initKeyBindings() {
	w.BindKeyFunc("sel-up", func(args ...any) { w.moveSel(-1) }, "<Up>", "k")
	w.BindKeyFunc("sel-down", func(args ...any) { w.moveSel(1) }, "<Down>", "j")
	w.BindKeyFunc("sel-left", func(args ...any) { w.moveCol(-1) }, "<Left>", "h")
	w.BindKeyFunc("sel-right", func(args ...any) { w.moveCol(1) }, "<Right>", "l")
	w.BindKeyFunc("page-up", func(args ...any) { w.moveSel(-10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.moveSel(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.moveSelTo(1) }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.moveSelTo(len(w.rawLines)) }, "<End>", "G")
	w.BindKeyFunc("break-toggle", func(args ...any) { w.breakAtSel() }, " ")
	w.BindKeyFunc("break-enable-toggle", func(args ...any) {
		if w.onToggleEnable != nil {
			w.onToggleEnable()
		}
	}, "e")
}

// MoveSel moves the bold cursor line by delta (exported for app-level normal-mode keys).
func (w *CodeWidget) MoveSel(delta int) { w.moveSel(delta) }

// GotoLine moves the browse caret (blue line) to 1-based line, clamped to the file.
func (w *CodeWidget) GotoLine(line int) { w.moveSelTo(line) }

// BreakAtSel fires OnBreakToggle for the selected line (exported for global Space).
func (w *CodeWidget) BreakAtSel() { w.breakAtSel() }

// ToggleEnableAtSel runs the enable/disable callback (same as BreakpointWidget e).
func (w *CodeWidget) ToggleEnableAtSel() {
	if w.onToggleEnable != nil {
		w.onToggleEnable()
	}
}

func (w *CodeWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	ln := lineIdx + 1
	if ln == w.pcLine {
		st = st.Background(w.pcColor())
	}
	// Browse cursor (blue): shown whenever selLine is set — including when
	// focus is on Breakpoints after a click-to-jump (━━▶ stays on real PC).
	if ln == w.selLine {
		st = st.Bold(true)
		if ln != w.pcLine {
			st = st.Background(w.codeSelColor())
		}
	}
	_ = line
	return st
}

// SetSearchPattern updates the live /search highlight (does not commit).
func (w *CodeWidget) SetSearchPattern(pattern string) {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.SetSearchColor(w.searchColor())
	w.viewport.SetSearchPattern(pattern)
}

// CommitSearch stores pattern as the lasting highlight and jumps to a match.
func (w *CodeWidget) CommitSearch(pattern string) {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.SetSearchColor(w.searchColor())
	// Keep viewport cursor aligned with selLine before commit jump.
	if w.selLine >= 1 {
		w.viewport.CursorLine = w.selLine - 1
	}
	w.viewport.CommitSearch(pattern)
}

// RevertSearch restores the last committed pattern (Esc from /search).
func (w *CodeWidget) RevertSearch() {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.RevertSearch()
}

// SearchPattern returns the live search text.
func (w *CodeWidget) SearchPattern() string {
	if w == nil || w.viewport == nil {
		return ""
	}
	return w.viewport.SearchPattern()
}

// SearchNext moves the cursor to the next matching line (wraps).
func (w *CodeWidget) SearchNext() bool {
	if w == nil || w.viewport == nil {
		return false
	}
	w.viewport.SetSearchColor(w.searchColor())
	if w.selLine >= 1 {
		w.viewport.CursorLine = w.selLine - 1
	}
	return w.viewport.SearchNext()
}

// SearchPrev moves the cursor to the previous matching line (wraps).
func (w *CodeWidget) SearchPrev() bool {
	if w == nil || w.viewport == nil {
		return false
	}
	w.viewport.SetSearchColor(w.searchColor())
	if w.selLine >= 1 {
		w.viewport.CursorLine = w.selLine - 1
	}
	return w.viewport.SearchPrev()
}

// WordAtCursor returns the identifier/token under the browse cursor.
func (w *CodeWidget) WordAtCursor() string {
	if w == nil || w.viewport == nil {
		return ""
	}
	if w.selLine >= 1 {
		w.viewport.CursorLine = w.selLine - 1
	}
	return w.viewport.WordAtCursor()
}

// CursorInSearchMatch reports whether the browse caret sits on a /search hit.
func (w *CodeWidget) CursorInSearchMatch() bool {
	if w == nil || w.viewport == nil {
		return false
	}
	if w.selLine >= 1 {
		w.viewport.CursorLine = w.selLine - 1
	}
	return w.viewport.CursorInSearchMatch()
}

func (w *CodeWidget) SetSearchColor(c tcell.Color) {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.SetSearchColor(c)
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
	w.setCursorContentCol(w.preferCol)
	w.viewport.EnsureCursorVisible()
}

// MoveCol moves the caret horizontally by delta visible content cells.
func (w *CodeWidget) MoveCol(delta int) { w.moveCol(delta) }

func (w *CodeWidget) moveCol(delta int) {
	if w == nil || w.viewport == nil || len(w.rawLines) == 0 {
		return
	}
	if w.selLine < 1 {
		w.selLine = 1
		w.viewport.CursorLine = 0
	}
	w.setCursorContentCol(w.contentCol() + delta)
	w.viewport.EnsureCursorVisible()
}

// contentCol is the 0-based column in source text (after the gutter).
func (w *CodeWidget) contentCol() int {
	if w == nil || w.viewport == nil || w.viewport.Buffer == nil {
		return 0
	}
	line := w.viewport.Buffer.Line(w.viewport.CursorLine)
	vis := termui.VisibleANSIColAtByte(line, w.viewport.CursorCol)
	col := vis - codeGutterCols
	if col < 0 {
		return 0
	}
	return col
}

// setCursorContentCol places the caret on a source column (0-based, past gutter).
func (w *CodeWidget) setCursorContentCol(contentCol int) {
	if w == nil || w.viewport == nil || w.viewport.Buffer == nil {
		return
	}
	if contentCol < 0 {
		contentCol = 0
	}
	line := w.viewport.Buffer.Line(w.viewport.CursorLine)
	maxContent := termui.VisibleANSIWidth(line) - codeGutterCols
	if maxContent < 0 {
		maxContent = 0
	}
	if contentCol > maxContent {
		contentCol = maxContent
	}
	w.preferCol = contentCol
	w.viewport.CursorCol = termui.ANSIByteIndexAtVisible(line, codeGutterCols+contentCol)
}

func (w *CodeWidget) breakAtSel() {
	if w.path == "" || len(w.rawLines) == 0 || w.onBreakToggle == nil {
		return
	}
	if w.selLine < 1 {
		w.selLine = 1
	}
	if w.selLine > len(w.rawLines) {
		w.selLine = len(w.rawLines)
	}
	w.onBreakToggle(w.path, w.selLine)
}

// ShowLocation loads path from disk (if needed), marks line with ━━▶, and scrolls to it.
// line is 1-based. If the source is missing or is a shared library without sources,
// shows a centered "not available" placeholder instead of returning an error.
func (w *CodeWidget) ShowLocation(path string, line int) error {
	if err := w.loadAndScroll(path, line); err != nil {
		return err
	}
	if w.unavailable {
		return nil
	}
	w.pcLine = line
	w.selLine = line
	w.rebuildBuffer()
	return nil
}

// ShowSelection loads path (if needed) and moves the browse cursor (blue) to
// line without moving ━━▶. Used when jumping from the Breakpoints list.
func (w *CodeWidget) ShowSelection(path string, line int) error {
	prevPath, prevPC := w.path, w.pcLine
	if err := w.loadAndScroll(path, line); err != nil {
		return err
	}
	if w.unavailable {
		return nil
	}
	// Preserve ━━▶ when staying on the same source file as the stop.
	if prevPC > 0 && models.SameSourcePath(prevPath, w.path) {
		w.pcLine = prevPC
	} else {
		w.pcLine = 0
	}
	w.selLine = line
	w.rebuildBuffer()
	return nil
}

// loadAndScroll loads path and centers the viewport on line (1-based).
func (w *CodeWidget) loadAndScroll(path string, line int) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	if line < 1 {
		line = 1
	}

	if isSharedLibPath(path) {
		w.ShowUnavailable(path, fmt.Sprintf("line %d", line))
		return nil
	}

	if path != w.path || w.unavailable {
		lines, err := readSourceLines(path)
		if err != nil {
			w.ShowUnavailable(path, fmt.Sprintf("line %d", line))
			return nil
		}
		w.unavailable = false
		w.unavailablePath = ""
		w.unavailableExtra = ""
		w.path = path
		w.PaneName = filepath.Base(path)
		w.rawLines = lines
		w.hiLines = highlightLines(path, lines)
	}

	idx := line - 1
	if idx < 0 {
		idx = 0
	}
	if n := len(w.rawLines); n > 0 && idx >= n {
		idx = n - 1
	}
	w.viewport.Left = 0
	w.viewport.CursorLine = idx
	w.preferCol = 0
	w.setCursorContentCol(0)
	pageH := w.viewport.Height()
	if pageH <= 0 {
		pageH = 20
	}
	w.viewport.Center(idx, pageH)
	return nil
}

// Clear resets the pane to an empty Code view (no source, no ━━▶, no BP marks).
func (w *CodeWidget) Clear() {
	if w == nil {
		return
	}
	w.unavailable = false
	w.unavailablePath = ""
	w.unavailableExtra = ""
	w.path = ""
	w.rawLines = nil
	w.hiLines = nil
	w.pcLine = 0
	w.selLine = 0
	w.preferCol = 0
	w.bpByLine = nil
	w.PaneName = "Code"
	if w.viewport != nil {
		w.viewport.CommitSearch("")
	}
	w.buf.Clear()
	w.viewport.Left = 0
	w.viewport.Top = 0
	w.viewport.CursorLine = 0
	w.viewport.CursorCol = 0
}

// ClearPC removes the ━━▶ execution mark (e.g. after kill / inferior exit).
// Keeps the loaded source, selection, and breakpoint gutters.
func (w *CodeWidget) ClearPC() {
	if w == nil || w.pcLine == 0 {
		return
	}
	w.pcLine = 0
	if !w.unavailable {
		w.rebuildBuffer()
	}
}

// ShowUnavailable clears source and shows a centered "not available" message
// with path (and optional extra detail) in the middle of the pane.
func (w *CodeWidget) ShowUnavailable(path, extra string) {
	w.unavailable = true
	w.unavailablePath = path
	w.unavailableExtra = extra
	w.path = path
	w.rawLines = nil
	w.hiLines = nil
	w.pcLine = 0
	w.selLine = 0
	w.bpByLine = nil
	if path != "" {
		w.PaneName = filepath.Base(path)
	}
	w.buf.Clear()
	w.viewport.Left = 0
	w.viewport.Top = 0
}

func isSharedLibPath(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, ".so")
}

// SetBreakInfos updates gutter state from breakpoint rows for this file.
// A nil slice means "no update" (failed refresh) so existing marks stay.
// A non-nil empty slice clears marks (no breakpoints for this file).
func (w *CodeWidget) SetBreakInfos(items []models.BreakInfo) {
	if items == nil {
		return
	}
	w.bpByLine = models.GuttersByLine(items)
	if w.path != "" || len(w.rawLines) > 0 {
		w.rebuildBuffer()
	}
}

// SetBreakpointLines marks enabled breakpoint lines (tests / simple callers).
func (w *CodeWidget) SetBreakpointLines(lines []int) {
	items := make([]models.BreakInfo, 0, len(lines))
	for i, ln := range lines {
		if ln > 0 {
			items = append(items, models.BreakInfo{
				Number:  i + 1,
				Line:    ln,
				Enabled: true,
				File:    w.path,
			})
		}
	}
	w.SetBreakInfos(items)
}

func (w *CodeWidget) gutterAt(line int) (models.BreakGutter, bool) {
	g, ok := w.bpByLine[line]
	return g, ok
}

// HasEnabledBreak reports whether line has an enabled breakpoint mark.
func (w *CodeWidget) HasEnabledBreak(line int) bool {
	g, ok := w.gutterAt(line)
	return ok && g.Enabled
}

// HasDisabledBreak reports whether line has a disabled breakpoint mark.
func (w *CodeWidget) HasDisabledBreak(line int) bool {
	g, ok := w.gutterAt(line)
	return ok && !g.Enabled
}

func (w *CodeWidget) breakColor() tcell.Color {
	return themeFrom{w.state}.Break()
}

func (w *CodeWidget) breakDisabledColor() tcell.Color {
	return themeFrom{w.state}.BreakDisabled()
}

func (w *CodeWidget) breakCondColor() tcell.Color {
	return themeFrom{w.state}.BreakCond()
}

func (w *CodeWidget) pcColor() tcell.Color {
	return themeFrom{w.state}.PC()
}

func (w *CodeWidget) codeSelColor() tcell.Color {
	return themeFrom{w.state}.CodeSel()
}

func (w *CodeWidget) searchColor() tcell.Color {
	return themeFrom{w.state}.Search()
}

// RebuildBuffer refreshes gutter ANSI from current AppState break colors.
func (w *CodeWidget) RebuildBuffer() {
	w.rebuildBuffer()
}

func (w *CodeWidget) rebuildBuffer() {
	if w.unavailable {
		return
	}
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
		if g, ok := w.gutterAt(ln); ok {
			numANSI = platform.BreakNumberANSI(num, breakGutterColor(g, w.state))
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
		// Delve often reports ./file.go — try absolute from cwd.
		if !filepath.IsAbs(path) {
			if abs, absErr := filepath.Abs(path); absErr == nil && abs != path {
				f, err = os.Open(abs)
			}
		}
		if err != nil {
			return nil, err
		}
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
	w.preferCol = w.contentCol()
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
	if w.viewport != nil {
		w.viewport.SetCursorVisible(focused && !w.unavailable)
	}
}

func (w *CodeWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

// statusLabel is the status-bar text: full source path when known, else PaneName.
func (w *CodeWidget) statusLabel() string {
	if w.path != "" {
		return w.path
	}
	return w.PaneName
}

// DrawStatusLine shows the full file path on the pane status bar.
func (w *CodeWidget) DrawStatusLine(c termui.Canvas, active bool) {
	name := w.statusLabel()
	if name == "" {
		return
	}
	if w.Focused() {
		termui.PaintStatusBar(c, name, active)
		return
	}
	termui.PaintInactiveStatusBar(c, name)
}

func (w *CodeWidget) Draw(c termui.Canvas) {
	if w.unavailable {
		w.drawUnavailable(c)
		return
	}
	w.viewport.Draw(c)
}

func (w *CodeWidget) drawUnavailable(c termui.Canvas) {
	h, width := c.H(), c.W()
	if h <= 0 || width <= 0 {
		return
	}
	for y := 0; y < h; y++ {
		c.ClearLine(y, tcell.StyleDefault)
	}
	title := "not available"
	path := w.unavailablePath
	extra := w.unavailableExtra
	nLines := 2
	if extra != "" {
		nLines = 3
	}
	startY := (h - nLines) / 2
	if startY < 0 {
		startY = 0
	}
	titleStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	pathStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	drawCentered(c, startY, width, title, titleStyle)
	if startY+1 < h {
		drawCentered(c, startY+1, width, path, pathStyle)
	}
	if extra != "" && startY+2 < h {
		drawCentered(c, startY+2, width, extra, pathStyle)
	}
}

func drawCentered(c termui.Canvas, y, width int, text string, st tcell.Style) {
	if text == "" || width <= 0 {
		return
	}
	runes := []rune(text)
	if len(runes) > width {
		runes = runes[:width]
	}
	x := (width - len(runes)) / 2
	if x < 0 {
		x = 0
	}
	for i, ch := range runes {
		c.SetContent(x+i, y, ch, st)
	}
}

func (w *CodeWidget) Viewport() *termui.Viewport {
	return w.viewport
}

func (w *CodeWidget) Path() string { return w.path }
func (w *CodeWidget) PCLine() int  { return w.pcLine }
func (w *CodeWidget) SelLine() int { return w.selLine }
func (w *CodeWidget) Unavailable() bool {
	return w.unavailable
}
func (w *CodeWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}
