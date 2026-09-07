package termui

import (
	"strings"
	"time"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// DocumentView holds scroll, search, selection, and input state for widgets
// that paint lines themselves (CodeWidget, AssemblyWidget).
type DocumentView struct {
	ScrollViewport

	searchPattern       string
	searchCommitted     string
	searchColor         tcell.Color
	searchContentOffset int
	onSearchJump        func(lineIdx int)

	selAnchor    docPos
	selCursor    docPos
	selActive    bool
	hasSel       bool
	lastClickTime time.Time
	lastClickPos  docPos
	clickCount    int
	suppressDrag  bool

	clipboard     ClipboardIO
	readOnly      bool
	cursor        CursorPainter
	cursorVisible bool
}

type docPos struct {
	line int
	col  int
}

func NewDocumentView() *DocumentView {
	dv := &DocumentView{
		cursor:   NewInverseCursor(),
		readOnly: true,
	}
	dv.dragAutoScroll = true
	return dv
}

func (dv *DocumentView) SetReadOnly(ro bool)       { dv.readOnly = ro }
func (dv *DocumentView) SetCursor(c CursorPainter) { dv.cursor = c }
func (dv *DocumentView) SetCursorVisible(v bool)   { dv.cursorVisible = v }
func (dv *DocumentView) SetClipboard(io ClipboardIO) {
	dv.clipboard = io
}
func (dv *DocumentView) SetDragAutoScroll(on bool) { dv.ScrollViewport.SetDragAutoScroll(on) }
func (dv *DocumentView) SetOnSearchJump(fn func(lineIdx int)) {
	dv.onSearchJump = fn
}
func (dv *DocumentView) SetSearchContentOffset(n int) {
	if n < 0 {
		n = 0
	}
	dv.searchContentOffset = n
}

func (dv *DocumentView) HasSelection() bool { return dv.hasSel }
func (dv *DocumentView) ClearSelection()    { dv.clearSelection() }

func (dv *DocumentView) CursorVisible() bool { return dv.cursorVisible }

func (dv *DocumentView) CursorDrawPos(lineCount int, lineAt func(lineIdx int) string) (localX, localY int, under rune, ok bool) {
	return dv.ScrollViewport.CursorDrawPos(lineAt, lineCount)
}

func (dv *DocumentView) DrawCursor(c Canvas, lineCount int, lineAt func(lineIdx int) string) {
	if dv.cursor == nil || dv.hasSel || !dv.cursorVisible {
		return
	}
	host := docCursorHost{dv: dv, lineCount: lineCount, lineAt: lineAt}
	dv.cursor.Draw(c, host)
}

type docCursorHost struct {
	dv        *DocumentView
	lineCount int
	lineAt    func(lineIdx int) string
}

func (h docCursorHost) CursorDrawPos() (localX, localY int, under rune, ok bool) {
	return h.dv.ScrollViewport.CursorDrawPos(h.lineAt, h.lineCount)
}
func (h docCursorHost) HasSelection() bool  { return h.dv.hasSel }
func (h docCursorHost) CursorVisible() bool { return h.dv.cursorVisible }

func (dv *DocumentView) ViewScrollColLeft() {
	dv.ScrollViewport.ViewScrollColLeft()
}

func (dv *DocumentView) ViewScrollColRightFor(lineCount int, lineAt func(lineIdx int) string) {
	max := 0
	for i := 0; i < lineCount; i++ {
		if lineAt == nil {
			break
		}
		if w := len([]rune(lineAt(i))); w > max {
			max = w
		}
	}
	dv.ViewScrollColRight(dv.MaxLeft(max))
}

func (dv *DocumentView) HandleEvent(ev tcell.Event, lineCount int, lineAt func(lineIdx int) string) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		dv.handleMouse(e, lineCount, lineAt)
	case *tcell.EventKey:
		dv.handleKey(e, lineCount, lineAt)
	}
}

