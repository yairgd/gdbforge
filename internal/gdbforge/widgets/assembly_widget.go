package widgets

import (
	"fmt"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

const (
	asmPCMarker    = "━━▶"
	asmPCGutterPad = "   "
	// asmGutterCols: "━━▶ " + 18-char addr + " │ " ≈ mark+space+addr+sep
	asmAddrCols   = 18
	asmGutterCols = 3 + 1 + asmAddrCols + 3
)

// AssemblyWidget shows disassembly around browse address X.
// ━━▶ marks real $pc; the blue line is the browse caret (same as CodeWidget).
// Space / e fire breakpoint intents at the browse address (app owns GDB).
type AssemblyWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer
	state    *debugstate.State

	items  []models.AsmLine
	pcAddr string // normalized $pc
	selIdx int    // browse caret index into items

	bpEnabled  map[string]struct{} // addr → enabled
	bpDisabled map[string]struct{} // addr → disabled

	// OnBreakToggle is Space — insert/clear breakpoint at browse addr.
	OnBreakToggle func(addr string)
	// OnToggleEnable is "e" — enable/disable at browse addr.
	OnToggleEnable func()

	// OnBrowse asks the app to refetch a window centered on addr (edge / resize).
	OnBrowse func(addr string, rows int)

	lastHeight int
	fetching   bool
}

func NewAssemblyWidget() *AssemblyWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursor(termui.NewInverseCursor())
	vp.SetCursorVisible(false)
	vp.ANSI = true

	w := &AssemblyWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Assembly"},
		viewport:   vp,
		buf:        buf,
		selIdx:     0,
	}
	vp.RowStyle = w.rowStyle
	vp.SetSearchContentOffset(asmGutterCols)
	vp.SetOnSearchJump(func(lineIdx int) {
		w.selIdx = lineIdx
		w.viewport.CursorLine = lineIdx
	})
	w.initKeyBindings()
	w.rebuild()
	return w
}

func (w *AssemblyWidget) SetAppState(st *debugstate.State) {
	w.state = st
}

func (w *AssemblyWidget) SetClipboard(io termui.ClipboardIO) {
	if w != nil && w.viewport != nil {
		w.viewport.SetClipboard(io)
	}
}

