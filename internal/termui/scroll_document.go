package termui

import (
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

type bufferPos struct {
	line int
	col  int
}

type ScrollDocument struct {
	Buffer *platform.Buffer

	// First visible line/column
	Top  int
	Left int

	// Logical cursor
	CursorLine int
	CursorCol  int

	// Last canvas size seen during Draw (used for scrolling).
	width      int
	height     int
	followTail bool
	// padTop is blank rows above content when follow-tail and the buffer is
	// shorter than the viewport (bottom-align short buffers). Unused today
	// when the draw area height matches the line count (e.g. GDB walking
	// prompt); kept for layouts that want content pinned to the bottom edge.
	padTop int

	// Screen origin of the widget rect (set during Draw).
	screenX int
	screenY int

	// Text selection (buffer coordinates).
	selAnchor bufferPos
	selCursor bufferPos
	selActive bool
	hasSel    bool

	// Multi-click (double = word, triple = line), like a native terminal.
	lastClickTime time.Time
	lastClickPos  bufferPos
	clickCount    int
	suppressDrag  bool // ignore drag after word/line multi-click until release
	// dragAutoScroll enables edge auto-scroll while dragging a selection
	// outside the pane (default on). Assembly disables this so mouse moves
	// do not scroll the dump while selecting text.
	dragAutoScroll bool

	clipboard ClipboardIO
	readOnly  bool // true → Cut copies only; Paste ignored

	// LineStyle optionally colors a full buffer line before selection reverse.
	LineStyle func(line string) tcell.Style

	// RowStyle is like LineStyle but also receives the 0-based buffer line index.
	// When set, it takes precedence over LineStyle.
	RowStyle func(lineIdx int, line string) tcell.Style

	// CellStyle optionally adjusts each painted cell's style. absVisCol is the
	// absolute visible column in the buffer line (0 = leftmost, before Left scroll).
	// Used for substring highlights (e.g. /search matches) without whole-line bg.
	CellStyle func(lineIdx int, absVisCol int, st tcell.Style) tcell.Style

	// /search state (see viewport_search.go). searchContentOffset skips a leading
	// gutter (CodeWidget). onSearchJump syncs list-pane selection rows.
	searchPattern       string
	searchCommitted     string
	searchColor         tcell.Color
	searchContentOffset int
	onSearchJump        func(lineIdx int)

	// OmitTail skips this many trailing buffer lines when drawing/scrolling
	// (legacy: live prompt shared the input row on the last line).
	OmitTail int

	cursor        CursorPainter
	cursorVisible bool
}

func NewScrollDocument(buf *platform.Buffer) *ScrollDocument {
	return &ScrollDocument{
		Buffer:         buf,
		cursor:         NewNativeCursor(),
		cursorVisible:  true,
		readOnly:       true,
		dragAutoScroll: true,
	}
}

// SetDragAutoScroll toggles edge auto-scroll while dragging a selection out of
// the pane. Default is on; Assembly turns it off.
func (d *ScrollDocument) SetDragAutoScroll(on bool) {
	if d != nil {
		d.dragAutoScroll = on
	}
}

// SetCursor replaces the viewport caret painter (NativeCursor by default).
func (d *ScrollDocument) SetCursor(c CursorPainter) {
	if c == nil {
		c = NewNativeCursor()
	}
	d.cursor = c
}

// SetCursorVisible toggles caret painting (typically only the focused pane).
func (d *ScrollDocument) SetCursorVisible(visible bool) {
	d.cursorVisible = visible
}

// cursorDrawPos maps CursorCol/CursorLine to pane-local paint coordinates.
func (d *ScrollDocument) CursorDrawPos() (localX, localY int, under rune, ok bool) {
	return d.cursorDrawPos()
}

// CursorVisible reports whether the caret should be painted.
func (d *ScrollDocument) CursorVisible() bool { return d.cursorVisible }

// cursorDrawPos maps CursorCol/CursorLine to pane-local paint coordinates.
// CursorCol is always a byte index; localX uses the visible cell column.
func (d *ScrollDocument) cursorDrawPos() (localX, localY int, under rune, ok bool) {
	if d == nil || d.Buffer == nil {
		return 0, 0, ' ', false
	}
	localY = d.CursorLine - d.Top + d.padTop
	if localY < 0 || localY >= d.height {
		return 0, 0, ' ', false
	}
	line := d.Buffer.Line(d.CursorLine)
	under = ' '
	visCol := VisibleColAtByte(line, d.CursorCol)
	if d.CursorCol >= 0 && d.CursorCol < len(line) {
		r, _ := utf8.DecodeRuneInString(line[d.CursorCol:])
		if r != utf8.RuneError {
			under = r
		}
	}
	localX = visCol - d.Left
	if localX < 0 || localX >= d.width {
		return 0, 0, ' ', false
	}
	return localX, localY, under, true
}

// SetMouseOrigin sets the screen position of this viewport's top-left cell.
func (d *ScrollDocument) SetMouseOrigin(screenX, screenY int) {
	if d == nil {
		return
	}
	d.screenX, d.screenY = screenX, screenY
}

// Draw renders the visible portion of the buffer.
func (d *ScrollDocument) Draw(c Canvas) {
	if d.Buffer == nil {
		return
	}

	d.width = c.W()
	d.height = c.H()
	d.screenX = c.ScreenX(0)
	d.screenY = c.ScreenY(0)

	if d.followTail {
		d.ScrollToBottom()
	}
	d.padTop = d.followPadTop()

	style := tcell.StyleDefault
	selStyle := style.Reverse(true)
	width := d.width
	height := d.height
	lineLimit := d.lineLimit()

	for row := 0; row < height; row++ {
		if row < d.padTop {
			c.ClearLine(row, style)
			continue
		}
		line := d.Top + (row - d.padTop)
		if line >= lineLimit {
			c.ClearLine(row, style)
			continue
		}

		full := d.Buffer.Line(line)
		lineStyle := style
		if d.RowStyle != nil {
			lineStyle = d.RowStyle(line, full)
		} else 		if d.LineStyle != nil {
			lineStyle = d.LineStyle(full)
		}

		// Horizontal scroll and columns are rune-based (not byte offsets), so
		// multi-byte glyphs like ━ / │ / ▶ do not leave gaps.
		runes := []rune(full)
		start := d.Left
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
		byteIdx := ByteIndexAtVisibleCol(full, start)
		for col, ch := range visible {
			absVisCol := start + col
			st := lineStyle
			if d.CellStyle != nil {
				st = d.CellStyle(line, absVisCol, st)
			}
			st = d.applySearchStyle(line, absVisCol, st)
			if d.containsSel(line, byteIdx) {
				st = selStyle
			}
			c.SetContent(col, row, ch, st)
			byteIdx += utf8.RuneLen(ch)
		}
	}

	d.cursor.Draw(c, d)
	// d.drawCursor(c)
}

func (d *ScrollDocument) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		d.handleMouse(e)
	case *tcell.EventKey:
		d.handleKey(e)
	case *tcell.EventPaste:
		d.PasteAtCursor()
	}
}

