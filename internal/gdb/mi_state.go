package gdb

import (
	"strconv"
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/mitext"
)

// MiUpdate is produced as soon as complete MI lines arrive — no debounce wait.
type MiUpdate struct {
	DisplayLines []string // console (~), make/shell raw PTY text, errors
	TargetLines  []string // inferior target stream (@) — legacy MI path
	PromptReady  bool
	// PromptLine is the exact MI prompt record GDB emitted (e.g. "(gdb)").
	// Empty unless PromptReady. UI must paint this text — never invent a prompt.
	PromptLine         string
	State              GdbState
	ErrorMsg           string
	Stopped            *MiStopMsg
	BreakpointsChanged bool
	// InferiorPID is set on =thread-group-started (non-empty).
	InferiorPID string
	// InferiorExited is set on =thread-group-exited.
	InferiorExited bool
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
	line = stripMILinePrefix(line)
	if line == "" {
		return
	}

	switch {
	case strings.HasPrefix(line, "~\""):
		out.DisplayLines = append(out.DisplayLines, decodeStreamPayload(line)...)

	case strings.HasPrefix(line, "@\""):
		// Target stream: optional legacy paint in GDB console (:set gdbtargetprint).
		out.TargetLines = append(out.TargetLines, decodeStreamPayload(line)...)

	case strings.HasPrefix(line, "&\""):
		// Log stream: mostly CLI echo (UI already EchoSubmit). Still surface
		// Ctrl-C feedback (&"Quit\n" / &"❌️ Quit\n") and similar markers.
		for _, p := range decodeStreamPayload(line) {
			if isCtrlCQuitLog(p) {
				out.DisplayLines = append(out.DisplayLines, p)
			}
		}

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
			File:     ExtractMIField(line, "fullname"),
			Func:     ExtractMIField(line, "func"),
		}
		if stop.File == "" {
			stop.File = ExtractMIField(line, "file")
		}
		if ln := ExtractMIField(line, "line"); ln != "" {
			if n, err := strconv.Atoi(ln); err == nil {
				stop.Line = n
			}
		}
		out.Stopped = &stop
		// Inferior is no longer running — clear so a same-batch (gdb) prompt
		// does not leave State==Running and re-arm InferiorRunning (Ctrl-Z).
		m.state = Done
		out.State = Done
		// Ctrl-C / signals: ensure the classic GDB one-liner appears even when
		// ~ streams were fragmented or prefixed with a ^C PTY echo.
		if stop.Reason == "signal-received" {
			if msg := formatSignalReceived(line); msg != "" && !displayHasSignalMsg(out.DisplayLines) {
				out.DisplayLines = append(out.DisplayLines, msg)
			}
		}

	case IsMIPromptRecord(line):
		out.PromptReady = true
		out.PromptLine = line

	case strings.HasPrefix(line, "=breakpoint-created"),
		strings.HasPrefix(line, "=breakpoint-deleted"):
		// Structural changes only. Ignore =breakpoint-modified (hit counts on
		// "n"/continue) — those raced -break-list and desynced the widgets.
		out.BreakpointsChanged = true

	case strings.HasPrefix(line, "=thread-group-started"):
		out.InferiorPID = ExtractMIField(line, "pid")

	case strings.HasPrefix(line, "=thread-group-exited"):
		out.InferiorExited = true

	default:
		// make / shell write child stdout as raw PTY text — not wrapped in
		// ~/"@" streams. Paint it so the console matches classic GDB.
		// Other MI records we chose not to handle above stay silent
		// (including +download status async during load).
		if len(line) > 0 {
			switch line[0] {
			case '~', '@', '&', '^', '*', '=', '+':
				return
			}
		}
		out.DisplayLines = append(out.DisplayLines, ExpandTabs(line, 8))
	}
}

// stripMILinePrefix removes PTY echo noise glued onto MI records — especially
// Ctrl-C (\x03), which GDB prints immediately before the first ~"/&" record.
// Also peels numeric MI tokens ("42^done").
// Do not strip ESC (0x1b): make/gcc color SGR sequences must stay intact.
func stripMILinePrefix(line string) string {
	for len(line) > 0 {
		switch line[0] {
		case '~', '@', '&', '^', '*', '=', '+', '(':
			return line
		case '\x1b': // ANSI CSI/OSC — keep for console colorization
			return line
		case '\x03': // Ctrl-C echo glued onto the next MI record
			line = line[1:]
			continue
		}
		if line[0] >= '0' && line[0] <= '9' {
			i := 0
			for i < len(line) && line[i] >= '0' && line[i] <= '9' {
				i++
			}
			if i > 0 && i < len(line) {
				switch line[i] {
				case '^', '*', '=', '+':
					return line[i:]
				}
			}
			return line
		}
		// Other ASCII controls / DEL — drop and keep looking.
		if line[0] < 0x20 || line[0] == 0x7f {
			line = line[1:]
			continue
		}
		return line
	}
	return line
}

func formatSignalReceived(line string) string {
	name := ExtractMIField(line, "signal-name")
	meaning := ExtractMIField(line, "signal-meaning")
	if name == "" {
		return ""
	}
	if meaning != "" {
		return "Program received signal " + name + ", " + meaning + "."
	}
	return "Program received signal " + name + "."
}

func displayHasSignalMsg(lines []string) bool {
	for _, ln := range lines {
		if strings.Contains(ln, "received signal") {
			return true
		}
	}
	return false
}

// IsCtrlCQuitLog reports GDB log-stream Ctrl-C feedback ("Quit" / "❌️ Quit").
func IsCtrlCQuitLog(s string) bool {
	return mitext.IsCtrlCQuitLog(s)
}

// Re-export MI prompt helpers from gdbforge/mitext for backend callers.
const (
	MIPromptToken    = mitext.MIPromptToken
	MIPromptLiveHost = mitext.MIPromptLiveHost
)

// IsMIPromptRecord reports whether line is GDB's MI prompt record.
func IsMIPromptRecord(line string) bool { return mitext.IsMIPromptRecord(line) }

// IsBareMIPromptHost reports a scrollback line that is only the MI prompt token.
func IsBareMIPromptHost(line string) bool { return mitext.IsBareMIPromptHost(line) }

// LivePromptHost returns fromGDB with exactly one trailing space for input.
func LivePromptHost(fromGDB string) string { return mitext.LivePromptHost(fromGDB) }

func isCtrlCQuitLog(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	return t == "Quit" || strings.HasSuffix(t, " Quit") || strings.Contains(t, "Quit")
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
