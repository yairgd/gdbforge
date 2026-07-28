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

type Viewport struct {
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

	// ANSI enables per-cell ANSI/SGR rendering (OSC/CSI sequences are not drawn).
	ANSI bool

	// OmitTail skips this many trailing buffer lines when drawing/scrolling
	// (ConsolePane uses 1 so the live prompt is painted on the input row).
	OmitTail int

	cursor        CursorPainter
	cursorVisible bool
}

func NewViewport(buf *platform.Buffer) *Viewport {
	return &Viewport{
		Buffer:        buf,
		cursor:        NewNativeCursor(),
		cursorVisible: true,
		readOnly:      true,
	}
}

// SetCursor replaces the viewport caret painter (NativeCursor by default).
func (v *Viewport) SetCursor(c CursorPainter) {
	if c == nil {
		c = NewNativeCursor()
	}
	v.cursor = c
}

// SetCursorVisible toggles caret painting (typically only the focused pane).
func (v *Viewport) SetCursorVisible(visible bool) {
	v.cursorVisible = visible
}

// cursorDrawPos maps CursorCol/CursorLine to pane-local paint coordinates.
// In ANSI mode CursorCol is a byte index; localX uses the visible cell column.
func (v *Viewport) cursorDrawPos() (localX, localY int, under rune, ok bool) {
	if v == nil || v.Buffer == nil {
		return 0, 0, ' ', false
	}
	localY = v.CursorLine - v.Top + v.padTop
	if localY < 0 || localY >= v.height {
		return 0, 0, ' ', false
	}
	line := v.Buffer.Line(v.CursorLine)
	visCol := v.CursorCol
	under = ' '
	if v.ANSI {
		visCol = VisibleANSIColAtByte(line, v.CursorCol)
		under = ANSIRuneAtVisible(line, visCol)
	} else if v.CursorCol >= 0 && v.CursorCol < len(line) {
		r, _ := utf8.DecodeRuneInString(line[v.CursorCol:])
		if r != utf8.RuneError {
			under = r
		}
	}
	localX = visCol - v.Left
	if localX < 0 || localX >= v.width {
		return 0, 0, ' ', false
	}
	return localX, localY, under, true
}

// Draw renders the visible portion of the buffer.
func (v *Viewport) Draw(c Canvas) {
	if v.Buffer == nil {
		return
	}

	v.width = c.W()
	v.height = c.H()
	v.screenX = c.ScreenX(0)
	v.screenY = c.ScreenY(0)

	if v.followTail {
		v.ScrollToBottom()
	}
	v.padTop = v.followPadTop()

	style := tcell.StyleDefault
	selStyle := style.Reverse(true)
	width := v.width
	height := v.height
	lineLimit := v.lineLimit()

	for row := 0; row < height; row++ {
		if row < v.padTop {
			c.ClearLine(row, style)
			continue
		}
		line := v.Top + (row - v.padTop)
		if line >= lineLimit {
			c.ClearLine(row, style)
			continue
		}

		full := v.Buffer.Line(line)
		lineStyle := style
		if v.RowStyle != nil {
			lineStyle = v.RowStyle(line, full)
		} else if v.LineStyle != nil {
			lineStyle = v.LineStyle(full)
		}

		if v.ANSI {
			c.ClearLine(row, lineStyle)
			lineNum := line
			drawn := c.DrawANSIText(0, row, v.Left, full, lineStyle, func(bufByte int) bool {
				return v.containsSel(lineNum, bufByte)
			}, func(absVisCol int, st tcell.Style) tcell.Style {
				if v.CellStyle != nil {
					st = v.CellStyle(lineNum, absVisCol, st)
				}
				return v.applySearchStyle(lineNum, absVisCol, st)
			})
			if drawn < width {
				c.ClearLineRange(row, drawn, width, lineStyle)
			}
			continue
		}

		// Horizontal scroll and columns are rune-based (not byte offsets), so
		// multi-byte glyphs like ━ / │ / ▶ do not leave gaps.
		runes := []rune(full)
		start := v.Left
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
		for col, ch := range visible {
			bufCol := start + col
			st := lineStyle
			if v.CellStyle != nil {
				st = v.CellStyle(line, bufCol, st)
			}
			st = v.applySearchStyle(line, bufCol, st)
			if v.containsSel(line, bufCol) {
				st = selStyle
			}
			c.SetContent(col, row, ch, st)
		}
	}

	v.cursor.Draw(c, v)
	// v.drawCursor(c)
}

