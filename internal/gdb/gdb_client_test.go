package gdb

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/ptyx"
)

func TestNewGDBClientStartsAndCloses(t *testing.T) {
	prog := filepath.Join("..", "..", "hello")
	if _, err := os.Stat(prog); err != nil {
		t.Skip("hello binary not present")
	}

	client, err := NewGDBClient("gdb", prog)
	if err != nil {
		t.Skipf("gdb/pty unavailable: %v", err)
	}
	defer client.Close()

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

func TestNewGDBClientRequiresProg(t *testing.T) {
	_, err := NewGDBClient("gdb", "")
	if err == nil {
		t.Fatal("expected error for empty prog")
	}
}

func TestConcurrentSendSerialized(t *testing.T) {
	prog := filepath.Join("..", "..", "hello")
	if _, err := os.Stat(prog); err != nil {
		t.Skip("hello binary not present")
	}
	client, err := NewGDBClient("gdb", prog)
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
