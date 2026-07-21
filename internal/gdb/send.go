package gdb

import (
	"context"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/platform"
)

// SendCmd writes a GDB CLI/MI command on the shared PTY.
//
// While the inferior is running, sync GDB will not process break/clear until
// interrupted — so we send Ctrl-C, then the command. Auto-resume with
// continue applies only to breakpoint insert (so the new break can be hit)
// and, optionally, to remove when AppState.ContinueAfterClear is set.
// Other commands (frame, thread, …) stay stopped after the interrupt —
// never send a surprise continue.
func SendCmd(sess core.Session, state *platform.AppState, cmd string) {
	if sess == nil || cmd == "" {
		return
	}
	send := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sess.WithWrite(ctx, func(pw core.PTYWriter) error {
			running := state != nil && state.InferiorRunning()
			if running {
				if err := pw.SendRaw("\x03"); err != nil {
					return err
				}
			}
			if err := pw.Send(cmd); err != nil {
				return err
			}
			if !running {
				return nil
			}
			if IsBreakRemoveCmd(cmd) {
				if state != nil && state.ContinueAfterClear() {
					return pw.Send("continue")
				}
				return nil
			}
			if IsBreakInsertCmd(cmd) {
				return pw.Send("continue")
			}
			return nil
		})
	}
	if state != nil {
		state.WithPTYOwner(platform.PTYOwnerApp, send)
	} else {
		send()
	}
}

// IsBreakRemoveCmd reports clear / -break-delete style commands.
func IsBreakRemoveCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	return strings.HasPrefix(cmd, "clear ") || strings.HasPrefix(cmd, "-break-delete")
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
