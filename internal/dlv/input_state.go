package dlv

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/termui"
)

// State mirrors gdb.GdbState values used by the console paint path.
type State = gdb.GdbState

const (
	Done    = gdb.Done
	Error   = gdb.Error
	Running = gdb.Running
)

// Update is produced as complete Delve CLI lines arrive (peer of gdb.MiUpdate).
type Update struct {
	DisplayLines       []string
	PromptReady        bool
	PromptLine         string
	// ConfirmReady is set when Delve waits for a yes/no answer (not a bare (dlv) prompt).
	ConfirmReady bool
	ConfirmHost  string
	State              State
	ErrorMsg           string
	Stopped            *gdb.MiStopMsg
	BreakpointsChanged bool
	InferiorExited     bool
}

// InputState splits PTY chunks into lines and streams display / stop updates.
type InputState struct {
	lineBuf string
	state   State
}

// NewInputState returns an empty Delve CLI parser.
func NewInputState() *InputState {
	return &InputState{state: Done}
}

// Clear resets the line buffer and state.
func (m *InputState) Clear() {
	if m == nil {
		return
	}
	m.lineBuf = ""
	m.state = Done
}

// PushRaw accepts raw PTY chunks, splits on '\n', and processes each line.
func (m *InputState) PushRaw(data string) Update {
	var out Update
	if m == nil || data == "" {
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
	// Prompt / yesno may arrive without a trailing newline while waiting for input.
	plainBuf := strings.TrimSpace(termui.StripANSI(m.lineBuf))
	if plainBuf == PromptToken {
		m.lineBuf = ""
		out.PromptReady = true
		out.PromptLine = PromptToken
		m.state = Done
		out.State = Done
	} else if LooksLikeYesNoPrompt(plainBuf) {
		m.lineBuf = ""
		out.ConfirmReady = true
		out.ConfirmHost = ConfirmLiveHost(plainBuf)
		m.state = Done
		out.State = Done
	}
	return out
}

func (m *InputState) consumeLine(line string, out *Update) {
	raw := strings.TrimRight(line, "\r")
	plain := strings.TrimSpace(termui.StripANSI(raw))
	if plain == "" {
		return
	}

	switch {
	case plain == PromptToken:
		out.PromptReady = true
		out.PromptLine = PromptToken
		m.state = Done
		out.State = Done

	case LooksLikeYesNoPrompt(plain):
		// Delve sometimes ends the question with a newline; treat as confirm host,
		// not scrollback-only (so the UI can attach a live caret).
		out.ConfirmReady = true
		out.ConfirmHost = ConfirmLiveHost(plain)
		m.state = Done
		out.State = Done

	case strings.HasPrefix(plain, "> "):
		// > [Breakpoint 1] main.main() ./hello.go:23 (hits goroutine(1):1 total:1) (PC: 0x…)
		if stop := parseStopLine(plain); stop != nil {
			out.Stopped = stop
			m.state = Done
			out.State = Done
		}
		out.DisplayLines = append(out.DisplayLines, raw)

	case looksLikeBreakpointSet(plain):
		out.BreakpointsChanged = true
		out.DisplayLines = append(out.DisplayLines, raw)

	case looksLikeBreakpointCleared(plain):
		out.BreakpointsChanged = true
		out.DisplayLines = append(out.DisplayLines, raw)

	case strings.HasPrefix(plain, "Command failed:") || strings.HasPrefix(plain, "Command failed"):
		m.state = Error
		out.State = Error
		out.ErrorMsg = plain
		out.DisplayLines = append(out.DisplayLines, raw)

	case plain == "Process has exited with status 0" ||
		strings.HasPrefix(plain, "Process has exited with status") ||
		strings.HasPrefix(plain, "Process ") && strings.Contains(plain, "has exited"):
		out.InferiorExited = true
		out.Stopped = &gdb.MiStopMsg{Reason: "exited-normally"}
		out.DisplayLines = append(out.DisplayLines, raw)

	default:
		if isPagerChrome(plain) {
			return
		}
		out.DisplayLines = append(out.DisplayLines, raw)
	}
}

func isPagerChrome(plain string) bool {
	if strings.Contains(plain, "(END)") && strings.Contains(plain, "lines ") {
		return true
	}
	if plain == ":" || plain == "(END)" {
		return true
	}
	return false
}

func looksLikeBreakpointSet(line string) bool {
	return strings.HasPrefix(line, "Breakpoint ") && strings.Contains(line, " set at ")
}

func looksLikeBreakpointCleared(line string) bool {
	return strings.HasPrefix(line, "Breakpoint ") &&
		(strings.Contains(line, " cleared") || strings.Contains(line, "deleted"))
}

// parseStopLine extracts file/line/func from a Delve "> …" location line.
// plain must already be ANSI-stripped.
func parseStopLine(line string) *gdb.MiStopMsg {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "> ") {
		return nil
	}
	rest := strings.TrimSpace(line[2:])
	stop := &gdb.MiStopMsg{Reason: "breakpoint-hit"}

	// Drop optional "[Breakpoint N]" / "[Breakpoint N.M]" prefix.
	if strings.HasPrefix(rest, "[") {
		if i := strings.IndexByte(rest, ']'); i >= 0 {
			rest = strings.TrimSpace(rest[i+1:])
		}
	}

	if file, lineNo, ok := findFileLine(rest); ok {
		stop.File = ResolveSourcePath(file)
		stop.Line = lineNo
	}

	if i := strings.IndexByte(rest, '('); i > 0 {
		fn := strings.TrimSpace(rest[:i])
		if fn != "" && !strings.HasPrefix(fn, "[") {
			stop.Func = fn
		}
	}

	if stop.File == "" && stop.Func == "" {
		return nil
	}
	return stop
}

