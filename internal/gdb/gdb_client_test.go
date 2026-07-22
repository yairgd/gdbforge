package gdb

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

func TestNewGDBClientStartsAndCloses(t *testing.T) {
	prog := filepath.Join("..", "..", "hello")
	if _, err := os.Stat(prog); err != nil {
		t.Skip("hello binary not present")
	}

	client, err := NewGDBClient("gdb", []string{prog})
	if err != nil {
		t.Skipf("gdb/pty unavailable: %v", err)
	}
	defer client.Close()

	if client.InferiorTTY() == nil || client.InferiorTTY().SlaveName() == "" {
		t.Fatal("expected inferior tty after NewGDBClient")
	}

	ch, cancel := client.Subscribe()
	defer cancel()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg.Err != nil {
				t.Fatalf("gdb output error: %v", msg.Err)
			}
			if msg.Data != "" {
				client.Close()
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for gdb output")
		}
	}
}

func TestNewGDBClientRequiresArgs(t *testing.T) {
	_, err := NewGDBClient("gdb", nil)
	if err == nil {
		t.Fatal("expected error for empty gdb args")
	}
}

func TestConcurrentSendSerialized(t *testing.T) {
	prog := filepath.Join("..", "..", "hello")
	if _, err := os.Stat(prog); err != nil {
		t.Skip("hello binary not present")
	}
	client, err := NewGDBClient("gdb", []string{prog})
	if err != nil {
		t.Skipf("gdb/pty unavailable: %v", err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Send("-gdb-show confirm")
		}()
	}
	wg.Wait()
}

func TestGDBClientIsSession(t *testing.T) {
	var _ core.Session = (*GDBClient)(nil)
	var _ core.Session = (*ptyx.Client)(nil)
}

func TestHasInitScript(t *testing.T) {
	if HasInitScript([]string{"prog"}) {
		t.Fatal("plain prog should not count as init script")
	}
	if !HasInitScript([]string{"-nx", "-x", "r5_debug.gdb", "prog.elf"}) {
		t.Fatal("expected -x to count")
	}
	if !HasInitScript([]string{"-ex=break main", "prog"}) {
		t.Fatal("expected -ex= to count")
	}
}
