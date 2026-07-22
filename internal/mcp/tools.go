package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yairgd/gdbforge/internal/gdbforge/domain"
)

// SetDomain attaches the app-owned domain surface (optional until wired).
func (s *GdbMcpService) SetDomain(d domain.DebugDomain) {
	if s == nil {
		return
	}
	s.domain = d
}

// runTool dispatches an LLM tool by name. gdb_command remains the escape hatch.
func (s *GdbMcpService) runTool(ctx context.Context, name string, input json.RawMessage) string {
	if s == nil {
		return "error: no service"
	}
	switch name {
	case "list_breakpoints":
		return s.toolListJSON(func() any {
			if s.domain == nil {
				return []domain.Breakpoint{}
			}
			return s.domain.ListBreakpoints()
		})
	case "list_threads":
		return s.toolListJSON(func() any {
			if s.domain == nil {
				return []domain.Thread{}
			}
			return s.domain.ListThreads()
		})
	case "list_frames":
		return s.toolListJSON(func() any {
			if s.domain == nil {
				return []domain.Frame{}
			}
			return s.domain.ListFrames()
		})
	case "set_breakpoint":
		var in struct {
			File string `json:"file"`
			Line int    `json:"line"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return "error: " + err.Error()
		}
		if s.domain == nil {
			return "error: domain not wired"
		}
		if err := s.domain.SetBreakpoint(in.File, in.Line); err != nil {
			return "error: " + err.Error()
		}
		return fmt.Sprintf("ok: breakpoint set at %s:%d", in.File, in.Line)
	case "clear_breakpoint":
		var in struct {
			File string `json:"file"`
			Line int    `json:"line"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return "error: " + err.Error()
		}
		if s.domain == nil {
			return "error: domain not wired"
		}
		if err := s.domain.ClearBreakpoint(in.File, in.Line); err != nil {
			return "error: " + err.Error()
		}
		return fmt.Sprintf("ok: breakpoint cleared at %s:%d", in.File, in.Line)
	case "gdb_command":
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return "error: " + err.Error()
		}
		out, err := s.GdbCommand(ctx, in.Command)
		if err != nil {
			return "error: " + err.Error()
		}
		if out == "" {
			return "(no output)"
		}
		return out
	default:
		return "error: unknown tool " + name
	}
}

func (s *GdbMcpService) toolListJSON(fn func() any) string {
	b, err := json.Marshal(fn())
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}
