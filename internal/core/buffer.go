package core

type Buffer struct {
	lines []string
}

// --- constructor ---

func NewBuffer() *Buffer {
	return &Buffer{
		lines: []string{},
	}
}

// --- basic API ---

//func (b *Buffer) AddLine(s string) {
//	b.lines = append(b.lines, s)
//}

func (b *Buffer) SetLine(i int, s string) {
	if i < 0 || i >= len(b.lines) {
		return
	}
	b.lines[i] = s
}

func (b *Buffer) GetLine(i int) string {
	if i < 0 || i >= len(b.lines) {
		return ""
	}
	return b.lines[i]
}

func (b *Buffer) NumLines() int {
	return len(b.lines)
}

func (b *Buffer) Clear() {
	b.lines = b.lines[:0]
}

// --- range access (viewport) ---

func (b *Buffer) GetLines(start, end int) []string {
	if start < 0 {
		start = 0
	}
	if end > len(b.lines) {
		end = len(b.lines)
	}
	if start > end {
		return nil
	}
	return b.lines[start:end]
}

// AppendText handles partial lines (important for PTY/GDB)
func (b *Buffer) AppendText(text string) {
	if len(b.lines) == 0 {
		b.lines = append(b.lines, "")
	}
	//	line := len(b.lines) - 1
	current := b.lines[len(b.lines)-1]

	for _, ch := range text {
		if ch == '\n' {
			b.lines[len(b.lines)-1] = current
			b.lines = append(b.lines, "")
			current = ""
		} else {
			current += string(ch)
		}
	}

	b.lines[len(b.lines)-1] = current
}