func (v *Viewport) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		v.handleMouse(e)
	case *tcell.EventKey:
		v.handleKey(e)
	case *tcell.EventPaste:
		v.PasteAtCursor()
	}
}

// ScrollLineUp moves the caret up one line. The viewport scrolls only once
// the caret is already on the first visible line (same as classic Up).
func (v *Viewport) ScrollLineUp() {
	v.leaveFollowTail()
	v.Up()
	v.extendSelectionAfterScroll(-1)
	v.EnsureVisible(v.width, v.height)
}

// ScrollLineDown moves the caret down one line; scrolls at the bottom edge.
func (v *Viewport) ScrollLineDown() {
	v.Down()
	v.maybeFollowTail()
	v.extendSelectionAfterScroll(1)
	v.EnsureVisible(v.width, v.height)
}

// ViewScrollLineUp always shifts the viewport one line (mouse wheel).
func (v *Viewport) ViewScrollLineUp() {
	v.leaveFollowTail()
	if v.Top > 0 {
		v.Top--
		if v.selActive {
			// Live drag: pull selection into newly revealed lines at the top edge.
			v.CursorLine = v.Top
			v.CursorCol = 0
		} else if v.CursorLine > 0 {
			v.CursorLine--
		}
	}
	v.clampCursorIntoView()
	v.extendSelectionAfterScroll(-1)
}

// ViewScrollLineDown always shifts the viewport one line (mouse wheel).
func (v *Viewport) ViewScrollLineDown() {
	if v.Buffer == nil {
		return
	}
	last := v.Buffer.NumLines() - 1
	if last < 0 {
		return
	}
	maxTop := 0
	if v.height > 0 && last >= v.height {
		maxTop = last - v.height + 1
	}
	if v.Top < maxTop {
		v.Top++
		if v.selActive {
			// Live drag: pull selection into newly revealed lines at the bottom edge.
			v.CursorLine = v.Top + v.height - 1
			if v.CursorLine > last {
				v.CursorLine = last
			}
			v.CursorCol = len(v.Buffer.Line(v.CursorLine))
		} else if v.CursorLine < last {
			v.CursorLine++
		}
	}
	v.clampCursorIntoView()
	v.maybeFollowTail()
	v.extendSelectionAfterScroll(1)
}

// lineContentWidth returns how many columns a buffer line occupies for
// horizontal scrolling (visible ANSI cells, or rune count otherwise).
func (v *Viewport) lineContentWidth(line string) int {
	if v.ANSI {
		return VisibleANSIWidth(line)
	}
	return utf8.RuneCountInString(line)
}

// maxContentWidth is the widest line in the buffer (columns).
func (v *Viewport) maxContentWidth() int {
	if v.Buffer == nil {
		return 0
	}
	max := 0
	n := v.Buffer.NumLines()
	for i := 0; i < n; i++ {
		if w := v.lineContentWidth(v.Buffer.Line(i)); w > max {
			max = w
		}
	}
	return max
}

// maxLeft is the largest Left such that some content remains visible.
func (v *Viewport) maxLeft() int {
	if v.width <= 0 {
		return 0
	}
	max := v.maxContentWidth() - v.width
	if max < 0 {
		return 0
	}
	return max
}

// ViewScrollColLeft shifts the viewport one column left (view-only).
func (v *Viewport) ViewScrollColLeft() {
	v.leaveFollowTail()
	if v.Left > 0 {
		v.Left--
	}
}

// ViewScrollColRight shifts the viewport one column right when content is
// wider than the pane (view-only).
func (v *Viewport) ViewScrollColRight() {
	v.leaveFollowTail()
	if v.Left < v.maxLeft() {
		v.Left++
	}
}

func (v *Viewport) ScrollPageUp(pageSize int) {
	v.leaveFollowTail()
	v.PageUp(pageSize)
	v.extendSelectionAfterScroll(-1)
	v.EnsureVisible(v.width, v.height)
}

func (v *Viewport) ScrollPageDown(pageSize int) {
	v.PageDown(pageSize)
	v.maybeFollowTail()
	v.extendSelectionAfterScroll(1)
	v.EnsureVisible(v.width, v.height)
}

func (v *Viewport) ScrollHome() {
	v.leaveFollowTail()
	v.Home()
	v.extendSelectionAfterScroll(-1)
	v.EnsureCursorVisible()
}

