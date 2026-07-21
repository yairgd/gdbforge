package core

import (
	"strings"
)

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
	b.lines = append(b.lines, text)
}

func (b *Buffer) AppendBuffer(buf []string) {
	for i := 0; i < len(buf); i++ {
		b.AppendText(buf[i])
	}
	// b.lines = append(b.lines, buf...)
}

func (b *Buffer) AppendText1111(text string) {

	if len(b.lines) == 0 {
		b.lines = append(b.lines, "")
	}

	// Split the incoming text by newlines immediately
	// This is more efficient than checking byte-by-byte
	parts := strings.Split(text, "\n")

	// The first part completes the current last line
	b.lines[len(b.lines)-1] += parts[0]

	// If there were newlines, add the subsequent parts as new lines
	if len(parts) > 1 {
		b.lines = append(b.lines, parts[1:]...)
	}
}
