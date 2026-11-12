package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/termforge/service"
)

const (
	defaultStopWait = 5 * time.Second
	maxStopWait     = 60 * time.Second
	stopPoll        = 50 * time.Millisecond
	maxReadMemory   = 4096
)

// stopInfo is the compact state returned by run_control: where the program is, or that it runs.
type stopInfo struct {
	State string `json:"state"` // "stopped" or "running"
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
}

func (c *luaCtl) stopState(ctx context.Context) (stopInfo, error) {
	// Delve resumes over rpc2 and the console never prints the stop, so the UI's running flag
	// is not reliable there: ask the server.
	if b := c.host.Backend(); b != nil && b.Kind() != backend.GDB {
		running, ok := b.TargetRunning()
		if !ok { // the state query fails once the program has exited
			return stopInfo{State: "stopped"}, nil
		}
		if running {
			return stopInfo{State: "running"}, nil
		}
		var q backend.Querier
		if m := c.host.GdbMcp(); m != nil {
			q = m
		}
		fr, _ := b.CurrentFrame(ctx, q)
		return stopInfo{State: "stopped", File: fr.File, Line: fr.Line}, nil
	}
	var si stopInfo
	err := c.onUI(ctx, func() {
		h := c.host
		si.State = "stopped"
		if h.Debug() == nil {
			return
		}
		if h.Debug().InferiorRunning() {
			si.State = "running"
			return
		}
		si.File, si.Line = h.Debug().StopFile(), h.Debug().StopLine()
	})
	return si, err
}

