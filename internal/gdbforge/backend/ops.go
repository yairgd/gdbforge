package backend

import (
	"context"
	"fmt"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
)

// NavigationOpts controls frame/thread selection side effects.
type NavigationOpts struct {
	// Async sends the command in a goroutine (Delve holds PTY lock during queries).
	Async bool
}

// SelectFrame sends a frame selection command.
func (b *GDBBackend) SelectFrame(env CommandEnv, level int, _ NavigationOpts) {
	SendDebuggerCmd(env, fmt.Sprintf("-stack-select-frame %d", level))
}

// SelectFrame sends a frame selection command on Delve CLI.
func (b *DLVBackend) SelectFrame(env CommandEnv, level int, opts NavigationOpts) {
	cmd := fmt.Sprintf("frame %d", level)
	if opts.Async {
		go SendDebuggerCmd(env, cmd)
		return
	}
	SendDebuggerCmd(env, cmd)
}

// SelectThread sends a thread/goroutine selection command.
func (b *GDBBackend) SelectThread(env CommandEnv, id string, _ NavigationOpts) {
	SendDebuggerCmd(env, "-thread-select "+id)
}

// SelectThread sends a goroutine selection command on Delve.
func (b *DLVBackend) SelectThread(env CommandEnv, id string, opts NavigationOpts) {
	if b.selectGoroutineRPC(id) {
		return
	}
	cmd := "goroutine " + id
	if opts.Async {
		go SendDebuggerCmd(env, cmd)
		return
	}
	SendDebuggerCmd(env, cmd)
}

// Exec sends a run-control command (continue, next, step, …).
func (b *GDBBackend) Exec(env CommandEnv, cmd string) {
	send, _ := b.MapExec(cmd)
	SendDebuggerCmd(env, send)
}

// ExecUI sends run-control from UI keybindings (PTYOwnerUI).
func (b *GDBBackend) ExecUI(env CommandEnv, cmd string) { execUI(b, env, cmd) }

// Exec sends a run-control command on Delve (rpc2 when available, else CLI).
func (b *DLVBackend) Exec(env CommandEnv, cmd string) {
	if b.execViaRPC(env, cmd, false) {
		return
	}
	send, _ := b.MapExec(cmd)
	SendDebuggerCmd(env, send)
}

func (b *DLVBackend) ExecUI(env CommandEnv, cmd string) {
	if b.execViaRPC(env, cmd, true) {
		return
	}
	execUI(b, env, cmd)
}

// InsertBreakpoint sends a break insert at file:line.
func (b *GDBBackend) InsertBreakpoint(env CommandEnv, file string, line int) {
	b.SendMappedBreak(env, BreakInsertAt(GDB, file, line))
}

func (b *DLVBackend) InsertBreakpoint(env CommandEnv, file string, line int) {
	if b.insertBreakpointRPC(file, line) {
		return
	}
	b.SendMappedBreak(env, BreakInsertAt(DLV, file, line))
}

// ClearBreakpointAt sends clear/break-delete at a source location.
func (b *GDBBackend) ClearBreakpointAt(env CommandEnv, file string, line int, number int) {
	if number > 0 {
		b.SendMappedBreak(env, BreakDeleteNum(GDB, number))
		return
	}
	b.SendMappedBreak(env, BreakClearAt(GDB, file, line, ""))
}

func (b *DLVBackend) ClearBreakpointAt(env CommandEnv, file string, line int, number int) {
	if b.clearBreakpointRPC(file, line, "", number) {
		return
	}
	if number > 0 {
		b.SendMappedBreak(env, BreakDeleteNum(DLV, number))
		return
	}
	b.SendMappedBreak(env, BreakClearAt(DLV, file, line, ""))
}

// ClearBreakpointAddr clears an address breakpoint.
func (b *GDBBackend) ClearBreakpointAddr(env CommandEnv, addr string, number int) {
	if number > 0 {
		b.SendMappedBreak(env, BreakDeleteNum(GDB, number))
		return
	}
	b.SendMappedBreak(env, BreakClearAt(GDB, "", 0, addr))
}

func (b *DLVBackend) ClearBreakpointAddr(env CommandEnv, addr string, number int) {
	if b.clearBreakpointRPC("", 0, addr, number) {
		return
	}
	if number > 0 {
		b.SendMappedBreak(env, BreakDeleteNum(DLV, number))
		return
	}
	b.SendMappedBreak(env, BreakClearAt(DLV, "", 0, addr))
}

// DisableBreakpoint disables by number.
func (b *GDBBackend) DisableBreakpoint(env CommandEnv, number int) {
	b.SendMappedBreak(env, BreakDisableNum(GDB, number))
}