// ScrollLineUp moves the caret up one line. The viewport scrolls only once
// the caret is already on the first visible line (same as classic Up).
func (d *ScrollDocument) ScrollLineUp() {
	d.leaveFollowTail()
	d.Up()
	d.extendSelectionAfterScroll(-1)
	d.EnsureVisible(d.width, d.height)
}

// ScrollLineDown moves the caret down one line; scrolls at the bottom edge.
func (d *ScrollDocument) ScrollLineDown() {
	d.Down()
	d.maybeFollowTail()
	d.extendSelectionAfterScroll(1)
	d.EnsureVisible(d.width, d.height)
}

// ViewScrollLineUp always shifts the viewport one line (mouse wheel).
func (d *ScrollDocument) ViewScrollLineUp() {
	d.leaveFollowTail()
	if d.Top > 0 {
		d.Top--
		if d.selActive {
			// Live drag: pull selection into newly revealed lines at the top edge.
			d.CursorLine = d.Top
			d.CursorCol = 0
		} else if d.CursorLine > 0 {
			d.CursorLine--
		}
	}
	d.clampCursorIntoView()
	d.extendSelectionAfterScroll(-1)
}

// ViewScrollLineDown always shifts the viewport one line (mouse wheel).
func (d *ScrollDocument) ViewScrollLineDown() {
	if d.Buffer == nil {
		return
	}
	last := d.Buffer.NumLines() - 1
	if last < 0 {
		return
	}
	maxTop := 0
	if d.height > 0 && last >= d.height {
		maxTop = last - d.height + 1
	}
	if d.Top < maxTop {
		d.Top++
		if d.selActive {
			// Live drag: pull selection into newly revealed lines at the bottom edge.
			d.CursorLine = d.Top + d.height - 1
			if d.CursorLine > last {
				d.CursorLine = last
			}
			d.CursorCol = len(d.Buffer.Line(d.CursorLine))
		} else if d.CursorLine < last {
			d.CursorLine++
		}
	}
	d.clampCursorIntoView()
	d.maybeFollowTail()
	d.extendSelectionAfterScroll(1)
}

