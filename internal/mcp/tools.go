package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yairgd/gdbforge/internal/gdbforge/domain"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/termforge/service"
)

// SetDomain attaches the app-owned domain surface (optional until wired).
func (s *GdbMcpService) SetDomain(d domain.DebugDomain) {
	if s == nil {
		return
	}
	s.domain = d
}

var errNoSession = errors.New("no debug session")

// Services is the debugger tool registry: the :AI agent and the -serve endpoint (REST and
// MCP) call these. get returns the current session service, which changes when the session
// restarts; gdb_command remains the escape hatch.
func Services(get func() *GdbMcpService) *service.Registry {
	r := &service.Registry{}
	fileLine := []service.Param{
		{Name: "file", Type: service.String, Help: "Source path or basename"},
		{Name: "line", Type: service.Integer, Help: "1-based line"},
	}
	// dom runs fn with the session's domain, or fails if there is none yet.
	dom := func(fn func(d domain.DebugDomain, a service.Args) (any, error)) func(context.Context, service.Args) (any, error) {
		return func(_ context.Context, a service.Args) (any, error) {
			s := get()
			if s == nil || s.domain == nil {
				return nil, errNoSession
			}
			return fn(s.domain, a)
		}
	}
	for _, sv := range []service.Service{
		{Name: "list_breakpoints", Remote: true,
			Help: "List breakpoints and watchpoints: number, type, location, enabled, hit count, temporary (tbreak) and condition. Read from the debugger, so hit counts are current.",
			Run: func(ctx context.Context, _ service.Args) (any, error) {
				s := get()
				if s == nil {
					return nil, errNoSession
				}
				if s.isMI() && s.sess != nil {
					raw, err := s.QueryToken(ctx, "-break-list", 10*time.Second)
					if err != nil {
						return nil, err
					}
					if msg, ok := parse.MIError(raw); ok {
						return nil, MIFailure(msg)
					}
					return breakRows(parse.ParseBreakListAll(raw)), nil
				}
				if s.domain == nil {
					return nil, errNoSession
				}
				return s.domain.ListBreakpoints(), nil
			}},
		{Name: "list_threads", Remote: true, Help: "List threads from the shared model (same as the Threads pane).",
			Run: dom(func(d domain.DebugDomain, _ service.Args) (any, error) { return d.ListThreads(), nil })},
		{Name: "list_frames", Remote: true, Help: "List call-stack frames from the shared model (same as the Call Stack pane).",
			Run: dom(func(d domain.DebugDomain, _ service.Args) (any, error) { return d.ListFrames(), nil })},
		{Name: "set_breakpoint", Remote: true, Help: "Set a breakpoint at file:line using the same path as the GUI (Space).", Params: fileLine,
			Run: dom(func(d domain.DebugDomain, a service.Args) (any, error) {
				file, line := a.String("file"), a.Int("line")
				if err := d.SetBreakpoint(file, line); err != nil {
					return nil, err
				}
				return fmt.Sprintf("ok: breakpoint set at %s:%d", file, line), nil
			})},
		{Name: "clear_breakpoint", Remote: true, Help: "Clear a breakpoint at file:line using the same path as the GUI (Space).", Params: fileLine,
			Run: dom(func(d domain.DebugDomain, a service.Args) (any, error) {
				file, line := a.String("file"), a.Int("line")
				if err := d.ClearBreakpoint(file, line); err != nil {
					return nil, err
				}
				return fmt.Sprintf("ok: breakpoint cleared at %s:%d", file, line), nil
			})},
		{Name: "gdb_command", Remote: true,
			Help: "Run a raw GDB/MI, GDB CLI or Delve command when no other tool fits (print, info, monitor, …). With GDB the reply is the console text only (format console, the default), the raw MI records (mi) or both. A command that needs a stopped target while it runs fails with target_running. timeout_ms (default 10000) is how long to wait for the reply.",
			Params: []service.Param{{Name: "command", Type: service.String, Help: "Debugger command line"},
				{Name: "format", Type: service.String, Help: "console (default) | mi | both", Optional: true},
				{Name: "timeout_ms", Type: service.Integer, Help: "wait for the reply, default 10000, max 300000", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				s := get()
				if s == nil {
					return nil, errNoSession
				}
				if !s.isMI() { // Delve: its CLI has no tokens or MI records
					out, err := s.GdbCommand(ctx, a.String("command"))
					if err != nil {
						return nil, err
					}
					if out == "" {
						return "(no output)", nil
					}
					return out, nil
				}
				timeout := 10 * time.Second
				if ms := a.Int("timeout_ms"); ms > 0 {
					timeout = min(time.Duration(ms)*time.Millisecond, 300*time.Second)
				}
				raw, err := s.CommandToken(ctx, a.String("command"), timeout)
				if err != nil {
					return nil, err
				}
				return FormatReply(raw, a.String("format"))
			}},
	} {
		r.Add(sv)
	}
	return r
}

// runTool dispatches an LLM tool by name through the registry; the result is text for the model.
func (s *GdbMcpService) runTool(ctx context.Context, name string, input json.RawMessage) string {
	if s == nil {
		return "error: no service"
	}
	var args service.Args
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return "error: " + err.Error()
		}
	}
	res, err := Services(func() *GdbMcpService { return s }).Call(ctx, name, args)
	if err != nil {
		return "error: " + err.Error()
	}
	if str, ok := res.(string); ok {
		return str
	}
	b, err := json.Marshal(res)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}

// BreakRow is one breakpoint or watchpoint as list_breakpoints reports it.
type BreakRow struct {
	Number    int    `json:"number"`
	Type      string `json:"type"`
	Location  string `json:"location"`
	Enabled   bool   `json:"enabled"`
	Hits      int    `json:"hits"`
	Temporary bool   `json:"temporary"`
	Condition string `json:"condition,omitempty"`
}

func breakRows(items []models.BreakInfo) []BreakRow {
	rows := make([]BreakRow, 0, len(items))
	for _, it := range items {
		loc := it.What
		switch {
		case it.File != "" && it.Line > 0:
			loc = fmt.Sprintf("%s:%d", it.File, it.Line)
		case loc == "":
			loc = it.Addr
		}
		rows = append(rows, BreakRow{Number: it.Number, Type: it.Type, Location: loc, Enabled: it.Enabled,
			Hits: it.Hits, Temporary: it.Temporary, Condition: it.Condition})
	}
	return rows
}
