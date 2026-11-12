package pydbg

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yairgd/termforge/ptyx"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

const (
	PromptToken = "(py)"

	startupWait = 30 * time.Second
	maxFrames   = 50
)

// EventKind tells PYBackend what changed.
type EventKind int

const (
	Stopped EventKind = iota + 1
	Exited
	Running
	BreaksChanged
	FrameSelected
)

// Event is a debugger state change, delivered through SetOnEvent.
type Event struct {
	Kind                  EventKind
	File, Func, Reason    string
	ThreadID              string
	Line, Level, ExitCode int
}

// Options configures NewClient.
type Options struct {
	InferiorTTY string // external pts path, "" = internal
}

// Client is a Python debug session: debugpy driven over DAP for machine
// control, a console PTY for human I/O (embedded; the REPL on it is attached
// separately) and an inferior PTY for the program's stdio.
type Client struct {
	*ptyx.TTY
	inferior *ptyx.TTY
	python   string
	args     []string

	mu       sync.Mutex
	d        *dapConn
	cmd      *exec.Cmd
	procDone chan struct{}
	gen      int
	initCh   chan struct{}
	running  bool
	exitSent bool
	thread   int
	frames   []dapFrame
	level    int
	onEvent  func(Event)
	queued   []Event // events emitted before SetOnEvent
	entry    *entry  // pending stop-on-entry breakpoint, nil once hit

	bmu       sync.Mutex // guards breaks and serializes setBreakpoints
	breaks    []*breakpoint
	nextBreak int

	console *console // REPL on the console PTY; set once in NewClient
}

var _ ptyx.Session = (*Client)(nil)

// NewClient starts `python -m debugpy` on args (`script.py a b` or `-m pytest x`)
// and attaches to it. The target is stopped on its first line.
func NewClient(python string, args []string, opts Options) (*Client, error) {
	if python == "" {
		python = "python3"
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("python arguments required (script or -m module)")
	}
	if _, err := exec.LookPath(python); err != nil {
		return nil, fmt.Errorf("cannot find python %q: %w", python, err)
	}
	if out, err := exec.Command(python, "-c", "import debugpy").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("debugpy is not available for %s (run `pip install debugpy`): %s",
			python, strings.TrimSpace(string(out)))
	}

	c := &Client{python: python, args: args}
	if ext := strings.TrimSpace(opts.InferiorTTY); ext != "" {
		c.inferior = ptyx.AttachPath(ext)
	} else {
		inf, err := ptyx.Open()
		if err != nil {
			return nil, err
		}
		c.inferior = inf
	}
	con, err := ptyx.Open()
	if err != nil {
		c.inferior.Close()
		return nil, err
	}
	c.TTY = con
	// The REPL starts first so that it sees every event, the entry stop included.
	if c.console, err = newConsole(c); err != nil {
		c.Close()
		return nil, err
	}

	if err := c.spawn(); err != nil {
		c.Close()
		return nil, err
	}
	c.mu.Lock()
	noEntry := c.entry == nil
	c.mu.Unlock()
	if noEntry {
		c.console.release() // no entry stop to wait for: show the first prompt
	}
	return c, nil
}

// pickListenAddr returns a free localhost TCP address.
func pickListenAddr() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr, nil
}

