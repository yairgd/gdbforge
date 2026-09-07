package gdb

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type liveCtl struct {
	mu       sync.Mutex
	running  bool
	suppress int
}

func (c *liveCtl) InferiorRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
func (c *liveCtl) setRunning(v bool) {
	c.mu.Lock()
	c.running = v
	c.mu.Unlock()
}
func (c *liveCtl) ContinueAfterClear() bool { return false }
func (c *liveCtl) NoteTransientStopSuppress() {
	c.mu.Lock()
	c.suppress++
	c.mu.Unlock()
}

type miTap struct {
	mu    sync.Mutex
	state *GdbInputState
	upds  []MiUpdate
}

func (t *miTap) push(data string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	u := t.state.PushRaw(data)
	t.upds = append(t.upds, u)
}

func (t *miTap) match(pred func(MiUpdate) bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, u := range t.upds {
		if pred(u) {
			return true
		}
	}
	return false
}

func (t *miTap) reset() {
	t.mu.Lock()
	t.upds = nil
	t.mu.Unlock()
}

func (t *miTap) wait(d time.Duration, pred func(MiUpdate) bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if t.match(pred) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

const helloSrc = `#include <stdio.h>

int main(void)
{
	char buf[64];
	int i = 0;

	while (1) {
		snprintf(buf, sizeof(buf), "hello, gdbforge %d", i++);
	}
	return 0;
}
`

const helloLoopLine = "hello.c:6"

func buildHello(t *testing.T) string {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("no cc in PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "hello.c")
	if err := os.WriteFile(src, []byte(helloSrc), 0o600); err != nil {
		t.Fatalf("write hello.c: %v", err)
	}
	bin := filepath.Join(dir, "hello")
	out, err := exec.Command(cc, "-g", "-O0", "-o", bin, src).CombinedOutput()
	if err != nil {
		t.Skipf("cc failed: %v: %s", err, out)
	}
	return bin
}

func startLiveGDB(t *testing.T) (*GDBClient, *miTap) {
	t.Helper()
	if _, err := exec.LookPath("gdb"); err != nil {
		t.Skip("no gdb in PATH")
	}
	bin := buildHello(t)

	c, err := NewGDBClient("gdb", []string{bin})
	if err != nil {
		t.Skipf("start gdb: %v", err)
	}
	t.Cleanup(c.Close)

	tap := &miTap{state: NewGdbInputState()}
	miCh, miCancel := c.MITTY().Subscribe()
	t.Cleanup(miCancel)
	go func() {
		for msg := range miCh {
			if msg.Data != "" {
				tap.push(msg.Data)
			}
		}
	}()

	if inf := c.InferiorTTY(); inf != nil {
		infCh, infCancel := inf.Subscribe()
		t.Cleanup(infCancel)
		go func() {
			for range infCh {
			}
		}()
	}
	cliCh, cliCancel := c.CLI.Subscribe()
	t.Cleanup(cliCancel)
	go func() {
		for range cliCh {
		}
	}()

	return c, tap
}

func runToRunning(t *testing.T, c *GDBClient, tap *miTap) {
	t.Helper()
	if err := c.Send("-exec-run"); err != nil {
		t.Fatalf("-exec-run: %v", err)
	}
	if !tap.wait(20*time.Second, func(u MiUpdate) bool { return u.State == Running }) {
		t.Fatal("gdb never reported ^running")
	}
	time.Sleep(300 * time.Millisecond)
}

func TestLiveSendCmdBreakWhileRunningStopsGDB(t *testing.T) {
	c, tap := startLiveGDB(t)
	runToRunning(t, c, tap)

	ctl := &liveCtl{}
	ctl.setRunning(true)
	tap.reset()

	SendCmd(c, nil, ctl, "break "+helloLoopLine, SendOpts{InterruptCmd: MIExecInterrupt})

	if !tap.wait(10*time.Second, func(u MiUpdate) bool { return u.Stopped != nil }) {
		t.Fatal("no *stopped after break-while-running")
	}
	if !tap.wait(10*time.Second, func(u MiUpdate) bool { return u.State == Running }) {
		t.Fatal("no ^running after auto-continue")
	}
}

func TestLiveInterruptStopsRunningInferior(t *testing.T) {
	c, tap := startLiveGDB(t)
	runToRunning(t, c, tap)

	tap.reset()
	if err := c.Interrupt(); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}
	if !tap.wait(10*time.Second, func(u MiUpdate) bool {
		return u.Stopped != nil && u.Stopped.Reason == "signal-received"
	}) {
		t.Fatal("Ctrl-C did not stop the running inferior")
	}
}
