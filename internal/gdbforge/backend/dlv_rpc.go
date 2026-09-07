package backend

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-delve/delve/service/api"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
)

func (b *DLVBackend) rpcClient() *dlv.Client {
	c := b.client()
	if c == nil || c.RPC == nil {
		return nil
	}
	return c
}

func (b *DLVBackend) FetchBreakpoints(ctx context.Context, q Querier) ([]models.BreakInfo, bool) {
	if c := b.rpcClient(); c != nil {
		bps, err := c.RPC.ListBreakpoints(false)
		if err != nil {
			return nil, false
		}
		return dlv.BreakpointsFromAPI(bps), true
	}
	if q == nil {
		return nil, false
	}
	raw, err := q.Query(ctx, "breakpoints")
	if err != nil {
		return nil, false
	}
	items := dlv.ParseBreakpoints(raw)
	low := strings.ToLower(raw)
	if len(items) == 0 {
		if strings.Contains(low, "no breakpoints") {
			return items, true
		}
		if strings.Contains(low, "breakpoint") {
			return nil, false
		}
		return nil, false
	}
	return items, true
}

func (b *DLVBackend) RefreshThreadsAndStack(ctx context.Context, q Querier, log LogFn) ([]models.ThreadInfo, []models.StackFrame, bool, bool) {
	if c := b.rpcClient(); c != nil {
		return b.refreshThreadsAndStackRPC(c, log)
	}
	return ThreadsAndStack(ctx, DLV, q, log)
}

func (b *DLVBackend) refreshThreadsAndStackRPC(c *dlv.Client, log LogFn) ([]models.ThreadInfo, []models.StackFrame, bool, bool) {
	st, err := c.RPC.GetState()
	if err != nil {
		if log != nil {
			log("threads", err.Error())
		}
		return nil, nil, false, false
	}
	gid := dlv.CurrentGoroutineID(st)
	gs, _, err := c.RPC.ListGoroutines(0, 256)
	if err != nil {
		if log != nil {
			log("threads", err.Error())
		}
		return nil, nil, false, false
	}
	if len(gs) == 0 {
		return nil, nil, false, false
	}
	threads := dlv.GoroutinesFromAPI(gs, gid)
	var frames []models.StackFrame
	var stackOK bool
	if gid != 0 {
		if stack, err := c.RPC.Stacktrace(gid, 50, 0, api.StacktraceSimple, nil); err == nil {
			frames = dlv.StackFromAPI(stack)
			stackOK = len(frames) > 0
		} else if log != nil {
			log("callstack", err.Error())
		}
	}
	return threads, frames, true, stackOK
}

func (b *DLVBackend) CurrentFrame(ctx context.Context, q Querier) (models.StackFrame, bool) {
	if c := b.rpcClient(); c != nil {
		st, err := c.RPC.GetState()
		if err != nil {
			return models.StackFrame{}, false
		}
		gid := dlv.CurrentGoroutineID(st)
		if gid == 0 {
			return models.StackFrame{}, false
		}
		stack, err := c.RPC.Stacktrace(gid, 1, 0, api.StacktraceSimple, nil)
		if err != nil || len(stack) == 0 {
			return models.StackFrame{}, false
		}
		frames := dlv.StackFromAPI(stack)
		if len(frames) == 0 {
			return models.StackFrame{}, false
		}
		return frames[0], true
	}
	if q == nil {
		return models.StackFrame{}, false
	}
	raw, err := q.Query(ctx, "stack 1")
	if err != nil {
		return models.StackFrame{}, false
	}
	return dlv.ParseStackInfoFrame(raw)
}

func (b *DLVBackend) FetchStackList(ctx context.Context, q Querier, _ bool) ([]models.StackFrame, bool) {
	if c := b.rpcClient(); c != nil {
		st, err := c.RPC.GetState()
		if err != nil {
			return nil, false
		}
		gid := dlv.CurrentGoroutineID(st)
		if gid == 0 {
			return nil, false
		}
		stack, err := c.RPC.Stacktrace(gid, 50, 0, api.StacktraceSimple, nil)
		if err != nil {
			return nil, false
		}
		frames := dlv.StackFromAPI(stack)
		return frames, len(frames) > 0
	}
	return StackList(ctx, DLV, q, false, nil)
}

