package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestNoFileLogUnlessEnabled(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseFlags([]string{"./prog"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogFile != "" {
		t.Fatalf("LogFile: got %q want empty", cfg.LogFile)
	}

	a := &DebuggerApp{cfg: cfg, ctx: platform.NewAppContext()}
	// Same gate as InitB: empty LogFile must not create a sink/file.
	if path := a.cfg.LogFile; path != "" {
		if err := a.enableFileLog(path); err != nil {
			t.Fatal(err)
		}
	}
	if a.fileLog != nil {
		t.Fatal("fileLog sink should be nil by default")
	}
	if _, err := os.Stat(filepath.Join(dir, "gdbforge.log")); !os.IsNotExist(err) {
		t.Fatalf("gdbforge.log must not be created by default: %v", err)
	}
}

func TestEnableFileLogDefaultName(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	a := &DebuggerApp{cfg: SessionConfig{}, ctx: platform.NewAppContext()}
	if err := a.enableFileLog(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("gdbforge.log"); err != nil {
		t.Fatalf(":set log should create gdbforge.log: %v", err)
	}
	if a.fileLog != nil {
		a.ctx.Log.RemoveSink(a.fileLog)
		_ = a.fileLog.Close()
		a.fileLog = nil
	}
}

func TestEnableFileLogCustomPath(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	a := &DebuggerApp{cfg: SessionConfig{}, ctx: platform.NewAppContext()}
	if err := a.enableFileLog("custom.log"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("custom.log"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("gdbforge.log"); !os.IsNotExist(err) {
		t.Fatal("should not create default name for custom path")
	}
	if a.fileLog != nil {
		a.ctx.Log.RemoveSink(a.fileLog)
		_ = a.fileLog.Close()
		a.fileLog = nil
	}
}