// stoppedQuery checks the target is stopped and returns the session to query. With GDB the
// run state comes from the MI tracker and the queries wait for their own reply.
func (c *luaCtl) stoppedQuery(ctx context.Context) (queryFn, longFn func(string) (string, error), isDLV bool, err error) {
	m := c.host.GdbMcp()
	if b := c.host.Backend(); b != nil && b.Kind() != backend.GDB {
		isDLV = true
	}
	if m != nil && !isDLV {
		if tr := m.Tracker(ctx); tr != nil {
			if tr.State() == gdb.StateRunningName {
				return nil, nil, false, mcp.ErrTargetRunning("")
			}
			return func(cmd string) (string, error) { return m.QueryToken(ctx, cmd, 10*time.Second) },
				func(cmd string) (string, error) { return m.QueryToken(ctx, cmd, 30*time.Second) }, false, nil
		}
	}
	si, err := c.stopState(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	if si.State == "running" {
		return nil, nil, false, mcp.ErrTargetRunning("")
	}
	if m == nil {
		return nil, nil, false, errNoSessionSvc
	}
	return func(cmd string) (string, error) { return m.Query(ctx, cmd) },
		func(cmd string) (string, error) { return m.QueryLong(ctx, cmd) }, isDLV, nil
}

var errNoSessionSvc = fmt.Errorf("no debug session")

// miQuote quotes an MI argument that may contain spaces (expressions).
func miQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// addDebugServices registers the typed run-control and inspection services. They return small
// structured results instead of raw debugger output, which keeps an agent's token use low.
func (c *luaCtl) addDebugServices(r *service.Registry) {
	str := func(n, help string) service.Param { return service.Param{Name: n, Type: service.String, Help: help} }
	for _, s := range []service.Service{
		{Name: "run_control", Remote: true,
			Help: "Run control: continue, next, step, finish, stepi, nexti, run, start or interrupt. Waits up to `wait` seconds (default 5) for the program to stop and returns {state, file, line, …}; state \"running\" means it is still running; stopped with no file means the program exited. The dedicated tools (interrupt, continue, step, …) take milliseconds.",
			Params: []service.Param{str("action", "continue|next|step|finish|stepi|nexti|run|start|interrupt"),
				{Name: "wait", Type: service.Number, Help: "seconds to wait for a stop, default 5, max 60", Optional: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				wait := defaultStopWait
				if w, ok := a["wait"].(float64); ok && w >= 0 {
					wait = time.Duration(w * float64(time.Second))
				}
				return c.runControl(ctx, strings.TrimSpace(a.String("action")), min(wait, maxStopWait))
			}},
		{Name: "evaluate", Remote: true,
			Help:   "Evaluate an expression in the current frame (the program must be stopped) and return its value as text.",
			Params: []service.Param{str("expression", "C/Go expression, e.g. x, *p, arr[2], $pc")},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				expr := strings.TrimSpace(a.String("expression"))
				if expr == "" {
					return nil, fmt.Errorf("empty expression")
				}
				q, _, dlv, err := c.stoppedQuery(ctx)
				if err != nil {
					return nil, err
				}
				if dlv {
					out, err := q("print " + expr)
					if err != nil {
						return nil, err
					}
					v := cleanDlvOutput(out, "print "+expr)
					if msg, ok := strings.CutPrefix(v, "Command failed: "); ok {
						return nil, fmt.Errorf("%s", msg)
					}
					if strings.Contains(v, "has exited") {
						return nil, fmt.Errorf("%s", v)
					}
					return v, nil
				}
				out, err := q("-data-evaluate-expression " + miQuote(expr))
				if err != nil {
					return nil, err
				}
				if v, ok := parse.MIEvaluateValue(out); ok {
					return v, nil
				}
				if msg, ok := parse.MIError(out); ok {
					return nil, mcp.MIFailure(msg)
				}
				return nil, fmt.Errorf("evaluate: unexpected reply")
			}},
		{Name: "read_memory", Remote: true,
			Help: "Read target memory (GDB only; the program must be stopped). Returns {address, length, hex}, plus words32 (0x%08x strings in the target's byte order) and endian when length is a multiple of 4; length is capped at 4096 bytes.",
			Params: []service.Param{str("address", "address or expression, e.g. 0x20000000 or &buf"),
				{Name: "length", Type: service.Integer, Help: "number of bytes, 1-4096"}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				addr, n := strings.TrimSpace(a.String("address")), a.Int("length")
				if addr == "" || n < 1 || n > maxReadMemory {
					return nil, fmt.Errorf("address required and length must be 1-%d", maxReadMemory)
				}
				q, _, dlv, err := c.stoppedQuery(ctx)
				if err != nil {
					return nil, err
				}
				if dlv {
					return nil, fmt.Errorf("read_memory: not supported with Delve")
				}
				out, err := q("-data-read-memory-bytes " + miQuote(addr) + " " + strconv.Itoa(n))
				if err != nil {
					return nil, err
				}
				if hex, ok := parse.MIMemoryHex(out); ok {
					res := map[string]any{"address": addr, "length": len(hex) / 2, "hex": hex}
					if len(hex)%8 == 0 && hex != "" {
						endian := c.host.GdbMcp().Endian(ctx)
						res["words32"], res["endian"] = words32(hex, endian), endian
					}
					return res, nil
				}
				if msg, ok := parse.MIError(out); ok {
					return nil, mcp.MIFailure(msg)
				}
				return nil, fmt.Errorf("read_memory: unexpected reply")
			}},
		{Name: "registers", Remote: true,
			Help:   "CPU registers as {name: hex} (GDB only; the program must be stopped). Default: the general registers (what `info registers` shows, e.g. r0-r15 and cpsr). Pass names to get only those, or \"all\" for every register (large).",
			Params: []service.Param{{Name: "names", Type: service.String, Help: "register names, e.g. pc sp lr; or all", Variadic: true}},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				q, ql, dlv, err := c.stoppedQuery(ctx)
				if err != nil {
					return nil, err
				}
				if dlv {
					return nil, fmt.Errorf("registers: not supported with Delve")
				}
				want := map[string]bool{}
				for _, n := range a.Strings("names") {
					want[strings.TrimPrefix(strings.TrimSpace(n), "$")] = true
				}
				if !want["all"] {
					// default and named lookups both start from the general registers; names outside
					// that group fall through to the full list below
					// A long capture: the reply is many lines and the short idle window cuts it off.
					out, err := ql("info registers")
					if err != nil {
						return nil, err
					}
					var general map[string]string
					if strings.Contains(out, "^done") {
						general = parse.InfoRegisters(parse.MIConsoleText(out))
					}
					if len(want) == 0 {
						if len(general) > 0 {
							return general, nil
						}
						return nil, fmt.Errorf("registers: could not read the general registers; pass names or \"all\"")
					}
					sel := map[string]string{}
					for n := range want {
						if v, ok := general[n]; ok {
							sel[n] = v
						}
					}
					if len(sel) == len(want) {
						return sel, nil
					}
				}
				namesOut, err := q("-data-list-register-names")
				if err != nil {
					return nil, err
				}
				names := parse.MIRegisterNames(namesOut)
				valsOut, err := q("-data-list-register-values x")
				if err != nil {
					return nil, err
				}
				vals := parse.MIRegisterValues(valsOut)
				if len(names) == 0 || len(vals) == 0 {
					if msg, ok := parse.MIError(valsOut + namesOut); ok {
						return nil, mcp.MIFailure(msg)
					}
					return nil, fmt.Errorf("registers: unexpected reply")
				}
				all := want["all"]
				out := map[string]string{}
				for i, n := range names {
					if v, ok := vals[i]; ok && n != "" && (all || want[n]) {
						out[n] = v
					}
				}
				return out, nil
			}},
	} {
		r.Add(s)
	}
}

// cleanDlvOutput drops the echoed command line and the prompt from a Delve CLI reply.
func cleanDlvOutput(out, cmd string) string {
	var keep []string
	for _, l := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		t := strings.TrimSpace(l)
		if t == "" || t == cmd || strings.HasPrefix(t, "(dlv)") {
			continue
		}
		keep = append(keep, t)
	}
	return strings.Join(keep, "\n")
}

// words32 splits memory hex into 32-bit words as 0x%08x strings, in the target's byte order.
func words32(hex, endian string) []string {
	out := make([]string, 0, len(hex)/8)
	for i := 0; i+8 <= len(hex); i += 8 {
		w := hex[i : i+8]
		if endian != "big" { // bytes are in memory order: little endian reverses them
			w = w[6:8] + w[4:6] + w[2:4] + w[0:2]
		}
		out = append(out, "0x"+strings.ToLower(w))
	}
	return out
}