// lineContentWidth returns how many columns a buffer line occupies for
// horizontal scrolling (rune count).
func (d *ScrollDocument) lineContentWidth(line string) int {
	return utf8.RuneCountInString(line)
}

// maxContentWidth is the widest line in the buffer (columns).
func (d *ScrollDocument) maxContentWidth() int {
	if d.Buffer == nil {
		return 0
	}
	max := 0
	n := d.Buffer.NumLines()
	for i := 0; i < n; i++ {
		if w := d.lineContentWidth(d.Buffer.Line(i)); w > max {
			max = w
		}
	}
	return max
}

// maxLeft is the largest Left such that some content remains visible.
func (d *ScrollDocument) maxLeft() int {
	if d.width <= 0 {
		return 0
	}
	max := d.maxContentWidth() - d.width
	if max < 0 {
		return 0
	}
	return max
}

// ViewScrollColLeft shifts the viewport one column left (view-only).
func (d *ScrollDocument) ViewScrollColLeft() {
	d.leaveFollowTail()
	if d.Left > 0 {
		d.Left--
	}
}

// ViewScrollColRight shifts the viewport one column right when content is
// wider than the pane (view-only).
func (d *ScrollDocument) ViewScrollColRight() {
	d.leaveFollowTail()
	if d.Left < d.maxLeft() {
		d.Left++
	}
}

func (d *ScrollDocument) ScrollPageUp(pageSize int) {
	d.leaveFollowTail()
	d.PageUp(pageSize)
	d.extendSelectionAfterScroll(-1)
	d.EnsureVisible(d.width, d.height)
}

func (d *ScrollDocument) ScrollPageDown(pageSize int) {
	d.PageDown(pageSize)
	d.maybeFollowTail()
	d.extendSelectionAfterScroll(1)
	d.EnsureVisible(d.width, d.height)
}

func (d *ScrollDocument) ScrollHome() {
	d.leaveFollowTail()
	d.Home()
	d.extendSelectionAfterScroll(-1)
	d.EnsureCursorVisible()
}

func (d *ScrollDocument) ScrollEnd() {
	d.End()
	d.maybeFollowTail()
	d.extendSelectionAfterScroll(1)
	d.EnsureCursorVisible()
}

// extendSelectionAfterScroll keeps an existing mark while scrolling, and while
// dragging extends selCursor to the caret on the newly revealed edge.
func (d *ScrollDocument) extendSelectionAfterScroll(dir int) {
	if !d.selActive && !d.hasSel {
		return
	}
	if d.selActive {
		d.selCursor = bufferPos{line: d.CursorLine, col: d.CursorCol}
		d.hasSel = d.selAnchor != d.selCursor
	}
	_ = dir
}

func (d *ScrollDocument) clampCursorIntoView() {
	if d.height <= 0 {
		return
	}
	if d.CursorLine < d.Top {
		d.CursorLine = d.Top
	}
	bottom := d.Top + d.height - 1
	if d.CursorLine > bottom {
		d.CursorLine = bottom
	}
	d.clampCursorCol()
}