func (dv *DocumentView) handleKey(key *tcell.EventKey, lineCount int, lineAt func(lineIdx int) string) {
	ctrl := key.Modifiers()&tcell.ModCtrl != 0
	switch {
	case key.Key() == tcell.KeyCtrlC || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'c'):
		if dv.hasSel {
			dv.CopySelection(lineAt)
		}
		return
	case key.Key() == tcell.KeyCtrlX || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'x'):
		if dv.hasSel {
			dv.CopySelection(lineAt)
		}
		return
	}
}

func (dv *DocumentView) handleMouse(e *tcell.EventMouse, lineCount int, lineAt func(lineIdx int) string) {
	if lineCount <= 0 || dv.width <= 0 || dv.height <= 0 {
		return
	}
	mx, my := e.Position()
	lx := mx - dv.screenX
	ly := my - dv.screenY

	if e.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		if !dv.selActive && (lx < 0 || ly < 0 || lx >= dv.width || ly >= dv.height) {
			return
		}
		if e.Buttons()&tcell.WheelUp != 0 {
			dv.viewScrollLineUp(lineCount)
		}
		if e.Buttons()&tcell.WheelDown != 0 {
			dv.viewScrollLineDown(lineCount, lineAt)
		}
		return
	}

	if isMiddlePaste(e) {
		if lx < 0 || ly < 0 || lx >= dv.width || ly >= dv.height {
			return
		}
		return
	}

	if e.Buttons() == tcell.ButtonNone {
		dv.suppressDrag = false
		if dv.selActive {
			dv.selActive = false
			if lx >= 0 && ly >= 0 && lx < dv.width && ly < dv.height {
				dv.selCursor = dv.posFromLocal(lx, ly, lineCount, lineAt)
			}
			dv.hasSel = dv.selAnchor != dv.selCursor
			if dv.hasSel {
				dv.CopySelection(lineAt)
			}
		}
		return
	}

	if e.Buttons()&tcell.ButtonPrimary == 0 {
		return
	}
	if dv.suppressDrag {
		return
	}

	if !dv.selActive {
		if lx < 0 || ly < 0 || lx >= dv.width || ly >= dv.height {
			return
		}
		pos := dv.posFromLocal(lx, ly, lineCount, lineAt)
		dv.noteClick(pos, e.When())
		switch dv.clickCount {
		case 2:
			if dv.selectWordAt(pos, lineCount, lineAt) {
				dv.CopySelection(lineAt)
				dv.suppressDrag = true
				return
			}
		case 3:
			if dv.selectLineAt(pos, lineAt) {
				dv.CopySelection(lineAt)
				dv.suppressDrag = true
				return
			}
		}
		dv.clearSelection()
		dv.selActive = true
		dv.selAnchor = pos
		dv.selCursor = pos
		dv.hasSel = false
		dv.CursorLine = pos.line
		dv.CursorCol = pos.col
		dv.clampCursorCol(lineAt)
		return
	}

	if dv.dragAutoScroll {
		for ly < 0 {
			before := dv.Top
			dv.viewScrollLineUp(lineCount)
			ly = 0
			if dv.Top == before {
				break
			}
		}
		for ly >= dv.height {
			before := dv.Top
			dv.viewScrollLineDown(lineCount, lineAt)
			ly = dv.height - 1
			if dv.Top == before {
				break
			}
		}
	}
	if lx < 0 {
		lx = 0
	}
	if lx >= dv.width {
		lx = dv.width - 1
	}
	if ly < 0 {
		ly = 0
	}
	if ly >= dv.height {
		ly = dv.height - 1
	}
	pos := dv.posFromLocal(lx, ly, lineCount, lineAt)
	dv.selCursor = pos
	dv.hasSel = dv.selAnchor != dv.selCursor
	dv.CursorLine = pos.line
	dv.CursorCol = pos.col
	dv.clampCursorCol(lineAt)
	if dv.dragAutoScroll {
		dv.EnsureCursorVisible(lineCount, func(lineIdx int) int {
			return utf8.RuneCountInString(lineAt(lineIdx))
		})
	}
}

func (dv *DocumentView) posFromLocal(lx, ly, lineCount int, lineAt func(lineIdx int) string) docPos {
	p := dv.ScrollViewport.PosFromLocal(lx, ly, lineCount, lineAt)
	return docPos{line: p.line, col: p.col}
}

