package termui

import (
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	xterm "github.com/gitpod-io/xterm-go"
)

type termPos struct {
	line int // absolute scrollback line index
	col  int // column within the line
}

// SetClipboard wires shared copy/paste callbacks for terminal mouse/keyboard.
func (c *CompositeTerminal) SetClipboard(io ClipboardIO) {
	if c == nil {
		return
	}
	c.clipboard = io
}

func (c *CompositeTerminal) HasSelection() bool {
	return c != nil && c.hasSel
}

func (c *CompositeTerminal) HandleMouse(e *tcell.EventMouse) {
	if c == nil || c.ctl == nil || e == nil {
		return
	}
	cols, rows := c.ctl.Size()
	if cols <= 0 || rows <= 0 {
		return
	}

	mx, my := e.Position()
	lx := mx - c.screenX
	ly := my - c.screenY

	if e.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		if !c.selActive && (lx < 0 || ly < 0 || lx >= cols || ly >= rows) {
			return
		}
		disp := 0
		if e.Buttons()&tcell.WheelUp != 0 {
			disp = -3
		}
		if e.Buttons()&tcell.WheelDown != 0 {
			disp = 3
		}
		c.ctl.WithTerminal(func(term *xterm.Terminal) {
			term.ScrollLines(disp)
		})
		return
	}

	if isMiddlePaste(e) {
		if lx < 0 || ly < 0 || lx >= cols || ly >= rows {
			return
		}
		text := c.clipboard.pastePrimaryText()
		if text != "" {
			_ = c.ctl.SendInput([]byte(text))
		}
		return
	}

	if e.Buttons() == tcell.ButtonNone {
		c.suppressDrag = false
		if c.selActive {
			c.selActive = false
			if lx >= 0 && ly >= 0 && lx < cols && ly < rows {
				c.selCursor = c.posFromLocal(lx, ly, cols, rows)
			}
			c.hasSel = c.selAnchor != c.selCursor
			if c.hasSel {
				c.copySelection()
			}
		}
		return
	}

	if e.Buttons()&tcell.ButtonPrimary == 0 {
		return
	}

	if c.suppressDrag {
		return
	}

	if !c.selActive {
		if lx < 0 || ly < 0 || lx >= cols || ly >= rows {
			return
		}
		pos := c.posFromLocal(lx, ly, cols, rows)
		c.noteClick(pos, e.When())
		switch c.clickCount {
		case 2:
			if c.selectWordAt(pos) {
				c.copySelection()
				c.suppressDrag = true
				return
			}
		case 3:
			if c.selectLineAt(pos) {
				c.copySelection()
				c.suppressDrag = true
				return
			}
		}
		c.clearSelection()
		c.selActive = true
		c.selAnchor = pos
		c.selCursor = pos
		c.hasSel = false
		return
	}

	for ly < 0 {
		before := c.viewDisp()
		c.ctl.WithTerminal(func(term *xterm.Terminal) {
			term.ScrollLines(-1)
		})
		ly = 0
		if c.viewDisp() == before {
			break
		}
	}
	for ly >= rows {
		before := c.viewDisp()
		c.ctl.WithTerminal(func(term *xterm.Terminal) {
			term.ScrollLines(1)
		})
		ly = rows - 1
		if c.viewDisp() == before {
			break
		}
	}
	if lx < 0 {
		lx = 0
	}
	if lx >= cols {
		lx = cols - 1
	}
	if ly < 0 {
		ly = 0
	}
	if ly >= rows {
		ly = rows - 1
	}

	pos := c.posFromLocal(lx, ly, cols, rows)
	c.selCursor = pos
	c.hasSel = c.selAnchor != c.selCursor
}

func (c *CompositeTerminal) viewDisp() int {
	var yDisp int
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		yDisp = term.Buffer().YDisp
	})
	return yDisp
}

func (c *CompositeTerminal) posFromLocal(lx, ly, cols, rows int) termPos {
	var yDisp, lastLine int
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		buf := term.Buffer()
		yDisp = buf.YDisp
		lastLine = buf.Lines.Length() - 1
	})
	line := yDisp + ly
	if line < 0 {
		line = 0
	}
	if line > lastLine {
		line = lastLine
	}
	col := lx
	if col < 0 {
		col = 0
	}
	if col > cols {
		col = cols
	}
	return termPos{line: line, col: col}
}