func (d *ScrollDocument) handleMouse(e *tcell.EventMouse) {
	if d.Buffer == nil || d.width <= 0 || d.height <= 0 {
		return
	}

	mx, my := e.Position()
	lx := mx - d.screenX
	ly := my - d.screenY

	if e.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		// Allow wheel during an active drag even if the pointer is outside;
		// otherwise require the pointer over the pane.
		if !d.selActive && (lx < 0 || ly < 0 || lx >= d.width || ly >= d.height) {
			return
		}
		if e.Buttons()&tcell.WheelUp != 0 {
			d.ViewScrollLineUp()
		}
		if e.Buttons()&tcell.WheelDown != 0 {
			d.ViewScrollLineDown()
		}
		return
	}

	// Middle-click paste (Linux PRIMARY selection); only inside the pane.
	if isMiddlePaste(e) {
		if lx < 0 || ly < 0 || lx >= d.width || ly >= d.height {
			return
		}
		d.PastePrimaryAtCursor()
		return
	}

	if e.Buttons() == tcell.ButtonNone {
		d.suppressDrag = false
		if d.selActive {
			d.selActive = false
			if lx >= 0 && ly >= 0 && lx < d.width && ly < d.height {
				d.selCursor = d.posFromLocal(lx, ly)
			}
			d.hasSel = d.selAnchor != d.selCursor
			if d.hasSel {
				d.CopySelection()
			}
		}
		return
	}

	if e.Buttons()&tcell.ButtonPrimary == 0 {
		return
	}

	// After double/triple-click, hold the selection until button release.
	if d.suppressDrag {
		return
	}

	// New click must start inside; drag may leave the pane and auto-scroll.
	if !d.selActive {
		if lx < 0 || ly < 0 || lx >= d.width || ly >= d.height {
			return
		}
		pos := d.posFromLocal(lx, ly)
		d.noteClick(pos, e.When())
		switch d.clickCount {
		case 2:
			if d.selectWordAt(pos) {
				d.CopySelection()
				d.suppressDrag = true
				return
			}
		case 3:
			if d.selectLineAt(pos) {
				d.CopySelection()
				d.suppressDrag = true
				return
			}
		}
		d.clearSelection()
		d.selActive = true
		d.selAnchor = pos
		d.selCursor = pos
		d.hasSel = false
		d.CursorLine = pos.line
		d.CursorCol = pos.col
		d.clampCursorCol()
		return
	}

	// Drag beyond the pane edges: optionally scroll to reveal more text and
	// pin the selection endpoint to that edge.
	if d.dragAutoScroll {
		for ly < 0 {
			before := d.Top
			d.ViewScrollLineUp()
			ly = 0
			if d.Top == before {
				break
			}
		}
		for ly >= d.height {
			before := d.Top
			d.ViewScrollLineDown()
			ly = d.height - 1
			if d.Top == before {
				break
			}
		}
	}
	if lx < 0 {
		lx = 0
	}
	if lx >= d.width {
		lx = d.width - 1
	}
	if ly < 0 {
		ly = 0
	}
	if ly >= d.height {
		ly = d.height - 1
	}

	pos := d.posFromLocal(lx, ly)
	d.selCursor = pos
	d.hasSel = d.selAnchor != d.selCursor
	d.CursorLine = pos.line
	d.CursorCol = pos.col
	d.clampCursorCol()
	if d.dragAutoScroll {
		d.EnsureVisible(d.width, d.height)
	}
}

