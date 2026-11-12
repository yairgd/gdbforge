package pydbg

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yairgd/termforge/ptyx"
	"golang.org/x/term"
)

const (
	prompt      = PromptToken + " "
	maxResult   = 4000
	runningText = "target is running (Ctrl-C to interrupt)"
	readyText   = "Python debugger ready (debugpy). Type Python or debugger commands."
)

// console is the REPL on the slave side of the console PTY. gdbforge writes
// typed bytes to the master; the REPL echoes, prints results and, through
// event, the stop banners. Lines are dispatched pdb style: debugger words win,
// anything else is Python evaluated in the selected frame.
type console struct {
	c     *Client
	term  *term.Terminal
	slave *os.File

	// Output is held until the first Subscribe (the PTY has no replay), and the
	// first prompt waits for that and for release (the entry stop).
	pmu      sync.Mutex
	attached bool
	released bool
	pend     []string
	ready    chan struct{} // closed when the first prompt may be shown
	done     chan struct{} // closed when the REPL goroutine returns
	closed   atomic.Bool

	dmu sync.Mutex // serializes dispatch (typed lines and RunLine)

	smu    sync.Mutex // guards the fields below
	file   string     // file of the last stop or selected frame
	level  int        // selected frame
	temp   map[int]bool
	exited bool
}

// newConsole opens the slave side of c's console PTY and starts the REPL.
// The first prompt waits for release (or the entry stop banner).
func newConsole(c *Client) (*console, error) {
	slave, err := os.OpenFile(c.TTY.SlaveName(), os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open console tty: %w", err)
	}
	if _, err := term.MakeRaw(int(slave.Fd())); err != nil {
		slave.Close()
		return nil, fmt.Errorf("console tty raw mode: %w", err)
	}
	k := &console{
		c: c, slave: slave, term: term.NewTerminal(slave, prompt),
		ready: make(chan struct{}), done: make(chan struct{}), temp: map[int]bool{},
	}
	go k.run()
	return k, nil
}

// release says the entry stop (or its absence) is known.
func (k *console) release() {
	k.pmu.Lock()
	k.released = true
	k.openLocked()
	k.pmu.Unlock()
}

// attach is called when a reader subscribed to the console output: it flushes
// the held text, then lets the first prompt appear.
func (k *console) attach() {
	k.pmu.Lock()
	defer k.pmu.Unlock()
	if k.attached {
		return
	}
	k.attached = true
	for _, s := range k.pend {
		_, _ = k.term.Write([]byte(s))
	}
	k.pend = nil
	k.openLocked()
}

func (k *console) openLocked() {
	if (k.attached && k.released || k.closed.Load()) && k.ready != nil {
		select {
		case <-k.ready:
		default:
			close(k.ready)
		}
	}
}

// Subscribe is the console output of ptyx.Session; the first call also
// releases the startup text and the first prompt.
func (c *Client) Subscribe() (<-chan ptyx.PtyOutputMsg, func()) {
	ch, cancel := c.TTY.Subscribe()
	if c.console != nil {
		c.console.attach()
	}
	return ch, cancel
}

// stop ends the REPL goroutine and closes the slave side; safe to call twice.
func (k *console) stop() {
	k.closed.Store(true)
	k.pmu.Lock()
	k.openLocked()
	k.pmu.Unlock()
	k.slave.Close()
	select {
	case <-k.done:
	case <-time.After(2 * time.Second):
	}
}

// print writes text (newline terminated) above the prompt, which the
// terminal redraws when a line is being edited.
func (k *console) print(format string, a ...any) {
	if k.closed.Load() {
		return
	}
	text := fmt.Sprintf(format, a...) + "\n"
	k.pmu.Lock()
	defer k.pmu.Unlock()
	if !k.attached {
		k.pend = append(k.pend, text)
		return
	}
	_, _ = k.term.Write([]byte(text))
}

func (k *console) run() {
	defer close(k.done)
	k.print("%s", readyText)
	<-k.ready
	eofs := 0
	for !k.closed.Load() {
		if w, h, err := term.GetSize(int(k.slave.Fd())); err == nil && w > 0 && h > 0 {
			_ = k.term.SetSize(w, h)
		}
		line, err := k.term.ReadLine()
		if k.closed.Load() {
			return
		}
		if err == io.EOF {
			// Ctrl-C (and Ctrl-D on an empty line) end ReadLine with EOF.
			if eofs++; eofs > 20 {
				return
			}
			k.print("^C")
			if k.c.Running() {
				_ = k.c.Pause()
			}
			continue
		}
		if err != nil && err != term.ErrPasteIndicator {
			return
		}
		eofs = 0
		k.dispatch(line)
	}
}

// RunLine executes one line exactly as if it was typed on the console.
func (c *Client) RunLine(line string) {
	if c.console != nil {
		c.console.dispatch(line)
	}
}

