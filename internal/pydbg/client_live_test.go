package pydbg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const liveScript = `def work(i):
    x = i * 2
    return x


for n in range(3):
    work(n)
print("done-marker")
`

// waitEvent returns the next event of kind k within a timeout.
func waitEvent(t *testing.T, ch <-chan Event, k EventKind) Event {
	t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Kind == k {
				return ev
			}
		case <-timeout:
			t.Fatalf("timeout waiting for event kind %d", k)
		}
	}
}

func TestLiveSession(t *testing.T) {
	py := os.Getenv("PYDBG_TEST_PYTHON")
	if py == "" {
		py = "python3"
	}
	if _, err := exec.LookPath(py); err != nil {
		t.Skip("no python")
	}
	if err := exec.Command(py, "-c", "import debugpy").Run(); err != nil {
		t.Skip("debugpy not importable")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "prog.py")
	if err := os.WriteFile(script, []byte(liveScript), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(py, []string{script}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	events := make(chan Event, 64)
	c.SetOnEvent(func(ev Event) { events <- ev })
	out, cancel := c.InferiorTTY().Subscribe()
	defer cancel()

	entry := waitEvent(t, events, Stopped)
	t.Logf("entry stop: %+v", entry)
	if c.Running() {
		t.Fatal("Running after stop")
	}

	n, err := c.SetBreak(script, 2)
	if err != nil || n != 1 {
		t.Fatalf("SetBreak: %d %v", n, err)
	}
	if bps := c.Breakpoints(); len(bps) != 1 || bps[0].Line != 2 || bps[0].File != script {
		t.Fatalf("breakpoints: %+v", bps)
	}
	if err := c.Continue(); err != nil {
		t.Fatal(err)
	}
	st := waitEvent(t, events, Stopped)
	if st.Line != 2 || st.Func != "work" || st.File != script {
		t.Fatalf("stopped: %+v", st)
	}

	stack, err := c.Stack()
	if err != nil || len(stack) == 0 || stack[0].Func != "work" || stack[0].Line != 2 {
		t.Fatalf("stack: %+v %v", stack, err)
	}
	ths, err := c.Threads()
	if err != nil || len(ths) == 0 {
		t.Fatalf("threads: %+v %v", ths, err)
	}
	if v, err := c.Evaluate("i"); err != nil || v != "0" {
		t.Fatalf("evaluate i: %q %v", v, err)
	}
	if _, err := c.Evaluate("x = 5"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if v, err := c.Evaluate("x"); err != nil || v != "5" {
		t.Fatalf("evaluate x: %q %v", v, err)
	}
	if _, err := c.Evaluate("no_such_name"); err == nil {
		t.Fatal("want error for undefined name")
	} else {
		t.Logf("error text: %v", err)
	}
	if err := c.SelectFrame(1); err != nil {
		t.Fatal(err)
	}
	ev := waitEvent(t, events, FrameSelected)
	if ev.Level != 1 {
		t.Fatalf("frame event %+v", ev)
	}
	if got, err := c.Complete("wor"); err != nil {
		t.Logf("completions: %v", err)
	} else {
		t.Logf("completions: %v", got)
	}
	t.Chdir(dir)
	files, err := c.SourceFiles()
	t.Logf("source files: %v %v", files, err)

	if err := c.ClearBreak(script, 2, 0); err != nil {
		t.Fatal(err)
	}
	if len(c.Breakpoints()) != 0 {
		t.Fatal("table not empty")
	}
	if err := c.Continue(); err != nil {
		t.Fatal(err)
	}
	ex := waitEvent(t, events, Exited)
	if ex.ExitCode != 0 {
		t.Fatalf("exit code %d", ex.ExitCode)
	}

	var got strings.Builder
	deadline := time.After(5 * time.Second)
	for !strings.Contains(got.String(), "done-marker") {
		select {
		case m, ok := <-out:
			if !ok {
				t.Fatalf("inferior closed; got %q", got.String())
			}
			got.WriteString(m.Data)
		case <-deadline:
			t.Fatalf("no program output; got %q", got.String())
		}
	}

	// Restart: same script, stopped on entry again.
	if err := c.Restart(); err != nil {
		t.Fatal(err)
	}
	if ev := waitEvent(t, events, Stopped); ev.Reason != "entry" {
		t.Fatalf("after restart: %+v", ev)
	}
}
