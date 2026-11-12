package pydbg

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAdapter returns a client conn and the adapter side of an in-memory pipe.
// A goroutine drains requests into the returned channel (net.Pipe is synchronous).
func fakeAdapter(t *testing.T, onEvent func(string, json.RawMessage)) (*dapConn, net.Conn, <-chan dapMessage) {
	t.Helper()
	a, b := net.Pipe()
	d := newDapConn(a, onEvent)
	t.Cleanup(func() { d.close(); b.Close() })
	reqs := make(chan dapMessage, 16)
	go func() {
		br := bufio.NewReader(b)
		for {
			data, err := readDAP(br)
			if err != nil {
				return
			}
			var m dapMessage
			if json.Unmarshal(data, &m) == nil {
				reqs <- m
			}
		}
	}()
	return d, b, reqs
}

func TestFramingRoundTrip(t *testing.T) {
	var sb strings.Builder
	in := dapMessage{Seq: 3, Type: "request", Command: "threads", Arguments: map[string]int{"a": 1}}
	if err := writeDAP(&sb, in); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sb.String(), "Content-Length: ") || !strings.Contains(sb.String(), "\r\n\r\n{") {
		t.Fatalf("bad frame %q", sb.String())
	}
	data, err := readDAP(bufio.NewReader(strings.NewReader(sb.String())))
	if err != nil {
		t.Fatal(err)
	}
	var out dapMessage
	if err := json.Unmarshal(data, &out); err != nil || out.Command != "threads" || out.Seq != 3 {
		t.Fatalf("got %+v %v", out, err)
	}
}

func TestOutOfOrderRepliesAndEvents(t *testing.T) {
	var mu sync.Mutex
	var evs []string
	d, b, reqs := fakeAdapter(t, func(n string, _ json.RawMessage) {
		mu.Lock()
		evs = append(evs, n)
		mu.Unlock()
	})
	c1, err := d.start("first", nil)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := d.start("second", nil)
	if err != nil {
		t.Fatal(err)
	}
	r1, r2 := <-reqs, <-reqs
	if r1.Command != "first" || r2.Command != "second" {
		t.Fatalf("order %s %s", r1.Command, r2.Command)
	}
	go func() {
		writeDAP(b, dapMessage{Seq: 1, Type: "event", Event: "output"})
		writeDAP(b, dapMessage{Seq: 2, Type: "response", RequestSeq: r2.Seq, Command: "second", Success: true, Body: json.RawMessage(`{"n":2}`)})
		writeDAP(b, dapMessage{Seq: 3, Type: "event", Event: "stopped"})
		writeDAP(b, dapMessage{Seq: 4, Type: "response", RequestSeq: r1.Seq, Command: "first", Success: true, Body: json.RawMessage(`{"n":1}`)})
	}()
	b1, err := c1.wait(2 * time.Second)
	if err != nil || string(b1) != `{"n":1}` {
		t.Fatalf("first: %s %v", b1, err)
	}
	b2, err := c2.wait(2 * time.Second)
	if err != nil || string(b2) != `{"n":2}` {
		t.Fatalf("second: %s %v", b2, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(evs)
		mu.Unlock()
		if n == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(evs) != 2 || evs[0] != "output" || evs[1] != "stopped" {
		t.Fatalf("events %v", evs)
	}
}

func TestErrorResponse(t *testing.T) {
	d, b, reqs := fakeAdapter(t, nil)
	go func() {
		r := <-reqs
		writeDAP(b, dapMessage{Type: "response", RequestSeq: r.Seq, Command: r.Command, Success: false, Message: "boom"})
	}()
	_, err := d.call("evaluate", nil, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestTimeoutAndClosed(t *testing.T) {
	d, b, _ := fakeAdapter(t, nil)
	_, err := d.call("threads", nil, 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("err = %v", err)
	}
	b.Close()
	_, err = d.call("threads", nil, time.Second)
	if err == nil {
		t.Fatal("want error after close")
	}
}
