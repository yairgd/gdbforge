package platform

import (
	"strings"
	"sync"
)

type Buffer struct {
	mu sync.RWMutex

	lines []string
}

func NewBuffer() *Buffer {
	return &Buffer{
		lines: make([]string, 0),
	}
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = b.lines[:0]
}

func (b *Buffer) NumLines() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.lines)
}

func (b *Buffer) Line(i int) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if i < 0 || i >= len(b.lines) {
		return ""
	}

	return b.lines[i]
}

func (b *Buffer) Lines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]string, len(b.lines))
	copy(out, b.lines)

	return out
}

func (b *Buffer) AppendLine(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, line)
}

// TrimTo keeps at most max lines by dropping the oldest. max <= 0 is a no-op.
func (b *Buffer) TrimTo(max int) {
	if max <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.lines) <= max {
		return
	}
	drop := len(b.lines) - max
	copy(b.lines, b.lines[drop:])
	b.lines = b.lines[:max]
}

func (b *Buffer) AppendText(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, strings.Split(text, "\n")...)
}

func (b *Buffer) InsertLine(index int, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if index < 0 {
		index = 0
	}

	if index > len(b.lines) {
		index = len(b.lines)
	}

	b.lines = append(b.lines, "")

	copy(b.lines[index+1:], b.lines[index:])

	b.lines[index] = line
}

func (b *Buffer) RemoveLine(index int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if index < 0 || index >= len(b.lines) {
		return
	}

	copy(b.lines[index:], b.lines[index+1:])

	b.lines = b.lines[:len(b.lines)-1]
}

func (b *Buffer) SetLine(index int, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if index < 0 || index >= len(b.lines) {
		return
	}

	b.lines[index] = line
}

func (b *Buffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return strings.Join(b.lines, "\n")
}
