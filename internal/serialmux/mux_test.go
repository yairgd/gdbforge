package serialmux

import "testing"

func TestParseOwner(t *testing.T) {
	cases := []struct {
		in   string
		want Owner
	}{
		{"terminal", OwnerTerminal},
		{"", OwnerTerminal},
		{"entering_debugger", OwnerEnteringDebugger},
		{"entering", OwnerEnteringDebugger},
		{"debugger", OwnerDebugger},
		{"gdb", OwnerDebugger},
	}
	for _, tc := range cases {
		got, err := ParseOwner(tc.in)
		if err != nil {
			t.Fatalf("ParseOwner(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseOwner(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := ParseOwner("bogus"); err == nil {
		t.Fatal("expected error for bogus owner")
	}
}

func TestOwnerString(t *testing.T) {
	if OwnerTerminal.String() != "terminal" {
		t.Fatal(OwnerTerminal.String())
	}
	if OwnerEnteringDebugger.String() != "entering_debugger" {
		t.Fatal(OwnerEnteringDebugger.String())
	}
	if OwnerDebugger.String() != "debugger" {
		t.Fatal(OwnerDebugger.String())
	}
}

func TestOnDebuggerStateKgdbStaysOnGDBWhenRunning(t *testing.T) {
	m := &Mux{owner: OwnerDebugger, miAuto: true, kgdbSerial: true}
	m.OnDebuggerState(false, false, true)
	if m.Owner() != OwnerDebugger {
		t.Fatalf("kgdb serial must keep debugger owner while running, got %v", m.Owner())
	}

	m.kgdbSerial = false
	m.OnDebuggerState(false, false, true)
	if m.Owner() != OwnerTerminal {
		t.Fatalf("normal mode should return to terminal on running, got %v", m.Owner())
	}
}

func TestOnDebuggerState(t *testing.T) {
	m := &Mux{owner: OwnerEnteringDebugger, miAuto: true}
	m.OnDebuggerState(false, true, false)
	if m.Owner() != OwnerDebugger {
		t.Fatalf("prompt after enter: got %v", m.Owner())
	}

	m.SetOwner(OwnerDebugger)
	m.OnDebuggerState(false, false, true)
	if m.Owner() != OwnerTerminal {
		t.Fatalf("running: got %v", m.Owner())
	}

	m.SetOwner(OwnerTerminal)
	m.OnDebuggerState(true, false, false)
	if m.Owner() != OwnerTerminal {
		t.Fatalf("idle stopped must not steal console: got %v", m.Owner())
	}

	m.SetOwner(OwnerEnteringDebugger)
	m.miAuto = true
	m.OnDebuggerState(true, false, false)
	if m.Owner() != OwnerDebugger {
		t.Fatalf("stopped after debug-enter: got %v", m.Owner())
	}
}

func TestClaimRelease(t *testing.T) {
	device := "/dev/ttyTEST_mux_registry"
	if err := claim(device); err != nil {
		t.Fatal(err)
	}
	register(device, &Mux{device: device})
	if err := claim(device); err != ErrAlreadyOpen {
		t.Fatalf("claim: got %v", err)
	}
	release(device)
	if err := claim(device); err != nil {
		t.Fatalf("after release: %v", err)
	}
	release(device)
}
