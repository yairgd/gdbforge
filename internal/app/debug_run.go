package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/termforge/service"
)

const (
	defaultInterruptWait = 3 * time.Second
	defaultStepWait      = 10 * time.Second
	maxWait              = 10 * time.Minute
	cmdReplyWait         = 10 * time.Second
)

var execActions = map[string]bool{"continue": true, "next": true, "step": true, "finish": true,
	"stepi": true, "nexti": true, "run": true, "start": true}

// batchForbidden are the tools a batch cannot contain: a batch is one halted snapshot, and
// these change whether the target runs.
var batchForbidden = map[string]bool{"run_control": true, "interrupt": true, "continue": true, "step": true,
	"next": true, "stepi": true, "finish": true, "run_to": true, "wait_for_stop": true, "batch": true}

var tempBkptRe = regexp.MustCompile(`bkpt=\{number="(\d+)"`)

func notSupported(what string) error {
	return &mcp.APIError{Code: "not_supported", Message: what + " is only available with GDB"}
}

// gdbTracker returns the GDB session and its run-state tracker, or nils for Delve or no session.
func (c *luaCtl) gdbTracker(ctx context.Context) (*mcp.GdbMcpService, *gdb.RunTracker) {
	m := c.host.GdbMcp()
	if m == nil {
		return nil, nil
	}
	if b := c.host.Backend(); b != nil && b.Kind() != backend.GDB {
		return nil, nil
	}
	return m, m.Tracker(ctx)
}

// stateFields is the run state that every REST reply carries: state is "running", "stopped"
// or "disconnected"; when stopped it also says why and where.
func (c *luaCtl) stateFields(ctx context.Context) map[string]any {
	f := map[string]any{"state": gdb.StateDisconnectedName}
	if c.host.GdbMcp() == nil {
		return f
	}
	if _, tr := c.gdbTracker(ctx); tr != nil {
		state, stop := tr.Snapshot()
		if state == "" {
			state = "unknown"
		}
		f["state"] = state
		if state == gdb.StateStoppedName || state == gdb.StateDisconnectedName {
			if stop.Reason != "" {
				f["stop_reason"] = stop.Reason
			}
			if state == gdb.StateStoppedName {
				if stop.Addr != "" {
					f["pc"] = stop.Addr
				}
				if stop.Func != "" {
					f["func"] = stop.Func
				}
				if stop.File != "" {
					f["file"] = stop.File
				}
				if stop.Line > 0 {
					f["line"] = stop.Line
				}
				if stop.BkptNo > 0 {
					f["bkpt_number"] = stop.BkptNo
				}
			}
		}
		return f
	}
	if si, err := c.stopState(ctx); err == nil { // Delve
		f["state"] = si.State
		if si.File != "" {
			f["file"], f["line"] = si.File, si.Line
		}
	}
	return f
}

// runControl is run_control: any run-control action, then wait up to wait for the stop.
func (c *luaCtl) runControl(ctx context.Context, action string, wait time.Duration) (any, error) {
	if m, tr := c.gdbTracker(ctx); tr != nil {
		if action == "interrupt" {
			return c.gdbInterrupt(ctx, m, tr, wait)
		}
		if !execActions[action] {
			return nil, fmt.Errorf("unknown action %q (continue, next, step, finish, stepi, nexti, run, start, interrupt)", action)
		}
		return c.gdbResume(ctx, m, tr, action, wait)
	}
	var err error
	if e := c.onUI(ctx, func() { err = c.host.RunControl(action) }); e != nil {
		return nil, e
	}
	if err != nil {
		return nil, err
	}
	return c.waitStopPoll(ctx, wait)
}

// waitStopPoll waits for a stop by polling the run state (Delve).
func (c *luaCtl) waitStopPoll(ctx context.Context, wait time.Duration) (stopInfo, error) {
	deadline := time.Now().Add(wait)
	for {
		si, err := c.stopState(ctx)
		if err != nil || si.State == "stopped" || !time.Now().Before(deadline) {
			return si, err
		}
		select {
		case <-ctx.Done():
			return si, ctx.Err()
		case <-time.After(stopPoll):
		}
	}
}

// gdbResume sends a run-control action and waits up to wait for the stop (0: do not wait).
func (c *luaCtl) gdbResume(ctx context.Context, m *mcp.GdbMcpService, tr *gdb.RunTracker, action string, wait time.Duration) (map[string]any, error) {
	if tr.State() == gdb.StateRunningName {
		if action == "continue" { // what was asked for is already the case: just wait for the stop
			if wait > 0 {
				tr.WaitStop(ctx, tr.Seq(), wait)
			}
			f := c.stateFields(ctx)
			f["note"] = "the target was already running"
			return f, nil
		}
		return nil, &mcp.APIError{Code: "target_running", Message: "the target is already running",
			Hint: "call wait_for_stop to wait for it, or interrupt to halt it"}
	}
	send := action
	if b := c.host.Backend(); b != nil {
		send, _ = b.MapExec(action)
	}
	seq := tr.Seq()
	raw, err := m.CommandToken(ctx, send, cmdReplyWait)
	if err != nil {
		return nil, err
	}
	if msg, ok := parse.MIError(raw); ok {
		return nil, mcp.MIFailure(msg)
	}
	if strings.Contains(raw, "^running") {
		tr.MarkRunning(seq)
	}
	if wait > 0 {
		tr.WaitStop(ctx, seq, wait)
	}
	return c.stateFields(ctx), nil
}