// spawn starts python under debugpy, attaches and sends the breakpoint table.
func (c *Client) spawn() error {
	addr, err := pickListenAddr()
	if err != nil {
		return err
	}
	argv := append([]string{"-m", "debugpy", "--listen", addr, "--wait-for-client"}, c.args...)
	cmd := exec.Command(c.python, argv...)
	env := os.Environ()
	if cwd, err := os.Getwd(); err == nil {
		pp := cwd
		if old := os.Getenv("PYTHONPATH"); old != "" {
			pp += string(os.PathListSeparator) + old
		}
		env = append(env, "PYTHONPATH="+pp)
	}
	cmd.Env = env
	slave, err := os.OpenFile(c.inferior.SlaveName(), os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open inferior tty: %w", err)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	_ = slave.Close()
	if err != nil {
		return fmt.Errorf("start python: %w", err)
	}
	done := make(chan struct{})
	c.mu.Lock()
	c.gen++
	gen := c.gen
	c.mu.Unlock()
	// debugpy does not always report the exit code of an attached process:
	// when the process ends without an exited event, report its status.
	go func() {
		_ = cmd.Wait()
		close(done)
		c.mu.Lock()
		stale := gen != c.gen
		c.mu.Unlock()
		if !stale {
			c.exited(cmd.ProcessState.ExitCode())
		}
	}()

	c.mu.Lock()
	c.cmd, c.procDone = cmd, done
	c.initCh = make(chan struct{}, 1)
	c.running, c.exitSent = true, false
	c.entry = entryPoint(c.python, c.args)
	c.thread, c.frames, c.level = 0, nil, 0
	c.mu.Unlock()

	conn, err := dialRetry(addr, startupWait, done)
	if err != nil {
		return err
	}
	d := newDapConn(conn, func(name string, body json.RawMessage) { c.onDAP(gen, name, body) })
	c.mu.Lock()
	c.d = d
	c.mu.Unlock()

	if _, err := d.call("initialize", map[string]any{
		"clientID": "gdbforge", "adapterID": "pydbg", "pathFormat": "path",
		"linesStartAt1": true, "columnsStartAt1": true,
	}, requestTimeout); err != nil {
		return err
	}
	// debugpy answers attach only after configurationDone.
	attach, err := d.start("attach", map[string]any{"justMyCode": true, "stopOnEntry": true})
	if err != nil {
		return err
	}
	select {
	case <-c.initCh:
	case <-time.After(startupWait):
		return fmt.Errorf("debugpy: no initialized event")
	case <-done:
		return fmt.Errorf("python exited during startup")
	}
	if err := c.syncAll(); err != nil {
		return err
	}
	if _, err := d.call("configurationDone", nil, requestTimeout); err != nil {
		return err
	}
	if _, err := attach.wait(startupWait); err != nil {
		return err
	}
	return nil
}

// dialRetry connects to addr until it succeeds, timeout passes or the process ends.
func dialRetry(addr string, timeout time.Duration, procDone <-chan struct{}) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		select {
		case <-procDone:
			return nil, fmt.Errorf("python exited before debugpy listened (see the inferior tty output)")
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("debugpy dial %s: %w", addr, lastErr)
}

func (c *Client) conn() *dapConn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.d
}

// InferiorTTY is the PTY carrying the program's stdin/stdout/stderr.
func (c *Client) InferiorTTY() *ptyx.TTY { return c.inferior }

// TakeStartupOutput returns text to show before the first prompt (none yet).
func (c *Client) TakeStartupOutput() string { return "" }

// SetOnEvent sets the event handler (called from the DAP goroutine); events
// emitted earlier are delivered first.
func (c *Client) SetOnEvent(fn func(Event)) {
	c.mu.Lock()
	c.onEvent = fn
	q := c.queued
	c.queued = nil
	c.mu.Unlock()
	if fn != nil {
		for _, ev := range q {
			fn(ev)
		}
	}
}

func (c *Client) emit(ev Event) {
	if c.console != nil {
		c.console.event(ev)
	}
	c.mu.Lock()
	fn := c.onEvent
	if fn == nil {
		c.queued = append(c.queued, ev)
	}
	c.mu.Unlock()
	if fn != nil {
		fn(ev)
	}
}

