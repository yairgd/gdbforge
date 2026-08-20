package mcp

import (
	"context"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/domain"
	"github.com/yairgd/gdbforge/internal/platform"
)

const (
	defaultCaptureIdle = 80 * time.Millisecond
	defaultCaptureMax  = 2 * time.Second
	defaultDrainWait   = 20 * time.Millisecond
	// kgdb serial: deep -stack-list-frames can arrive with long gaps between chunks.
	stackCaptureIdle = 500 * time.Millisecond
	stackCaptureMax  = 15 * time.Second
)

// GdbMcpService exposes GDB tools over a shared core.Session (same process
// as the UI). It never owns or Closes the session.
type GdbMcpService struct {
	sess  core.Session
	state *platform.AppState

	captureIdle time.Duration
	captureMax  time.Duration

	// promptToken ends a captured reply (default "(gdb)"; Delve uses "(dlv)").
	promptToken string

	// domain is the app-owned shared model surface (AI tools; future Lua).
	domain domain.DebugDomain

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
		promptToken: gdb.MIPromptToken,
	}
}

// SetPromptToken overrides the capture end marker (e.g. dlv.PromptToken).
func (s *GdbMcpService) SetPromptToken(token string) {
	if s == nil {
		return
	}
	if token == "" {
		token = gdb.MIPromptToken
	}
	s.promptToken = token
}

// SetSession replaces the shared debugger session (e.g. after Delve restart
// to change --tty). Does not Close the previous session — caller owns lifetime.
func (s *GdbMcpService) SetSession(sess core.Session) {
	if s == nil {
		return
	}
	s.sess = sess
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
	return s.queryCapture(ctx, command, platform.PTYOwnerApp, s.captureIdle, s.captureMax)
}

// QueryLong uses relaxed PTY capture timing for large MI replies (kgdb stack list).
func (s *GdbMcpService) QueryLong(ctx context.Context, command string) (string, error) {
	return s.queryCapture(ctx, command, platform.PTYOwnerApp, stackCaptureIdle, stackCaptureMax)
}

func (s *GdbMcpService) query(ctx context.Context, command string, owner platform.PTYOwner) (string, error) {
	return s.queryCapture(ctx, command, owner, s.captureIdle, s.captureMax)
}

func (s *GdbMcpService) queryCapture(ctx context.Context, command string, owner platform.PTYOwner, idle, max time.Duration) (string, error) {
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
	tok := s.promptToken
	if tok == "" {
		tok = gdb.MIPromptToken
	}
	run := func() error {
		return s.sess.WithWrite(ctx, func(w core.PTYWriter) error {
			if err := w.Send(command); err != nil {
				return err
			}
			capture(ctx, ch, &out, idle, max, tok)
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
	core.Drain(ch, wait)
}

func capture(ctx context.Context, ch <-chan core.PtyOutputMsg, out *strings.Builder, idle, max time.Duration, promptToken string) {
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
	if promptToken == "" {
		promptToken = gdb.MIPromptToken
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
				// Prompt ends the reply — don't wait for idle timeout (was
				// 250ms per -break-list and froze the console under load).
				if strings.Contains(msg.Data, promptToken) {
					return
				}
				resetIdle()
			}
		}
	}
}
