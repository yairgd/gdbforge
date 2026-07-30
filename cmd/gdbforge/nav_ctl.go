package main

import (
	"fmt"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

func (c *navCtl) ActivateCallStack(fr models.StackFrame) {
	a := c.app
	if a == nil {
		return
	}
	// User is browsing — cancel any in-flight stop refresh that would snap
	// Code back to frame 0.
	a.codeNavGen++

	// Drive Code from the selected row first — do not wait on the debugger PTY
	// (Delve `stack` / `goroutines` queries hold the write lock for a long time).
	a.showFrameSource(fr)
	a.RequestFrame()

	if a.gdbWidget == nil {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	cmd := fmt.Sprintf("-stack-select-frame %d", fr.Level)
	if a.backend != nil {
		cmd = a.backend.SelectFrameCmd(fr.Level)
	}
	if a.isDLV() {
		// Selecting a call-stack row must update Code from the row's file:line.
		// Sending `frame N` makes Delve re-emit "> …" and dump source, which we
		// used to treat as a new stop (goroutines/stack refresh → snap to frame 0).
		a.dlvSuppressStopUI++
		go gdb.SendCmd(sess, a.State(), a.Debug(), cmd)
		return
	}
	// GDB MI frame select is cheap; keep it on the UI path like before.
	gdb.SendCmd(sess, a.State(), a.Debug(), cmd)
}

func (c *navCtl) ActivateThread(th models.ThreadInfo) {
	a := c.app
	if a == nil {
		return
	}
	if a.gdbWidget == nil || th.ID == "" {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	cmd := "-thread-select " + th.ID
	if a.backend != nil {
		cmd = a.backend.SelectThreadCmd(th.ID)
	}
	gdb.SendCmd(sess, a.State(), a.Debug(), cmd)
	a.refreshThreadsAndStack()
	a.syncThreadViews()
	a.syncCallStackViews()

	file, line := th.File, th.Line
	if a.callstack != nil {
		if frames := a.callstack.Items(); len(frames) > 0 {
			if frames[0].File != "" {
				file, line = frames[0].File, frames[0].Line
			}
		}
	}
	if file != "" {
		w := a.showCodeAt(file, line)
		if w != nil && w.Unavailable() {
			fn := th.Func
			if a.callstack != nil {
				if frames := a.callstack.Items(); len(frames) > 0 && frames[0].Func != "" {
					fn = frames[0].Func
				}
			}
			w.ShowUnavailable(file, formatUnavailableExtra(fn, line))
		}
	}
	a.RequestFrame()
}