// Running reports whether the target is executing.
func (c *Client) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// onDAP handles an adapter event; stale generations (before Restart) are dropped.
func (c *Client) onDAP(gen int, name string, body json.RawMessage) {
	c.mu.Lock()
	if gen != c.gen {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	switch name {
	case "initialized":
		select {
		case c.initCh <- struct{}{}:
		default:
		}
	case "stopped":
		var b struct {
			Reason   string `json:"reason"`
			ThreadID int    `json:"threadId"`
		}
		_ = json.Unmarshal(body, &b)
		c.mu.Lock()
		c.running = false
		if b.ThreadID != 0 {
			c.thread = b.ThreadID
		}
		c.level = 0
		c.mu.Unlock()
		ev := Event{Kind: Stopped, Reason: b.Reason}
		if tid, err := c.curThread(); err == nil {
			ev.ThreadID = strconv.Itoa(tid)
			if frames, err := c.fetchFrames(tid); err == nil && len(frames) > 0 {
				ev.File, ev.Line, ev.Func = frames[0].file(), frames[0].Line, frames[0].Name
			}
		}
		c.entryHit(&ev)
		c.emit(ev)
	case "continued":
		c.mu.Lock()
		was := c.running
		c.running = true
		c.mu.Unlock()
		if !was {
			c.emit(Event{Kind: Running})
		}
	case "exited":
		var b struct {
			ExitCode int `json:"exitCode"`
		}
		_ = json.Unmarshal(body, &b)
		c.exited(b.ExitCode)
	}
}

// exited emits Exited once per process.
func (c *Client) exited(code int) {
	c.mu.Lock()
	sent := c.exitSent
	c.exitSent, c.running = true, false
	c.mu.Unlock()
	if !sent {
		c.emit(Event{Kind: Exited, ExitCode: code})
	}
}

// fetchFrames loads and caches the stack of a thread.
func (c *Client) fetchFrames(tid int) ([]dapFrame, error) {
	d := c.conn()
	if d == nil {
		return nil, fmt.Errorf("not connected")
	}
	body, err := d.call("stackTrace", map[string]any{"threadId": tid, "startFrame": 0, "levels": maxFrames}, requestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		StackFrames []dapFrame `json:"stackFrames"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if tid == c.thread {
		c.frames = r.StackFrames
	}
	c.mu.Unlock()
	return r.StackFrames, nil
}

// curThread returns the selected thread, asking the adapter when none is known.
func (c *Client) curThread() (int, error) {
	c.mu.Lock()
	t := c.thread
	d := c.d
	c.mu.Unlock()
	if t != 0 {
		return t, nil
	}
	if d == nil {
		return 0, fmt.Errorf("not connected")
	}
	body, err := d.call("threads", nil, requestTimeout)
	if err != nil {
		return 0, err
	}
	var r struct {
		Threads []dapThread `json:"threads"`
	}
	if err := json.Unmarshal(body, &r); err != nil || len(r.Threads) == 0 {
		return 0, fmt.Errorf("no threads")
	}
	c.mu.Lock()
	c.thread = r.Threads[0].ID
	c.mu.Unlock()
	return r.Threads[0].ID, nil
}

// resume sends a run-control request; the outcome arrives as an Event.
func (c *Client) resume(command string) error {
	c.mu.Lock()
	if c.exitSent {
		c.mu.Unlock()
		return fmt.Errorf("the program has exited (use restart)")
	}
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("target is running")
	}
	c.running = true
	d := c.d
	c.mu.Unlock()
	tid, err := c.curThread()
	if err == nil {
		_, err = d.call(command, map[string]any{"threadId": tid}, requestTimeout)
	}
	if err != nil {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		return err
	}
	c.emit(Event{Kind: Running})
	return nil
}

func (c *Client) Continue() error { return c.resume("continue") }
func (c *Client) Next() error     { return c.resume("next") }
func (c *Client) StepIn() error   { return c.resume("stepIn") }
func (c *Client) StepOut() error  { return c.resume("stepOut") }

// Pause interrupts a running target; it ends with a Stopped event.
func (c *Client) Pause() error {
	if !c.Running() {
		return fmt.Errorf("target is not running")
	}
	tid, err := c.curThread()
	if err != nil {
		return err
	}
	_, err = c.conn().call("pause", map[string]any{"threadId": tid}, requestTimeout)
	return err
}

// Restart kills the process and starts it again with the same arguments and
// breakpoint table; both PTYs are kept.
func (c *Client) Restart() error {
	c.killProc()
	c.emit(Event{Kind: Running})
	if err := c.spawn(); err != nil {
		c.killProc()
		return err
	}
	return nil
}

// killProc drops the connection and kills and reaps the process group.
func (c *Client) killProc() {
	c.mu.Lock()
	c.gen++
	d, cmd, done := c.d, c.cmd, c.procDone
	c.d, c.cmd, c.procDone = nil, nil, nil
	c.running = false
	c.mu.Unlock()
	if d != nil {
		d.close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
}

// Close disconnects, kills python and releases both PTYs.
func (c *Client) Close() {
	if c == nil {
		return
	}
	if d := c.conn(); d != nil {
		if call, err := d.start("disconnect", map[string]any{"terminateDebuggee": true}); err == nil {
			_, _ = call.wait(500 * time.Millisecond)
		}
	}
	c.killProc()
	if c.console != nil {
		c.console.stop()
	}
	c.TTY.Close()
	c.inferior.Close()
}

// requireStopped guards read requests, which block while the target runs.
func (c *Client) requireStopped() (*dapConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return nil, fmt.Errorf("target is running")
	}
	if c.d == nil {
		return nil, fmt.Errorf("not connected")
	}
	return c.d, nil
}

// Threads lists the threads; the top frame is filled in for each.
func (c *Client) Threads() ([]models.ThreadInfo, error) {
	d, err := c.requireStopped()
	if err != nil {
		return nil, err
	}
	body, err := d.call("threads", nil, requestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		Threads []dapThread `json:"threads"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	c.mu.Lock()
	cur := c.thread
	c.mu.Unlock()
	out := make([]models.ThreadInfo, 0, len(r.Threads))
	for _, t := range r.Threads {
		ti := threadInfoFromDAP(t, "stopped", t.ID == cur)
		if frames, err := c.topFrame(d, t.ID); err == nil && len(frames) > 0 {
			ti.File, ti.Line, ti.Func = frames[0].file(), frames[0].Line, frames[0].Name
		}
		out = append(out, ti)
	}
	return out, nil
}

func (c *Client) topFrame(d *dapConn, tid int) ([]dapFrame, error) {
	body, err := d.call("stackTrace", map[string]any{"threadId": tid, "startFrame": 0, "levels": 1}, requestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		StackFrames []dapFrame `json:"stackFrames"`
	}
	return r.StackFrames, json.Unmarshal(body, &r)
}

// Stack lists the frames (up to 50) of the selected thread.
func (c *Client) Stack() ([]models.StackFrame, error) {
	if _, err := c.requireStopped(); err != nil {
		return nil, err
	}
	tid, err := c.curThread()
	if err != nil {
		return nil, err
	}
	frames, err := c.fetchFrames(tid)
	if err != nil {
		return nil, err
	}
	out := make([]models.StackFrame, len(frames))
	for i, f := range frames {
		out[i] = stackFrameFromDAP(i, f)
	}
	return out, nil
}

// SelectFrame makes frame level the target of Evaluate and Complete.
func (c *Client) SelectFrame(level int) error {
	if _, err := c.requireStopped(); err != nil {
		return err
	}
	c.mu.Lock()
	have := len(c.frames)
	c.mu.Unlock()
	if have == 0 {
		if _, err := c.Stack(); err != nil {
			return err
		}
	}
	c.mu.Lock()
	if level < 0 || level >= len(c.frames) {
		n := len(c.frames)
		c.mu.Unlock()
		return fmt.Errorf("frame %d out of range (0-%d)", level, n-1)
	}
	c.level = level
	f := c.frames[level]
	c.mu.Unlock()
	c.emit(Event{Kind: FrameSelected, Level: level, File: f.file(), Line: f.Line, Func: f.Name})
	return nil
}

// SelectThread makes thread id current and selects its top frame.
func (c *Client) SelectThread(id string) error {
	if _, err := c.requireStopped(); err != nil {
		return err
	}
	tid, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("bad thread id %q", id)
	}
	c.mu.Lock()
	c.thread = tid
	c.frames = nil
	c.mu.Unlock()
	return c.SelectFrame(0)
}

func (c *Client) frameID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.level < len(c.frames) {
		return c.frames[c.level].ID
	}
	return 0
}

