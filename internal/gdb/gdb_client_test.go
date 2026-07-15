package gdb

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewGDBClientStartsAndCloses(t *testing.T) {
	prog := filepath.Join("..", "..", "hello")
	if _, err := os.Stat(prog); err != nil {
		t.Skip("hello binary not present")
	}

	client, ch, err := NewGDBClient("gdb", prog)
	if err != nil {
		t.Skipf("gdb/pty unavailable: %v", err)
	}
	defer client.Close()

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
	_, _, err := NewGDBClient("gdb", "")
	if err == nil {
		t.Fatal("expected error for empty prog")
	}
}
