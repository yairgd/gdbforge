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
	if d.app == nil || d.app.breaks.List() == nil {
		return nil
	}
	items := d.app.breaks.Items()
	out := make([]domain.Breakpoint, len(items))
	for i, it := range items {
		out[i] = domain.Breakpoint{
			Number:    it.Number,
			Enabled:   it.Enabled,
			Condition: it.Condition,
			File:      it.File,
			Line:      it.Line,
		}
	}
	return out
}

func (d appDebugDomain) ListThreads() []domain.Thread {
	if d.app == nil || d.app.debugInfo.Threads() == nil {
		return nil
	}
	items := d.app.debugInfo.Threads().Items()
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
	if d.app == nil || d.app.debugInfo.Stack() == nil {
		return nil
	}
	items := d.app.debugInfo.Stack().Items()
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
	list := d.app.breaks.List()
	if list == nil {
		return fmt.Errorf("no breakpoint model")
	}
	if list.HasEnabledAt(file, line) {
		return nil
	}
	cmd, ok := list.ToggleInsertClear(file, line)
	if !ok {
		return fmt.Errorf("could not set breakpoint")
	}
	d.app.breaks.syncBreakpointViews()
	d.app.breaks.sendBreakpointCmd(cmd)
	return nil
}

func (d appDebugDomain) ClearBreakpoint(file string, line int) error {
	if d.app == nil {
		return fmt.Errorf("no app")
	}
	if file == "" || line < 1 {
		return fmt.Errorf("file and line required")
	}
	list := d.app.breaks.List()
	if list == nil {
		return fmt.Errorf("no breakpoint model")
	}
	if !list.HasEnabledAt(file, line) {
		return nil
	}
	cmd, ok := list.ToggleInsertClear(file, line)
	if !ok {
		return fmt.Errorf("could not clear breakpoint")
	}
	d.app.breaks.syncBreakpointViews()
	d.app.breaks.sendBreakpointCmd(cmd)
	return nil
}

var _ domain.DebugDomain = appDebugDomain{}
