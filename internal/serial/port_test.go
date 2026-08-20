package serial

import (
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

type duplex struct {
	r *os.File
	w *os.File
}

func (d *duplex) Read(b []byte) (int, error)  { return d.r.Read(b) }
func (d *duplex) Write(b []byte) (int, error) { return d.w.Write(b) }
func (d *duplex) Close() error {
	_ = d.r.Close()
	return d.w.Close()
}

var _ io.ReadWriteCloser = (*duplex)(nil)

func TestPortConcurrentReadWrite(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})

	p := &Port{rw: &duplex{r: pr, w: pw}, name: "test"}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 64)
		_, _ = p.Read(buf)
	}()

	time.Sleep(20 * time.Millisecond)
	if _, err := p.Write([]byte("tx")); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Write blocked while Read is waiting")
	}
}
