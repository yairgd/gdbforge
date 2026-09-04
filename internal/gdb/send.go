package gdb

import (
	"context"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/platform"
)

// InferiorCtl is debugger run-state used while sending break/clear under a live inferior.
// Implemented by gdbforge/debugstate.State (not platform.AppState).
type InferiorCtl interface {
	InferiorRunning() bool
	ContinueAfterClear() bool
	// NoteTransientStopSuppress arms one skip of stop UI for the Ctrl-C that
	// SendCmd issues before break/clear + auto-continue (keeps Code browse).
	NoteTransientStopSuppress()
}

// SendOpts controls how SendCmd interrupts a running inferior.
type SendOpts struct {
	// InterruptCmd stops the running inferior over the command channel itself
	// (MIExecInterrupt for GDB). It is required whenever that channel is not the
	// debugger's own terminal: the new-ui MI PTY has no foreground process group,
	// so a \x03 byte written there is swallowed by the line discipline instead of
	// becoming SIGINT. Empty means write an inline ^C, which is correct for Delve
	// and single-PTY GDB.
	InterruptCmd string
}

// MIExecInterrupt is the MI command that stops a running inferior.
const MIExecInterrupt = "-exec-interrupt"

// interruptSettle gives GDB time to report the stop before the break/clear that
// follows, which it would otherwise reject while the target still runs.
const interruptSettle = 30 * time.Millisecond

// SendCmd writes a GDB CLI/MI command on the shared PTY.
//
// While the inferior is running, GDB will not process break/clear until
// interrupted — so we interrupt first, then send the command. Auto-resume with
// continue applies only to breakpoint insert (so the new break can be hit)
// and, optionally, to remove when InferiorCtl.ContinueAfterClear is set.
// Other commands (frame, thread, …) stay stopped after the interrupt —
// never send a surprise continue.
func SendCmd(sess core.Session, app *platform.AppState, ctl InferiorCtl, cmd string, opts SendOpts) {
	if sess == nil || cmd == "" {
		return
	}
	send := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sess.WithWrite(ctx, func(pw core.PTYWriter) error {
			running := ctl != nil && ctl.InferiorRunning()
			willResume := false
			if running {
				switch {
				case IsBreakInsertCmd(cmd):
					willResume = true
				case IsBreakRemoveCmd(cmd) && ctl.ContinueAfterClear():
					willResume = true
				}
				// Arm before the interrupt so a fast *stopped cannot race past an
				// unarmed suppress and later eat the real breakpoint-hit UI.
				if willResume {
					ctl.NoteTransientStopSuppress()
				}
				if err := interruptInferior(pw, opts); err != nil {
					return err
				}
				if opts.InterruptCmd != "" {
					time.Sleep(interruptSettle)
				}
			}
			if err := pw.Send(cmd); err != nil {
				return err
			}
			if !running || !willResume {
				return nil
			}
			return pw.Send("continue")
		})
	}
	if app != nil {
		app.WithPTYOwner(platform.PTYOwnerApp, send)
	} else {
		send()
	}
}

// interruptInferior stops the target on the command channel, so the interrupt
// cannot be reordered against the break/clear that follows it.
func interruptInferior(pw core.PTYWriter, opts SendOpts) error {
	if opts.InterruptCmd != "" {
		return pw.Send(opts.InterruptCmd)
	}
	return pw.SendRaw("\x03")
}

// IsBreakRemoveCmd reports clear / -break-delete style commands.
func IsBreakRemoveCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	return strings.HasPrefix(cmd, "clear ") ||
		strings.HasPrefix(cmd, "clearall ") ||
		strings.HasPrefix(cmd, "-break-delete")
}

// IsBreakInsertCmd reports break / tbreak / -break-insert style commands.
func IsBreakInsertCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	switch {
	case strings.HasPrefix(cmd, "break "):
		return true
	case strings.HasPrefix(cmd, "tbreak "):
		return true
	case strings.HasPrefix(cmd, "-break-insert"):
		return true
	case cmd == "break" || cmd == "tbreak":
		return true
	default:
		return false
	}
}

// IsContinueCmd reports GDB continue commands (c / continue only).
func IsContinueCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	switch strings.Fields(cmd)[0] {
	case "c", "continue":
		return true
	default:
		return false
	}
}

// IsTargetRemoteCmd reports GDB attach commands (target remote / tar rem / …).
func IsTargetRemoteCmd(cmd string) bool {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) < 2 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "target", "tar":
	default:
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "remote", "r", "rem", "extended-remote", "extended", "e":
		return true
	default:
		return false
	}
}

// CLIExecToMI maps common CLI run-control commands to MI -exec-* so the
// console does not print source/line listings (Code widget follows *stopped).
func CLIExecToMI(cmd string) string {
	switch strings.TrimSpace(cmd) {
	case "n", "next":
		return "-exec-next"
	case "s", "step":
		return "-exec-step"
	case "c", "cont", "continue":
		return "-exec-continue"
	case "finish":
		return "-exec-finish"
	case "ni", "nexti":
		return "-exec-next-instruction"
	case "si", "stepi":
		return "-exec-step-instruction"
	case "run":
		return "-exec-run"
	default:
		return cmd
	}
}