// event prints the banner of a debugger event; called from Client.emit.
func (k *console) event(ev Event) {
	switch ev.Kind {
	case Stopped:
		bp := k.breakAt(ev.File, ev.Line)
		k.smu.Lock()
		k.file, k.level, k.exited = ev.File, 0, false
		tmp := bp > 0 && k.temp[bp]
		delete(k.temp, bp)
		k.smu.Unlock()
		k.print("%s", stopBanner(ev, bp))
		if tmp {
			go k.c.ClearBreak(ev.File, ev.Line, bp)
		}
		k.release()
	case Exited:
		k.smu.Lock()
		k.exited = true
		k.smu.Unlock()
		k.print("Process has exited with status %d", ev.ExitCode)
		k.release()
	case FrameSelected:
		k.smu.Lock()
		k.file, k.level = ev.File, ev.Level
		k.smu.Unlock()
		k.print("%s", frameBanner(ev))
	}
}

// breakAt returns the number of the enabled breakpoint at file:line, or 0.
func (k *console) breakAt(file string, line int) int {
	for _, b := range k.c.Breakpoints() {
		if b.Enabled && b.File == file && b.Line == line {
			return b.Number
		}
	}
	return 0
}

// debuggerWords are the commands that win over Python.
var debuggerWords = map[string]bool{
	"b": true, "break": true, "tbreak": true, "clear": true, "delete": true, "disable": true,
	"enable": true, "condition": true, "c": true, "continue": true, "n": true, "next": true,
	"s": true, "step": true, "finish": true, "up": true, "down": true, "frame": true, "bt": true,
	"stack": true, "threads": true, "thread": true, "breakpoints": true, "files": true,
	"restart": true, "run": true, "start": true, "quit": true, "exit": true, "help": true,
	"pause": true,
}

// parseCommand splits a line into a debugger word and its arguments. Python
// lines (`!expr`, other words, or a word used as a name: `n = 3`, `c.x`)
// return cmd "!" and the expression.
func parseCommand(line string) (cmd, args string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	if strings.HasPrefix(line, "!") {
		return "!", strings.TrimSpace(line[1:])
	}
	word, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)
	// pdb habit: `p x`, `pp x` and `print x` show a value (invalid Python anyway).
	if (word == "p" || word == "pp" || word == "print") && rest != "" && !strings.ContainsAny(rest[:1], "=.[(,") {
		return "!", rest
	}
	if !debuggerWords[word] || strings.ContainsAny(rest[:min(1, len(rest))], "=.[(,") {
		return "!", line
	}
	return word, rest
}

var lineRe = regexp.MustCompile(`^(.*):(\d+)$`)

// parseLocation reads `file:line`, `line` (relative to cur) or a function
// name (file "" and fn set).
func parseLocation(arg, cur string) (file string, line int, fn string, err error) {
	if m := lineRe.FindStringSubmatch(arg); m != nil {
		line, _ = strconv.Atoi(m[2])
		file = m[1]
		if cur != "" && !strings.Contains(file, "/") && strings.HasSuffix(cur, "/"+file) {
			file = cur
		}
		return file, line, "", nil
	}
	if n, e := strconv.Atoi(arg); e == nil {
		if cur == "" {
			return "", 0, "", fmt.Errorf("no current file; use file:line")
		}
		return cur, n, "", nil
	}
	if arg == "" {
		return "", 0, "", fmt.Errorf("location required")
	}
	return "", 0, arg, nil
}

// stopBanner is the one-line report of a stop; bp is the user breakpoint hit (0 = none).
func stopBanner(ev Event, bp int) string {
	fn := ev.Func
	if fn == "" {
		fn = "?"
	}
	loc := fmt.Sprintf("%s() %s:%d", fn, ev.File, ev.Line)
	switch {
	case strings.Contains(ev.Reason, "exception"):
		return "> [exception] " + loc
	case bp > 0:
		return fmt.Sprintf("> [Breakpoint %d] %s", bp, loc)
	}
	return "> " + loc
}

// frameBanner reports a frame selection (not a stop).
func frameBanner(ev Event) string {
	return fmt.Sprintf("> %s() %s:%d (frame %d)", ev.Func, ev.File, ev.Line, ev.Level)
}

// truncate shortens long results.
func truncate(s string) string {
	r := []rune(s)
	if len(r) <= maxResult {
		return s
	}
	return string(r[:maxResult]) + fmt.Sprintf("... (%d more chars)", len(r)-maxResult)
}

const helpText = `Debugger commands (anything else is Python; start with ! to force Python):
  b|break [file:line | line | func]   set a breakpoint (no argument: list)
  tbreak <location>                   breakpoint removed at its first hit
  clear <file:line | N>   delete N   disable N   enable N   condition N [expr]
  c|continue  n|next  s|step  finish  pause (or Ctrl-C while running)
  up  down  frame N  bt|stack  threads  thread ID  breakpoints  files
  restart|run|start   quit|exit   help`