// gdbInterrupt halts the target and waits for the stop, sending -exec-interrupt a second time
// if the first did not take. A target that is already stopped just reports where.
func (c *luaCtl) gdbInterrupt(ctx context.Context, m *mcp.GdbMcpService, tr *gdb.RunTracker, wait time.Duration) (map[string]any, error) {
	if tr.State() != gdb.StateRunningName {
		return c.stateFields(ctx), nil
	}
	half := max(wait/2, 250*time.Millisecond)
	for range 2 {
		seq := tr.Seq()
		// The reply only says the request was accepted; the stop is what counts. An error
		// here usually means the target stopped on its own a moment ago.
		_, _ = m.CommandToken(ctx, gdb.MIExecInterrupt, 3*time.Second)
		if _, ok := tr.WaitStop(ctx, seq, half); ok || tr.State() != gdb.StateRunningName {
			return c.stateFields(ctx), nil
		}
	}
	return nil, &mcp.APIError{Code: "interrupt_timeout", Message: fmt.Sprintf("the target did not stop within %s", wait),
		Hint: "state is still running; check the connection to the probe or gdbserver"}
}

// waitForStop waits until the target is stopped; an already stopped target returns at once.
func (c *luaCtl) waitForStop(ctx context.Context, wait time.Duration) (any, error) {
	if _, tr := c.gdbTracker(ctx); tr != nil {
		if tr.State() == gdb.StateRunningName {
			tr.WaitStop(ctx, tr.Seq(), wait)
		}
		return c.stateFields(ctx), nil
	}
	return c.waitStopPoll(ctx, wait)
}

// runTo runs to a location: a temporary breakpoint, continue, and wait, in one call.
func (c *luaCtl) runTo(ctx context.Context, location string, wait time.Duration) (any, error) {
	m, tr := c.gdbTracker(ctx)
	if tr == nil {
		return nil, notSupported("run_to")
	}
	if tr.State() == gdb.StateRunningName {
		return nil, mcp.ErrTargetRunning("run_to needs a stopped target")
	}
	loc := location
	if strings.ContainsAny(loc, " \t") {
		loc = miQuote(loc)
	}
	raw, err := m.CommandToken(ctx, "-break-insert -t "+loc, cmdReplyWait)
	if err != nil {
		return nil, err
	}
	if msg, ok := parse.MIError(raw); ok {
		return nil, mcp.MIFailure(msg)
	}
	mm := tempBkptRe.FindStringSubmatch(raw)
	if mm == nil {
		return nil, fmt.Errorf("run_to: could not set a breakpoint at %q", location)
	}
	res, err := c.gdbResume(ctx, m, tr, "continue", wait)
	if err != nil {
		return nil, err
	}
	res["temp_breakpoint"] = mm[1]
	_, stop := tr.Snapshot()
	reached := res["state"] == gdb.StateStoppedName && fmt.Sprint(stop.BkptNo) == mm[1]
	res["reached"] = reached
	if res["state"] == gdb.StateStoppedName && !reached { // stopped elsewhere: the temporary breakpoint is still armed
		_, _ = m.QueryToken(ctx, "-break-delete "+mm[1], 3*time.Second)
	}
	return res, nil
}

// batch runs read tools in order on one halted snapshot. With halt, a running target is
// interrupted first; with resume, it is continued afterwards, but only if this call halted it
// and nothing else (a breakpoint) stopped it, so a real stop is never swallowed.
func (c *luaCtl) batch(ctx context.Context, ops []any, halt, resume bool) (any, error) {
	m, tr := c.gdbTracker(ctx)
	if tr == nil {
		return nil, notSupported("batch")
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("batch: ops is empty")
	}
	type op struct {
		tool string
		args service.Args
	}
	list := make([]op, 0, len(ops))
	for i, o := range ops {
		om, _ := o.(map[string]any)
		tool, _ := om["tool"].(string)
		if tool == "" {
			return nil, fmt.Errorf("batch: ops[%d] needs a \"tool\"", i)
		}
		if batchForbidden[tool] {
			return nil, fmt.Errorf("batch: ops[%d]: %s is not allowed in a batch", i, tool)
		}
		args, _ := om["args"].(map[string]any)
		list = append(list, op{tool, args})
	}

	halted := false
	if halt && tr.State() == gdb.StateRunningName {
		if _, err := c.gdbInterrupt(ctx, m, tr, defaultInterruptWait); err != nil {
			return nil, err
		}
		_, stop := tr.Snapshot()
		halted = stop.BkptNo == 0 && !strings.Contains(stop.Reason, "breakpoint") && !strings.Contains(stop.Reason, "watchpoint")
	}
	results := make([]map[string]any, 0, len(list))
	for _, o := range list {
		r := map[string]any{"tool": o.tool}
		res, err := c.services().Call(ctx, o.tool, o.args)
		if err != nil {
			r["error"] = err.Error()
		} else {
			r["result"] = res
		}
		results = append(results, r)
	}
	out := map[string]any{"results": results, "halted": halted, "resumed": false}
	if resume && halted {
		if tr.State() == gdb.StateStoppedName {
			if _, err := c.gdbResume(ctx, m, tr, "continue", 0); err != nil {
				out["resume_error"] = err.Error()
			} else {
				out["resumed"] = true
			}
		}
	} else if resume && halt && tr.State() == gdb.StateStoppedName {
		out["note"] = "left stopped: it was stopped by a breakpoint or was not running"
	}
	return out, nil
}

