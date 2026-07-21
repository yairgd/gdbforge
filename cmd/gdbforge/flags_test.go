package main

import (
	"errors"
	"flag"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags([]string{"./hello"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GDBPath != "gdb" {
		t.Fatalf("GDBPath: got %q want gdb", cfg.GDBPath)
	}
	if cfg.Prog != "./hello" {
		t.Fatalf("Prog: got %q", cfg.Prog)
	}
	if len(cfg.ProgArgs) != 0 {
		t.Fatalf("ProgArgs: got %v", cfg.ProgArgs)
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
	if len(cfg.ProgArgs) != 2 || cfg.ProgArgs[0] != "a" || cfg.ProgArgs[1] != "b" {
		t.Fatalf("ProgArgs: got %v", cfg.ProgArgs)
	}
}

func TestParseFlagsMissingProg(t *testing.T) {
	_, err := parseFlags(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseFlagsHelp(t *testing.T) {
	_, err := parseFlags([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("got %v want flag.ErrHelp", err)
	}
}
