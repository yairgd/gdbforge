package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/domain"
)

type fakeDomain struct {
	bps     []domain.Breakpoint
	threads []domain.Thread
	frames  []domain.Frame
	set     []string
	clear   []string
}

func (f *fakeDomain) ListBreakpoints() []domain.Breakpoint { return f.bps }
func (f *fakeDomain) ListThreads() []domain.Thread         { return f.threads }
func (f *fakeDomain) ListFrames() []domain.Frame           { return f.frames }
func (f *fakeDomain) SetBreakpoint(file string, line int) error {
	f.set = append(f.set, file)
	f.bps = append(f.bps, domain.Breakpoint{File: file, Line: line, Enabled: true})
	return nil
}
func (f *fakeDomain) ClearBreakpoint(file string, line int) error {
	f.clear = append(f.clear, file)
	return nil
}

func TestRunToolDomainReadsAndWrites(t *testing.T) {
	dom := &fakeDomain{
		bps:     []domain.Breakpoint{{Number: 1, File: "a.c", Line: 10, Enabled: true}},
		threads: []domain.Thread{{ID: "1", State: "stopped"}},
		frames:  []domain.Frame{{Level: 0, Func: "main", File: "a.c", Line: 10}},
	}
	s := NewGdbMcpService(nil, nil)
	s.SetDomain(dom)
	ctx := context.Background()

	out := s.runTool(ctx, "list_breakpoints", json.RawMessage(`{}`))
	if !strings.Contains(out, "a.c") {
		t.Fatalf("list_breakpoints=%s", out)
	}
	out = s.runTool(ctx, "list_threads", nil)
	if !strings.Contains(out, "stopped") {
		t.Fatalf("list_threads=%s", out)
	}
	out = s.runTool(ctx, "list_frames", nil)
	if !strings.Contains(out, "main") {
		t.Fatalf("list_frames=%s", out)
	}

	out = s.runTool(ctx, "set_breakpoint", json.RawMessage(`{"file":"b.c","line":20}`))
	if !strings.HasPrefix(out, "ok:") || len(dom.set) != 1 {
		t.Fatalf("set_breakpoint out=%q set=%v", out, dom.set)
	}
	out = s.runTool(ctx, "clear_breakpoint", json.RawMessage(`{"file":"b.c","line":20}`))
	if !strings.HasPrefix(out, "ok:") || len(dom.clear) != 1 {
		t.Fatalf("clear_breakpoint out=%q clear=%v", out, dom.clear)
	}
}

func TestRunToolUnknown(t *testing.T) {
	s := NewGdbMcpService(nil, nil)
	out := s.runTool(context.Background(), "nope", nil)
	if !strings.Contains(out, "unknown tool") {
		t.Fatalf("got %q", out)
	}
}