// addRunServices registers the blocking run-control tools.
func (c *luaCtl) addRunServices(r *service.Registry) {
	ms := func(a service.Args, name string, def time.Duration) time.Duration {
		if v, ok := a[name].(float64); ok && v >= 0 {
			return min(time.Duration(v*float64(time.Millisecond)), maxWait)
		}
		return def
	}
	step := func(name, help string) service.Service {
		return service.Service{Name: name, Remote: true, Help: help,
			Params: []service.Param{{Name: "timeout_ms", Type: service.Integer, Help: "wait for the stop, default 10000", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				return c.runControl(ctx, name, ms(a, "timeout_ms", defaultStepWait))
			}}
	}
	for _, s := range []service.Service{
		{Name: "interrupt", Remote: true,
			Help:   "Halt the running target and wait until it has stopped (default 3000 ms, retried once). Returns the stop location. If the target is already stopped it only reports where. Fails with interrupt_timeout if it will not stop.",
			Params: []service.Param{{Name: "timeout_ms", Type: service.Integer, Help: "wait for the stop, default 3000", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				return c.runControl(ctx, "interrupt", ms(a, "timeout_ms", defaultInterruptWait))
			}},
		{Name: "continue", Remote: true,
			Help:   "Resume the target, then wait up to wait_ms (default 5000; 0 returns at once) for it to stop. Returns state stopped with where, why and bkpt_number, or state running if it did not stop in time.",
			Params: []service.Param{{Name: "wait_ms", Type: service.Integer, Help: "wait for a stop, default 5000", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				return c.runControl(ctx, "continue", ms(a, "wait_ms", defaultStopWait))
			}},
		{Name: "wait_for_stop", Remote: true,
			Help:   "Wait until the running target stops (default 5000 ms) without resuming it. A stopped target returns at once. Use it after continue returned state running.",
			Params: []service.Param{{Name: "timeout_ms", Type: service.Integer, Help: "how long to wait, default 5000", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				return c.waitForStop(ctx, ms(a, "timeout_ms", defaultStopWait))
			}},
		step("step", "Step one source line, into calls. Waits for it to finish and returns the new location."),
		step("next", "Step one source line, over calls. Waits for it to finish and returns the new location."),
		step("stepi", "Step one machine instruction. Waits for it to finish and returns the new location."),
		step("finish", "Run until the current function returns. Waits (default 10000 ms) and returns the new location, or state running."),
		{Name: "run_to", Remote: true,
			Help: "Run to a location in one call: set a temporary breakpoint, continue and wait. Returns the stop, reached (true if it stopped at the location) and temp_breakpoint. GDB only; the target must be stopped.",
			Params: []service.Param{{Name: "location", Type: service.String, Help: "file.c:123, a function name or *0xADDR"},
				{Name: "timeout_ms", Type: service.Integer, Help: "wait for the stop, default 10000", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				loc := strings.TrimSpace(a.String("location"))
				if loc == "" {
					return nil, fmt.Errorf("run_to: location is empty")
				}
				return c.runTo(ctx, loc, ms(a, "timeout_ms", defaultStepWait))
			}},
		{Name: "batch", Remote: true,
			Help: "Run several tools in order in one call. With halt true a running target is interrupted first, so all reads see one consistent snapshot; with resume true it is continued afterwards (only if this call halted it and no breakpoint stopped it). " +
				`Example: {"ops":[{"tool":"read_memory","args":{"address":"0xE000ED00","length":16}},{"tool":"registers"}],"halt":true,"resume":true}. ` +
				"Returns {results:[{tool,result|error}], halted, resumed}. Run-control tools are not allowed inside. GDB only.",
			Params: []service.Param{{Name: "ops", Type: "object", Variadic: true, Help: "list of {tool, args}"},
				{Name: "halt", Type: service.Boolean, Help: "interrupt a running target first", Optional: true},
				{Name: "resume", Type: service.Boolean, Help: "continue afterwards if this call halted it", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				ops, _ := a["ops"].([]any)
				return c.batch(ctx, ops, a.Bool("halt"), a.Bool("resume"))
			}},
	} {
		r.Add(s)
	}
}
