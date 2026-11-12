package app

import (
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := ParseFlags([]string{"./hello"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Kind != BackendGDB {
		t.Fatalf("Kind: got %q want gdb", cfg.Kind)
	}
	if cfg.GDBPath != "gdb" {
		t.Fatalf("GDBPath: got %q want gdb", cfg.GDBPath)
	}
	if cfg.Prog != "./hello" {
		t.Fatalf("Prog: got %q", cfg.Prog)
	}
	if !reflect.DeepEqual(cfg.GDBArgs, []string{"./hello"}) {
		t.Fatalf("GDBArgs: got %v", cfg.GDBArgs)
	}
	if cfg.LogFile != "" {
		t.Fatalf("LogFile: got %q want empty (no file logging by default)", cfg.LogFile)
	}
}

func TestParseFlagsLogFile(t *testing.T) {
	cfg, err := ParseFlags([]string{"-log", "debug.log", "./hello"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogFile != "debug.log" {
		t.Fatalf("LogFile: got %q want debug.log", cfg.LogFile)
	}

	cfg, err = ParseFlags([]string{"--log", "gdbforge.log", "./hello"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogFile != "gdbforge.log" {
		t.Fatalf("LogFile: got %q want gdbforge.log", cfg.LogFile)
	}
}

func TestParseFlagsWithDebuggerAndArgs(t *testing.T) {
	cfg, err := ParseFlags([]string{"-d", "/usr/bin/gdb", "./hello", "a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GDBPath != "/usr/bin/gdb" {
		t.Fatalf("GDBPath: got %q", cfg.GDBPath)
	}
	if cfg.Prog != "./hello" {
		t.Fatalf("Prog: got %q", cfg.Prog)
	}
	want := []string{"--args", "./hello", "a", "b"}
	if !reflect.DeepEqual(cfg.GDBArgs, want) {
		t.Fatalf("GDBArgs: got %v want %v", cfg.GDBArgs, want)
	}
}

func TestParseFlagsDLV(t *testing.T) {
	cfg, err := ParseFlags([]string{"-g", "dlv", "./hello", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Kind != BackendDLV {
		t.Fatalf("Kind: got %q", cfg.Kind)
	}
	if cfg.GDBPath != "dlv" {
		t.Fatalf("GDBPath: got %q want dlv", cfg.GDBPath)
	}
	want := []string{"./hello", "a"}
	if !reflect.DeepEqual(cfg.GDBArgs, want) {
		t.Fatalf("GDBArgs: got %v want %v", cfg.GDBArgs, want)
	}
}

func TestParseFlagsDLVExplicitBinary(t *testing.T) {
	cfg, err := ParseFlags([]string{"-g", "dlv", "-d", "/usr/local/bin/dlv", "./pkg"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GDBPath != "/usr/local/bin/dlv" {
		t.Fatalf("GDBPath: got %q", cfg.GDBPath)
	}
	if !cfg.IsDLV() {
		t.Fatal("expected IsDLV")
	}
}

func TestParseFlagsUnknownBackend(t *testing.T) {
	_, err := ParseFlags([]string{"-g", "lldb", "./hello"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseFlagsPassThroughGDBOptions(t *testing.T) {
	cfg, err := ParseFlags([]string{
		"-d", "/usr/bin/gdb", "--",
		"-nx", "-x", "/tmp/r5_debug.gdb", "/tmp/zephyr.elf",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GDBPath != "/usr/bin/gdb" {
		t.Fatalf("GDBPath: got %q", cfg.GDBPath)
	}
	want := []string{"-nx", "-x", "/tmp/r5_debug.gdb", "/tmp/zephyr.elf"}
	if !reflect.DeepEqual(cfg.GDBArgs, want) {
		t.Fatalf("GDBArgs: got %v want %v", cfg.GDBArgs, want)
	}
	if cfg.Prog != "/tmp/zephyr.elf" {
		t.Fatalf("Prog: got %q", cfg.Prog)
	}
}

func TestParseFlagsGDBWithoutProg(t *testing.T) {
	cfg, err := ParseFlags([]string{"-g", "gdb"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Kind != BackendGDB || cfg.GDBPath != "gdb" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Prog != "" || len(cfg.GDBArgs) != 0 {
		t.Fatalf("programless GDB config has program: %+v", cfg)
	}
}

func TestParseFlagsDLVRequiresProg(t *testing.T) {
	_, err := ParseFlags([]string{"-g", "dlv"})
	if err == nil {
		t.Fatal("expected missing program error")
	}
}

func TestParseFlagsHelp(t *testing.T) {
	_, err := ParseFlags([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("got %v want flag.ErrHelp", err)
	}
}

func TestParseFlagsServeNeedsToken(t *testing.T) {
	t.Setenv("GDBFORGE_TOKEN", "")
	if _, err := ParseFlags([]string{"-serve", "127.0.0.1:7777", "./hello"}); err == nil {
		t.Fatal("-serve without a token must fail")
	}
	cfg, err := ParseFlags([]string{"-serve", "127.0.0.1:7777", "-serve-token", "t", "./hello"})
	if err != nil || cfg.Serve != "127.0.0.1:7777" || cfg.ServeToken != "t" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	t.Setenv("GDBFORGE_TOKEN", "envtok")
	cfg, err = ParseFlags([]string{"-serve", "127.0.0.1:7777", "./hello"})
	if err != nil || cfg.ServeToken != "envtok" {
		t.Fatalf("env token: cfg=%+v err=%v", cfg, err)
	}
}

func TestParseFlagsPY(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantArgs []string
		wantCall string
	}{
		{"script", []string{"-g", "py", "x.py"}, []string{"x.py"}, ""},
		{"script args", []string{"-g", "py", "x.py", "a", "b"}, []string{"x.py", "a", "b"}, ""},
		{"pass-through", []string{"-g", "py", "--", "-m", "pytest", "t.py::test_foo"}, []string{"-m", "pytest", "t.py::test_foo"}, ""},
		{"call", []string{"-g", "py", "--call", "pkg.mod:func"}, nil, "pkg.mod:func"},
		{"call single dash", []string{"-g", "py", "-call", "pkg.mod:func"}, nil, "pkg.mod:func"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := ParseFlags(c.args)
			if err != nil {
				t.Fatal(err)
			}
			if !cfg.IsPY() || cfg.IsGDB() || cfg.IsDLV() {
				t.Fatalf("kind: %+v", cfg)
			}
			if cfg.GDBPath != "python3" {
				t.Fatalf("GDBPath: got %q want python3", cfg.GDBPath)
			}
			if !reflect.DeepEqual(cfg.GDBArgs, c.wantArgs) {
				t.Fatalf("GDBArgs: got %v want %v", cfg.GDBArgs, c.wantArgs)
			}
			if cfg.Call != c.wantCall {
				t.Fatalf("Call: got %q want %q", cfg.Call, c.wantCall)
			}
		})
	}
}

func TestParseFlagsPYExplicitBinary(t *testing.T) {
	cfg, err := ParseFlags([]string{"-g", "py", "-d", "/usr/bin/python3.12", "x.py"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GDBPath != "/usr/bin/python3.12" {
		t.Fatalf("GDBPath: got %q", cfg.GDBPath)
	}
}

func TestParseFlagsPYRequiresScript(t *testing.T) {
	_, err := ParseFlags([]string{"-g", "py"})
	if err == nil || !strings.Contains(err.Error(), "missing script (required for py)") {
		t.Fatalf("got %v", err)
	}
}

func TestParseFlagsCallNeedsPY(t *testing.T) {
	for _, g := range []string{"gdb", "dlv"} {
		if _, err := ParseFlags([]string{"-g", g, "--call", "a.b:c", "./hello"}); err == nil {
			t.Fatalf("-g %s: expected error", g)
		}
	}
}
