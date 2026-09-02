package ptyx

import (
	"io"
	"os"
	"strings"
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

// TestTTYImplementsSession verifies the unified TTY satisfies core.Session.
func TestTTYImplementsSession(t *testing.T) {
	var _ core.Session = (*TTY)(nil)
}

func TestFanOutDeliversToMultipleSubscribers(t *testing.T) {
	tty := &TTY{
		master: os.NewFile(0, "fake"),
		subs:   make(map[*subscriber]struct{}),
	}
	ch1, cancel1 := tty.Subscribe()
	ch2, cancel2 := tty.Subscribe()
	defer cancel1()
	defer cancel2()

	tty.broadcast(core.PtyOutputMsg{Data: "hello"})

	got1 := <-ch1
	got2 := <-ch2
	if got1.Data != "hello" || got2.Data != "hello" {
		t.Fatalf("got %q and %q, want hello", got1.Data, got2.Data)
	}
}

func TestSubscribeCancelStopsDelivery(t *testing.T) {
	tty := &TTY{
		master: os.NewFile(0, "fake"),
		subs:   make(map[*subscriber]struct{}),
	}
	ch, cancel := tty.Subscribe()
	cancel()

	tty.broadcast(core.PtyOutputMsg{Data: "x"})
	select {
	case msg := <-ch:
		t.Fatalf("unexpected delivery after cancel: %q", msg.Data)
	default:
	}
}

func TestAttachPathHasNoMaster(t *testing.T) {
	tty := AttachPath("/dev/pts/99")
	if tty.HasMaster() {
		t.Fatal("AttachPath must not have master")
	}
	if !tty.IsExternal() {
		t.Fatal("AttachPath must be external")
	}
	if tty.SlaveName() != "/dev/pts/99" {
		t.Fatalf("SlaveName=%q", tty.SlaveName())
	}
	ch, cancel := tty.Subscribe()
	defer cancel()
	if _, ok := <-ch; ok {
		t.Fatal("expected closed subscribe channel for external tty")
	}
	if err := tty.Send("x"); err != io.ErrClosedPipe {
		t.Fatalf("Send err=%v want ErrClosedPipe", err)
	}
}

func TestStartProcessSubscribe(t *testing.T) {
	tty, err := Start([]string{"echo", "hello"}, Options{})
	if err != nil {
		t.Skipf("start echo: %v", err)
	}
	defer tty.Close()

	if tty.Pid() == 0 {
		t.Fatal("expected pid")
	}

	ch, cancel := tty.Subscribe()
	defer cancel()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg.Err != nil {
				t.Fatalf("read err: %v", msg.Err)
			}
			if strings.Contains(msg.Data, "hello") {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for echo output")
		}
	}
}
