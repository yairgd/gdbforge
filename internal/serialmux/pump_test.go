package serialmux

import (
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestOpenLegTx(t *testing.T) {
	leg, err := openLeg()
	if err != nil {
		t.Fatal(err)
	}
	defer leg.close()

	ch := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 32)
		n, _ := leg.master.Read(buf)
		ch <- buf[:n]
	}()

	user, err := os.OpenFile(leg.slaveName, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	time.Sleep(50 * time.Millisecond)
	if _, err := user.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-ch:
		if string(got) != "hello" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestOpenLegRx(t *testing.T) {
	leg, err := openLeg()
	if err != nil {
		t.Fatal(err)
	}
	defer leg.close()

	user, err := os.OpenFile(leg.slaveName, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	time.Sleep(50 * time.Millisecond)

	if _, err := leg.master.Write([]byte("board")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	deadline := time.Now().Add(2 * time.Second)
	for {
		n, _ := user.Read(buf)
		if n > 0 {
			if string(buf[:n]) != "board" {
				t.Fatalf("got %q", buf[:n])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPTYDirectRead(t *testing.T) {
	m, s, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	name := s.Name()
	_ = s.Close()

	ch := make(chan int, 1)
	go func() {
		buf := make([]byte, 32)
		n, _ := m.Read(buf)
		ch <- n
	}()

	s2, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if _, err := s2.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-ch:
		if n == 0 {
			t.Fatal("no data read")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	_ = m.Close()
}