func (v *Viewport) ScrollEnd() {
	v.End()
	v.maybeFollowTail()
	v.extendSelectionAfterScroll(1)
	v.EnsureCursorVisible()
}

// extendSelectionAfterScroll keeps an existing mark while scrolling, and while
// dragging extends selCursor to the caret on the newly revealed edge.
func (v *Viewport) extendSelectionAfterScroll(dir int) {
	if !v.selActive && !v.hasSel {
		return
	}
	if v.selActive {
		v.selCursor = bufferPos{line: v.CursorLine, col: v.CursorCol}
		v.hasSel = v.selAnchor != v.selCursor
	}
	_ = dir
}

func (v *Viewport) clampCursorIntoView() {
	if v.height <= 0 {
		return
	}
	if v.CursorLine < v.Top {
		v.CursorLine = v.Top
	}
	bottom := v.Top + v.height - 1
	if v.CursorLine > bottom {
		v.CursorLine = bottom
	}
	v.clampCursorCol()
}

func (v *Viewport) handleMouse(e *tcell.EventMouse) {
	if v.Buffer == nil || v.width <= 0 || v.height <= 0 {
		return
	}

	mx, my := e.Position()
	lx := mx - v.screenX
	ly := my - v.screenY

	if e.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		// Allow wheel during an active drag even if the pointer is outside;
		// otherwise require the pointer over the pane.
		if !v.selActive && (lx < 0 || ly < 0 || lx >= v.width || ly >= v.height) {
			return
		}
		if e.Buttons()&tcell.WheelUp != 0 {
			v.ViewScrollLineUp()
		}
		if e.Buttons()&tcell.WheelDown != 0 {
			v.ViewScrollLineDown()
		}
		return
	}

	// Middle-click paste (Linux PRIMARY selection); only inside the pane.
	if isMiddlePaste(e) {
		if lx < 0 || ly < 0 || lx >= v.width || ly >= v.height {
			return
		}
		v.PastePrimaryAtCursor()
		return
	}

	if e.Buttons() == tcell.ButtonNone {
		v.suppressDrag = false
		if v.selActive {
			v.selActive = false
			if lx >= 0 && ly >= 0 && lx < v.width && ly < v.height {
				v.selCursor = v.posFromLocal(lx, ly)
			}
			v.hasSel = v.selAnchor != v.selCursor
			if v.hasSel {
				v.CopySelection()
			}
		}
		return
	}

	if e.Buttons()&tcell.ButtonPrimary == 0 {
		return
	}

	// After double/triple-click, hold the selection until button release.
	if v.suppressDrag {
		return
	}

	// New click must start inside; drag may leave the pane and auto-scroll.
	if !v.selActive {
		if lx < 0 || ly < 0 || lx >= v.width || ly >= v.height {
			return
		}
		pos := v.posFromLocal(lx, ly)
		v.noteClick(pos, e.When())
		switch v.clickCount {
		case 2:
			if v.selectWordAt(pos) {
				v.CopySelection()
				v.suppressDrag = true
				return
			}
		case 3:
			if v.selectLineAt(pos) {
				v.CopySelection()
				v.suppressDrag = true
				return
			}
		}
		v.clearSelection()
		v.selActive = true
		v.selAnchor = pos
		v.selCursor = pos
		v.hasSel = false
		v.CursorLine = pos.line
		v.CursorCol = pos.col
		v.clampCursorCol()
		return
	}

	// Drag beyond the pane edges: scroll to reveal the missing area and pin
	// the selection endpoint to that edge (lines not visible on first click).
	for ly < 0 {
		before := v.Top
		v.ViewScrollLineUp()
		ly = 0
		if v.Top == before {
			break
		}
	}
	for ly >= v.height {
		before := v.Top
		v.ViewScrollLineDown()
		ly = v.height - 1
		if v.Top == before {
			break
		}
	}
	if lx < 0 {
		lx = 0
	}
	if lx >= v.width {
		lx = v.width - 1
	}
	if ly < 0 {
		ly = 0
	}
	if ly >= v.height {
		ly = v.height - 1
	}

	pos := v.posFromLocal(lx, ly)
	v.selCursor = pos
	v.hasSel = v.selAnchor != v.selCursor
	v.CursorLine = pos.line
	v.CursorCol = pos.col
	v.clampCursorCol()
	v.EnsureVisible(v.width, v.height)
}