func (d *ScrollDocument) noteClick(pos bufferPos, when time.Time) {
	if when.IsZero() {
		when = time.Now()
	}
	same := pos.line == d.lastClickPos.line && absInt(pos.col-d.lastClickPos.col) <= 1
	if same && when.Sub(d.lastClickTime) <= clickMultiTimeoutMs*time.Millisecond {
		d.clickCount++
		if d.clickCount > 3 {
			d.clickCount = 1
		}
	} else {
		d.clickCount = 1
	}
	d.lastClickTime = when
	d.lastClickPos = pos
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// selectWordAt marks the word under pos and returns true if anything selected.
func (d *ScrollDocument) selectWordAt(pos bufferPos) bool {
	if d.Buffer == nil || pos.line < 0 || pos.line >= d.Buffer.NumLines() {
		return false
	}
	line := d.Buffer.Line(pos.line)
	start, end := wordBoundsAt(line, pos.col)
	if start >= end {
		return false
	}
	d.selActive = false
	d.selAnchor = bufferPos{line: pos.line, col: start}
	d.selCursor = bufferPos{line: pos.line, col: end}
	d.hasSel = true
	d.CursorLine = pos.line
	d.CursorCol = end
	d.clampCursorCol()
	return true
}

// selectLineAt marks the whole buffer line under pos.
func (d *ScrollDocument) selectLineAt(pos bufferPos) bool {
	if d.Buffer == nil || pos.line < 0 || pos.line >= d.Buffer.NumLines() {
		return false
	}
	line := d.Buffer.Line(pos.line)
	d.selActive = false
	d.selAnchor = bufferPos{line: pos.line, col: 0}
	d.selCursor = bufferPos{line: pos.line, col: len(line)}
	d.hasSel = d.selAnchor != d.selCursor
	d.CursorLine = pos.line
	d.CursorCol = len(line)
	d.clampCursorCol()
	return d.hasSel
}

func (d *ScrollDocument) handleKey(key *tcell.EventKey) {
	ctrl := key.Modifiers()&tcell.ModCtrl != 0
	switch {
	case key.Key() == tcell.KeyCtrlC || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'c'):
		if d.hasSel {
			d.CopySelection()
		}
		return
	case key.Key() == tcell.KeyCtrlX || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'x'):
		if d.hasSel {
			d.CutSelection()
		}
		return
	case key.Key() == tcell.KeyCtrlV || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'v'):
		d.PasteAtCursor()
		return
	}

	switch key.Key() {

	case tcell.KeyUp:
		d.leaveFollowTail()
		d.Up()
		d.extendSelectionAfterScroll(-1)

	case tcell.KeyDown:
		d.Down()
		d.maybeFollowTail()
		d.extendSelectionAfterScroll(1)

	case tcell.KeyLeft:
		d.leaveFollowTail()
		d.LeftChar()
		if !d.selActive {
			d.clearSelection()
		}

	case tcell.KeyRight:
		d.RightChar()
		d.maybeFollowTail()
		if !d.selActive {
			d.clearSelection()
		}

	case tcell.KeyPgUp:
		d.leaveFollowTail()
		d.PageUp(10)
		d.extendSelectionAfterScroll(-1)

	case tcell.KeyPgDn:
		d.PageDown(10)
		d.maybeFollowTail()
		d.extendSelectionAfterScroll(1)

	case tcell.KeyHome:
		d.leaveFollowTail()
		d.Home()
		d.extendSelectionAfterScroll(-1)

	case tcell.KeyEnd:
		d.End()
		d.maybeFollowTail()
		d.extendSelectionAfterScroll(1)

	default:
		return
	}

	d.EnsureCursorVisible()
}

func (d *ScrollDocument) followPadTop() int {
	if !d.followTail || d.Buffer == nil || d.height <= 0 {
		return 0
	}
	n := d.lineLimit()
	if n <= 0 || n >= d.height {
		return 0
	}
	return d.height - n
}

// lineLimit is the exclusive end line index for drawing/scrolling (respects OmitTail).
func (d *ScrollDocument) lineLimit() int {
	if d.Buffer == nil {
		return 0
	}
	n := d.Buffer.NumLines() - d.OmitTail
	if n < 0 {
		return 0
	}
	return n
}

func (d *ScrollDocument) posFromLocal(lx, ly int) bufferPos {
	line := d.Top + (ly - d.padTop)
	if line < 0 {
		line = 0
	}
	if d.Buffer != nil {
		last := d.Buffer.NumLines() - 1
		if line > last {
			line = last
		}
	}

	col := d.Left + lx
	lineLen := 0
	if d.Buffer != nil && line >= 0 && line < d.Buffer.NumLines() {
		raw := d.Buffer.Line(line)
		lineLen = len(raw)
		// CursorCol is a byte index; Left+lx is a visible/rune column.
		col = ByteIndexAtVisibleCol(raw, d.Left+lx)
	}
	if col > lineLen {
		col = lineLen
	}

	return bufferPos{line: line, col: col}
}

func (d *ScrollDocument) normalizedSel() (start, end bufferPos) {
	start = d.selAnchor
	end = d.selCursor
	if start.line > end.line || (start.line == end.line && start.col > end.col) {
		start, end = end, start
	}
	return start, end
}

