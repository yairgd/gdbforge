package dlv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestTargetRunningAndInterruptLive covers the Ctrl-C path against a real Delve
// session. gdbforge used to gate the interrupt on a flag armed by watching
// keystrokes, so a target resumed through Delve's own line editor (history
// recall, bare Enter repeat) looked idle and Ctrl-C was dropped. TargetRunning
// asks the server instead.
func TestTargetRunningAndInterruptLive(t *testing.T) {
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("no dlv")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	prog := filepath.Join(dir, "prog")
	code := "package main\n\nimport \"time\"\n\nfunc main() {\n\tfor {\n\t\ttime.Sleep(10 * time.Millisecond)\n\t}\n}\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-o", prog, src).CombinedOutput(); err != nil {
		t.Skipf("build: %v %s", err, out)
	}

	c, err := NewClient("dlv", []string{prog})
	if err != nil {
		t.Skipf("dlv: %v", err)
	}
	defer c.Close()

	if running, ok := c.TargetRunning(); !ok || running {
		t.Fatalf("at startup prompt: running=%v ok=%v, want false/true", running, ok)
	}

	// Resume over rpc2, i.e. without any keystroke the CLI tap could observe.
	go func() {
		if resumed := c.RPC.Continue(); resumed != nil {
			<-resumed
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		running, ok := c.TargetRunning()
		if ok && running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("target never reported running (ok=%v)", ok)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := c.Interrupt(); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}
	if running, ok := c.TargetRunning(); !ok || running {
		t.Fatalf("after interrupt: running=%v ok=%v, want false/true", running, ok)
	}
}
