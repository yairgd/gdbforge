package mcp

import (
	"context"
	"strings"
	"time"

	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/platform"
)

const (
	defaultCaptureIdle = 80 * time.Millisecond
	defaultCaptureMax  = 2 * time.Second
	defaultDrainWait   = 20 * time.Millisecond
)

// GdbMcpService exposes GDB tools over a shared core.Session (same process
// as the UI). It never owns or Closes the session.
type GdbMcpService struct {
	sess  core.Session
	state *platform.AppState

	captureIdle time.Duration
	captureMax  time.Duration

	// OnBreakpointsChanged is invoked after a command whose PTY output
	// includes =breakpoint-created/deleted. Lets the UI refresh even if a
	// Subscribe fan-out chunk was dropped.
	OnBreakpointsChanged func()
}

func NewGdbMcpService(sess core.Session, state *platform.AppState) *GdbMcpService {
	return &GdbMcpService{
		sess:        sess,
		state:       state,
		captureIdle: defaultCaptureIdle,
		captureMax:  defaultCaptureMax,
	}
}

// Close cancels in-flight work placeholders; does not close the GDB session.
func (s *GdbMcpService) Close() {}

// GdbCommand sends a GDB/MI command under the exclusive write lock and
// returns captured PTY output (also visible to UI subscribers unless the
// GDBWidget suppresses display for non-UI owners).
func (s *GdbMcpService) GdbCommand(ctx context.Context, command string) (string, error) {
	return s.query(ctx, command, platform.PTYOwnerMCP)
}

// Query runs an app MI command (PTYOwnerApp) for exclusive write tracking.
// GDB console paint for App/MCP replies is controlled by AppState.GdbListenPrint
// (default on; :set nogdblistenprint to silence listener traffic).
func (s *GdbMcpService) Query(ctx context.Context, command string) (string, error) {
	return s.query(ctx, command, platform.PTYOwnerApp)
}

func (s *GdbMcpService) query(ctx context.Context, command string, owner platform.PTYOwner) (string, error) {
	if s == nil || s.sess == nil {
		return "", context.Canceled
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil
	}

	ch, cancel := s.sess.Subscribe()
	defer cancel()
	drain(ch, defaultDrainWait)

	var out strings.Builder
	run := func() error {
		return s.sess.WithWrite(ctx, func(w core.PTYWriter) error {
			if err := w.Send(command); err != nil {
				return err
			}
			capture(ctx, ch, &out, s.captureIdle, s.captureMax)
			return nil
		})
	}
	var err error
	if s.state != nil {
		s.state.WithPTYOwner(owner, func() {
			err = run()
		})
	} else {
		err = run()
	}
	raw := out.String()
	// Only MCP (not silent App -break-list) may re-notify: App queries must not
	// re-enter onBreakpointsChanged or they flood the PTY write lock.
	if err == nil && owner == platform.PTYOwnerMCP && s.OnBreakpointsChanged != nil &&
		(strings.Contains(raw, "=breakpoint-created") || strings.Contains(raw, "=breakpoint-deleted")) {
		s.OnBreakpointsChanged()
	}
	return raw, err
}

func drain(ch <-chan core.PtyOutputMsg, wait time.Duration) {
	deadline := time.After(wait)
	for {
		select {
		case <-ch:
		case <-deadline:
			return
		}
	}
}

func capture(ctx context.Context, ch <-chan core.PtyOutputMsg, out *strings.Builder, idle, max time.Duration) {
	deadline := time.NewTimer(max)
	defer deadline.Stop()
	idleT := time.NewTimer(idle)
	defer idleT.Stop()

	resetIdle := func() {
		if !idleT.Stop() {
			select {
			case <-idleT.C:
			default:
			}
		}
		idleT.Reset(idle)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-idleT.C:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg.Err != nil {
				return
			}
			if msg.Data != "" {
				out.WriteString(msg.Data)
				// MI prompt ends the reply — don't wait for idle timeout (was
				// 250ms per -break-list and froze the console under load).
				if strings.Contains(msg.Data, "(gdb)") {
					return
				}
				resetIdle()
			}
		}
	}
}
