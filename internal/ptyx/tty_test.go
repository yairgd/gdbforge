package ptyx

import (
	"testing"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
)

func TestOpenTTYSubscribeAndWrite(t *testing.T) {
	tty, err := OpenTTY()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer tty.Close()

	if tty.SlaveName() == "" {
		t.Fatal("empty slave name")
	}

	ch, cancel := tty.Subscribe()
	defer cancel()

	// Writing to the slave appears on the master (what Subscribe reads).
	slave, err := openSlave(tty.SlaveName())
	if err != nil {
		t.Fatalf("open slave: %v", err)
	}
	defer slave.Close()

	if _, err := slave.Write([]byte("hello")); err != nil {
		t.Fatalf("slave write: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				t.Fatal("subscribe closed early")
			}
			if msg.Err != nil {
				t.Fatalf("read err: %v", msg.Err)
			}
			if msg.Data != "" {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for inferior tty output")
		}
	}
}

func TestTTYSendRawToSlave(t *testing.T) {
	tty, err := OpenTTY()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer tty.Close()

	slave, err := openSlave(tty.SlaveName())
	if err != nil {
		t.Fatalf("open slave: %v", err)
	}
	defer slave.Close()

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := slave.Read(buf)
		done <- string(buf[:n])
	}()

	if err := tty.SendRaw("hi\n"); err != nil {
		t.Fatalf("SendRaw: %v", err)
	}

	select {
	case got := <-done:
		if got == "" {
			t.Fatal("empty read from slave")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout reading from slave")
	}
}

func TestTTYImplementsDebugger(t *testing.T) {
	var _ core.Debugger = (*TTY)(nil)
}
