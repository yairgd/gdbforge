package gdb

import (
	"strings"
)

type MiStopMsg struct {
	Reason   string
	ThreadId string
}
type MiMsg struct {
	CmdLine      []string
	GdbLog       []string
	GdbError     []string
	TargetStdOut []string
	MiStopMsg    MiStopMsg
	gdbState     GdbState
	Buffer       []string
}

func NewMiMsg(buf []string) MiMsg {

	msg := MiMsg{}
	for i := 0; i < len(buf); i++ {
		msg.processLine(buf[i])
	}
	return msg
}
func (m *MiMsg) GdbState() GdbState {
	return m.gdbState
}
func (m *MiMsg) CreateBufferForLine() []string {
	var buf []string

	if m.gdbState == Done || m.gdbState == Running {
		buf = append(buf, m.CmdLine...)
	}
	if m.gdbState == Error {
		buf = append(buf, m.GdbLog[0:]...)

	}

	return buf
}

func (m *MiMsg) processLine(line string) {
	//	lines := strings.Split(data, "\n")

	line = strings.TrimSpace(line)
	line = strings.TrimRight(line, "\r\n")

	switch {

	//
	// --- Streams ---
	//

	case strings.HasPrefix(line, "~\""):

		text := DecodeMIString(line[2 : len(line)-1])
		text = ExpandTabs(text, 8)
		if len(m.CmdLine) == 0 {
			m.CmdLine = append(m.CmdLine, "\n")
		}
		m.CmdLine = append(m.CmdLine, text)

	case strings.HasPrefix(line, "@\""):

		text := DecodeMIString(line[2 : len(line)-1])
		text = ExpandTabs(text, 8)
		m.TargetStdOut = append(m.TargetStdOut, text)

	case strings.HasPrefix(line, "&\""):

		text := DecodeMIString(line[2 : len(line)-1])
		text = ExpandTabs(text, 8)
		m.GdbLog = append(m.GdbLog, text)

	//
	// --- Result record ---
	//

	case strings.HasPrefix(line, "^"):

		switch {

		case strings.HasPrefix(line, "^error"):
			m.gdbState = Error
			msg := ExtractMIField(line, "msg")
			m.GdbError = append(m.GdbError, msg)

		case strings.HasPrefix(line, "^running"):
			m.gdbState = Running

		case strings.HasPrefix(line, "^done"):
			m.gdbState = Done
		}

	//
	// --- Async exec ---
	//

	case strings.HasPrefix(line, "*"):

		if strings.HasPrefix(line, "*stopped") {
			m.MiStopMsg.Reason =
				ExtractMIField(line, "reason")

			m.MiStopMsg.ThreadId =
				ExtractMIField(line, "thread-id")
		}

	//
	// --- Notify ---
	//

	case strings.HasPrefix(line, "="):

		// breakpoint-created
		// thread-created
		// library-loaded
		// etc.

	//
	// --- Prompt ---
	//

	case line == "(gdb)":

		if state != Running {
			// ready
		}

	default:

		// partial / garbage
	}
}
