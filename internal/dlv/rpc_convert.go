package dlv

import (
	"fmt"
	"strconv"

	"github.com/go-delve/delve/service/api"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

// BreakpointsFromAPI converts rpc2 breakpoints to shared models (user BPs only).
func BreakpointsFromAPI(bps []*api.Breakpoint) []models.BreakInfo {
	if len(bps) == 0 {
		return nil
	}
	out := make([]models.BreakInfo, 0, len(bps))
	for _, bp := range bps {
		if bp == nil || bp.ID < 1 {
			continue
		}
		if bp.Tracepoint || bp.TraceReturn {
			continue
		}
		if bp.WatchExpr != "" {
			continue
		}
		if bp.File == "" && bp.Line < 1 && len(bp.Addrs) == 0 && bp.Addr == 0 {
			continue
		}
		it := models.BreakInfo{
			Number:    bp.ID,
			Enabled:   !bp.Disabled,
			Condition: bp.Cond,
			File:      ResolveSourcePath(bp.File),
			Line:      bp.Line,
		}
		if len(bp.Addrs) > 0 {
			it.Addr = models.NormalizeAddr(fmt.Sprintf("0x%x", bp.Addrs[0]))
		} else if bp.Addr != 0 {
			it.Addr = models.NormalizeAddr(fmt.Sprintf("0x%x", bp.Addr))
		}
		out = append(out, it)
	}
	return out
}

// GoroutinesFromAPI converts rpc2 goroutines to thread rows.
func GoroutinesFromAPI(gs []*api.Goroutine, selectedID int64) []models.ThreadInfo {
	if len(gs) == 0 {
		return nil
	}
	out := make([]models.ThreadInfo, 0, len(gs))
	for _, g := range gs {
		if g == nil {
			continue
		}
		loc := g.UserCurrentLoc
		if loc.File == "" && loc.Line == 0 {
			loc = g.CurrentLoc
		}
		fn := ""
		if loc.Function != nil {
			fn = loc.Function.Name()
		}
		out = append(out, models.ThreadInfo{
			ID:      strconv.FormatInt(g.ID, 10),
			State:   goroutineStatus(g),
			Name:    fn,
			File:    ResolveSourcePath(loc.File),
			Line:    loc.Line,
			Func:    fn,
			Current: g.ID == selectedID,
		})
	}
	return out
}

func goroutineStatus(g *api.Goroutine) string {
	switch g.Status {
	case api.GoroutineWaiting:
		return "waiting"
	case api.GoroutineSyscall:
		return "syscall"
	default:
		return "running"
	}
}

// StackFromAPI converts rpc2 stack frames to shared models.
func StackFromAPI(frames []api.Stackframe) []models.StackFrame {
	if len(frames) == 0 {
		return nil
	}
	out := make([]models.StackFrame, 0, len(frames))
	for i, fr := range frames {
		fn := ""
		if fr.Function != nil {
			fn = fr.Function.Name()
		}
		out = append(out, models.StackFrame{
			Level: i,
			Func:  fn,
			File:  ResolveSourcePath(fr.File),
			Line:  fr.Line,
			Addr:  models.NormalizeAddr(fmt.Sprintf("0x%x", fr.PC)),
		})
	}
	return out
}

// CurrentGoroutineID returns the selected goroutine from debugger state.
func CurrentGoroutineID(st *api.DebuggerState) int64 {
	if st == nil || st.SelectedGoroutine == nil {
		return 0
	}
	return st.SelectedGoroutine.ID
}
