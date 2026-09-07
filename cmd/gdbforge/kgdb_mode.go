package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/gdb"
)

// setKgdbMode enables kgdb/remote workarounds: CLI next/step (not MI -exec-*)
// and skip heavy post-stop MI queries that block slow serial after each stop.
func (a *DebuggerApp) setKgdbMode(on bool) {
	if a == nil || a.Debug() == nil {
		return
	}
	a.Debug().SetKgdbMode(on)
	if gb := a.gdbBackend(); gb != nil {
		gb.SetCLIExec(on)
	}
}

// maybeEnableRemoteMode turns on kgdb mode once after target remote (idempotent).
func (a *DebuggerApp) maybeEnableRemoteMode(cmd string) {
	if a == nil || !gdb.IsTargetRemoteCmd(cmd) {
		return
	}
	if a.serialActive() {
		gdbPty, _ := a.serial.DebuggerPTY()
		conPty, _ := a.serial.TerminalPTY()
		switch {
		case gdbPty != "" && strings.Contains(cmd, gdbPty):
			_ = a.serial.SwitchToGDB()
			a.printHostLine("serial-switch: gdb (target remote on gdb PTY)")
		case conPty != "" && strings.Contains(cmd, conPty):
			a.printHostLine("WARN: target remote on console PTY — use gdb PTY: " + gdbPty)
		}
		if a.Debug() != nil {
			a.Debug().ArmSkipKgdbAttachStackRefresh()
		}
	}
	if a.Debug() != nil && a.Debug().KgdbMode() {
		return
	}
	a.setKgdbMode(true)
	if a.gdbWidget != nil {
		a.gdbWidget.AppendLines([]string{
			"[gdbforge] remote target — CLI next/step/continue (like cgdb); n/s/c keys too",
		})
	}
}

// MaybeSwitchSerialConsoleOnContinue arms UART→console on the next ^running
// (not immediately — switching early blocks gdb PTY TX and breaks continue).
func (a *DebuggerApp) MaybeSwitchSerialConsoleOnContinue(cmd string) {
	if a == nil || !gdb.IsContinueCmd(cmd) {
		return
	}
	if !a.serialActive() || a.Debug() == nil || !a.Debug().KgdbMode() {
		return
	}
	a.serial.ArmSwitchConsoleOnRunning()
}

func (a *DebuggerApp) maybeSwitchSerialConsoleOnRunning(running bool) {
	if a == nil || !running || !a.serial.takeSwitchConsoleOnRunning() {
		return
	}
	if err := a.serial.SwitchToConsole(); err != nil {
		a.printHostLine("serial-switch: " + err.Error())
		return
	}
	a.printHostLine("serial-switch: console (kernel running) — use IO pane")
	if err := a.wireSerialConsole(); err != nil {
		a.printHostLine("serial-switch: wire IO: " + err.Error())
	}
}
