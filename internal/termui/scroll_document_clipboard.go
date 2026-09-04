package termui

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/platform"
)

// SetClipboard wires shared copy/paste callbacks used by scrollable document widgets.
func (d *ScrollDocument) SetClipboard(io ClipboardIO) {
	d.clipboard = io
}

// SetCopyToClipboard keeps the older API; prefer SetClipboard.
func (d *ScrollDocument) SetCopyToClipboard(fn func(string)) {
	d.clipboard.Copy = fn
}

func (d *ScrollDocument) SetPasteFromClipboard(fn func() string) {
	d.clipboard.Paste = fn
}

// SetReadOnly controls whether Cut/Paste can mutate the buffer.
// Logger panes are read-only (Cut == Copy, Paste is ignored).
func (d *ScrollDocument) SetReadOnly(ro bool) {
	d.readOnly = ro
}

func (d *ScrollDocument) ReadOnly() bool { return d.readOnly }

func (d *ScrollDocument) HasSelection() bool { return d.hasSel }

// SelectedText returns the marked region as plain text for search/clipboard.
func (d *ScrollDocument) SelectedText() string {
	return platform.StripANSI(d.selectedText())
}

func (d *ScrollDocument) selectedText() string {
	if !d.hasSel || d.Buffer == nil {
		return ""
	}

	start, end := d.normalizedSel()
	if start == end {
		return ""
	}

	if start.line == end.line {
		line := d.Buffer.Line(start.line)
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
	first := d.Buffer.Line(start.line)
	if start.col > len(first) {
		start.col = len(first)
	}
	b.WriteString(first[start.col:])
	for line := start.line + 1; line < end.line; line++ {
		b.WriteByte('\n')
		b.WriteString(d.Buffer.Line(line))
	}
	last := d.Buffer.Line(end.line)
	if end.col > len(last) {
		end.col = len(last)
	}
	b.WriteByte('\n')
	b.WriteString(last[:end.col])
	return b.String()
}

// CopySelection copies the current mark to the clipboard and keeps the highlight.
func (d *ScrollDocument) CopySelection() {
	d.clipboard.copyText(platform.StripANSI(d.selectedText()))
}

// CutSelection copies then deletes when the viewport is editable.
func (d *ScrollDocument) CutSelection() {
	text := d.selectedText()
	if text == "" {
		return
	}
	d.clipboard.copyText(platform.StripANSI(text))
	if d.readOnly {
		return
	}
	d.deleteSelection()
}

// PasteAtCursor inserts CLIPBOARD text at the caret (editable viewports only).
func (d *ScrollDocument) PasteAtCursor() {
	d.pasteAtCursor(d.clipboard.pasteText())
}

// PastePrimaryAtCursor inserts PRIMARY (middle-click) text at the caret.
func (d *ScrollDocument) PastePrimaryAtCursor() {
	d.pasteAtCursor(d.clipboard.pastePrimaryText())
}

func (d *ScrollDocument) pasteAtCursor(text string) {
	if d.readOnly || d.Buffer == nil {
		return
	}
	if text == "" {
		return
	}
	if d.hasSel {
		d.deleteSelection()
	}
	d.insertTextAtCursor(text)
}

func (d *ScrollDocument) copySelection() { d.CopySelection() }

func (d *ScrollDocument) deleteSelection() {
	if !d.hasSel || d.Buffer == nil {
		return
	}
	start, end := d.normalizedSel()
	first := d.Buffer.Line(start.line)
	last := d.Buffer.Line(end.line)
	if start.col > len(first) {
		start.col = len(first)
	}
	if end.col > len(last) {
		end.col = len(last)
	}

	merged := first[:start.col] + last[end.col:]
	// Remove middle lines, then replace the start line.
	for line := end.line; line > start.line; line-- {
		d.Buffer.RemoveLine(line)
	}
	d.Buffer.SetLine(start.line, merged)
	d.CursorLine = start.line
	d.CursorCol = start.col
	d.clearSelection()
	d.clampCursorCol()
}

func (d *ScrollDocument) insertTextAtCursor(text string) {
	if d.Buffer == nil || text == "" {
		return
	}
	parts := strings.Split(text, "\n")
	line := d.Buffer.Line(d.CursorLine)
	if d.CursorCol > len(line) {
		d.CursorCol = len(line)
	}
	prefix := line[:d.CursorCol]
	suffix := line[d.CursorCol:]

	if len(parts) == 1 {
		d.Buffer.SetLine(d.CursorLine, prefix+parts[0]+suffix)
		d.CursorCol += len(parts[0])
		return
	}

	d.Buffer.SetLine(d.CursorLine, prefix+parts[0])
	for i := 1; i < len(parts)-1; i++ {
		d.Buffer.InsertLine(d.CursorLine+i, parts[i])
	}
	lastIdx := d.CursorLine + len(parts) - 1
	d.Buffer.InsertLine(lastIdx, parts[len(parts)-1]+suffix)
	d.CursorLine = lastIdx
	d.CursorCol = len(parts[len(parts)-1])
}