func (b *DLVBackend) DisableBreakpoint(env CommandEnv, number int) {
	if b.disableBreakpointRPC(number) {
		return
	}
	b.SendMappedBreak(env, BreakDisableNum(DLV, number))
}

// SetBreakpointCondition sets condition on breakpoint number.
func (b *GDBBackend) SetBreakpointCondition(env CommandEnv, number int, cond string) {
	b.SendMappedBreak(env, BreakConditionNum(GDB, number, cond))
}

func (b *DLVBackend) SetBreakpointCondition(env CommandEnv, number int, cond string) {
	if b.setBreakpointConditionRPC(number, cond) {
		return
	}
	b.SendMappedBreak(env, BreakConditionNum(DLV, number, cond))
}

// InsertBreakpointAddr inserts break at address.
func (b *GDBBackend) InsertBreakpointAddr(env CommandEnv, addr string) {
	b.SendMappedBreak(env, BreakInsertAddr(GDB, addr))
}

func (b *DLVBackend) InsertBreakpointAddr(env CommandEnv, addr string) {
	if b.insertBreakpointAddrRPC(addr) {
		return
	}
	b.SendMappedBreak(env, BreakInsertAddr(DLV, addr))
}

// DefaultBreakMainCmd returns the default entry breakpoint command.
func (b *GDBBackend) DefaultBreakMainCmd() string { return b.DefaultBreakMain() }
func (b *DLVBackend) DefaultBreakMainCmd() string { return b.DefaultBreakMain() }

// InsertDefaultBreakMain sends the default entry breakpoint.
func (b *GDBBackend) InsertDefaultBreakMain(env CommandEnv) {
	b.SendMappedBreak(env, b.DefaultBreakMain())
}

func (b *DLVBackend) InsertDefaultBreakMain(env CommandEnv) {
	if b.insertDefaultBreakMainRPC() {
		return
	}
	b.SendMappedBreak(env, b.DefaultBreakMain())
}

// CurrentFrame queries the selected stack frame.
func (b *GDBBackend) CurrentFrame(ctx context.Context, q Querier) (models.StackFrame, bool) {
	if q == nil {
		return models.StackFrame{}, false
	}
	raw, err := q.Query(ctx, "-stack-info-frame")
	if err != nil {
		return models.StackFrame{}, false
	}
	return parse.ParseStackInfoFrame(raw)
}

// FetchStackList queries stack frames only.
func (b *GDBBackend) FetchStackList(ctx context.Context, q Querier, longCapture bool) ([]models.StackFrame, bool) {
	return StackList(ctx, GDB, q, longCapture, nil)
}

// ListSourceFiles queries executable source file list.
func (b *GDBBackend) ListSourceFiles(ctx context.Context, q Querier) ([]string, bool) {
	if q == nil {
		return nil, false
	}
	raw, err := q.Query(ctx, "-file-list-exec-source-files")
	if err != nil {
		return nil, false
	}
	files := parse.ParseSourceFileList(raw)
	return files, len(files) > 0
}

// NavigationAsync reports whether frame/thread select should run async.
func (b *GDBBackend) NavigationAsync() bool { return false }
func (b *DLVBackend) NavigationAsync() bool { return true }

// ConfirmingInterrupt sends interrupt during a confirm gate (Delve: "n").
func (b *GDBBackend) ConfirmingInterrupt(_ CommandEnv) error {
	return b.Interrupt(false, true)
}

func (b *DLVBackend) ConfirmingInterrupt(_ CommandEnv) error {
	return b.Interrupt(false, true)
}

// ConsoleEOFCommand returns the command sent on debugger console EOF (Ctrl-D).
func (b *GDBBackend) ConsoleEOFCommand() string { return "" }
func (b *DLVBackend) ConsoleEOFCommand() string { return "quit" }

// WireCLILineTap reports whether CLI keystroke line tap is needed for side effects.
func (b *GDBBackend) WireCLILineTap() bool { return false }
func (b *DLVBackend) WireCLILineTap() bool  { return true }

// DeferBreakpointRefresh reports whether BP refresh should defer during confirm.
func (b *GDBBackend) DeferBreakpointRefresh() bool { return false }
func (b *DLVBackend) DeferBreakpointRefresh() bool  { return true }

// StackNavIsStackNavCmd reports CLI commands that change frame without a new stop.
func StackNavIsStackNavCmd(cmd string) bool {
	return gdb.IsStackNavCmd(cmd) || dlv.IsStackNavCmd(cmd)
}
