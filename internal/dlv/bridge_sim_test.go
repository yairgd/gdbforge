package dlv

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"os/exec"
	"path/filepath"

	"github.com/yairgd/gdbforge/internal/gdbforge/events"
)

// Simulates the app bridge: Subscribe -> collect InferiorOutputMsg payloads.
func TestBridgeLikeSubscribeGetsStdout(t *testing.T) {
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("no dlv")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	prog := filepath.Join(dir, "prog")
	body := "package main\nimport \"fmt\"\nfunc main() {\nfmt.Println(\"start\")\nfmt.Println(\"hello\")\n}\n"
	if err := os.WriteFile(src, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-o", prog, src).CombinedOutput(); err != nil {
		t.Skipf("build: %v %s", err, out)
	}
	c, err := NewClient("dlv", []string{prog})
	if err != nil {
		t.Skipf("dlv/pty unavailable: %v", err)
	}
	defer c.Close()

	inf := c.InferiorTTY()
	ch, cancel := inf.Subscribe()
	defer cancel()

	var mu sync.Mutex
	var got strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ch {
			_ = events.InferiorOutputMsg{Data: msg.Data, Err: msg.Err}
			mu.Lock()
			got.WriteString(msg.Data)
			s := got.String()
			mu.Unlock()
			if strings.Contains(s, "start") && strings.Contains(s, "hello") {
				return
			}
		}
	}()

	_ = c.Send("break main.main")
	time.Sleep(200 * time.Millisecond)
	_ = c.Send("continue")
	time.Sleep(200 * time.Millisecond)
	_ = c.Send("continue")

	select {
	case <-done:
		// ok
	case <-time.After(8 * time.Second):
		mu.Lock()
		s := got.String()
		mu.Unlock()
		t.Fatalf("timeout; got %q", s)
	}
}
