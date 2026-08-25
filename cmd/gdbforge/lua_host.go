package main

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// luaSerialHost is the serial-terminal surface exposed to Lua scripts.
type luaSerialHost interface {
	OpenShared(device string, baud int) error
	DebuggerPTY() (string, error)
	TerminalPTY() (string, error)
	Send(line string) error
	SwitchOwner(mode string) error
	Owner() (string, error)
	BeginDebugEntry() error
	SwitchToGDB() error
	SwitchToConsole() error
	SysrqDelayed(delaySec float64) error
	Close()
	SpawnConsole(pty string, baud int) error
}

// luaHost is the narrow surface luaCtl needs from the composition root.
type luaHost interface {
	AppLog() *platform.Logger
	Mode() platform.Mode
	SetMode(mode platform.Mode)
	RequestFrame()
	Tab() *termui.TabWidget
	Screen() tcell.Screen
	FocusedWidget() termui.Widget
	OutputWidget() *widgets.OutputWidget
	LuaConsoleWidget() *widgets.LuaConsoleWidget
	SwapFocusedWidget(w termui.Widget) bool
	GDBWidget() *widgets.GDBWidget
	ActiveCodeWidget() *widgets.CodeWidget
	CodeBufferForB() *widgets.CodeWidget
	Debug() *debugstate.State
	Backend() backend.Backend
	GdbMcp() *mcp.GdbMcpService
	OnRun(args ...any)
	SpawnExec(argv []string) error
	RunForeground(argv []string) error
	OpenExternalTTY() (string, error)
	SpawnTerminal(argv []string) error
	TrackChild(pid int)
	SetInferiorTTY(path string) error
	ConnectDlv(addr string) error
	SpawnDlvHeadless(port string, extraArgs []string) error
	SessionProgram() string
	DebuggerPath() string
	SetKgdbMode(on bool)
	MaybeEnableRemoteMode(cmd string)
	WithGdbUIOwner(fn func())
	SendGdbExec(cmd string)
	PlaceCodeInSlot(w *widgets.CodeWidget)
	FocusCode()
	FindGdbLeaf() *termui.Node
	FocusBufferWidget(w termui.Widget)
	OpenOrCreateBuffer(name string, from *luahost.Runtime)
	RegisterBuiltin(name string, w termui.Widget)
	BuiltinWidget(name string) termui.Widget
	Serial() luaSerialHost
}

type luaSerialAdapter struct{ a *DebuggerApp }

func (d luaSerialAdapter) OpenShared(device string, baud int) error {
	return d.a.serial.OpenShared(device, baud)
}
func (d luaSerialAdapter) DebuggerPTY() (string, error) { return d.a.serial.DebuggerPTY() }
func (d luaSerialAdapter) TerminalPTY() (string, error)  { return d.a.serial.TerminalPTY() }
func (d luaSerialAdapter) Send(line string) error         { return d.a.serial.Send(line) }
func (d luaSerialAdapter) SwitchOwner(mode string) error  { return d.a.serial.SwitchOwner(mode) }
func (d luaSerialAdapter) Owner() (string, error)         { return d.a.serial.Owner() }
func (d luaSerialAdapter) BeginDebugEntry() error         { return d.a.serial.BeginDebugEntry() }
func (d luaSerialAdapter) SwitchToGDB() error             { return d.a.serial.SwitchToGDB() }
func (d luaSerialAdapter) SwitchToConsole() error         { return d.a.serial.SwitchToConsole() }
func (d luaSerialAdapter) SysrqDelayed(delaySec float64) error {
	return d.a.serial.SysrqDelayed(delaySec)
}
func (d luaSerialAdapter) Close() { d.a.serial.Close() }
func (d luaSerialAdapter) SpawnConsole(pty string, baud int) error {
	return spawnConsoleTerminal(d.a, pty, baud)
}

func (a *DebuggerApp) AppLog() *platform.Logger {
	if a == nil {
		return nil
	}
	return a.ctx.Log
}

func (a *DebuggerApp) DebuggerPath() string {
	if a == nil {
		return ""
	}
	return a.cfg.GDBPath
}

func (a *DebuggerApp) WithGdbUIOwner(fn func()) {
	if a == nil || fn == nil {
		return
	}
	a.console.withGdbUIOwner(fn)
}

func (a *DebuggerApp) SendGdbExec(cmd string) {
	if a == nil || a.backend == nil {
		return
	}
	sendCmd, _ := a.backend.MapExec(cmd)
	a.console.withGdbUIOwner(func() { _ = a.backend.SendLine(sendCmd) })
}

func (a *DebuggerApp) PlaceCodeInSlot(w *widgets.CodeWidget) { a.placeCodeInSlot(w) }
func (a *DebuggerApp) FindGdbLeaf() *termui.Node             { return a.findGdbLeaf() }

func (a *DebuggerApp) FocusBufferWidget(w termui.Widget) {
	if a != nil {
		a.bufs.focusBufferWidget(w)
	}
}

func (a *DebuggerApp) OpenOrCreateBuffer(name string, from *luahost.Runtime) {
	if a != nil {
		a.bufs.openOrCreate(name, from)
	}
}

func (a *DebuggerApp) RegisterBuiltin(name string, w termui.Widget) { a.registerBuiltin(name, w) }

func (a *DebuggerApp) BuiltinWidget(name string) termui.Widget {
	if a == nil || a.builtins == nil {
		return nil
	}
	return a.builtins[name]
}

func (a *DebuggerApp) TrackChild(pid int) {
	if a != nil {
		a.children.Track(pid, false)
	}
}

func (a *DebuggerApp) SwapFocusedWidget(w termui.Widget) bool { return a.swapFocusedWidget(w) }

func (a *DebuggerApp) SetKgdbMode(on bool) { a.setKgdbMode(on) }

func (a *DebuggerApp) Serial() luaSerialHost {
	if a == nil {
		return nil
	}
	return luaSerialAdapter{a}
}
