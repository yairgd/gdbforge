package main

import (
	"fmt"

	"github.com/yairgd/gdbforge/internal/gdbforge/domain"
)

// appDebugDomain exposes DebuggerApp shared models to peer controllers (AI).
// A future Lua controller can bind the same domain.DebugDomain methods.
type appDebugDomain struct {
	app *DebuggerApp
}

func (d appDebugDomain) ListBreakpoints() []domain.Breakpoint {
	if d.app == nil || d.app.breakpoints == nil {
		return nil
	}
	items := d.app.breakpoints.Items()
	out := make([]domain.Breakpoint, len(items))
	for i, it := range items {
		out[i] = domain.Breakpoint{
			Number:  it.Number,
			Enabled: it.Enabled,
			File:    it.File,
			Line:    it.Line,
		}
	}
	return out
}

func (d appDebugDomain) ListThreads() []domain.Thread {
	if d.app == nil || d.app.threads == nil {
		return nil
	}
	items := d.app.threads.Items()
	out := make([]domain.Thread, len(items))
	for i, it := range items {
		out[i] = domain.Thread{
			ID:      it.ID,
			State:   it.State,
			Name:    it.Name,
			File:    it.File,
			Line:    it.Line,
			Func:    it.Func,
			Current: it.Current,
		}
	}
	return out
}

func (d appDebugDomain) ListFrames() []domain.Frame {
	if d.app == nil || d.app.callstack == nil {
		return nil
	}
	items := d.app.callstack.Items()
	out := make([]domain.Frame, len(items))
	for i, it := range items {
		out[i] = domain.Frame{
			Level: it.Level,
			Func:  it.Func,
			File:  it.File,
			Line:  it.Line,
			Addr:  it.Addr,
		}
	}
	return out
}

func (d appDebugDomain) SetBreakpoint(file string, line int) error {
	if d.app == nil {
		return fmt.Errorf("no app")
	}
	if file == "" || line < 1 {
		return fmt.Errorf("file and line required")
	}
	if d.app.breakpoints == nil {
		return fmt.Errorf("no breakpoint model")
	}
	if d.app.breakpoints.HasEnabledAt(file, line) {
		return nil
	}
	cmd, ok := d.app.breakpoints.ToggleInsertClear(file, line)
	if !ok {
		return fmt.Errorf("could not set breakpoint")
	}
	d.app.syncBreakpointViews()
	d.app.sendBreakpointCmd(cmd)
	return nil
}

func (d appDebugDomain) ClearBreakpoint(file string, line int) error {
	if d.app == nil {
		return fmt.Errorf("no app")
	}
	if file == "" || line < 1 {
		return fmt.Errorf("file and line required")
	}
	if d.app.breakpoints == nil {
		return fmt.Errorf("no breakpoint model")
	}
	if !d.app.breakpoints.HasEnabledAt(file, line) {
		return nil
	}
	cmd, ok := d.app.breakpoints.ToggleInsertClear(file, line)
	if !ok {
		return fmt.Errorf("could not clear breakpoint")
	}
	d.app.syncBreakpointViews()
	d.app.sendBreakpointCmd(cmd)
	return nil
}

var _ domain.DebugDomain = appDebugDomain{}
