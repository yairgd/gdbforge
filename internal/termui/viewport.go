package termui

import (
	"strings"

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

	// Screen origin of the widget rect (set during Draw).
	screenX int
	screenY int

	// Text selection (buffer coordinates).
	selAnchor bufferPos
	selCursor bufferPos
	selActive bool
	hasSel    bool

	copyToClipboard func(string)

	cursor CursorPainter
}

func NewViewport(buf *platform.Buffer) *Viewport {
	return &Viewport{
		Buffer: buf,
		cursor: &NativeCursor{},
	}
}

func (v *Viewport) SetCopyToClipboard(fn func(string)) {
	v.copyToClipboard = fn
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

	style := tcell.StyleDefault
	selStyle := style.Reverse(true)
	width := v.width
	height := v.height

	for row := 0; row < height; row++ {
		line := v.Top + row
		if line >= v.Buffer.NumLines() {
			c.ClearLine(row, style)
			continue
		}

		text := v.Buffer.Line(line)
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
			c.ClearLineRange(row, visibleLen, width, style)
		}

		for col, ch := range text {
			bufCol := v.Left + col
			st := style
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
	}
}

func (v *Viewport) handleMouse(e *tcell.EventMouse) {
	if v.Buffer == nil || v.width <= 0 || v.height <= 0 {
		return
	}

	mx, my := e.Position()
	lx := mx - v.screenX
	ly := my - v.screenY

	if e.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		if lx < 0 || ly < 0 || lx >= v.width || ly >= v.height {
			return
		}
		v.clearSelection()
		if e.Buttons()&tcell.WheelUp != 0 {
			v.leaveFollowTail()
			v.Up()
		}
		if e.Buttons()&tcell.WheelDown != 0 {
			v.Down()
			v.maybeFollowTail()
		}
		v.EnsureVisible(v.width, v.height)
		return
	}

	if e.Buttons() == tcell.ButtonNone && v.selActive {
		v.selActive = false
		if lx >= 0 && ly >= 0 && lx < v.width && ly < v.height {
			v.selCursor = v.posFromLocal(lx, ly)
		}
		v.hasSel = v.selAnchor != v.selCursor
		if v.hasSel {
			v.copySelection()
		}
		return
	}

	if lx < 0 || ly < 0 || lx >= v.width || ly >= v.height {
		return
	}

	pos := v.posFromLocal(lx, ly)

	if e.Buttons()&tcell.ButtonPrimary != 0 {
		if !v.selActive {
			v.clearSelection()
			v.selActive = true
			v.selAnchor = pos
			v.hasSel = false
		}
		v.selCursor = pos
		v.hasSel = v.selAnchor != v.selCursor
		v.CursorLine = pos.line
		v.CursorCol = pos.col
		v.clampCursorCol()
		v.EnsureVisible(v.width, v.height)
	}
}

func (v *Viewport) handleKey(key *tcell.EventKey) {
	if key.Key() == tcell.KeyCtrlC ||
		(key.Key() == tcell.KeyRune && key.Rune() == 'c' && key.Modifiers()&tcell.ModCtrl != 0) {
		if v.hasSel {
			v.copySelection()
			return
		}
	}

	switch key.Key() {

	case tcell.KeyUp:
		v.leaveFollowTail()
		v.Up()

	case tcell.KeyDown:
		v.Down()
		v.maybeFollowTail()

	case tcell.KeyLeft:
		v.leaveFollowTail()
		v.LeftChar()

	case tcell.KeyRight:
		v.RightChar()
		v.maybeFollowTail()

	case tcell.KeyPgUp:
		v.leaveFollowTail()
		v.PageUp(10)

	case tcell.KeyPgDn:
		v.PageDown(10)
		v.maybeFollowTail()

	case tcell.KeyHome:
		v.leaveFollowTail()
		v.Home()

	case tcell.KeyEnd:
		v.End()
		v.maybeFollowTail()

	default:
		return
	}

	v.clearSelection()
	v.EnsureVisible(v.width, v.height)
}

func (v *Viewport) posFromLocal(lx, ly int) bufferPos {
	line := v.Top + ly
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

func (v *Viewport) selectedText() string {
	if !v.hasSel || v.Buffer == nil {
		return ""
	}

	start, end := v.normalizedSel()
	if start == end {
		return ""
	}

	if start.line == end.line {
		return v.Buffer.Line(start.line)[start.col:end.col]
	}

	var b strings.Builder
	b.WriteString(v.Buffer.Line(start.line)[start.col:])
	for line := start.line + 1; line < end.line; line++ {
		b.WriteByte('\n')
		b.WriteString(v.Buffer.Line(line))
	}
	b.WriteByte('\n')
	b.WriteString(v.Buffer.Line(end.line)[:end.col])
	return b.String()
}

func (v *Viewport) copySelection() {
	if v.copyToClipboard == nil {
		return
	}
	text := v.selectedText()
	if text == "" {
		return
	}
	v.copyToClipboard(text)
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

func (v *Viewport) Home() {

	v.CursorCol = 0
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