func findFileLine(s string) (file string, line int, ok bool) {
	cut := s
	for _, mark := range []string{" (hits", " (PC:", " (thread", " (goroutine"} {
		if i := strings.Index(cut, mark); i >= 0 {
			cut = cut[:i]
		}
	}
	cut = strings.TrimSpace(cut)
	if i := strings.LastIndex(cut, ") "); i >= 0 {
		cut = strings.TrimSpace(cut[i+2:])
	}
	return splitFileLine(cut)
}

func splitFileLine(s string) (file string, line int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, false
	}
	i := strings.LastIndexByte(s, ':')
	if i <= 0 || i+1 >= len(s) {
		return "", 0, false
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil || n < 1 {
		return "", 0, false
	}
	return s[:i], n, true
}

// ResolveSourcePath turns Delve relative paths (./hello.go) into absolute ones
// so CodeWidget can open them regardless of how the binary was built.
func ResolveSourcePath(p string) string {
	p = strings.TrimSpace(termui.StripANSI(p))
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

// IsPromptRecord reports whether line is Delve's interactive prompt.
func IsPromptRecord(line string) bool {
	return strings.TrimSpace(termui.StripANSI(line)) == PromptToken
}

// IsBarePromptHost reports a scrollback line that is only the Delve prompt.
func IsBarePromptHost(line string) bool {
	return strings.TrimSpace(termui.StripANSI(line)) == PromptToken
}

// LivePromptHost returns fromDLV with exactly one trailing space for input.
func LivePromptHost(fromDLV string) string {
	if fromDLV == "" {
		return ""
	}
	return strings.TrimRight(fromDLV, " ") + " "
}

// StopNeedsUIRefresh is true for stops that should update Code / threads / stack.
func StopNeedsUIRefresh(stop *gdb.MiStopMsg) bool {
	return gdb.StopNeedsUIRefresh(stop)
}

// IsStackNavCmd reports Delve commands that change the selected frame without
// a new "> " stop line.
func IsStackNavCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "frame", "up", "down":
		return true
	default:
		return false
	}
}

// FrameNavTargetLevel returns the absolute stack level after a Delve frame/up/down
// command. cur is the level before the command (usually the call-stack selection).
func FrameNavTargetLevel(cmd string, cur int) (int, bool) {
	cmd = strings.TrimSpace(cmd)
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return 0, false
	}
	n := 1
	if len(fields) > 1 {
		if v, err := strconv.Atoi(fields[1]); err == nil {
			n = v
		}
	}
	switch fields[0] {
	case "frame":
		if len(fields) < 2 || n < 0 {
			return 0, false
		}
		return n, true
	case "up":
		if n < 1 {
			n = 1
		}
		return cur + n, true
	case "down":
		if n < 1 {
			n = 1
		}
		if cur < n {
			return 0, true
		}
		return cur - n, true
	default:
		return 0, false
	}
}

// MapBreakCmd maps GDB-oriented break/clear/delete strings to Delve CLI.
func MapBreakCmd(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	switch {
	case strings.HasPrefix(cmd, "-break-delete "):
		return "clear " + strings.TrimSpace(strings.TrimPrefix(cmd, "-break-delete "))
	case strings.HasPrefix(cmd, "disable "):
		return "clear " + strings.TrimSpace(strings.TrimPrefix(cmd, "disable "))
	case strings.HasPrefix(cmd, "clear "):
		arg := strings.TrimSpace(strings.TrimPrefix(cmd, "clear "))
		if arg == "" {
			return cmd
		}
		if isAllDigits(arg) {
			return "clear " + arg
		}
		return "clearall " + arg
	default:
		return cmd
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