func (dv *DocumentView) noteClick(pos docPos, when time.Time) {
	if when.IsZero() {
		when = time.Now()
	}
	same := pos.line == dv.lastClickPos.line && absInt(pos.col-dv.lastClickPos.col) <= 1
	if same && when.Sub(dv.lastClickTime) <= clickMultiTimeoutMs*time.Millisecond {
		dv.clickCount++
		if dv.clickCount > 3 {
			dv.clickCount = 1
		}
	} else {
		dv.clickCount = 1
	}
	dv.lastClickTime = when
	dv.lastClickPos = pos
}

func (dv *DocumentView) selectWordAt(pos docPos, lineCount int, lineAt func(lineIdx int) string) bool {
	if pos.line < 0 || pos.line >= lineCount {
		return false
	}
	line := lineAt(pos.line)
	start, end := wordBoundsAt(line, pos.col)
	if start >= end {
		return false
	}
	dv.selActive = false
	dv.selAnchor = docPos{line: pos.line, col: start}
	dv.selCursor = docPos{line: pos.line, col: end}
	dv.hasSel = true
	dv.CursorLine = pos.line
	dv.CursorCol = end
	dv.clampCursorCol(lineAt)
	return true
}

func (dv *DocumentView) selectLineAt(pos docPos, lineAt func(lineIdx int) string) bool {
	if pos.line < 0 {
		return false
	}
	line := lineAt(pos.line)
	dv.selActive = false
	dv.selAnchor = docPos{line: pos.line, col: 0}
	dv.selCursor = docPos{line: pos.line, col: len(line)}
	dv.hasSel = dv.selAnchor != dv.selCursor
	dv.CursorLine = pos.line
	dv.CursorCol = len(line)
	dv.clampCursorCol(lineAt)
	return dv.hasSel
}

func (dv *DocumentView) clampCursorCol(lineAt func(lineIdx int) string) {
	if dv.CursorLine < 0 {
		return
	}
	line := lineAt(dv.CursorLine)
	dv.ScrollViewport.ClampCursorCol(len(line))
}

func (dv *DocumentView) clearSelection() {
	dv.selActive = false
	dv.hasSel = false
}

func (dv *DocumentView) normalizedSel() (start, end docPos) {
	start = dv.selAnchor
	end = dv.selCursor
	if start.line > end.line || (start.line == end.line && start.col > end.col) {
		start, end = end, start
	}
	return start, end
}

func (dv *DocumentView) containsSel(line, col int) bool {
	if !dv.hasSel {
		return false
	}
	start, end := dv.normalizedSel()
	if line < start.line || line > end.line {
		return false
	}
	if start.line == end.line {
		return col >= start.col && col < end.col
	}
	if line == start.line {
		return col >= start.col
	}
	if line == end.line {
		return col < end.col
	}
	return true
}

func (dv *DocumentView) SelectedText(lineAt func(lineIdx int) string) string {
	return platform.StripANSI(dv.selectedText(lineAt))
}

func (dv *DocumentView) selectedText(lineAt func(lineIdx int) string) string {
	if !dv.hasSel || lineAt == nil {
		return ""
	}
	start, end := dv.normalizedSel()
	if start == end {
		return ""
	}
	if start.line == end.line {
		line := lineAt(start.line)
		if start.col > len(line) {
			start.col = len(line)
		}
		if end.col > len(line) {
			end.col = len(line)
		}
		if start.col >= end.col {
			return ""
		}
		return line[start.col:end.col]
	}
	var b strings.Builder
	first := lineAt(start.line)
	if start.col > len(first) {
		start.col = len(first)
	}
	b.WriteString(first[start.col:])
	for line := start.line + 1; line < end.line; line++ {
		b.WriteByte('\n')
		b.WriteString(lineAt(line))
	}
	last := lineAt(end.line)
	if end.col > len(last) {
		end.col = len(last)
	}
	b.WriteByte('\n')
	b.WriteString(last[:end.col])
	return b.String()
}

