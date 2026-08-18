package main

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestChildProcCtlKillAll(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var ctl childProcCtl
	ctl.Track(cmd.Process.Pid, true)
	ctl.KillAll()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("pid %d still alive after KillAll", cmd.Process.Pid)
	}
}

func TestProcessAlive(t *testing.T) {
	if processAlive(999999999) {
		t.Fatal("expected dead pid")
	}
	cmd := exec.Command("sleep", "1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if !processAlive(pid) {
		t.Fatal("expected live pid")
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}
