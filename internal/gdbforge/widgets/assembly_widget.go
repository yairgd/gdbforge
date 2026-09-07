package widgets

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

const (
	asmPCMarker    = "━━▶"
	asmPCGutterPad = "   "
	// asmGutterCols: mark + addr + " <+N>: " before instruction text.
	asmAddrCols = 18
	// asmOffsetColsMin: right-align <+N> so <+ 0>: lines up with <+12>:.
	asmOffsetColsMin = 2
	asmGutterCols    = 3 + 1 + asmAddrCols + 10
)

// AssemblyWidget shows disassembly around browse address X.
// ━━▶ marks real $pc; the blue line is the browse caret (same as CodeWidget).
// Space / e fire breakpoint intents at the browse address (app owns GDB).
type AssemblyWidget struct {
	termui.BaseWidget
	doc *termui.DocumentView
	
	state    *debugstate.State

	items    []models.AsmLine
	pcAddr   string   // normalized $pc
	selIdx   int      // browse caret index into items
	ctxLines []string // stack-frame preamble (not selectable)
	funcName string
	prefix   int // buffer lines before first instruction (ctx + Dump)

	bpByAddr map[string]models.BreakGutter

	lastHeight int
	fetching   bool

	// browseDir is -1 (Up) / +1 (Down) for the in-flight window slide; 0 otherwise.
	browseDir int
	// browsePreserveRow is the screen row of the blue line to keep across a slide
	// (-1 = none). Stops Up/Down edge refetch from jumping the caret to the
	// opposite edge of the viewport.
	browsePreserveRow int

	// offWidth is the right-align width for <+N> in dump mode (set in rebuild).
	offWidth int
}

func NewAssemblyWidget() *AssemblyWidget {
	w := &AssemblyWidget{
		BaseWidget:        termui.BaseWidget{PaneName: "Assembly"},
		doc:               termui.NewDocumentView(),
		selIdx:            0,
		browsePreserveRow: -1,
		offWidth:          asmOffsetColsMin,
	}
	w.doc.SetReadOnly(true)
	w.doc.SetDragAutoScroll(false)
	w.doc.SetCursor(termui.NewInverseCursor())
	w.doc.SetCursorVisible(false)
	w.doc.SetSearchContentOffset(asmGutterCols)
	w.doc.SetOnSearchJump(func(lineIdx int) {
		idx := lineIdx - w.prefix
		if idx < 0 {
			idx = 0
		}
		if idx >= len(w.items) {
			idx = len(w.items) - 1
		}
		if idx >= 0 {
			w.selIdx = idx
			w.doc.CursorLine = w.selIdx + w.prefix
		}
	})
	w.initKeyBindings()
	w.syncPrefix()
	return w
}

func (w *AssemblyWidget) SetAppState(st *debugstate.State) {
	w.state = st
}

func (w *AssemblyWidget) SetClipboard(io termui.ClipboardIO) {
	if w != nil && w.doc != nil {
		w.doc.SetClipboard(io)
	}
}

