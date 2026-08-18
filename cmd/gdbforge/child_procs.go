package main

import (
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// childProcCtl tracks processes started by gdbforge (Go spawn APIs and
// gdbforge.system background jobs). Lua never registers PIDs manually.
type childProcCtl struct {
	mu sync.Mutex
	// pid -> kill the whole process group (Setpgid leader).
	groups map[int]bool
}

func (c *childProcCtl) Track(pid int, killGroup bool) {
	if pid <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.groups == nil {
		c.groups = make(map[int]bool)
	}
	c.groups[pid] = killGroup
}

func (c *childProcCtl) KillAll() {
	c.mu.Lock()
	entries := make([]struct {
		pid       int
		killGroup bool
	}, 0, len(c.groups))
	for pid, killGroup := range c.groups {
		entries = append(entries, struct {
			pid       int
			killGroup bool
		}{pid, killGroup})
	}
	c.groups = nil
	c.mu.Unlock()

	for _, e := range entries {
		signalProcess(e.pid, e.killGroup, syscall.SIGTERM)
	}
	time.Sleep(300 * time.Millisecond)
	for _, e := range entries {
		if processAlive(e.pid) {
			signalProcess(e.pid, e.killGroup, syscall.SIGKILL)
		}
	}
}

func signalProcess(pid int, killGroup bool, sig syscall.Signal) {
	if killGroup {
		_ = syscall.Kill(-pid, sig)
	}
	_ = syscall.Kill(pid, sig)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func (a *DebuggerApp) trackStartedCmd(cmd *exec.Cmd, killGroup bool) {
	if a == nil || cmd == nil || cmd.Process == nil {
		return
	}
	a.children.Track(cmd.Process.Pid, killGroup)
}