func (d *ScrollDocument) containsSel(line, col int) bool {
	if !d.hasSel {
		return false
	}
	start, end := d.normalizedSel()
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

func (d *ScrollDocument) clearSelection() {
	d.selActive = false
	d.hasSel = false
}

// ClearSelection drops any active text mark (e.g. after */# takes the caret word).
func (d *ScrollDocument) ClearSelection() {
	if d != nil {
		d.clearSelection()
	}
}

func (d *ScrollDocument) SetFollowTail(follow bool) {
	d.followTail = follow
}

func (d *ScrollDocument) FollowTail() bool {
	return d.followTail
}

func (d *ScrollDocument) leaveFollowTail() {
	d.followTail = false
}

func (d *ScrollDocument) maybeFollowTail() {
	if d.Buffer == nil {
		return
	}

	last := d.Buffer.NumLines() - 1
	if last < 0 {
		d.followTail = true
		return
	}

	if d.CursorLine < last {
		return
	}

	if d.height <= 0 {
		d.followTail = true
		return
	}

	maxTop := last - d.height + 1
	if maxTop < 0 {
		maxTop = 0
	}
	if d.Top >= maxTop {
		d.followTail = true
	}
}

func (d *ScrollDocument) ScrollToBottom() {
	if d.Buffer == nil {
		return
	}

	last := d.lineLimit() - 1
	if last < 0 {
		return
	}

	d.CursorLine = last
	d.clampCursorCol()

	if d.height > 0 && last >= d.height {
		d.Top = last - d.height + 1
		return
	}

	d.Top = 0
}

func (d *ScrollDocument) Up() {

	if d.CursorLine > 0 {
		d.CursorLine--
		d.clampCursorCol()
	}

	if d.CursorLine < d.Top {
		d.Top = d.CursorLine
	}
}

func (d *ScrollDocument) Down() {

	if d.Buffer == nil {
		return
	}

	last := d.Buffer.NumLines() - 1
	if d.CursorLine < last {
		d.CursorLine++
		d.clampCursorCol()
	}

	if d.height > 0 && d.CursorLine >= d.Top+d.height {
		d.Top = d.CursorLine - d.height + 1
	}
}

func (d *ScrollDocument) LeftChar() {
	if d.Buffer == nil {
		if d.CursorCol > 0 {
			d.CursorCol--
		}
		return
	}
	line := d.Buffer.Line(d.CursorLine)
	vis := VisibleColAtByte(line, d.CursorCol)
	if vis > 0 {
		vis--
		d.CursorCol = ByteIndexAtVisibleCol(line, vis)
	}
	if vis < d.Left {
		d.Left = vis
	}
}

func (d *ScrollDocument) RightChar() {
	if d.Buffer == nil {
		d.CursorCol++
		return
	}
	line := d.Buffer.Line(d.CursorLine)
	vis := VisibleColAtByte(line, d.CursorCol)
	maxVis := utf8.RuneCountInString(line)
	if vis < maxVis {
		vis++
		d.CursorCol = ByteIndexAtVisibleCol(line, vis)
	}
}

func (d *ScrollDocument) PageDown(pageSize int) {

	if d.Buffer == nil {
		return
	}

	d.CursorLine += pageSize

	last := d.Buffer.NumLines() - 1
	if d.CursorLine > last {
		d.CursorLine = last
	}

	d.Top += pageSize
	if d.Top > last {
		d.Top = last
	}
}

func (d *ScrollDocument) PageUp(pageSize int) {

	d.CursorLine -= pageSize
	if d.CursorLine < 0 {
		d.CursorLine = 0
	}

	d.Top -= pageSize
	if d.Top < 0 {
		d.Top = 0
	}
}

// Home moves the caret and view to the top-left of the buffer.
func (d *ScrollDocument) Home() {
	d.CursorLine = 0
	d.CursorCol = 0
	d.Top = 0
	d.Left = 0
}

func (d *ScrollDocument) End() {
	if d.Buffer == nil {
		return
	}

	last := d.lineLimit() - 1
	if last < 0 {
		last = 0
	}

	d.CursorLine = last
	h := d.height
	if h <= 0 {
		h = 20
	}
	maxTop := 0
	if last >= h {
		maxTop = last - h + 1
	}
	d.Top = maxTop
	d.Left = 0
}

func (d *ScrollDocument) Center(line int, pageHeight int) {

	if d.Buffer == nil {
		return
	}

	d.CursorLine = line
	if pageHeight <= 0 {
		pageHeight = 20
	}

	d.Top = line - pageHeight/2
	if d.Top < 0 {
		d.Top = 0
	}

	last := d.Buffer.NumLines() - 1
	if last < 0 {
		return
	}
	maxTop := 0
	if last >= pageHeight {
		maxTop = last - pageHeight + 1
	}
	if d.Top > maxTop {
		d.Top = maxTop
	}
}

// Height returns the last canvas height seen in Draw (0 before first paint).
func (d *ScrollDocument) Height() int { return d.height }

// Width returns the last canvas width seen in Draw (0 before first paint).
func (d *ScrollDocument) Width() int { return d.width }

// HitContentLine reports the buffer line under screen coordinates, or false
// when the pointer is outside the pane or on empty padding below the last line.
// List panes use this so clicks in the blank area do not clamp to the last row.
func (d *ScrollDocument) HitContentLine(screenX, screenY int) (line int, ok bool) {
	if d == nil || d.Buffer == nil || d.width <= 0 || d.height <= 0 {
		return 0, false
	}
	lx := screenX - d.screenX
	ly := screenY - d.screenY
	if lx < 0 || ly < 0 || lx >= d.width || ly >= d.height {
		return 0, false
	}
	if ly < d.padTop {
		return 0, false
	}
	line = d.Top + (ly - d.padTop)
	last := d.lineLimit() - 1
	if line < 0 || line > last {
		return 0, false
	}
	return line, true
}

// EnsureCursorVisible scrolls so CursorLine is on-screen using the last Draw size.
func (d *ScrollDocument) EnsureCursorVisible() {
	w, h := d.width, d.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 20
	}
	d.EnsureVisible(w, h)
}