func (v *Viewport) noteClick(pos bufferPos, when time.Time) {
	if when.IsZero() {
		when = time.Now()
	}
	same := pos.line == v.lastClickPos.line && absInt(pos.col-v.lastClickPos.col) <= 1
	if same && when.Sub(v.lastClickTime) <= clickMultiTimeoutMs*time.Millisecond {
		v.clickCount++
		if v.clickCount > 3 {
			v.clickCount = 1
		}
	} else {
		v.clickCount = 1
	}
	v.lastClickTime = when
	v.lastClickPos = pos
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// selectWordAt marks the word under pos and returns true if anything selected.
func (v *Viewport) selectWordAt(pos bufferPos) bool {
	if v.Buffer == nil || pos.line < 0 || pos.line >= v.Buffer.NumLines() {
		return false
	}
	line := v.Buffer.Line(pos.line)
	start, end := wordBoundsAt(line, pos.col)
	if start >= end {
		return false
	}
	v.selActive = false
	v.selAnchor = bufferPos{line: pos.line, col: start}
	v.selCursor = bufferPos{line: pos.line, col: end}
	v.hasSel = true
	v.CursorLine = pos.line
	v.CursorCol = end
	v.clampCursorCol()
	return true
}

// selectLineAt marks the whole buffer line under pos.
func (v *Viewport) selectLineAt(pos bufferPos) bool {
	if v.Buffer == nil || pos.line < 0 || pos.line >= v.Buffer.NumLines() {
		return false
	}
	line := v.Buffer.Line(pos.line)
	v.selActive = false
	v.selAnchor = bufferPos{line: pos.line, col: 0}
	v.selCursor = bufferPos{line: pos.line, col: len(line)}
	v.hasSel = v.selAnchor != v.selCursor
	v.CursorLine = pos.line
	v.CursorCol = len(line)
	v.clampCursorCol()
	return v.hasSel
}

func (v *Viewport) handleKey(key *tcell.EventKey) {
	ctrl := key.Modifiers()&tcell.ModCtrl != 0
	switch {
	case key.Key() == tcell.KeyCtrlC || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'c'):
		if v.hasSel {
			v.CopySelection()
		}
		return
	case key.Key() == tcell.KeyCtrlX || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'x'):
		if v.hasSel {
			v.CutSelection()
		}
		return
	case key.Key() == tcell.KeyCtrlV || (ctrl && key.Key() == tcell.KeyRune && key.Rune() == 'v'):
		v.PasteAtCursor()
		return
	}

	switch key.Key() {

	case tcell.KeyUp:
		v.leaveFollowTail()
		v.Up()
		v.extendSelectionAfterScroll(-1)

	case tcell.KeyDown:
		v.Down()
		v.maybeFollowTail()
		v.extendSelectionAfterScroll(1)

	case tcell.KeyLeft:
		v.leaveFollowTail()
		v.LeftChar()
		if !v.selActive {
			v.clearSelection()
		}

	case tcell.KeyRight:
		v.RightChar()
		v.maybeFollowTail()
		if !v.selActive {
			v.clearSelection()
		}

	case tcell.KeyPgUp:
		v.leaveFollowTail()
		v.PageUp(10)
		v.extendSelectionAfterScroll(-1)

	case tcell.KeyPgDn:
		v.PageDown(10)
		v.maybeFollowTail()
		v.extendSelectionAfterScroll(1)

	case tcell.KeyHome:
		v.leaveFollowTail()
		v.Home()
		v.extendSelectionAfterScroll(-1)

	case tcell.KeyEnd:
		v.End()
		v.maybeFollowTail()
		v.extendSelectionAfterScroll(1)

	default:
		return
	}

	v.EnsureCursorVisible()
}

func (v *Viewport) followPadTop() int {
	if !v.followTail || v.Buffer == nil || v.height <= 0 {
		return 0
	}
	n := v.lineLimit()
	if n <= 0 || n >= v.height {
		return 0
	}
	return v.height - n
}

// lineLimit is the exclusive end line index for drawing/scrolling (respects OmitTail).
func (v *Viewport) lineLimit() int {
	if v.Buffer == nil {
		return 0
	}
	n := v.Buffer.NumLines() - v.OmitTail
	if n < 0 {
		return 0
	}
	return n
}

