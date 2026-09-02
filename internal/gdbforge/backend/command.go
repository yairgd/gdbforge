package backend

import (
	"context"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/platform"
)

// CommandEnv is the session context for backend-initiated debugger commands.
type CommandEnv struct {
	Session  core.Session
	App      *platform.AppState
	Inferior gdb.InferiorCtl
}

// SendDebuggerCmd sends a debugger command using the shared interrupt/continue policy.
func SendDebuggerCmd(env CommandEnv, cmd string) {
	if cmd == "" {
		return
	}
	gdb.SendCmd(env.Session, env.App, env.Inferior, cmd)
}

// SendMappedBreak applies MapBreak then SendDebuggerCmd.
func (b *GDBBackend) SendMappedBreak(env CommandEnv, cmd string) {
	if b != nil {
		cmd = b.MapBreak(cmd)
	}
	SendDebuggerCmd(env, cmd)
}

func (b *DLVBackend) SendMappedBreak(env CommandEnv, cmd string) {
	if b != nil {
		cmd = b.MapBreak(cmd)
	}
	SendDebuggerCmd(env, cmd)
}

// sendExecUI writes a run-control command with UI PTY ownership (keybindings).
func sendExecUI(env CommandEnv, sendCmd string) {
	if sendCmd == "" || env.Session == nil {
		return
	}
	fn := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = env.Session.WithWrite(ctx, func(pw core.PTYWriter) error {
			return pw.Send(sendCmd)
		})
	}
	if env.App != nil {
		env.App.WithPTYOwner(platform.PTYOwnerUI, fn)
	} else {
		fn()
	}
}

func execUI(b Backend, env CommandEnv, cmd string) {
	if b == nil || cmd == "" {
		return
	}
	send, marksRunning := b.MapExec(cmd)
	if marksRunning {
		if rs, ok := env.Inferior.(interface{ SetInferiorRunning(bool) }); ok {
			rs.SetInferiorRunning(true)
		}
	}
	sendExecUI(env, send)
}
