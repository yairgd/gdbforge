package gdb

import (
	"strconv"
	"strings"
	"time"
)

type MiState int

const (
	Done1 MiState = iota
	Error1
	Running1
)

type GdbInputState struct {
	gdbState MiState

	// raw MI lines (burst collection)
	buffer []string

	// console output (what you show)
	Console []string

	// line buffer for stream-safe splitting
	lineBuf string

	Timer *time.Timer
}

func NewGdbInputState() *GdbInputState {

	m := &GdbInputState{
		gdbState: Done1,
		buffer:   []string{},
		Console:  []string{},
		Timer:    time.NewTimer(100 * time.Millisecond),
	}

	// stop timer initially
	if !m.Timer.Stop() {
		select {
		case <-m.Timer.C:
		default:
		}
	}

	return m
}
func (m *GdbInputState) Buffer() []string {
	return m.buffer
}
func (m *GdbInputState) Clear() {
	m.gdbState = Done1
	m.buffer = m.buffer[:0]
	m.lineBuf = ""

	if !m.Timer.Stop() {
		select {
		case <-m.Timer.C:
		default:
		}
	}
}

//
// ---------- INPUT SIDE ----------
//

// PushRaw should receive RAW chunks from GDB (not split lines!)
func (m *GdbInputState) PushRaw(data string) {

	m.lineBuf += data

	for {
		i := strings.IndexByte(m.lineBuf, '\n')
		if i == -1 {
			break
		}

		line := m.lineBuf[:i]
		m.lineBuf = m.lineBuf[i+1:]

		m.PushLine(line)
	}
}

func (m *GdbInputState) PushLine(line string) {

	if line == "" {
		return
	}

	// collect burst
	m.buffer = append(m.buffer, line)

	// detect state
	switch {
	case strings.HasPrefix(line, "^error"):
		m.gdbState = Error1
	case strings.HasPrefix(line, "^running"):
		m.gdbState = Running1
	case strings.HasPrefix(line, "^done"):
		m.gdbState = Done1
	}

	// reset timer
	if !m.Timer.Stop() {
		select {
		case <-m.Timer.C:
		default:
		}
	}
	m.Timer.Reset(10 * time.Millisecond)
}

//
// ---------- PROCESSING ----------
//

// Call this when timer fires
func (m *GdbInputState) Flush() {

	for _, line := range m.buffer {

		if len(line) == 0 {
			continue
		}

		switch line[0] {

		case '~': // console output
			text := extractQuoted(line)
			text = unescape(text)
			m.appendConsole(text)

		case '&':
			// optional: ignore (or handle as log)

		case '^':
			// result (state only)

		case '=', '*':
			// async events → ignore for console
		}
	}

	m.buffer = m.buffer[:0]
}

//
// ---------- CONSOLE ----------
//

func (m *GdbInputState) appendConsole(text string) {

	parts := strings.Split(text, "\n")

	for i, p := range parts {
		if i == len(parts)-1 && p == "" {
			continue
		}
		m.Console = append(m.Console, p)
	}
}

//
// ---------- HELPERS ----------
//

func extractQuoted(line string) string {
	i := strings.IndexByte(line, '"')
	if i == -1 {
		return ""
	}
	return line[i:]
}

func unescape(s string) string {
	out, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return out
}
