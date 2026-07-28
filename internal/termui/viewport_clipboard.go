package termui

import (
	"strings"
)

// SetClipboard wires shared copy/paste callbacks used by all Viewport widgets.
func (v *Viewport) SetClipboard(io ClipboardIO) {
	v.clipboard = io
}

// SetCopyToClipboard keeps the older API; prefer SetClipboard.
func (v *Viewport) SetCopyToClipboard(fn func(string)) {
	v.clipboard.Copy = fn
}

func (v *Viewport) SetPasteFromClipboard(fn func() string) {
	v.clipboard.Paste = fn
}

// SetReadOnly controls whether Cut/Paste can mutate the buffer.
// Logger panes are read-only (Cut == Copy, Paste is ignored).
func (v *Viewport) SetReadOnly(ro bool) {
	v.readOnly = ro
}

func (v *Viewport) ReadOnly() bool { return v.readOnly }

func (v *Viewport) HasSelection() bool { return v.hasSel }

// SelectedText returns the marked region with ANSI stripped for search/clipboard.
func (v *Viewport) SelectedText() string {
	return StripANSI(v.selectedText())
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
		line := v.Buffer.Line(start.line)
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
	first := v.Buffer.Line(start.line)
	if start.col > len(first) {
		start.col = len(first)
	}
	b.WriteString(first[start.col:])
	for line := start.line + 1; line < end.line; line++ {
		b.WriteByte('\n')
		b.WriteString(v.Buffer.Line(line))
	}
	last := v.Buffer.Line(end.line)
	if end.col > len(last) {
		end.col = len(last)
	}
	b.WriteByte('\n')
	b.WriteString(last[:end.col])
	return b.String()
}

// CopySelection copies the current mark to the clipboard and keeps the highlight.
func (v *Viewport) CopySelection() {
	v.clipboard.copyText(StripANSI(v.selectedText()))
}

// CutSelection copies then deletes when the viewport is editable.
func (v *Viewport) CutSelection() {
	text := v.selectedText()
	if text == "" {
		return
	}
	v.clipboard.copyText(StripANSI(text))
	if v.readOnly {
		return
	}
	v.deleteSelection()
}

// PasteAtCursor inserts CLIPBOARD text at the caret (editable viewports only).
func (v *Viewport) PasteAtCursor() {
	v.pasteAtCursor(v.clipboard.pasteText())
}

// PastePrimaryAtCursor inserts PRIMARY (middle-click) text at the caret.
func (v *Viewport) PastePrimaryAtCursor() {
	v.pasteAtCursor(v.clipboard.pastePrimaryText())
}

func (v *Viewport) pasteAtCursor(text string) {
	if v.readOnly || v.Buffer == nil {
		return
	}
	if text == "" {
		return
	}
	if v.hasSel {
		v.deleteSelection()
	}
	v.insertTextAtCursor(text)
}

func (v *Viewport) copySelection() { v.CopySelection() }

func (v *Viewport) deleteSelection() {
	if !v.hasSel || v.Buffer == nil {
		return
	}
	start, end := v.normalizedSel()
	first := v.Buffer.Line(start.line)
	last := v.Buffer.Line(end.line)
	if start.col > len(first) {
		start.col = len(first)
	}
	if end.col > len(last) {
		end.col = len(last)
	}

	merged := first[:start.col] + last[end.col:]
	// Remove middle lines, then replace the start line.
	for line := end.line; line > start.line; line-- {
		v.Buffer.RemoveLine(line)
	}
	v.Buffer.SetLine(start.line, merged)
	v.CursorLine = start.line
	v.CursorCol = start.col
	v.clearSelection()
	v.clampCursorCol()
}

func (v *Viewport) insertTextAtCursor(text string) {
	if v.Buffer == nil || text == "" {
		return
	}
	parts := strings.Split(text, "\n")
	line := v.Buffer.Line(v.CursorLine)
	if v.CursorCol > len(line) {
		v.CursorCol = len(line)
	}
	prefix := line[:v.CursorCol]
	suffix := line[v.CursorCol:]

	if len(parts) == 1 {
		v.Buffer.SetLine(v.CursorLine, prefix+parts[0]+suffix)
		v.CursorCol += len(parts[0])
		return
	}

	v.Buffer.SetLine(v.CursorLine, prefix+parts[0])
	for i := 1; i < len(parts)-1; i++ {
		v.Buffer.InsertLine(v.CursorLine+i, parts[i])
	}
	lastIdx := v.CursorLine + len(parts) - 1
	v.Buffer.InsertLine(lastIdx, parts[len(parts)-1]+suffix)
	v.CursorLine = lastIdx
	v.CursorCol = len(parts[len(parts)-1])
}