func (w *AssemblyWidget) initKeyBindings() {
	w.BindKeyFunc("sel-up", func(args ...any) { w.MoveSel(-1) }, "<Up>", "k")
	w.BindKeyFunc("sel-down", func(args ...any) { w.MoveSel(1) }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.viewport.ViewScrollColLeft() }, "<Left>", "h")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.viewport.ViewScrollColRight() }, "<Right>", "l")
	w.BindKeyFunc("page-up", func(args ...any) { w.MoveSel(-w.pageRows()) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.MoveSel(w.pageRows()) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("break-toggle", func(args ...any) { w.BreakAtSel() }, " ")
	w.BindKeyFunc("break-enable-toggle", func(args ...any) {
		if w.OnToggleEnable != nil {
			w.OnToggleEnable()
		}
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
	if w == nil || w.viewport == nil {
		return 0
	}
	return w.viewport.Height()
}

// SetItems replaces the instruction list. pcAddr is $pc for ━━▶.
// selAddr is the browse caret (empty → keep prior sel if still present, else pc).
func (w *AssemblyWidget) SetItems(items []models.AsmLine, pcAddr, selAddr string) {
	if w == nil {
		return
	}
	w.items = append([]models.AsmLine(nil), items...)
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
	w.rebuild()
	if len(w.items) > 0 {
		w.viewport.CursorLine = w.selIdx
		w.viewport.EnsureCursorVisible()
	}
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
	w.viewport.CursorLine = w.selIdx
	w.viewport.EnsureCursorVisible()

	// Near either edge → ask app to recenter the window on current X.
	margin := 2
	if n > 8 {
		margin = 3
	}
	if needBrowse || w.selIdx <= margin || w.selIdx >= n-1-margin {
		w.requestBrowse()
	}
}

func (w *AssemblyWidget) requestBrowse() {
	if w == nil || w.OnBrowse == nil || w.fetching {
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
	w.OnBrowse(addr, rows)
}

// SelAddr returns the browse caret address.
func (w *AssemblyWidget) SelAddr() string {
	if w == nil || w.selIdx < 0 || w.selIdx >= len(w.items) {
		return ""
	}
	return w.items[w.selIdx].Addr
}

// BreakAtSel fires OnBreakToggle for the selected address (Space).
func (w *AssemblyWidget) BreakAtSel() {
	if w == nil || w.OnBreakToggle == nil {
		return
	}
	addr := w.SelAddr()
	if addr == "" {
		return
	}
	w.OnBreakToggle(addr)
}

// HasEnabledBreak reports an enabled breakpoint mark at addr.
func (w *AssemblyWidget) HasEnabledBreak(addr string) bool {
	return w.hasEnabledBreak(normalizeAsmAddr(addr))
}

func (w *AssemblyWidget) hasEnabledBreak(addr string) bool {
	if w == nil || w.bpEnabled == nil || addr == "" {
		return false
	}
	_, ok := w.bpEnabled[addr]
	return ok
}

func (w *AssemblyWidget) hasDisabledBreak(addr string) bool {
	if w == nil || w.bpDisabled == nil || addr == "" {
		return false
	}
	_, ok := w.bpDisabled[addr]
	return ok
}

// SetBreakInfos updates address gutter marks from the shared breakpoint model.
// A nil slice means "no update"; a non-nil empty slice clears marks.
func (w *AssemblyWidget) SetBreakInfos(items []models.BreakInfo) {
	if w == nil || items == nil {
		return
	}
	w.bpEnabled = make(map[string]struct{})
	w.bpDisabled = make(map[string]struct{})
	for _, it := range items {
		addr := normalizeAsmAddr(it.Addr)
		if addr == "" {
			continue
		}
		if it.Enabled {
			w.bpEnabled[addr] = struct{}{}
		} else {
			w.bpDisabled[addr] = struct{}{}
		}
	}
	w.rebuild()
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
	}
}

func (w *AssemblyWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st
	}
	addr := ""
	if lineIdx >= 0 && lineIdx < len(w.items) {
		addr = normalizeAsmAddr(w.items[lineIdx].Addr)
	}
	if addr != "" && addr == w.pcAddr {
		st = st.Background(w.pcColor())
	}
	if lineIdx == w.selIdx {
		st = st.Bold(true)
		if addr == "" || addr != w.pcAddr {
			st = st.Background(w.codeSelColor())
		}
	}
	_ = line
	return st
}

func (w *AssemblyWidget) breakColor() tcell.Color {
	if w.state != nil {
		return w.state.BreakColor()
	}
	return platform.DefaultBreakColor
}

func (w *AssemblyWidget) breakDisabledColor() tcell.Color {
	if w.state != nil {
		return w.state.BreakDisabledColor()
	}
	return platform.DefaultBreakDisabledColor
}

func (w *AssemblyWidget) pcColor() tcell.Color {
	if w.state != nil {
		return w.state.PCColor()
	}
	return platform.DefaultPCColor
}

func (w *AssemblyWidget) codeSelColor() tcell.Color {
	if w.state != nil {
		return w.state.CodeSelColor()
	}
	return platform.DefaultCodeSelColor
}

func (w *AssemblyWidget) searchColor() tcell.Color {
	if w.state != nil {
		return w.state.SearchColor()
	}
	return platform.DefaultSearchColor
}

func (w *AssemblyWidget) rebuild() {
	if w == nil || w.buf == nil {
		return
	}
	w.buf.Clear()
	if len(w.items) == 0 {
		w.buf.AppendLine("\x1b[38;5;244m(no disassembly)\x1b[0m")
		return
	}
	for _, it := range w.items {
		mark := asmPCGutterPad
		norm := normalizeAsmAddr(it.Addr)
		if norm == w.pcAddr && w.pcAddr != "" {
			mark = "\x1b[1;38;5;226m" + asmPCMarker + "\x1b[0m"
		}
		addr := it.Addr
		if len(addr) > asmAddrCols {
			addr = addr[len(addr)-asmAddrCols:]
		}
		addrPad := fmt.Sprintf("%-*s", asmAddrCols, addr)
		var addrANSI string
		switch {
		case w.hasEnabledBreak(norm):
			addrANSI = platform.BreakNumberANSI(addrPad, w.breakColor())
		case w.hasDisabledBreak(norm):
			addrANSI = platform.BreakNumberANSI(addrPad, w.breakDisabledColor())
		default:
			addrANSI = "\x1b[38;5;245m" + addrPad + "\x1b[0m"
		}
		inst := it.Inst
		if it.Opcodes != "" {
			inst = it.Opcodes + "  " + it.Inst
		}
		line := fmt.Sprintf("%s %s\x1b[38;5;240m │\x1b[0m %s",
			mark, addrANSI, inst)
		w.buf.AppendLine(line)
	}
}

func (w *AssemblyWidget) Draw(c termui.Canvas) {
	if w == nil {
		return
	}
	h := c.H()
	if h > 0 && h != w.lastHeight {
		w.lastHeight = h
		// Refetch so the window fills the new rectangle.
		if len(w.items) > 0 && w.OnBrowse != nil && !w.fetching {
			w.requestBrowse()
		}
	}
	w.viewport.Draw(c)
}

func (w *AssemblyWidget) syncSelFromViewport() {
	if w == nil || w.viewport == nil || len(w.items) == 0 {
		return
	}
	n := len(w.items)
	idx := w.viewport.CursorLine
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	prev := w.selIdx
	w.selIdx = idx
	// Click near the edge of a loaded window should still refetch, like Up/Down.
	if w.selIdx != prev {
		margin := 2
		if n > 8 {
			margin = 3
		}
		if w.selIdx <= margin || w.selIdx >= n-1-margin {
			w.requestBrowse()
		}
	}
}

func (w *AssemblyWidget) HandleEvent(ev tcell.Event) {
	if w == nil {
		return
	}
	switch e := ev.(type) {
	case *tcell.EventMouse:
		btns := e.Buttons()
		// Wheel moves the blue browse line (same idea as j/k), refetching at edges.
		if btns&tcell.WheelUp != 0 {
			w.MoveSel(-1)
			return
		}
		if btns&tcell.WheelDown != 0 {
			w.MoveSel(1)
			return
		}
		w.viewport.HandleEvent(e)
		// Keep the bold blue cursor line in sync with mouse click / drag.
		w.syncSelFromViewport()
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.viewport.HandleEvent(e)
	default:
		w.viewport.HandleEvent(ev)
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
	if w.viewport != nil {
		w.viewport.SetCursorVisible(focused && len(w.items) > 0)
	}
}

func (w *AssemblyWidget) DrawStatusLine(c termui.Canvas, active bool) {
	title := "Assembly"
	if w.pcAddr != "" {
		title = fmt.Sprintf("Assembly  pc=%s", w.pcAddr)
		if sel := w.SelAddr(); sel != "" && sel != w.pcAddr {
			title = fmt.Sprintf("Assembly  pc=%s  @%s", w.pcAddr, sel)
		}
	}
	w.PaneName = title
	if w.Focused() {
		termui.PaintStatusBar(c, title, active)
		return
	}
	termui.PaintInactiveStatusBar(c, title)
}

func (w *AssemblyWidget) Viewport() *termui.Viewport {
	if w == nil {
		return nil
	}
	return w.viewport
}

// SearchHost implementation (mirrors CodeWidget).

func (w *AssemblyWidget) SetSearchPattern(pattern string) {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.SetSearchColor(w.searchColor())
	w.viewport.SetSearchPattern(pattern)
}

func (w *AssemblyWidget) CommitSearch(pattern string) {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.SetSearchColor(w.searchColor())
	w.viewport.CursorLine = w.selIdx
	w.viewport.CommitSearch(pattern)
}

func (w *AssemblyWidget) RevertSearch() {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.RevertSearch()
}

func (w *AssemblyWidget) SearchPattern() string {
	if w == nil || w.viewport == nil {
		return ""
	}
	return w.viewport.SearchPattern()
}

func (w *AssemblyWidget) SearchNext() bool {
	if w == nil || w.viewport == nil {
		return false
	}
	w.viewport.SetSearchColor(w.searchColor())
	w.viewport.CursorLine = w.selIdx
	ok := w.viewport.SearchNext()
	if ok {
		w.selIdx = w.viewport.CursorLine
	}
	return ok
}

func (w *AssemblyWidget) SearchPrev() bool {
	if w == nil || w.viewport == nil {
		return false
	}
	w.viewport.SetSearchColor(w.searchColor())
	w.viewport.CursorLine = w.selIdx
	ok := w.viewport.SearchPrev()
	if ok {
		w.selIdx = w.viewport.CursorLine
	}
	return ok
}

func (w *AssemblyWidget) SetSearchColor(c tcell.Color) {
	if w == nil || w.viewport == nil {
		return
	}
	w.viewport.SetSearchColor(c)
}

func (w *AssemblyWidget) WordAtCursor() string {
	if w == nil || w.viewport == nil {
		return ""
	}
	w.viewport.CursorLine = w.selIdx
	return w.viewport.WordAtCursor()
}

func (w *AssemblyWidget) CursorInSearchMatch() bool {
	if w == nil || w.viewport == nil {
		return false
	}
	w.viewport.CursorLine = w.selIdx
	return w.viewport.CursorInSearchMatch()
}

func normalizeAsmAddr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Match parse.NormalizeAddr without importing parse (widgets stay free of MI).
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	n := uint64(0)
	ok := true
	for _, c := range s {
		n <<= 4
		switch {
		case c >= '0' && c <= '9':
			n |= uint64(c - '0')
		case c >= 'a' && c <= 'f':
			n |= uint64(c - 'a' + 10)
		default:
			ok = false
		}
	}
	if !ok || s == "" {
		return "0x" + s
	}
	return fmt.Sprintf("0x%x", n)
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