// EnsureLineVisible scrolls vertically so CursorLine is on-screen without
// changing the horizontal offset (for list panes with view-only Left/Right).
func (d *ScrollDocument) EnsureLineVisible() {
	h := d.height
	if h <= 0 {
		h = 20
	}
	if d.CursorLine < d.Top {
		d.Top = d.CursorLine
	}
	if d.CursorLine >= d.Top+h {
		d.Top = d.CursorLine - h + 1
	}
	if d.Buffer == nil {
		return
	}
	last := d.Buffer.NumLines() - 1
	if last < 0 {
		return
	}
	maxTop := last
	if last >= h-1 {
		maxTop = last - h + 1
	}
	if d.Top > maxTop {
		d.Top = maxTop
	}
	if d.Top < 0 {
		d.Top = 0
	}
}

func (d *ScrollDocument) clampCursorCol() {
	if d.Buffer == nil {
		return
	}

	lineLen := len(d.Buffer.Line(d.CursorLine))
	if d.CursorCol > lineLen {
		d.CursorCol = lineLen
	}
}

func (d *ScrollDocument) EnsureVisible(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	if d.CursorLine < d.Top {
		d.Top = d.CursorLine
	}

	if d.CursorLine >= d.Top+height {
		d.Top = d.CursorLine - height + 1
	}

	if d.Buffer == nil {
		return
	}

	last := d.Buffer.NumLines() - 1
	if last < 0 {
		return
	}

	maxTop := last
	if last >= height-1 {
		maxTop = last - height + 1
	}
	if d.Top > maxTop {
		d.Top = maxTop
	}
	if d.Top < 0 {
		d.Top = 0
	}

	// CursorCol is a byte offset; Left is a visible-cell skip count.
	line := d.Buffer.Line(d.CursorLine)
	vis := d.lineContentWidth(line)
	curVis := VisibleColAtByte(line, d.CursorCol)
	if curVis < d.Left {
		d.Left = curVis
	}
	if curVis >= d.Left+width {
		d.Left = curVis - width + 1
	}
	if d.Left < 0 {
		d.Left = 0
	}
	if vis <= width {
		d.Left = 0
	} else if d.Left > vis-width {
		d.Left = vis - width
	}
}
