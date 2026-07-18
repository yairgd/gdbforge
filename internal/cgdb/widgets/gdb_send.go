package widgets

import (
	"context"
	"time"

	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/platform"
)

// sendGdbCmd writes a GDB CLI/MI command on the shared PTY.
//
// While the inferior is running, sync GDB will not process break/clear until
// interrupted — so we send Ctrl-C, then the command, then continue so the
// new breakpoint can actually be hit.
func sendGdbCmd(sess core.Session, state *platform.AppState, cmd string) {
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
			if running {
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