func (dv *DocumentView) CopySelection(lineAt func(lineIdx int) string) {
	dv.clipboard.copyText(platform.StripANSI(dv.selectedText(lineAt)))
}

func (dv *DocumentView) viewScrollLineUp(lineCount int) {
	if dv.Top > 0 {
		dv.Top--
		if dv.selActive {
			dv.CursorLine = dv.Top
			dv.CursorCol = 0
		} else if dv.CursorLine > 0 {
			dv.CursorLine--
		}
	}
	if dv.CursorLine >= lineCount && lineCount > 0 {
		dv.CursorLine = lineCount - 1
	}
}

func (dv *DocumentView) viewScrollLineDown(lineCount int, lineAt func(lineIdx int) string) {
	if lineCount <= 0 {
		return
	}
	last := lineCount - 1
	maxTop := 0
	if dv.height > 0 && last >= dv.height {
		maxTop = last - dv.height + 1
	}
	if dv.Top < maxTop {
		dv.Top++
		if dv.selActive {
			dv.CursorLine = dv.Top + dv.height - 1
			if dv.CursorLine > last {
				dv.CursorLine = last
			}
			dv.CursorCol = len(lineAt(dv.CursorLine))
		} else if dv.CursorLine < last {
			dv.CursorLine++
		}
	}
}

// SearchHost methods for DocumentView.

func (dv *DocumentView) SetSearchColor(c tcell.Color) { dv.searchColor = c }

func (dv *DocumentView) SetSearchPattern(pattern string) { dv.searchPattern = pattern }

func (dv *DocumentView) CommitSearch(pattern string, lineCount int, lineAt func(lineIdx int) string) {
	dv.searchPattern = pattern
	dv.searchCommitted = pattern
	if pattern == "" {
		return
	}
	if dv.lineMatches(dv.CursorLine, lineAt) {
		dv.jumpToSearchLine(dv.CursorLine, lineCount, lineAt)
		return
	}
	_ = dv.SearchNext(lineCount, lineAt)
}

func (dv *DocumentView) RevertSearch() { dv.searchPattern = dv.searchCommitted }

func (dv *DocumentView) SearchPattern() string { return dv.searchPattern }

func (dv *DocumentView) SearchNext(lineCount int, lineAt func(lineIdx int) string) bool {
	return dv.searchJump(1, lineCount, lineAt)
}

func (dv *DocumentView) SearchPrev(lineCount int, lineAt func(lineIdx int) string) bool {
	return dv.searchJump(-1, lineCount, lineAt)
}

func (dv *DocumentView) WordAtCursor(lineAt func(lineIdx int) string) string {
	content := dv.searchContent(dv.CursorLine, lineAt)
	if content == "" {
		return ""
	}
	return identAtOrNear(content, dv.contentByteAtCursor(lineAt))
}

func (dv *DocumentView) CursorInSearchMatch(lineAt func(lineIdx int) string) bool {
	if dv.searchPattern == "" {
		return false
	}
	content := dv.searchContent(dv.CursorLine, lineAt)
	if content == "" {
		return false
	}
	byteAt := dv.contentByteAtCursor(lineAt)
	if byteAt < 0 {
		byteAt = 0
	}
	if byteAt > len(content) {
		byteAt = len(content)
	}
	contentCol := utf8.RuneCountInString(content[:byteAt])
	if contentCol > 0 && byteAt >= len(content) {
		contentCol--
	}
	return dv.runeInSearchMatch(dv.CursorLine, contentCol, lineAt)
}

func (dv *DocumentView) searchJump(dir, lineCount int, lineAt func(lineIdx int) string) bool {
	if dv.searchPattern == "" || lineCount <= 0 {
		return false
	}
	start := dv.CursorLine
	if start < 0 {
		start = 0
	}
	if start >= lineCount {
		start = lineCount - 1
	}
	for step := 1; step <= lineCount; step++ {
		idx := (start + dir*step) % lineCount
		if idx < 0 {
			idx += lineCount
		}
		if dv.lineMatches(idx, lineAt) {
			dv.jumpToSearchLine(idx, lineCount, lineAt)
			return true
		}
	}
	return false
}

