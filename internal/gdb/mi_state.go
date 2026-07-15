package gdb

import (
	"strings"
)

// MiUpdate is produced as soon as complete MI lines arrive — no debounce wait.
type MiUpdate struct {
	DisplayLines []string
	PromptReady  bool
	State        GdbState
	ErrorMsg     string
	Stopped      *MiStopMsg
}

// GdbInputState splits PTY chunks into MI lines and streams display updates.
type GdbInputState struct {
	lineBuf string
	state   GdbState
}

func NewGdbInputState() *GdbInputState {
	return &GdbInputState{state: Done}
}

func (m *GdbInputState) Clear() {
	m.lineBuf = ""
	m.state = Done
}

// PushRaw accepts raw PTY chunks, splits on '\n', and processes each complete
// MI record immediately (streams, results, async, prompt).
func (m *GdbInputState) PushRaw(data string) MiUpdate {
	var out MiUpdate
	if data == "" {
		return out
	}
	m.lineBuf += data

	for {
		i := strings.IndexByte(m.lineBuf, '\n')
		if i == -1 {
			break
		}
		line := m.lineBuf[:i]
		m.lineBuf = m.lineBuf[i+1:]
		m.consumeLine(line, &out)
	}
	return out
}

func (m *GdbInputState) consumeLine(line string, out *MiUpdate) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	switch {
	case strings.HasPrefix(line, "~\""):
		out.DisplayLines = append(out.DisplayLines, decodeStreamPayload(line)...)

	case strings.HasPrefix(line, "@\""):
		out.DisplayLines = append(out.DisplayLines, decodeStreamPayload(line)...)

	case strings.HasPrefix(line, "&\""):
		// Log stream is usually the CLI echo / noise; the UI already echoes
		// submitted commands. Surface it only when paired with ^error.

	case strings.HasPrefix(line, "^error"):
		m.state = Error
		out.State = Error
		msg := ExtractMIField(line, "msg")
		out.ErrorMsg = msg
		if msg != "" {
			out.DisplayLines = append(out.DisplayLines, msg)
		}

	case strings.HasPrefix(line, "^running"):
		m.state = Running
		out.State = Running

	case strings.HasPrefix(line, "^done"), strings.HasPrefix(line, "^connected"), strings.HasPrefix(line, "^exit"):
		m.state = Done
		out.State = Done

	case strings.HasPrefix(line, "*stopped"):
		stop := MiStopMsg{
			Reason:   ExtractMIField(line, "reason"),
			ThreadId: ExtractMIField(line, "thread-id"),
		}
		out.Stopped = &stop

	case line == "(gdb)":
		out.PromptReady = true

	default:
		// notify (=...), other async, or partial — ignore for display
	}
}

func decodeStreamPayload(line string) []string {
	// ~"..." / @"..." / &"..."
	if len(line) < 3 || line[len(line)-1] != '"' {
		return nil
	}
	text := DecodeMIString(line[2 : len(line)-1])
	text = ExpandTabs(text, 8)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	// Drop a trailing empty segment from a final '\n'.
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}
