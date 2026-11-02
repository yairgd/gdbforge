package gdb

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yairgd/termforge/ptyx"
)

// Run states reported by RunTracker.
const (
	StateRunningName      = "running"
	StateStoppedName      = "stopped"
	StateDisconnectedName = "disconnected"
)

// StopInfo is where and why the target last stopped.
type StopInfo struct {
	Reason string
	Addr   string
	Func   string
	File   string
	Line   int
	BkptNo int // breakpoint or watchpoint number that caused the stop; 0 if none
	Thread string
}

// RunTracker follows the target's run state from GDB/MI async records (*running, *stopped,
// =thread-group-exited) on its own PTY subscription, so it does not depend on the UI thread
// or on prompts. The state is GDB's view: monitor commands that halt or resume the target
// behind GDB's back emit no record and are not seen.
type RunTracker struct {
	mu     sync.Mutex
	state  string // "" until the first record or Seed
	seq    uint64 // counts stops, so a waiter can ask for "a stop after X"
	stop   StopInfo
	buf    string
	change chan struct{} // closed and replaced on every change
	cancel func()
}

// NewRunTracker subscribes to sess and starts following it. Call Seed with the answer to
// -thread-info, because records from before the subscription are not seen.
func NewRunTracker(sess ptyx.Session) *RunTracker {
	t := &RunTracker{change: make(chan struct{})}
	ch, cancel := sess.Subscribe()
	t.cancel = cancel
	go func() {
		for msg := range ch {
			if msg.Err != nil {
				break
			}
			t.Feed(msg.Data)
		}
		t.disconnect("")
	}()
	return t
}

// Close stops following the session.
func (t *RunTracker) Close() {
	if t != nil && t.cancel != nil {
		t.cancel()
	}
}

// State is "running", "stopped", "disconnected" or "" while unknown.
func (t *RunTracker) State() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// Seq counts stops so far. Read it before sending a resume, then WaitStop(after = seq).
func (t *RunTracker) Seq() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.seq
}

// Snapshot returns the state and the last stop.
func (t *RunTracker) Snapshot() (string, StopInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state, t.stop
}

// Seed sets the state from a -thread-info answer. It applies only while the state is unknown
// or "disconnected", which covers records from before the subscription and a connect (such as
// `target remote`) whose records were not seen; a live running/stopped state is never replaced.
func (t *RunTracker) Seed(state string, stop StopInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if state == "" || (t.state != "" && t.state != StateDisconnectedName) || state == t.state {
		return
	}
	t.state, t.stop = state, stop
	t.notifyLocked()
}

// FillLocation adds the frame to a stop that came without one (a freshly connected target).
func (t *RunTracker) FillLocation(stop StopInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == StateStoppedName && t.stop.Addr == "" && t.stop.Func == "" {
		stop.Reason, stop.BkptNo = t.stop.Reason, t.stop.BkptNo
		t.stop = stop
	}
}

// MarkRunning records a resume whose ^running reply was seen but not yet processed here.
// It does nothing if a stop arrived since seq was read.
func (t *RunTracker) MarkRunning(seq uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.seq == seq && t.state != StateRunningName {
		t.state = StateRunningName
		t.notifyLocked()
	}
}

// WaitStop blocks until a stop newer than after is seen, or timeout. ok is false on timeout.
func (t *RunTracker) WaitStop(ctx context.Context, after uint64, timeout time.Duration) (StopInfo, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		t.mu.Lock()
		if t.seq > after {
			s := t.stop
			t.mu.Unlock()
			return s, true
		}
		ch := t.change
		t.mu.Unlock()
		select {
		case <-ch:
		case <-timer.C:
			return StopInfo{}, false
		case <-ctx.Done():
			return StopInfo{}, false
		}
	}
}

func (t *RunTracker) notifyLocked() {
	close(t.change)
	t.change = make(chan struct{})
}

// Feed consumes raw PTY output.
func (t *RunTracker) Feed(data string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf += data
	for {
		i := strings.IndexByte(t.buf, '\n')
		if i < 0 {
			break
		}
		line := t.buf[:i]
		t.buf = t.buf[i+1:]
		t.lineLocked(stripMILinePrefix(strings.TrimSpace(line)))
	}
	if len(t.buf) > 1<<20 { // a never-ending line without a newline
		t.buf = ""
	}
}

func (t *RunTracker) lineLocked(l string) {
	switch {
	case strings.HasPrefix(l, "^running"), strings.HasPrefix(l, "*running"):
		if t.state != StateRunningName {
			t.state = StateRunningName
			t.notifyLocked()
		}
	case strings.HasPrefix(l, "*stopped"):
		s := StopInfo{
			Reason: ExtractMIField(l, "reason"),
			Addr:   ExtractMIField(l, "addr"),
			Func:   ExtractMIField(l, "func"),
			File:   ExtractMIField(l, "fullname"),
			Thread: ExtractMIField(l, "thread-id"),
		}
		if s.File == "" {
			s.File = ExtractMIField(l, "file")
		}
		s.Line, _ = strconv.Atoi(ExtractMIField(l, "line"))
		if n, err := strconv.Atoi(ExtractMIField(l, "bkptno")); err == nil {
			s.BkptNo = n
		} else if strings.Contains(s.Reason, "watchpoint") {
			s.BkptNo, _ = strconv.Atoi(ExtractMIField(l, "number"))
		}
		t.stop = s
		t.seq++
		t.state = StateStoppedName
		if strings.HasPrefix(s.Reason, "exited") {
			t.state = StateDisconnectedName
		}
		t.notifyLocked()
	case strings.HasPrefix(l, "^connected"):
		// A target just attached (target remote) is halted; GDB sends no *stopped for it.
		if t.state != StateRunningName {
			t.state, t.stop = StateStoppedName, StopInfo{}
			t.notifyLocked()
		}
	case strings.HasPrefix(l, "=thread-group-exited"), strings.HasPrefix(l, "^exit"):
		t.disconnectLocked("exited")
	}
}

func (t *RunTracker) disconnect(reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.disconnectLocked(reason)
}

// disconnectLocked ends the session's target. It counts as a stop so waiters return.
func (t *RunTracker) disconnectLocked(reason string) {
	if t.state == StateDisconnectedName {
		return
	}
	if reason == "" {
		reason = "disconnected"
	}
	if t.state == StateRunningName {
		t.stop = StopInfo{Reason: reason}
		t.seq++
	}
	t.state = StateDisconnectedName
	t.notifyLocked()
}

// ParseThreadInfo reads the run state, and the stop location if stopped, from a -thread-info
// reply. state is "" when the reply does not say.
func ParseThreadInfo(raw string) (state string, stop StopInfo) {
	switch {
	case strings.Contains(raw, `state="running"`):
		return StateRunningName, StopInfo{}
	case strings.Contains(raw, `state="stopped"`):
		stop = StopInfo{
			Reason: "",
			Addr:   ExtractMIField(raw, "addr"),
			Func:   ExtractMIField(raw, "func"),
			File:   ExtractMIField(raw, "fullname"),
			Thread: ExtractMIField(raw, "id"),
		}
		if stop.File == "" {
			stop.File = ExtractMIField(raw, "file")
		}
		stop.Line, _ = strconv.Atoi(ExtractMIField(raw, "line"))
		return StateStoppedName, stop
	case strings.Contains(raw, "threads=[]"):
		return StateDisconnectedName, StopInfo{}
	}
	return "", StopInfo{}
}