func (v *Viewport) posFromLocal(lx, ly int) bufferPos {
	line := v.Top + (ly - v.padTop)
	if line < 0 {
		line = 0
	}
	if v.Buffer != nil {
		last := v.Buffer.NumLines() - 1
		if line > last {
			line = last
		}
	}

	col := v.Left + lx
	lineLen := 0
	if v.Buffer != nil && line >= 0 && line < v.Buffer.NumLines() {
		raw := v.Buffer.Line(line)
		if v.ANSI {
			col = ANSIByteIndexAtVisible(raw, v.Left+lx)
			lineLen = len(raw)
		} else {
			lineLen = len(raw)
		}
	}
	if col > lineLen {
		col = lineLen
	}

	return bufferPos{line: line, col: col}
}

func (v *Viewport) normalizedSel() (start, end bufferPos) {
	start = v.selAnchor
	end = v.selCursor
	if start.line > end.line || (start.line == end.line && start.col > end.col) {
		start, end = end, start
	}
	return start, end
}

func (v *Viewport) containsSel(line, col int) bool {
	if !v.hasSel {
		return false
	}
	start, end := v.normalizedSel()
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

func (v *Viewport) clearSelection() {
	v.selActive = false
	v.hasSel = false
}

// ClearSelection drops any active text mark (e.g. after */# takes the caret word).
func (v *Viewport) ClearSelection() {
	if v != nil {
		v.clearSelection()
	}
}

func (v *Viewport) SetFollowTail(follow bool) {
	v.followTail = follow
}

func (v *Viewport) FollowTail() bool {
	return v.followTail
}

func (v *Viewport) leaveFollowTail() {
	v.followTail = false
}

func (v *Viewport) maybeFollowTail() {
	if v.Buffer == nil {
		return
	}

	last := v.Buffer.NumLines() - 1
	if last < 0 {
		v.followTail = true
		return
	}

	if v.CursorLine < last {
		return
	}

	if v.height <= 0 {
		v.followTail = true
		return
	}

	maxTop := last - v.height + 1
	if maxTop < 0 {
		maxTop = 0
	}
	if v.Top >= maxTop {
		v.followTail = true
	}
}

func (v *Viewport) ScrollToBottom() {
	if v.Buffer == nil {
		return
	}

	last := v.lineLimit() - 1
	if last < 0 {
		return
	}

	v.CursorLine = last
	v.clampCursorCol()

	if v.height > 0 && last >= v.height {
		v.Top = last - v.height + 1
		return
	}

	v.Top = 0
}

func (v *Viewport) Up() {

	if v.CursorLine > 0 {
		v.CursorLine--
		v.clampCursorCol()
	}

	if v.CursorLine < v.Top {
		v.Top = v.CursorLine
	}
}

func (v *Viewport) Down() {

	if v.Buffer == nil {
		return
	}

	last := v.Buffer.NumLines() - 1
	if v.CursorLine < last {
		v.CursorLine++
		v.clampCursorCol()
	}

	if v.height > 0 && v.CursorLine >= v.Top+v.height {
		v.Top = v.CursorLine - v.height + 1
	}
}

func (v *Viewport) LeftChar() {
	if v.ANSI && v.Buffer != nil {
		line := v.Buffer.Line(v.CursorLine)
		vis := VisibleANSIColAtByte(line, v.CursorCol)
		if vis > 0 {
			vis--
			v.CursorCol = ANSIByteIndexAtVisible(line, vis)
		}
		if vis < v.Left {
			v.Left = vis
		}
		return
	}

	if v.CursorCol > 0 {
		v.CursorCol--
	}

	if v.CursorCol < v.Left {
		v.Left = v.CursorCol
	}
}

func (v *Viewport) RightChar() {
	if v.ANSI && v.Buffer != nil {
		line := v.Buffer.Line(v.CursorLine)
		vis := VisibleANSIColAtByte(line, v.CursorCol)
		maxVis := VisibleANSIWidth(line)
		if vis < maxVis {
			vis++
			v.CursorCol = ANSIByteIndexAtVisible(line, vis)
		}
		return
	}

	if v.Buffer != nil {
		lineLen := len(v.Buffer.Line(v.CursorLine))
		if v.CursorCol < lineLen {
			v.CursorCol++
		}
		return
	}

	v.CursorCol++
}

func (v *Viewport) PageDown(pageSize int) {

	if v.Buffer == nil {
		return
	}

	v.CursorLine += pageSize

	last := v.Buffer.NumLines() - 1
	if v.CursorLine > last {
		v.CursorLine = last
	}

	v.Top += pageSize
	if v.Top > last {
		v.Top = last
	}
}

func (v *Viewport) PageUp(pageSize int) {

	v.CursorLine -= pageSize
	if v.CursorLine < 0 {
		v.CursorLine = 0
	}

	v.Top -= pageSize
	if v.Top < 0 {
		v.Top = 0
	}
}

// Home moves the caret and view to the top-left of the buffer.
func (v *Viewport) Home() {
	v.CursorLine = 0
	v.CursorCol = 0
	v.Top = 0
	v.Left = 0
}

func (v *Viewport) End() {
	if v.Buffer == nil {
		return
	}

	last := v.lineLimit() - 1
	if last < 0 {
		last = 0
	}

	v.CursorLine = last
	h := v.height
	if h <= 0 {
		h = 20
	}
	maxTop := 0
	if last >= h {
		maxTop = last - h + 1
	}
	v.Top = maxTop
	v.Left = 0
}

func (v *Viewport) Center(line int, pageHeight int) {

	if v.Buffer == nil {
		return
	}

	v.CursorLine = line
	if pageHeight <= 0 {
		pageHeight = 20
	}

	v.Top = line - pageHeight/2
	if v.Top < 0 {
		v.Top = 0
	}

	last := v.Buffer.NumLines() - 1
	if last < 0 {
		return
	}
	maxTop := 0
	if last >= pageHeight {
		maxTop = last - pageHeight + 1
	}
	if v.Top > maxTop {
		v.Top = maxTop
	}
}

// Height returns the last canvas height seen in Draw (0 before first paint).
func (v *Viewport) Height() int { return v.height }

// Width returns the last canvas width seen in Draw (0 before first paint).
func (v *Viewport) Width() int { return v.width }

// EnsureCursorVisible scrolls so CursorLine is on-screen using the last Draw size.
func (v *Viewport) EnsureCursorVisible() {
	w, h := v.width, v.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 20
	}
	v.EnsureVisible(w, h)
}