func (c *CompositeTerminal) noteClick(pos termPos, when time.Time) {
	if when.IsZero() {
		when = time.Now()
	}
	same := pos.line == c.lastClickPos.line && absInt(pos.col-c.lastClickPos.col) <= 1
	if same && when.Sub(c.lastClickTime) <= clickMultiTimeoutMs*time.Millisecond {
		c.clickCount++
		if c.clickCount > 3 {
			c.clickCount = 1
		}
	} else {
		c.clickCount = 1
	}
	c.lastClickTime = when
	c.lastClickPos = pos
}

func (c *CompositeTerminal) selectWordAt(pos termPos) bool {
	line := c.terminalLineText(pos.line)
	start, end := wordBoundsAt(line, pos.col)
	if start >= end {
		return false
	}
	c.selActive = false
	c.selAnchor = termPos{line: pos.line, col: start}
	c.selCursor = termPos{line: pos.line, col: end}
	c.hasSel = true
	return true
}

func (c *CompositeTerminal) selectLineAt(pos termPos) bool {
	cols, _ := c.ctl.Size()
	c.selActive = false
	c.selAnchor = termPos{line: pos.line, col: 0}
	c.selCursor = termPos{line: pos.line, col: cols}
	c.hasSel = c.selAnchor != c.selCursor
	return c.hasSel
}

func (c *CompositeTerminal) clearSelection() {
	c.selActive = false
	c.hasSel = false
}

func (c *CompositeTerminal) normalizedSel() (start, end termPos) {
	start = c.selAnchor
	end = c.selCursor
	if start.line > end.line || (start.line == end.line && start.col > end.col) {
		start, end = end, start
	}
	return start, end
}

func (c *CompositeTerminal) containsSel(absLine, col int) bool {
	if !c.hasSel {
		return false
	}
	start, end := c.normalizedSel()
	if absLine < start.line || absLine > end.line {
		return false
	}
	if start.line == end.line {
		return col >= start.col && col < end.col
	}
	if absLine == start.line {
		return col >= start.col
	}
	if absLine == end.line {
		return col < end.col
	}
	return true
}

func (c *CompositeTerminal) selectedText() string {
	if !c.hasSel {
		return ""
	}
	start, end := c.normalizedSel()
	if start == end {
		return ""
	}
	if start.line == end.line {
		line := c.terminalLineText(start.line)
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
	first := c.terminalLineText(start.line)
	if start.col > len(first) {
		start.col = len(first)
	}
	b.WriteString(first[start.col:])
	for line := start.line + 1; line < end.line; line++ {
		b.WriteByte('\n')
		b.WriteString(c.terminalLineText(line))
	}
	last := c.terminalLineText(end.line)
	if end.col > len(last) {
		end.col = len(last)
	}
	b.WriteByte('\n')
	b.WriteString(last[:end.col])
	return b.String()
}

func (c *CompositeTerminal) copySelection() {
	c.clipboard.copyText(c.selectedText())
}

func (c *CompositeTerminal) terminalLineText(absLine int) string {
	var text string
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		text = readTerminalLine(term, absLine)
	})
	return text
}

func readTerminalLine(term *xterm.Terminal, absLine int) string {
	if term == nil {
		return ""
	}
	buf := term.Buffer()
	if absLine < 0 || absLine >= buf.Lines.Length() {
		return ""
	}
	line := buf.Lines.Get(absLine)
	if line == nil {
		return ""
	}
	cols := term.Cols()
	var b strings.Builder
	cell := xterm.NewCellData()
	for x := 0; x < cols; x++ {
		line.LoadCell(x, cell)
		ch := ' '
		if chars := cell.GetChars(); chars != "" {
			for _, r := range chars {
				ch = r
				break
			}
		}
		b.WriteRune(ch)
	}
	return strings.TrimRight(b.String(), " ")
}
