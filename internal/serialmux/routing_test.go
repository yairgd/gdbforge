package serialmux

import "testing"

func TestConsoleTxAllowedByOwner(t *testing.T) {
	m := &Mux{owner: OwnerTerminal}
	if !m.consoleTxAllowed() {
		t.Fatal("terminal owner should allow console TX")
	}
	m.owner = OwnerEnteringDebugger
	if !m.consoleTxAllowed() {
		t.Fatal("entering_debugger should allow console TX")
	}
	m.owner = OwnerDebugger
	if m.consoleTxAllowed() {
		t.Fatal("debugger owner must block console TX")
	}
}

func TestGdbTxAllowedByOwner(t *testing.T) {
	m := &Mux{owner: OwnerTerminal}
	if m.gdbTxAllowed() {
		t.Fatal("terminal owner should block gdb TX")
	}
	m.owner = OwnerDebugger
	if !m.gdbTxAllowed() {
		t.Fatal("debugger owner should allow gdb TX")
	}
}

func TestSwitchToGDBAndConsole(t *testing.T) {
	m := &Mux{owner: OwnerTerminal}
	m.SwitchToGDB()
	if m.Owner() != OwnerDebugger {
		t.Fatalf("SwitchToGDB: got %v", m.Owner())
	}
	if !m.miAuto {
		t.Fatal("SwitchToGDB should enable miAuto")
	}
	m.SwitchToConsole()
	if m.Owner() != OwnerTerminal {
		t.Fatalf("SwitchToConsole: got %v", m.Owner())
	}
	if m.miAuto {
		t.Fatal("SwitchToConsole should disable miAuto")
	}
}