// EnsureLineVisible scrolls vertically so CursorLine is on-screen without
// changing the horizontal offset (for list panes with view-only Left/Right).
func (v *Viewport) EnsureLineVisible() {
	h := v.height
	if h <= 0 {
		h = 20
	}
	if v.CursorLine < v.Top {
		v.Top = v.CursorLine
	}
	if v.CursorLine >= v.Top+h {
		v.Top = v.CursorLine - h + 1
	}
	if v.Buffer == nil {
		return
	}
	last := v.Buffer.NumLines() - 1
	if last < 0 {
		return
	}
	maxTop := last
	if last >= h-1 {
		maxTop = last - h + 1
	}
	if v.Top > maxTop {
		v.Top = maxTop
	}
	if v.Top < 0 {
		v.Top = 0
	}
}

func (v *Viewport) clampCursorCol() {
	if v.Buffer == nil {
		return
	}

	lineLen := len(v.Buffer.Line(v.CursorLine))
	if v.CursorCol > lineLen {
		v.CursorCol = lineLen
	}
}

func (v *Viewport) EnsureVisible(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	if v.CursorLine < v.Top {
		v.Top = v.CursorLine
	}

	if v.CursorLine >= v.Top+height {
		v.Top = v.CursorLine - height + 1
	}

	if v.Buffer == nil {
		return
	}

	last := v.Buffer.NumLines() - 1
	if last < 0 {
		return
	}

	maxTop := last
	if last >= height-1 {
		maxTop = last - height + 1
	}
	if v.Top > maxTop {
		v.Top = maxTop
	}
	if v.Top < 0 {
		v.Top = 0
	}

	// Horizontal scroll. In ANSI mode CursorCol is a byte offset into the
	// escape-laden string, while Left is a visible-cell skip count — mix them
	// carefully so the caret stays in view.
	if v.ANSI {
		line := v.Buffer.Line(v.CursorLine)
		vis := VisibleANSIWidth(line)
		curVis := VisibleANSIColAtByte(line, v.CursorCol)
		if curVis < v.Left {
			v.Left = curVis
		}
		if curVis >= v.Left+width {
			v.Left = curVis - width + 1
		}
		if v.Left < 0 {
			v.Left = 0
		}
		if vis <= width {
			v.Left = 0
		} else if v.Left > vis-width {
			v.Left = vis - width
		}
		return
	}

	if v.CursorCol < v.Left {
		v.Left = v.CursorCol
	}
	if v.CursorCol >= v.Left+width {
		v.Left = v.CursorCol - width + 1
	}
	if v.Left < 0 {
		v.Left = 0
	}
}