// Evaluate runs an expression or statement in the selected frame.
func (c *Client) Evaluate(expr string) (string, error) {
	d, err := c.requireStopped()
	if err != nil {
		return "", err
	}
	args := map[string]any{"expression": expr, "context": "repl"}
	if id := c.frameID(); id != 0 {
		args["frameId"] = id
	}
	body, err := d.call("evaluate", args, requestTimeout)
	if err != nil {
		// A raised exception arrives as a traceback; keep its last line.
		msg := strings.TrimSpace(err.Error())
		if i := strings.LastIndex(msg, "\n"); i >= 0 {
			msg = strings.TrimSpace(msg[i+1:])
		}
		return "", fmt.Errorf("%s", msg)
	}
	var r struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	return r.Result, nil
}

// Complete returns completions for text in the selected frame.
func (c *Client) Complete(text string) ([]string, error) {
	d, err := c.requireStopped()
	if err != nil {
		return nil, err
	}
	args := map[string]any{"text": text, "column": len([]rune(text)) + 1}
	if id := c.frameID(); id != 0 {
		args["frameId"] = id
	}
	body, err := d.call("completions", args, requestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		Targets []struct {
			Label string `json:"label"`
			Text  string `json:"text"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(r.Targets))
	for _, t := range r.Targets {
		if t.Text != "" {
			out = append(out, t.Text)
		} else {
			out = append(out, t.Label)
		}
	}
	return out, nil
}

// SourceFiles lists .py files under the current directory: the loaded modules
// when stopped, else (or when the adapter has none) a directory walk.
func (c *Client) SourceFiles() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var out []string
	if d, err := c.requireStopped(); err == nil {
		if body, err := d.call("modules", map[string]any{}, requestTimeout); err == nil {
			var r struct {
				Modules []struct {
					Path string `json:"path"`
				} `json:"modules"`
			}
			if json.Unmarshal(body, &r) == nil {
				for _, m := range r.Modules {
					if strings.HasSuffix(m.Path, ".py") && strings.HasPrefix(m.Path, cwd+string(os.PathSeparator)) {
						out = append(out, m.Path)
					}
				}
			}
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	err = filepath.WalkDir(cwd, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if de.IsDir() && p != cwd && (strings.HasPrefix(de.Name(), ".") || de.Name() == "__pycache__") {
			return filepath.SkipDir
		}
		if !de.IsDir() && strings.HasSuffix(p, ".py") {
			out = append(out, p)
		}
		return nil
	})
	return out, err
}