func (dv *DocumentView) jumpToSearchLine(lineIdx, lineCount int, lineAt func(lineIdx int) string) {
	dv.CursorLine = lineIdx
	dv.placeCursorOnSearchMatch(lineIdx, lineAt)
	dv.EnsureLineVisible(lineCount)
	dv.EnsureCursorVisible(lineCount, func(i int) int {
		return utf8.RuneCountInString(lineAt(i))
	})
	if dv.onSearchJump != nil {
		dv.onSearchJump(lineIdx)
	}
}

func (dv *DocumentView) placeCursorOnSearchMatch(lineIdx int, lineAt func(lineIdx int) string) {
	if dv.searchPattern == "" {
		dv.CursorCol = 0
		return
	}
	content := dv.searchContent(lineIdx, lineAt)
	rel := strings.Index(content, dv.searchPattern)
	contentCol := 0
	if rel >= 0 {
		contentCol = utf8.RuneCountInString(content[:rel])
	}
	vis := dv.searchContentOffset + contentCol
	raw := lineAt(lineIdx)
	runes := []rune(raw)
	if vis > len(runes) {
		vis = len(runes)
	}
	dv.CursorCol = len(string(runes[:vis]))
}

func (dv *DocumentView) lineMatches(lineIdx int, lineAt func(lineIdx int) string) bool {
	if dv.searchPattern == "" {
		return false
	}
	return strings.Contains(dv.searchContent(lineIdx, lineAt), dv.searchPattern)
}

func (dv *DocumentView) searchContent(lineIdx int, lineAt func(lineIdx int) string) string {
	if lineAt == nil || lineIdx < 0 {
		return ""
	}
	line := lineAt(lineIdx)
	plain := []rune(line)
	off := dv.searchContentOffset
	if off > len(plain) {
		return ""
	}
	return string(plain[off:])
}

func (dv *DocumentView) contentByteAtCursor(lineAt func(lineIdx int) string) int {
	content := dv.searchContent(dv.CursorLine, lineAt)
	if content == "" {
		return 0
	}
	raw := lineAt(dv.CursorLine)
	visCol := utf8.RuneCountInString(raw[:min(dv.CursorCol, len(raw))])
	col := visCol - dv.searchContentOffset
	if col < 0 {
		col = 0
	}
	runes := []rune(content)
	if len(runes) == 0 {
		return 0
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}
	return len(string(runes[:col]))
}

func (dv *DocumentView) searchBG() tcell.Color {
	if dv.searchColor == tcell.ColorDefault {
		return platform.DefaultSearchColor
	}
	return dv.searchColor
}

func (dv *DocumentView) ApplySearchStyle(lineIdx, absVisCol int, st tcell.Style, lineAt func(lineIdx int) string) tcell.Style {
	if dv.searchPattern == "" {
		return st
	}
	contentCol := absVisCol - dv.searchContentOffset
	if contentCol < 0 {
		return st
	}
	if dv.runeInSearchMatch(lineIdx, contentCol, lineAt) {
		bg := dv.searchBG()
		return st.Background(bg).Foreground(platform.ContrastColor(bg))
	}
	return st
}

func (dv *DocumentView) runeInSearchMatch(lineIdx, contentCol int, lineAt func(lineIdx int) string) bool {
	if dv.searchPattern == "" || contentCol < 0 {
		return false
	}
	line := dv.searchContent(lineIdx, lineAt)
	pat := dv.searchPattern
	if pat == "" || line == "" {
		return false
	}
	for start := 0; start <= len(line); {
		rel := strings.Index(line[start:], pat)
		if rel < 0 {
			return false
		}
		byteStart := start + rel
		runeStart := utf8.RuneCountInString(line[:byteStart])
		runeEnd := runeStart + utf8.RuneCountInString(pat)
		if contentCol >= runeStart && contentCol < runeEnd {
			return true
		}
		next := byteStart + len(pat)
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return false
}

func (dv *DocumentView) CellSelected(lineIdx, byteCol int) bool {
	return dv.containsSel(lineIdx, byteCol)
}
