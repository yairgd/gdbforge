package termui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
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
	width  int
	height int
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

	clipboard ClipboardIO
	readOnly  bool // true → Cut copies only; Paste ignored

	// LineStyle optionally colors a full buffer line before selection reverse.
	LineStyle func(line string) tcell.Style

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

	for row := 0; row < height; row++ {
		if row < v.padTop {
			c.ClearLine(row, style)
			continue
		}
		line := v.Top + (row - v.padTop)
		if line >= v.Buffer.NumLines() {
			c.ClearLine(row, style)
			continue
		}

		full := v.Buffer.Line(line)
		lineStyle := style
		if v.LineStyle != nil {
			lineStyle = v.LineStyle(full)
		}
		text := full
		if v.Left < len(text) {
			text = text[v.Left:]
		} else {
			text = ""
		}

		if len(text) > width {
			text = text[:width]
		}

		visibleLen := len(text)
		if visibleLen < width {
			c.ClearLineRange(row, visibleLen, width, lineStyle)
		}

		for col, ch := range text {
			bufCol := v.Left + col
			st := lineStyle
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
	v.EnsureVisible(v.width, v.height)
}

func (v *Viewport) ScrollEnd() {
	v.End()
	v.maybeFollowTail()
	v.extendSelectionAfterScroll(1)
	v.EnsureVisible(v.width, v.height)
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

	if e.Buttons() == tcell.ButtonNone && v.selActive {
		v.selActive = false
		if lx >= 0 && ly >= 0 && lx < v.width && ly < v.height {
			v.selCursor = v.posFromLocal(lx, ly)
		}
		v.hasSel = v.selAnchor != v.selCursor
		if v.hasSel {
			v.CopySelection()
		}
		return
	}

	if e.Buttons()&tcell.ButtonPrimary == 0 {
		return
	}

	// New click must start inside; drag may leave the pane and auto-scroll.
	if !v.selActive {
		if lx < 0 || ly < 0 || lx >= v.width || ly >= v.height {
			return
		}
		pos := v.posFromLocal(lx, ly)
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

	v.EnsureVisible(v.width, v.height)
}

func (v *Viewport) followPadTop() int {
	if !v.followTail || v.Buffer == nil || v.height <= 0 {
		return 0
	}
	n := v.Buffer.NumLines()
	if n <= 0 || n >= v.height {
		return 0
	}
	return v.height - n
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
		lineLen = len(v.Buffer.Line(line))
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

	last := v.Buffer.NumLines() - 1
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

	if v.CursorCol > 0 {
		v.CursorCol--
	}

	if v.CursorCol < v.Left {
		v.Left = v.CursorCol
	}
}

func (v *Viewport) RightChar() {
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

	last := v.Buffer.NumLines() - 1
	if last < 0 {
		last = 0
	}

	v.CursorLine = last
	v.Top = last
}

func (v *Viewport) Center(line int, pageHeight int) {

	if v.Buffer == nil {
		return
	}

	v.CursorLine = line

	v.Top = line - pageHeight/2
	if v.Top < 0 {
		v.Top = 0
	}

	last := v.Buffer.NumLines() - 1
	if v.Top > last {
		v.Top = last
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

	if v.CursorCol < v.Left {
		v.Left = v.CursorCol
	}

	if v.CursorCol >= v.Left+width {
		v.Left = v.CursorCol - width + 1
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
}
