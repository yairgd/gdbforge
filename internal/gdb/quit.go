package gdb

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/core"
)

// QuitAction is a quit-policy decision for a UI to present and/or a Session to send.
type QuitAction int

const (
	QuitNoop QuitAction = iota
	// QuitShowConfirm: inferior is alive — UI should ask "Quit anyway?" (MI never does).
	QuitShowConfirm
	// QuitReprompt: invalid y/n answer while confirming.
	QuitReprompt
	// QuitSendQ: send "q" (Ctrl-D / first quit with no inferior).
	QuitSendQ
	// QuitSendQuit: user confirmed — send "quit".
	QuitSendQuit
	// QuitSendEmpty: user cancelled confirm — solicit a real MI (gdb) prompt.
	QuitSendEmpty
)

// Sends reports whether ApplyQuitAction should write to the PTY.
func (a QuitAction) Sends() bool {
	switch a {
	case QuitSendQ, QuitSendQuit, QuitSendEmpty:
		return true
	default:
		return false
	}
}

// QuitGate tracks inferior lifetime and mirrors CLI quit confirmation for MI.
// UI-agnostic: callers paint from QuitAction and send via ApplyQuitAction.
type QuitGate struct {
	alive      bool
	pid        string
	confirming bool
}

// Observe updates inferior/quit state from a parsed MI update.
func (g *QuitGate) Observe(u MiUpdate) {
	if g == nil {
		return
	}
	if u.InferiorPID != "" {
		g.pid = u.InferiorPID
		g.alive = true
	}
	if u.InferiorExited {
		g.pid = ""
		g.alive = false
		g.confirming = false
	}
	if u.Stopped != nil {
		switch u.Stopped.Reason {
		case "exited-normally", "exited", "exited-signalled":
			g.alive = false
			g.pid = ""
		}
	}
}

// Confirming is true while waiting for y/n after QuitShowConfirm.
func (g *QuitGate) Confirming() bool {
	return g != nil && g.confirming
}

// InferiorAlive reports whether a debuggee process is considered running.
func (g *QuitGate) InferiorAlive() bool {
	return g != nil && g.alive
}

// InferiorPID is the last known inferior pid string, or "".
func (g *QuitGate) InferiorPID() string {
	if g == nil {
		return ""
	}
	return g.pid
}

// RequestQuit is Ctrl-D / EOF: confirm when an inferior is alive, else send q.
func (g *QuitGate) RequestQuit() QuitAction {
	if g == nil {
		return QuitSendQ
	}
	if g.alive && !g.confirming {
		g.confirming = true
		return QuitShowConfirm
	}
	return QuitSendQ
}

// SubmitQuitCommand handles a typed q/quit line. Returns QuitNoop when cmd is
// not a quit command (caller should send cmd normally).
func (g *QuitGate) SubmitQuitCommand(cmd string) QuitAction {
	if g == nil {
		return QuitNoop
	}
	if g.confirming {
		return g.Answer(cmd)
	}
	if !IsQuitCmd(cmd) {
		return QuitNoop
	}
	if g.alive {
		g.confirming = true
		return QuitShowConfirm
	}
	return QuitSendQ
}

// Answer resolves a pending quit confirm (y/n / Enter=n).
func (g *QuitGate) Answer(raw string) QuitAction {
	if g == nil {
		return QuitNoop
	}
	ans := strings.TrimSpace(strings.ToLower(raw))
	yes := ans == "y" || ans == "yes"
	no := ans == "" || ans == "n" || ans == "no"
	if !yes && !no {
		return QuitReprompt
	}
	g.confirming = false
	if yes {
		return QuitSendQuit
	}
	return QuitSendEmpty
}

// IsQuitCmd reports whether cmd is q or quit (CLI).
func IsQuitCmd(cmd string) bool {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "q", "quit":
		return true
	default:
		return false
	}
}

// QuitConfirmHost is the live input line CLI GDB uses for quit confirmation.
// MI never emits this; QuitGate mirrors it for UIs.
const QuitConfirmHost = "Quit anyway? (y or n) "

// QuitConfirmLines returns the scrollback block CLI GDB prints before
// QuitConfirmHost when an inferior is alive (MI suppresses this text).
func QuitConfirmLines(pid string) []string {
	if pid == "" {
		pid = "?"
	}
	return []string{
		"A debugging session is active.",
		"",
		"\tInferior 1 [process " + pid + "] will be killed.",
		"",
	}
}

// QuitRepromptLines returns CLI GDB's invalid y/n reply before re-asking.
func QuitRepromptLines() []string {
	return []string{"Please answer y or n."}
}

// ApplyQuitAction performs the PTY write for a sending QuitAction on a Session.
func ApplyQuitAction(d core.Debugger, a QuitAction) error {
	if d == nil || !a.Sends() {
		return nil
	}
	switch a {
	case QuitSendQ:
		return d.Send("q")
	case QuitSendQuit:
		return d.Send("quit")
	case QuitSendEmpty:
		return d.Send("")
	default:
		return nil
	}
}

// ApplyQuitActionCLI writes quit bytes to the GDB console PTY (PTY #1).
func ApplyQuitActionCLI(cli interface {
	Send(string) error
}, a QuitAction) error {
	if cli == nil || !a.Sends() {
		return nil
	}
	switch a {
	case QuitSendQ:
		return cli.Send("q")
	case QuitSendQuit:
		return cli.Send("quit")
	case QuitSendEmpty:
		return cli.Send("")
	default:
		return nil
	}
}
