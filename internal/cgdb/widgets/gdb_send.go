package widgets

import (
	"context"
	"strings"
	"time"

	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/platform"
)

// SendGdbCmd writes a GDB CLI/MI command on the shared PTY.
//
// While the inferior is running, sync GDB will not process break/clear until
// interrupted — so we send Ctrl-C, then the command. Inserting a breakpoint
// always resumes with continue so the new break can be hit. Removing one
// resumes only when AppState.ContinueAfterClear is set (default off).
func SendGdbCmd(sess core.Session, state *platform.AppState, cmd string) {
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
			if isBreakRemoveCmd(cmd) {
				if state != nil && state.ContinueAfterClear() {
					return pw.Send("continue")
				}
				return nil
			}
			return pw.Send("continue")
		})
	}
	if state != nil {
		state.WithPTYOwner(platform.PTYOwnerApp, send)
	} else {
		send()
	}
}

func sendGdbCmd(sess core.Session, state *platform.AppState, cmd string) {
	SendGdbCmd(sess, state, cmd)
}

func isBreakRemoveCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	return strings.HasPrefix(cmd, "clear ") || strings.HasPrefix(cmd, "-break-delete")
}
