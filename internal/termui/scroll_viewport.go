package termui

import "unicode/utf8"

// ScrollViewport is scroll/cursor camera state for document panes.
// Widgets own model data and paint visible rows; the viewport only tracks
// where to look (Top/Left) and the logical caret (CursorLine/CursorCol).
type ScrollViewport struct {
	Top, Left int
	CursorLine, CursorCol int

	width, height int
	screenX, screenY int

	// dragAutoScroll enables edge auto-scroll while dragging a selection
	// outside the pane (default on). Assembly disables this.
	dragAutoScroll bool
}

func NewScrollViewport() *ScrollViewport {
	return &ScrollViewport{dragAutoScroll: true}
}

func (sv *ScrollViewport) SetDragAutoScroll(on bool) {
	if sv != nil {
		sv.dragAutoScroll = on
	}
}

func (sv *ScrollViewport) DragAutoScroll() bool {
	if sv == nil {
		return true
	}
	return sv.dragAutoScroll
}

func (sv *ScrollViewport) SetWindow(w, h int) {
	if sv == nil {
		return
	}
	sv.width = w
	sv.height = h
}

func (sv *ScrollViewport) SetMouseOrigin(screenX, screenY int) {
	if sv == nil {
		return
	}
	sv.screenX, sv.screenY = screenX, screenY
}

func (sv *ScrollViewport) MouseOrigin() (screenX, screenY int) {
	if sv == nil {
		return 0, 0
	}
	return sv.screenX, sv.screenY
}

func (sv *ScrollViewport) Height() int {
	if sv == nil {
		return 0
	}
	return sv.height
}

func (sv *ScrollViewport) Width() int {
	if sv == nil {
		return 0
	}
	return sv.width
}

func (sv *ScrollViewport) EnsureLineVisible(lineCount int) {
	if sv == nil {
		return
	}
	h := sv.height
	if h <= 0 {
		h = 20
	}
	if sv.CursorLine < sv.Top {
		sv.Top = sv.CursorLine
	}
	if sv.CursorLine >= sv.Top+h {
		sv.Top = sv.CursorLine - h + 1
	}
	if lineCount <= 0 {
		return
	}
	last := lineCount - 1
	maxTop := last
	if last >= h-1 {
		maxTop = last - h + 1
	}
	if sv.Top > maxTop {
		sv.Top = maxTop
	}
	if sv.Top < 0 {
		sv.Top = 0
	}
}

func (sv *ScrollViewport) EnsureCursorVisible(lineCount int, lineWidth func(lineIdx int) int) {
	if sv == nil {
		return
	}
	w, h := sv.width, sv.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 20
	}
	sv.EnsureLineVisible(lineCount)
	if lineWidth == nil || lineCount <= 0 || sv.CursorLine < 0 || sv.CursorLine >= lineCount {
		return
	}
	lineLen := lineWidth(sv.CursorLine)
	curVis := sv.CursorCol
	if curVis < 0 {
		curVis = 0
	}
	if curVis > lineLen {
		curVis = lineLen
	}
	if curVis < sv.Left {
		sv.Left = curVis
	}
	if curVis >= sv.Left+w {
		sv.Left = curVis - w + 1
	}
	if sv.Left < 0 {
		sv.Left = 0
	}
	if lineLen <= w {
		sv.Left = 0
	} else if sv.Left > lineLen-w {
		sv.Left = lineLen - w
	}
}

func (sv *ScrollViewport) Center(line, pageHeight, lineCount int) {
	if sv == nil {
		return
	}
	sv.CursorLine = line
	if pageHeight <= 0 {
		pageHeight = 20
	}
	sv.Top = line - pageHeight/2
	if sv.Top < 0 {
		sv.Top = 0
	}
	if lineCount <= 0 {
		return
	}
	last := lineCount - 1
	maxTop := 0
	if last >= pageHeight {
		maxTop = last - pageHeight + 1
	}
	if sv.Top > maxTop {
		sv.Top = maxTop
	}
}

func (sv *ScrollViewport) ViewScrollColLeft() {
	if sv != nil && sv.Left > 0 {
		sv.Left--
	}
}

func (sv *ScrollViewport) ViewScrollColRight(maxLeft int) {
	if sv == nil {
		return
	}
	if maxLeft < 0 {
		maxLeft = 0
	}
	if sv.Left < maxLeft {
		sv.Left++
	}
}

func (sv *ScrollViewport) MaxLeft(maxContentWidth int) int {
	if sv == nil || sv.width <= 0 {
		return 0
	}
	max := maxContentWidth - sv.width
	if max < 0 {
		return 0
	}
	return max
}

func (sv *ScrollViewport) HitContentLine(screenX, screenY, lineCount int) (line int, ok bool) {
	if sv == nil || sv.width <= 0 || sv.height <= 0 || lineCount <= 0 {
		return 0, false
	}
	lx := screenX - sv.screenX
	ly := screenY - sv.screenY
	if lx < 0 || ly < 0 || lx >= sv.width || ly >= sv.height {
		return 0, false
	}
	line = sv.Top + ly
	last := lineCount - 1
	if line < 0 || line > last {
		return 0, false
	}
	return line, true
}

func (sv *ScrollViewport) PosFromLocal(lx, ly int, lineCount int, lineAt func(lineIdx int) string) bufferPos {
	line := sv.Top + ly
	if line < 0 {
		line = 0
	}
	if lineCount > 0 {
		last := lineCount - 1
		if line > last {
			line = last
		}
	}
	col := sv.Left + lx
	lineLen := 0
	if lineAt != nil && line >= 0 && line < lineCount {
		raw := lineAt(line)
		lineLen = len(raw)
		col = ByteIndexAtVisibleCol(raw, sv.Left+lx)
	}
	if col > lineLen {
		col = lineLen
	}
	return bufferPos{line: line, col: col}
}

func (sv *ScrollViewport) CursorDrawPos(lineAt func(lineIdx int) string, lineCount int) (localX, localY int, under rune, ok bool) {
	if sv == nil || lineCount <= 0 || sv.CursorLine < 0 || sv.CursorLine >= lineCount {
		return 0, 0, ' ', false
	}
	localY = sv.CursorLine - sv.Top
	if localY < 0 || localY >= sv.height {
		return 0, 0, ' ', false
	}
	line := ""
	if lineAt != nil {
		line = lineAt(sv.CursorLine)
	}
	under = ' '
	visCol := VisibleColAtByte(line, sv.CursorCol)
	if sv.CursorCol >= 0 && sv.CursorCol < len(line) {
		r, _ := utf8.DecodeRuneInString(line[sv.CursorCol:])
		if r != utf8.RuneError {
			under = r
		}
	}
	localX = visCol - sv.Left
	if localX < 0 || localX >= sv.width {
		return 0, 0, ' ', false
	}
	return localX, localY, under, true
}

func (sv *ScrollViewport) ClampCursorCol(lineLen int) {
	if sv == nil {
		return
	}
	if sv.CursorCol > lineLen {
		sv.CursorCol = lineLen
	}
	if sv.CursorCol < 0 {
		sv.CursorCol = 0
	}
}
