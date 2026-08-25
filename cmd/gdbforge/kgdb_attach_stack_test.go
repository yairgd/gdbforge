package main

import (
	"testing"

	"github.com/creack/pty"

	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/serialmux"
)

func TestSkipKgdbAttachStackRefreshFlag(t *testing.T) {
	s := debugstate.New(nil)
	if s.TakeSkipKgdbAttachStackRefresh() {
		t.Fatal("flag should start false")
	}
	s.ArmSkipKgdbAttachStackRefresh()
	if !s.TakeSkipKgdbAttachStackRefresh() {
		t.Fatal("flag should be true after arm")
	}
	if s.TakeSkipKgdbAttachStackRefresh() {
		t.Fatal("flag should be false after take")
	}
}

func TestMaybeEnableRemoteModeArmsAttachStackSkip(t *testing.T) {
	_, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()

	m, err := serialmux.Open(slave.Name(), 115200)
	if err != nil {
		t.Skip("serialmux open on pty:", err)
	}
	defer m.Close()

	a := &DebuggerApp{DebugSession: DebugSession{debug: debugstate.New(nil)}}
	a.serial = serialCtl{app: a, mux: m}

	a.maybeEnableRemoteMode("target remote " + m.DebuggerPTY())
	if !a.Debug().TakeSkipKgdbAttachStackRefresh() {
		t.Fatal("target remote on serial mux should arm attach stack skip")
	}
}
