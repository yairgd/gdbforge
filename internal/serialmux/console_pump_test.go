package serialmux

import (
	"io"
	"os"
	"testing"
	"time"
)

type testPort struct {
	rx *os.File
	tx *os.File
}

func newTestPort() (*testPort, *os.File, error) {
	rxR, rxW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	txR, txW, err := os.Pipe()
	if err != nil {
		rxR.Close()
		rxW.Close()
		return nil, nil, err
	}
	_ = rxW.Close()
	return &testPort{rx: rxR, tx: txW}, txR, nil
}

func (p *testPort) Read(b []byte) (int, error)  { return p.rx.Read(b) }
func (p *testPort) Write(b []byte) (int, error) { return p.tx.Write(b) }
func (p *testPort) Close() error {
	var err error
	if p.rx != nil {
		err = p.rx.Close()
	}
	if p.tx != nil {
		if e := p.tx.Close(); err == nil {
			err = e
		}
	}
	return err
}

func TestConsolePumpForwards(t *testing.T) {
	port, txR, err := newTestPort()
	if err != nil {
		t.Fatal(err)
	}
	defer port.Close()
	defer txR.Close()

	termLeg, err := openTermLeg()
	if err != nil {
		t.Fatal(err)
	}
	defer termLeg.Close()

	m := &Mux{
		termLeg: termLeg,
		port:    port,
		owner:   OwnerTerminal,
		stop:    make(chan struct{}),
	}
	m.wg.Add(1)
	go m.pumpConsoleToUSB()

	user, err := os.OpenFile(termLeg.SlaveName(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	time.Sleep(50 * time.Millisecond)

	want := []byte("ls\r")
	if _, err := user.Write(want); err != nil {
		t.Fatal(err)
	}

	got := make([]byte, len(want))
	deadline := time.Now().Add(2 * time.Second)
	for n := 0; n < len(want); {
		rn, err := txR.Read(got[n:])
		if rn > 0 {
			n += rn
			continue
		}
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout: got %q want %q", got[:n], want)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	close(m.stop)
	m.wg.Wait()
}
