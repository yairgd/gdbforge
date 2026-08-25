package main

import (
	"context"
	"testing"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/termui"
)

func TestActivitySnapForCtrlC(t *testing.T) {
	cases := []struct {
		name string
		snap activitySnap
		want Activity
	}{
		{"idle", activitySnap{}, ActivityIdle},
		{"running", activitySnap{InferiorRunning: true}, ActivityInferiorRunning},
		{"lua", activitySnap{LuaJob: true}, ActivityLuaJob},
		{"lua_wins_over_running", activitySnap{InferiorRunning: true, LuaJob: true}, ActivityLuaJob},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.snap.forCtrlC(); got != tc.want {
				t.Fatalf("forCtrlC=%v want %v", got, tc.want)
			}
		})
	}
}

func TestActivitySnapForCtrlZ(t *testing.T) {
	cases := []struct {
		name string
		snap activitySnap
		want Activity
	}{
		{"idle", activitySnap{}, ActivityIdle},
		{"running", activitySnap{InferiorRunning: true}, ActivityInferiorRunning},
		{"lua", activitySnap{LuaJob: true}, ActivityLuaJob},
		{"running_wins_over_lua", activitySnap{InferiorRunning: true, LuaJob: true}, ActivityInferiorRunning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.snap.forCtrlZ(); got != tc.want {
				t.Fatalf("forCtrlZ=%v want %v", got, tc.want)
			}
		})
	}
}

func TestCtrlCCancelsLuaJob(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.host = a
	ctx, cancel := context.WithCancel(context.Background())
	a.lua.jobMu.Lock()
	a.lua.jobCancel = cancel
	a.lua.jobBusy.Store(true)
	a.lua.jobMu.Unlock()

	ev := tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	if !a.tryGlobalInterrupt(ev) {
		t.Fatal("Ctrl-C should be handled")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("lua job should be cancelled by Ctrl-C")
	}
}

func TestCtrlZCancelsLuaWhenInferiorIdle(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.host = a
	ctx, cancel := context.WithCancel(context.Background())
	a.lua.jobMu.Lock()
	a.lua.jobCancel = cancel
	a.lua.jobBusy.Store(true)
	a.lua.jobMu.Unlock()

	ev := tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone)
	if !a.tryGlobalSuspend(ev) {
		t.Fatal("Ctrl-Z should be handled")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("lua job should be cancelled by Ctrl-Z when inferior is idle")
	}
}

func TestCtrlZPrefersInferiorOverLua(t *testing.T) {
	a := &DebuggerApp{TermApp: &termui.TermApp{}}
	a.lua.host = a
	a.debug = debugstate.New(nil)
	a.debug.SetInferiorRunning(true)
	a.console.host = a

	ctx, cancel := context.WithCancel(context.Background())
	a.lua.jobMu.Lock()
	a.lua.jobCancel = cancel
	a.lua.jobBusy.Store(true)
	a.lua.jobMu.Unlock()

	if a.activitySnapshot().forCtrlZ() != ActivityInferiorRunning {
		t.Fatal("snapshot should prefer inferior over lua for Ctrl-Z")
	}

	ev := tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone)
	if !a.tryGlobalSuspend(ev) {
		t.Fatal("Ctrl-Z should be handled")
	}
	select {
	case <-ctx.Done():
		t.Fatal("Ctrl-Z must not cancel lua when inferior is running")
	default:
	}
}

func TestActivitySnapshotReadsJobBusy(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.host = a
	a.lua.jobBusy.Store(true)
	snap := a.activitySnapshot()
	if !snap.LuaJob || snap.forCtrlC() != ActivityLuaJob {
		t.Fatalf("snap=%+v", snap)
	}
}

func TestConfirmingFalseWhenUnwired(t *testing.T) {
	a := &DebuggerApp{}
	if a.confirming() {
		t.Fatal("expected not confirming")
	}
}