func (w *AssemblyWidget) initKeyBindings() {
	w.BindKeyFunc("sel-up", func(args ...any) { w.MoveSel(-1) }, "<Up>", "k")
	w.BindKeyFunc("sel-down", func(args ...any) { w.MoveSel(1) }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.doc.ViewScrollColLeft() }, "<Left>", "h")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.doc.ViewScrollColRightFor(w.lineCount(), w.lineAt) }, "<Right>", "l")
	w.BindKeyFunc("page-up", func(args ...any) { w.MoveSel(-w.pageRows()) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.MoveSel(w.pageRows()) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("break-toggle", func(args ...any) { w.BreakAtSel() }, " ")
	w.BindKeyFunc("break-enable-toggle", func(args ...any) {
		w.Publish(events.AsmBreakEnableToggleMsg{})
	}, "e")
}

func (w *AssemblyWidget) pageRows() int {
	h := w.VisibleRows()
	if h < 1 {
		return 10
	}
	return h
}

// VisibleRows is the last painted viewport height (0 before first Draw).
func (w *AssemblyWidget) VisibleRows() int {
	if w == nil || w.doc == nil {
		return 0
	}
	return w.doc.Height()
}

// SetContext sets the stack-frame preamble painted above the disassembly and
// rebuilds immediately so ?? / stop updates are visible without waiting on disasm.
// Does not call revealSel — that would pin Top=0 and undo Down-scroll past $pc.
func (w *AssemblyWidget) SetContext(lines []string) {
	if w == nil {
		return
	}
	w.ctxLines = append([]string(nil), lines...)
	w.syncPrefix()
	if w.doc != nil && len(w.items) > 0 {
		w.doc.CursorLine = w.selIdx + w.prefix
	}
}

// FuncName returns the current disassembled function name, if known.
func (w *AssemblyWidget) FuncName() string {
	if w == nil {
		return ""
	}
	return w.funcName
}

// SetItems replaces the instruction list. pcAddr is $pc for ━━▶.
// selAddr is the browse caret (empty → keep prior sel if still present, else pc).
// dumpFunc non-empty enables the CGDB Dump/End header (whole-function only).
func (w *AssemblyWidget) SetItems(items []models.AsmLine, pcAddr, selAddr, dumpFunc string) {
	if w == nil {
		return
	}
	w.items = append([]models.AsmLine(nil), items...)
	w.funcName = dumpFunc
	w.pcAddr = normalizeAsmAddr(pcAddr)
	w.fetching = false

	want := normalizeAsmAddr(selAddr)
	if want == "" {
		want = normalizeAsmAddr(w.SelAddr())
	}
	if want == "" {
		want = w.pcAddr
	}
	w.selIdx = indexOfAsmAddr(w.items, want)
	if w.selIdx < 0 {
		w.selIdx = indexOfAsmAddr(w.items, w.pcAddr)
	}
	if w.selIdx < 0 && len(w.items) > 0 {
		w.selIdx = len(w.items) / 2
	}
	if w.selIdx < 0 {
		w.selIdx = 0
	}
	w.syncPrefix()
	w.revealSel()
}

// revealSel places the browse caret on-screen after a fresh load.
// Pins Top=0 (frame/Dump header visible) only when the browse line is still on
// $pc and that line fits on the first page. Once the user moves past $pc,
// only EnsureCursorVisible runs so Down can roll the header off-screen.
// Edge-browse slides restore browsePreserveRow so the blue line does not jump.
func (w *AssemblyWidget) revealSel() {
	if w == nil || w.doc == nil || len(w.items) == 0 {
		return
	}
	selLine := w.selIdx + w.prefix
	w.doc.CursorLine = selLine
	h := w.doc.Height()
	if h < 1 {
		h = 20
	}

	if row := w.browsePreserveRow; row >= 0 {
		// Keep the blue line on the same screen row. Do not clamp to maxTop —
		// after an Up-slide the caret is at the end of the window and clamping
		// would jump it to the bottom of the viewport again.
		top := selLine - row
		if top < 0 {
			top = 0
		}
		w.doc.Top = top
		w.browsePreserveRow = -1
		w.browseDir = 0
		return
	}

	pcLine := selLine
	if idx := indexOfAsmAddr(w.items, w.pcAddr); idx >= 0 {
		pcLine = idx + w.prefix
	}
	selIsPC := w.pcAddr != "" && w.selIdx >= 0 && w.selIdx < len(w.items) &&
		normalizeAsmAddr(w.items[w.selIdx].Addr) == w.pcAddr
	if selIsPC {
		w.doc.CursorLine = pcLine
		if w.completeDump() && pcLine < h {
			// Whole-function dump: keep frame/Dump header on the first page.
			w.doc.Top = 0
		} else {
			// ?? / windowed: center $pc so code above and below is visible.
			top := pcLine - h/2
			if top < 0 {
				top = 0
			}
			w.doc.Top = top
		}
		w.doc.CursorLine = selLine
		return
	}
	w.doc.EnsureCursorVisible(w.lineCount(), w.lineWidth)
}

// BrowseDir returns the in-flight slide direction (-1 Up, +1 Down, 0 none).
func (w *AssemblyWidget) BrowseDir() int {
	if w == nil {
		return 0
	}
	return w.browseDir
}

// BrowsePreserveRow returns the screen row to keep for the in-flight slide (-1 none).
func (w *AssemblyWidget) BrowsePreserveRow() int {
	if w == nil {
		return -1
	}
	return w.browsePreserveRow
}

// completeDump is true for a whole-function view (Dump/End present). No edge
// refetch — scrolling must leave $pc and roll the frame header off-screen.
func (w *AssemblyWidget) completeDump() bool {
	return w != nil && w.funcName != ""
}

// endLine is the buffer index of "End of assembler dump.", or -1.
func (w *AssemblyWidget) endLine() int {
	if !w.completeDump() || len(w.items) == 0 {
		return -1
	}
	return w.prefix + len(w.items)
}

// ensureEndVisible scrolls so "End of assembler dump." sits under the last insn.
func (w *AssemblyWidget) ensureEndVisible() {
	if w == nil || w.doc == nil {
		return
	}
	el := w.endLine()
	if el < 0 {
		return
	}
	h := w.doc.Height()
	if h < 1 {
		h = 20
	}
	if el >= w.doc.Top+h {
		w.doc.Top = el - h + 1
	}
	if w.doc.Top < 0 {
		w.doc.Top = 0
	}
}

// ensureHeaderVisible pins Top=0 so the frame line + Dump header show again
// when the browse line is on the first instruction of the function.
func (w *AssemblyWidget) ensureHeaderVisible() {
	if w == nil || w.doc == nil {
		return
	}
	w.doc.Top = 0
}

// MoveSel moves the blue browse line by delta instructions.
func (w *AssemblyWidget) MoveSel(delta int) {
	if w == nil || len(w.items) == 0 {
		return
	}
	n := len(w.items)
	next := w.selIdx + delta
	needBrowse := false
	if next < 0 {
		next = 0
		needBrowse = delta < 0
	}
	if next >= n {
		next = n - 1
		needBrowse = delta > 0
	}
	w.selIdx = next
	w.doc.CursorLine = w.selIdx + w.prefix
	w.doc.EnsureCursorVisible(w.lineCount(), w.lineWidth)

	// Whole-function dump is finite — edge browse would re-fetch and snap the view.
	// At the bottom show End after the last insn; at the top restore frame/Dump.
	if w.completeDump() {
		if w.selIdx <= 0 {
			w.ensureHeaderVisible()
		}
		if w.selIdx >= n-1 {
			w.ensureEndVisible()
		}
		return
	}
	// Windowed / ??: slide only when the caret hits a hard edge. Prefetching
	// inside a margin re-anchored the blue line every few Down presses.
	if !needBrowse {
		return
	}
	if delta < 0 {
		w.browseDir = -1
	} else {
		w.browseDir = 1
	}
	row := w.doc.CursorLine - w.doc.Top
	if row < 0 {
		row = 0
	}
	w.browsePreserveRow = row
	w.requestBrowse()
}

func (w *AssemblyWidget) requestBrowse() {
	if w == nil || w.fetching || w.completeDump() {
		return
	}
	addr := w.SelAddr()
	if addr == "" {
		return
	}
	rows := w.VisibleRows()
	if rows < 1 {
		rows = 20
	}
	w.fetching = true
	w.Publish(events.AsmBrowseMsg{Addr: addr, Rows: rows})
}

// SelAddr returns the browse caret address.
func (w *AssemblyWidget) SelAddr() string {
	if w == nil || w.selIdx < 0 || w.selIdx >= len(w.items) {
		return ""
	}
	return w.items[w.selIdx].Addr
}

// BreakAtSel fires ToggleAsmBreak for the selected address (Space).
func (w *AssemblyWidget) BreakAtSel() {
	if w == nil {
		return
	}
	addr := w.SelAddr()
	if addr == "" {
		return
	}
	w.Publish(events.AsmBreakToggleMsg{Addr: addr})
}

// HasEnabledBreak reports an enabled breakpoint mark at addr.
func (w *AssemblyWidget) HasEnabledBreak(addr string) bool {
	g, ok := w.gutterAt(normalizeAsmAddr(addr))
	return ok && g.Enabled
}

func (w *AssemblyWidget) gutterAt(addr string) (models.BreakGutter, bool) {
	if w == nil || w.bpByAddr == nil || addr == "" {
		return models.BreakGutter{}, false
	}
	g, ok := w.bpByAddr[addr]
	return g, ok
}

// SetBreakInfos updates address gutter marks from the shared breakpoint model.
// A nil slice means "no update"; a non-nil empty slice clears marks.
func (w *AssemblyWidget) SetBreakInfos(items []models.BreakInfo) {
	if w == nil || items == nil {
		return
	}
	w.bpByAddr = models.GuttersByAddr(items)
	w.syncPrefix()
}

// PCAddr returns the marked program-counter address.
func (w *AssemblyWidget) PCAddr() string {
	if w == nil {
		return ""
	}
	return w.pcAddr
}

// ClearFetchAck clears the in-flight refetch guard (call if a browse failed).
func (w *AssemblyWidget) ClearFetchAck() {
	if w != nil {
		w.fetching = false
		w.browsePreserveRow = -1
		w.browseDir = 0
	}
}

func (w *AssemblyWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	itemIdx := lineIdx - w.prefix
	if itemIdx < 0 || itemIdx >= len(w.items) {
		_ = line
		return st
	}
	addr := normalizeAsmAddr(w.items[itemIdx].Addr)
	if addr != "" && addr == w.pcAddr {
		st = st.Background(w.pcColor())
	}
	if itemIdx == w.selIdx {
		st = st.Bold(true)
		if addr == "" || addr != w.pcAddr {
			st = st.Background(w.codeSelColor())
		}
	}
	_ = line
	return st
}

func (w *AssemblyWidget) breakColor() tcell.Color {
	return themeFrom{w.state}.Break()
}

func (w *AssemblyWidget) breakDisabledColor() tcell.Color {
	return themeFrom{w.state}.BreakDisabled()
}

func (w *AssemblyWidget) breakCondColor() tcell.Color {
	return themeFrom{w.state}.BreakCond()
}

func (w *AssemblyWidget) pcColor() tcell.Color {
	return themeFrom{w.state}.PC()
}

func (w *AssemblyWidget) codeSelColor() tcell.Color {
	return themeFrom{w.state}.CodeSel()
}

func (w *AssemblyWidget) searchColor() tcell.Color {
	return themeFrom{w.state}.Search()
}

func (w *AssemblyWidget) mutedColor() tcell.Color {
	return themeFrom{w.state}.Muted()
}

// cellStyle paints mark / address / <+N>: / chrome via Canvas (no buffer ANSI).
func (w *AssemblyWidget) cellStyle(lineIdx int, absVisCol int, st tcell.Style) tcell.Style {
	if w == nil {
		return st
	}
	itemIdx := lineIdx - w.prefix
	if itemIdx < 0 || itemIdx >= len(w.items) {
		return st.Foreground(w.mutedColor())
	}
	it := w.items[itemIdx]
	norm := normalizeAsmAddr(it.Addr)

	// mark ━━▶ / pad (cols 0..2)
	if absVisCol < 3 {
		if norm != "" && norm == w.pcAddr {
			return st.Foreground(tcell.ColorYellow).Bold(true)
		}
		return st
	}

	dumpMode := w.funcName != "" && it.Offset != ""
	if dumpMode {
		addrStart := 4
		addrEnd := 4 + asmAddrCols
		if absVisCol >= addrStart && absVisCol < addrEnd {
			return w.addrCellStyle(norm, st)
		}
		offStart := addrEnd + 1 // space after addr
		offEnd := offStart + 2 + w.offWidth + 2 + 1 // <+N>:␠
		if absVisCol >= offStart && absVisCol < offEnd {
			return st.Foreground(w.mutedColor())
		}
		return st
	}

	addr := it.Addr
	if len(addr) > asmAddrCols {
		addr = addr[len(addr)-asmAddrCols:]
	}
	addrStart := 4
	addrEnd := addrStart + len(addr)
	if absVisCol >= addrStart && absVisCol < addrEnd {
		return w.addrCellStyle(norm, st)
	}
	if absVisCol == addrEnd { // trailing ':'
		return st.Foreground(w.mutedColor())
	}
	return st
}

func (w *AssemblyWidget) addrCellStyle(norm string, st tcell.Style) tcell.Style {
	if g, ok := w.gutterAt(norm); ok {
		bg := breakGutterColor(g, w.state)
		return st.Background(bg).Foreground(platform.ContrastColor(bg)).Bold(true)
	}
	return st.Foreground(w.mutedColor())
}

func (w *AssemblyWidget) syncPrefix() {
	if w == nil {
		return
	}
	w.prefix = len(w.ctxLines)
	if w.funcName != "" && len(w.items) > 0 {
		w.prefix++
	}
	w.offWidth = asmOffsetColsMin
	if w.funcName != "" {
		for _, it := range w.items {
			if n := len(it.Offset); n > w.offWidth {
				w.offWidth = n
			}
		}
	}
	w.syncSearchOffset()
}

func (w *AssemblyWidget) syncSearchOffset() {
	if w.doc == nil {
		return
	}
	searchOff := 3 + 1 + asmAddrCols + 1 + 2 + w.offWidth + 2 + 1
	if w.funcName == "" {
		searchOff = asmGutterCols
	}
	w.doc.SetSearchContentOffset(searchOff)
}

func (w *AssemblyWidget) lineCount() int {
	n := w.prefix + len(w.items)
	if w.funcName != "" && len(w.items) > 0 {
		n++ // End of assembler dump.
	}
	if len(w.items) == 0 && w.prefix == 0 {
		return 1 // "(no disassembly)"
	}
	return n
}

func (w *AssemblyWidget) lineAt(i int) string { return w.displayLine(i) }

func (w *AssemblyWidget) lineWidth(lineIdx int) int {
	return len([]rune(w.displayLine(lineIdx)))
}

func (w *AssemblyWidget) displayLine(lineIdx int) string {
	if w == nil {
		return ""
	}
	if len(w.items) == 0 {
		if lineIdx == 0 && w.prefix == 0 {
			return "(no disassembly)"
		}
		return ""
	}
	if lineIdx < len(w.ctxLines) {
		return w.ctxLines[lineIdx]
	}
	lineIdx -= len(w.ctxLines)
	if w.funcName != "" && len(w.items) > 0 {
		if lineIdx == 0 {
			return fmt.Sprintf("Dump of assembler code for function %s:", w.funcName)
		}
		lineIdx--
	}
	if lineIdx < 0 {
		return ""
	}
	if lineIdx >= len(w.items) {
		if w.funcName != "" && lineIdx == len(w.items) {
			return "End of assembler dump."
		}
		return ""
	}
	it := w.items[lineIdx]
	mark := asmPCGutterPad
	norm := normalizeAsmAddr(it.Addr)
	if norm == w.pcAddr && w.pcAddr != "" {
		mark = asmPCMarker
	}
	addr := it.Addr
	if len(addr) > asmAddrCols {
		addr = addr[len(addr)-asmAddrCols:]
	}
	if it.Offset != "" && w.funcName != "" {
		addrPad := fmt.Sprintf("%-*s", asmAddrCols, addr)
		off := fmt.Sprintf("<+%*s>: ", w.offWidth, it.Offset)
		return fmt.Sprintf("%s %s %s%s", mark, addrPad, off, it.Inst)
	}
	return fmt.Sprintf("%s %s:  %s", mark, addr, it.Inst)
}

func (w *AssemblyWidget) Draw(c termui.Canvas) {
	if w == nil {
		return
	}
	h := c.H()
	if h > 0 && h != w.lastHeight {
		w.lastHeight = h
		w.browsePreserveRow = -1
		w.browseDir = 0
		if len(w.items) > 0 && !w.fetching {
			w.requestBrowse()
		}
	}
	w.doc.SetWindow(c.W(), c.H())
	w.doc.SetMouseOrigin(c.ScreenX(0), c.ScreenY(0))
	width := c.W()
	selStyle := tcell.StyleDefault.Reverse(true)
	lineAt := w.lineAt
	nLines := w.lineCount()
	for row := 0; row < h; row++ {
		lineIdx := w.doc.Top + row
		if lineIdx >= nLines {
			c.ClearLine(row, tcell.StyleDefault)
			continue
		}
		full := w.displayLine(lineIdx)
		lineStyle := w.rowStyle(lineIdx, full)
		runes := []rune(full)
		start := w.doc.Left
		if start < 0 {
			start = 0
		}
		if start > len(runes) {
			start = len(runes)
		}
		visible := runes[start:]
		if len(visible) > width {
			visible = visible[:width]
		}
		c.ClearLineRange(row, len(visible), width, lineStyle)
		byteIdx := termui.ByteIndexAtVisibleCol(full, start)
		for col, ch := range visible {
			absVisCol := start + col
			st := lineStyle
			st = w.cellStyle(lineIdx, absVisCol, st)
			st = w.doc.ApplySearchStyle(lineIdx, absVisCol, st, lineAt)
			if w.doc.CellSelected(lineIdx, byteIdx) {
				st = selStyle
			}
			c.SetContent(col, row, ch, st)
			byteIdx += utf8.RuneLen(ch)
		}
	}
	w.doc.DrawCursor(c, nLines, lineAt)
}

func (w *AssemblyWidget) syncSelFromViewport() {
	if w == nil || w.doc == nil || len(w.items) == 0 {
		return
	}
	n := len(w.items)
	idx := w.doc.CursorLine - w.prefix
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	prev := w.selIdx
	w.selIdx = idx
	w.doc.CursorLine = w.selIdx + w.prefix
	// Click on a hard edge of a windowed view should still refetch.
	if w.completeDump() || w.selIdx == prev {
		return
	}
	switch {
	case w.selIdx <= 0:
		w.browseDir = -1
	case w.selIdx >= n-1:
		w.browseDir = 1
	default:
		return
	}
	row := w.doc.CursorLine - w.doc.Top
	if row < 0 {
		row = 0
	}
	w.browsePreserveRow = row
	w.requestBrowse()
}

func (w *AssemblyWidget) HandleEvent(ev tcell.Event) {
	if w == nil {
		return
	}
	switch e := ev.(type) {
	case *tcell.EventMouse:
		btns := e.Buttons()
		if btns&tcell.WheelUp != 0 {
			w.MoveSel(-1)
			return
		}
		if btns&tcell.WheelDown != 0 {
			w.MoveSel(1)
			return
		}
		w.doc.HandleEvent(e, w.lineCount(), w.lineAt)
		w.syncSelFromViewport()
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.doc.HandleEvent(e, w.lineCount(), w.lineAt)
	}
}

func (w *AssemblyWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if w == nil {
		return false
	}
	return w.HandleBoundKey(ev)
}

func (w *AssemblyWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	if w.doc != nil {
		w.doc.SetCursorVisible(focused && len(w.items) > 0)
	}
}

// StatusLabel matches CGDB's asm buffer title:
//
//	** #2  0x… in write () from libc… (start - end) **     — named frame
//	**    0x…:   add … (start - end) **                   — ?? / $pc insn
func (w *AssemblyWidget) StatusLabel() string {
	if w == nil || len(w.items) == 0 {
		return "Assembly"
	}
	start := trimAsm0x(w.items[0].Addr)
	end := trimAsm0x(w.items[len(w.items)-1].Addr)
	rangePart := ""
	if start != "" && end != "" {
		rangePart = fmt.Sprintf(" (%s - %s)", start, end)
	}

	var head string
	if len(w.ctxLines) > 0 && strings.TrimSpace(w.ctxLines[0]) != "" {
		head = strings.TrimSpace(w.ctxLines[0])
	} else if pc := w.pcInsnStatus(); pc != "" {
		head = "   " + pc
	} else if w.pcAddr != "" {
		head = "   " + w.pcAddr
	} else {
		return "Assembly"
	}
	return fmt.Sprintf("** %s%s **", head, rangePart)
}

// pcInsnStatus is the $pc instruction text for the CGDB-style status title.
func (w *AssemblyWidget) pcInsnStatus() string {
	if w == nil || w.pcAddr == "" {
		return ""
	}
	idx := indexOfAsmAddr(w.items, w.pcAddr)
	if idx < 0 {
		return ""
	}
	it := w.items[idx]
	return fmt.Sprintf("%s:\t%s", it.Addr, it.Inst)
}

func trimAsm0x(addr string) string {
	addr = strings.TrimSpace(addr)
	if len(addr) >= 2 && (addr[0] == '0' && (addr[1] == 'x' || addr[1] == 'X')) {
		return addr[2:]
	}
	return addr
}

func (w *AssemblyWidget) DrawStatusLine(c termui.Canvas, active bool) {
	title := w.StatusLabel()
	w.PaneName = title
	if w.Focused() {
		termui.PaintStatusBar(c, title, active)
		return
	}
	termui.PaintInactiveStatusBar(c, title)
}

func (w *AssemblyWidget) SetSearchPattern(pattern string) {
	if w == nil || w.doc == nil {
		return
	}
	w.doc.SetSearchColor(w.searchColor())
	w.doc.SetSearchPattern(pattern)
}

func (w *AssemblyWidget) CommitSearch(pattern string) {
	if w == nil || w.doc == nil {
		return
	}
	w.doc.SetSearchColor(w.searchColor())
	w.doc.CursorLine = w.selIdx + w.prefix
	w.doc.CommitSearch(pattern, w.lineCount(), w.lineAt)
}

func (w *AssemblyWidget) RevertSearch() {
	if w == nil || w.doc == nil {
		return
	}
	w.doc.RevertSearch()
}

func (w *AssemblyWidget) SearchPattern() string {
	if w == nil || w.doc == nil {
		return ""
	}
	return w.doc.SearchPattern()
}

func (w *AssemblyWidget) SearchNext() bool {
	if w == nil || w.doc == nil {
		return false
	}
	w.doc.SetSearchColor(w.searchColor())
	w.doc.CursorLine = w.selIdx + w.prefix
	ok := w.doc.SearchNext(w.lineCount(), w.lineAt)
	if ok {
		idx := w.doc.CursorLine - w.prefix
		if idx < 0 {
			idx = 0
		}
		if idx >= len(w.items) {
			idx = len(w.items) - 1
		}
		if idx >= 0 {
			w.selIdx = idx
			w.doc.CursorLine = w.selIdx + w.prefix
		}
	}
	return ok
}

func (w *AssemblyWidget) SearchPrev() bool {
	if w == nil || w.doc == nil {
		return false
	}
	w.doc.SetSearchColor(w.searchColor())
	w.doc.CursorLine = w.selIdx + w.prefix
	ok := w.doc.SearchPrev(w.lineCount(), w.lineAt)
	if ok {
		idx := w.doc.CursorLine - w.prefix
		if idx < 0 {
			idx = 0
		}
		if idx >= len(w.items) {
			idx = len(w.items) - 1
		}
		if idx >= 0 {
			w.selIdx = idx
			w.doc.CursorLine = w.selIdx + w.prefix
		}
	}
	return ok
}

func (w *AssemblyWidget) SetSearchColor(c tcell.Color) {
	if w == nil || w.doc == nil {
		return
	}
	w.doc.SetSearchColor(c)
}

func (w *AssemblyWidget) WordAtCursor() string {
	if w == nil || w.doc == nil {
		return ""
	}
	w.doc.CursorLine = w.selIdx + w.prefix
	return w.doc.WordAtCursor(w.lineAt)
}

func (w *AssemblyWidget) CursorInSearchMatch() bool {
	if w == nil || w.doc == nil {
		return false
	}
	w.doc.CursorLine = w.selIdx + w.prefix
	return w.doc.CursorInSearchMatch(w.lineAt)
}

func (w *AssemblyWidget) LinesForTest() []string {
	n := w.lineCount()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.displayLine(i)
	}
	return out
}

func normalizeAsmAddr(s string) string {
	return models.NormalizeAddr(s)
}

func indexOfAsmAddr(items []models.AsmLine, addr string) int {
	norm := normalizeAsmAddr(addr)
	if norm == "" {
		return -1
	}
	for i, it := range items {
		if normalizeAsmAddr(it.Addr) == norm {
			return i
		}
	}
	return -1
}
