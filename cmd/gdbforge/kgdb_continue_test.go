package main

import (
	"testing"

	"github.com/creack/pty"

	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/serialmux"
)

func TestMaybeSwitchSerialConsoleOnContinue(t *testing.T) {
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

	a := &DebuggerApp{debug: debugstate.New(nil)}
	a.serial = serialCtl{app: a, mux: m}

	a.Debug().SetKgdbMode(true)
	if err := a.serial.SwitchToGDB(); err != nil {
		t.Fatal(err)
	}

	a.MaybeSwitchSerialConsoleOnContinue("next")
	owner, err := a.serial.Owner()
	if err != nil {
		t.Fatal(err)
	}
	if owner != "debugger" {
		t.Fatalf("next must not arm console switch, got owner %q", owner)
	}

	a.MaybeSwitchSerialConsoleOnContinue("c")
	owner, err = a.serial.Owner()
	if err != nil {
		t.Fatal(err)
	}
	if owner != "debugger" {
		t.Fatalf("continue must not switch before ^running, got owner %q", owner)
	}

	a.serialOnState(false, false, true)
	owner, err = a.serial.Owner()
	if err != nil {
		t.Fatal(err)
	}
	if owner != "terminal" {
		t.Fatalf("continue must switch to console on ^running, got owner %q", owner)
	}
}

func TestMaybeSwitchSerialConsoleOnContinueNoSerial(t *testing.T) {
	a := &DebuggerApp{debug: debugstate.New(nil)}
	a.Debug().SetKgdbMode(true)
	a.MaybeSwitchSerialConsoleOnContinue("continue") // must not panic
}