func (b *DLVBackend) ListSourceFiles(_ context.Context, _ Querier) ([]string, bool) {
	c := b.rpcClient()
	if c == nil {
		return nil, false
	}
	files, err := c.RPC.ListSources("")
	if err != nil || len(files) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		f = dlv.ResolveSourcePath(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out, len(out) > 0
}

func (b *DLVBackend) SupportsSourceFileList() bool {
	return b.rpcClient() != nil
}

func (b *DLVBackend) execViaRPC(env CommandEnv, cmd string, ui bool) bool {
	c := b.rpcClient()
	if c == nil {
		return false
	}
	send, marksRunning := b.MapExec(cmd)
	if marksRunning {
		if rs, ok := env.Inferior.(interface{ SetInferiorRunning(bool) }); ok {
			rs.SetInferiorRunning(true)
		}
	}
	run := func() { b.runRPCExec(c, send) }
	if ui && env.App != nil {
		env.App.WithPTYOwner(platform.PTYOwnerUI, run)
	} else {
		run()
	}
	return true
}

func (b *DLVBackend) runRPCExec(c *dlv.Client, cmd string) {
	switch strings.TrimSpace(cmd) {
	case "continue":
		go func() {
			ch := c.RPC.Continue()
			if ch != nil {
				<-ch
			}
		}()
	case "next":
		go func() { _, _ = c.RPC.Next() }()
	case "step":
		go func() { _, _ = c.RPC.Step() }()
	case "stepout":
		go func() { _, _ = c.RPC.StepOut() }()
	case "nexti":
		go func() { _, _ = c.RPC.StepInstruction(false) }()
	case "stepi":
		go func() { _, _ = c.RPC.StepInstruction(true) }()
	case "restart":
		go func() { _, _ = c.RPC.Restart(false) }()
	default:
		_ = b.SendLine(cmd)
	}
}

func (b *DLVBackend) insertBreakpointRPC(file string, line int) bool {
	c := b.rpcClient()
	if c == nil {
		return false
	}
	_, err := c.RPC.CreateBreakpoint(&api.Breakpoint{File: file, Line: line})
	return err == nil
}

func (b *DLVBackend) insertBreakpointAddrRPC(addr string) bool {
	c := b.rpcClient()
	if c == nil || addr == "" {
		return false
	}
	loc := "*" + models.NormalizeAddr(addr)
	locs, _, err := c.RPC.FindLocation(api.EvalScope{}, loc, false, nil)
	if err != nil || len(locs) == 0 {
		return false
	}
	bp := &api.Breakpoint{File: locs[0].File, Line: locs[0].Line}
	if locs[0].PC != 0 {
		bp.Addrs = []uint64{locs[0].PC}
	}
	_, err = c.RPC.CreateBreakpoint(bp)
	return err == nil
}

func (b *DLVBackend) clearBreakpointRPC(file string, line int, addr string, number int) bool {
	c := b.rpcClient()
	if c == nil {
		return false
	}
	if number > 0 {
		_, err := c.RPC.ClearBreakpoint(number)
		return err == nil
	}
	bps, err := c.RPC.ListBreakpoints(false)
	if err != nil {
		return false
	}
	wantAddr := models.NormalizeAddr(addr)
	for _, bp := range bps {
		if bp == nil {
			continue
		}
		if file != "" && line > 0 && models.SameSourcePath(bp.File, file) && bp.Line == line {
			_, err := c.RPC.ClearBreakpoint(bp.ID)
			return err == nil
		}
		if wantAddr != "" {
			for _, a := range bp.Addrs {
				if models.NormalizeAddr(fmt.Sprintf("0x%x", a)) == wantAddr {
					_, err := c.RPC.ClearBreakpoint(bp.ID)
					return err == nil
				}
			}
		}
	}
	return false
}

func (b *DLVBackend) disableBreakpointRPC(number int) bool {
	c := b.rpcClient()
	if c == nil || number < 1 {
		return false
	}
	bp, err := c.RPC.GetBreakpoint(number)
	if err != nil || bp == nil || bp.Disabled {
		return err == nil
	}
	_, err = c.RPC.ToggleBreakpoint(number)
	return err == nil
}

func (b *DLVBackend) setBreakpointConditionRPC(number int, cond string) bool {
	c := b.rpcClient()
	if c == nil || number < 1 || strings.TrimSpace(cond) == "" {
		return false
	}
	bp, err := c.RPC.GetBreakpoint(number)
	if err != nil || bp == nil {
		return false
	}
	bp.Cond = cond
	return c.RPC.AmendBreakpoint(bp) == nil
}

func (b *DLVBackend) insertDefaultBreakMainRPC() bool {
	c := b.rpcClient()
	if c == nil {
		return false
	}
	_, err := c.RPC.CreateBreakpoint(&api.Breakpoint{
		FunctionName: "main.main",
	})
	return err == nil
}

func (b *DLVBackend) selectGoroutineRPC(id string) bool {
	c := b.rpcClient()
	if c == nil || id == "" {
		return false
	}
	gid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return false
	}
	_, err = c.RPC.SwitchGoroutine(gid)
	return err == nil
}
