package mcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/termforge/platform"
	"github.com/yairgd/termforge/ptyx"
)

// APIError is an error with a stable code a client can act on, such as target_running.
type APIError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e *APIError) Error() string {
	if e.Hint != "" {
		return e.Code + ": " + e.Message + " (" + e.Hint + ")"
	}
	return e.Code + ": " + e.Message
}

// ErrTargetRunning is the error for a command that needs a halted target.
func ErrTargetRunning(msg string) error {
	if msg == "" {
		msg = "the target is running"
	}
	return &APIError{Code: "target_running", Message: msg, Hint: "call interrupt first"}
}

// MIFailure turns the message of an ^error record into an error. GDB's own wording for a
// command that needs a stopped target ("Selected thread is running.", "Cannot execute this
// command while the target is running.") becomes target_running.
func MIFailure(msg string) error {
	if strings.Contains(strings.ToLower(msg), "is running") {
		return ErrTargetRunning(msg)
	}
	return errors.New(msg)
}

// tokenSeq numbers MI commands so a reply is matched to its command, not to "whatever came
// next". It starts high so it cannot collide with tokens other senders use.
var tokenSeq atomic.Uint64

func init() { tokenSeq.Store(1_000_000) }

// isMI reports whether the session speaks GDB/MI (not Delve's CLI).
func (s *GdbMcpService) isMI() bool {
	return s != nil && (s.promptToken == "" || s.promptToken == gdb.MIPromptToken)
}

// QueryToken sends an MI or CLI command with a token and waits for its result record, however
// long GDB takes (up to timeout). Unlike Query it cannot end early on an idle gap or on the
// prompt GDB prints after other output, and a late reply cannot be taken by the next command.
func (s *GdbMcpService) QueryToken(ctx context.Context, command string, timeout time.Duration) (string, error) {
	return s.queryTok(ctx, command, platform.PTYOwnerApp, timeout)
}

// CommandToken is QueryToken for commands the agent chose (shown as MCP traffic).
func (s *GdbMcpService) CommandToken(ctx context.Context, command string, timeout time.Duration) (string, error) {
	return s.queryTok(ctx, command, platform.PTYOwnerMCP, timeout)
}

func (s *GdbMcpService) queryTok(ctx context.Context, command string, owner platform.PTYOwner, timeout time.Duration) (string, error) {
	if s == nil || s.sess == nil {
		return "", errNoSession
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil
	}
	tok := strconv.FormatUint(tokenSeq.Add(1), 10)
	done := regexp.MustCompile(`(?m)^` + tok + `\^(done|error|running|connected|exit)`)
	ch, cancel := s.sess.Subscribe()
	defer cancel()

	var out strings.Builder
	var err error
	run := func() error {
		return s.sess.WithWrite(ctx, func(w ptyx.PTYWriter) error {
			if e := w.Send(tok + command); e != nil {
				return e
			}
			timer := time.NewTimer(timeout)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timer.C:
					return &APIError{Code: "timeout", Message: fmt.Sprintf("no reply from the debugger to %q within %s", command, timeout)}
				case msg, ok := <-ch:
					if !ok || msg.Err != nil {
						return errors.New("debugger session closed")
					}
					out.WriteString(msg.Data)
					if loc := done.FindStringIndex(out.String()); loc != nil && strings.Contains(out.String()[loc[0]:], "\n") {
						return nil
					}
				}
			}
		})
	}
	if s.state != nil {
		s.state.WithPTYOwner(owner, func() { err = run() })
	} else {
		err = run()
	}
	raw := out.String()
	if err == nil && owner == platform.PTYOwnerMCP && s.OnBreakpointsChanged != nil &&
		(strings.Contains(raw, "=breakpoint-created") || strings.Contains(raw, "=breakpoint-deleted")) {
		s.OnBreakpointsChanged()
	}
	return raw, err
}

// Tracker returns the run-state tracker, starting it on the first call (so a session nobody
// calls through the API never pays for it). nil for Delve, whose state comes from rpc2.
func (s *GdbMcpService) Tracker(ctx context.Context) *gdb.RunTracker {
	if !s.isMI() || s.sess == nil {
		return nil
	}
	s.trMu.Lock()
	defer s.trMu.Unlock()
	if s.tracker == nil {
		s.tracker = gdb.NewRunTracker(s.sess)
	}
	st := s.tracker.State()
	if st == "" || st == gdb.StateDisconnectedName || st == gdb.StateStoppedName {
		// Ask GDB when the records may have been missed (before the subscription, or a connect
		// whose records were not seen), and for the frame of a stop that came without one.
		if _, stop := s.tracker.Snapshot(); st != gdb.StateStoppedName || (stop.Addr == "" && stop.Func == "") {
			if raw, err := s.queryTok(ctx, "-thread-info", platform.PTYOwnerApp, 3*time.Second); err == nil {
				ts, loc := gdb.ParseThreadInfo(raw)
				s.tracker.Seed(ts, loc)
				s.tracker.FillLocation(loc)
			}
		}
	}
	return s.tracker
}

// Endian is "little" or "big", from GDB's `show endian`; little if it cannot tell.
func (s *GdbMcpService) Endian(ctx context.Context) string {
	s.trMu.Lock()
	e := s.endian
	s.trMu.Unlock()
	if e != "" {
		return e
	}
	raw, err := s.queryTok(ctx, "show endian", platform.PTYOwnerApp, 3*time.Second)
	if err != nil {
		return "little"
	}
	text := parse.MIConsoleText(raw)
	switch {
	case strings.Contains(text, "big endian"):
		e = "big"
	case strings.Contains(text, "little endian"):
		e = "little"
	default:
		return "little"
	}
	s.trMu.Lock()
	s.endian = e
	s.trMu.Unlock()
	return e
}
