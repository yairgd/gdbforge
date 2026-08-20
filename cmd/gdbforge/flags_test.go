package main

import (
	"errors"
	"flag"
	"reflect"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags([]string{"./hello"})
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
	cfg, err := parseFlags([]string{"-log", "debug.log", "./hello"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogFile != "debug.log" {
		t.Fatalf("LogFile: got %q want debug.log", cfg.LogFile)
	}

	cfg, err = parseFlags([]string{"--log", "gdbforge.log", "./hello"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogFile != "gdbforge.log" {
		t.Fatalf("LogFile: got %q want gdbforge.log", cfg.LogFile)
	}
}

func TestParseFlagsWithDebuggerAndArgs(t *testing.T) {
	cfg, err := parseFlags([]string{"-d", "/usr/bin/gdb", "./hello", "a", "b"})
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
	cfg, err := parseFlags([]string{"-g", "dlv", "./hello", "a"})
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
	cfg, err := parseFlags([]string{"-g", "dlv", "-d", "/usr/local/bin/dlv", "./pkg"})
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
	_, err := parseFlags([]string{"-g", "lldb", "./hello"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseFlagsPassThroughGDBOptions(t *testing.T) {
	cfg, err := parseFlags([]string{
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
	cfg, err := parseFlags([]string{"-g", "gdb"})
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
	_, err := parseFlags([]string{"-g", "dlv"})
	if err == nil {
		t.Fatal("expected missing program error")
	}
}

func TestParseFlagsHelp(t *testing.T) {
	_, err := parseFlags([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("got %v want flag.ErrHelp", err)
	}
}