// dispatch runs one line; every source (typed, RunLine) comes through here.
func (k *console) dispatch(line string) {
	k.dmu.Lock()
	defer k.dmu.Unlock()
	cmd, args := parseCommand(line)
	if cmd == "" {
		return
	}
	c := k.c
	if c.Running() && cmd != "pause" && cmd != "quit" && cmd != "exit" && cmd != "help" {
		k.print("%s", runningText)
		return
	}
	k.smu.Lock()
	cur, level := k.file, k.level
	k.smu.Unlock()
	fail := func(err error) {
		if err != nil {
			k.print("Command failed: %v", err)
		}
	}
	switch cmd {
	case "b", "break", "tbreak":
		if args == "" && cmd != "tbreak" {
			k.listBreaks()
			return
		}
		file, ln, fn, err := parseLocation(args, cur)
		if err == nil && fn != "" {
			file, ln, err = k.findFunc(fn, cur)
		}
		if err != nil {
			fail(err)
			return
		}
		n, err := c.SetBreak(file, ln)
		if err != nil {
			fail(err)
			return
		}
		if cmd == "tbreak" {
			k.smu.Lock()
			k.temp[n] = true
			k.smu.Unlock()
		}
		k.print("Breakpoint %d set at %s:%d", n, file, ln)
	case "clear":
		if n, err := strconv.Atoi(args); err == nil {
			if err := c.ClearBreak("", 0, n); err != nil {
				fail(err)
			} else {
				k.print("Deleted breakpoint %d", n)
			}
			return
		}
		file, ln, _, err := parseLocation(args, cur)
		if err != nil {
			fail(err)
			return
		}
		fail(c.ClearBreak(file, ln, 0))
	case "delete", "disable", "enable", "condition":
		num, rest, _ := strings.Cut(args, " ")
		n, err := strconv.Atoi(num)
		if err != nil {
			fail(fmt.Errorf("usage: %s N%s", cmd, map[bool]string{true: " expr"}[cmd == "condition"]))
			return
		}
		switch cmd {
		case "delete":
			fail(c.ClearBreak("", 0, n))
		case "disable":
			fail(c.EnableBreak(n, false))
		case "enable":
			fail(c.EnableBreak(n, true))
		default:
			fail(c.SetCondition(n, strings.TrimSpace(rest)))
		}
	case "c", "continue":
		fail(c.Continue())
	case "n", "next":
		fail(c.Next())
	case "s", "step":
		fail(c.StepIn())
	case "finish":
		fail(c.StepOut())
	case "pause":
		fail(c.Pause())
	case "up":
		fail(c.SelectFrame(level + 1))
	case "down":
		if level == 0 {
			fail(fmt.Errorf("already at the newest frame"))
			return
		}
		fail(c.SelectFrame(level - 1))
	case "frame":
		n := level
		if args != "" {
			var err error
			if n, err = strconv.Atoi(args); err != nil {
				fail(fmt.Errorf("usage: frame N"))
				return
			}
		}
		fail(c.SelectFrame(n))
	case "bt", "stack":
		st, err := c.Stack()
		if err != nil {
			fail(err)
			return
		}
		for _, f := range st {
			k.print("#%d  %s() %s:%d", f.Level, f.Func, f.File, f.Line)
		}
	case "threads":
		ths, err := c.Threads()
		if err != nil {
			fail(err)
			return
		}
		for _, t := range ths {
			mark := " "
			if t.Current {
				mark = "*"
			}
			k.print("%s %s  %s  %s() %s:%d", mark, t.ID, t.Name, t.Func, t.File, t.Line)
		}
	case "thread":
		fail(c.SelectThread(args))
	case "breakpoints":
		k.listBreaks()
	case "files":
		files, err := c.SourceFiles()
		fail(err)
		for _, f := range files {
			k.print("%s", f)
		}
	case "restart", "run", "start":
		fail(c.Restart())
	case "quit", "exit":
		// Close waits for this goroutine, so it must run on its own.
		go c.Close()
	case "help":
		k.print("%s", helpText)
	default:
		res, err := c.Evaluate(args)
		if err != nil {
			fail(err)
		} else if res != "" {
			k.print("%s", truncate(res))
		}
	}
}

func (k *console) listBreaks() {
	bps := k.c.Breakpoints()
	if len(bps) == 0 {
		k.print("No breakpoints")
	}
	for _, b := range bps {
		s := fmt.Sprintf("%d  %s  %s:%d", b.Number, map[bool]string{true: "enabled", false: "disabled"}[b.Enabled], b.File, b.Line)
		if b.Condition != "" {
			s += "  if " + b.Condition
		}
		k.print("%s", s)
	}
}

// findFunc locates `def fn(` in the current file, then in the project's
// sources, and returns the first body line.
func (k *console) findFunc(fn, cur string) (string, int, error) {
	re := regexp.MustCompile(`^\s*(async\s+)?def\s+` + regexp.QuoteMeta(fn) + `\s*\(`)
	var files []string
	if cur != "" {
		files = append(files, cur)
	}
	if all, err := k.c.SourceFiles(); err == nil {
		files = append(files, all...)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for i, l := range strings.Split(string(data), "\n") {
			if re.MatchString(l) {
				return f, i + 2, nil
			}
		}
	}
	return "", 0, fmt.Errorf("function %q not found; use file:line", fn)
}
