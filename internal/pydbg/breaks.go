package pydbg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

// breakpoint is one row of the table. DAP setBreakpoints replaces the whole
// list of one source file, so the table lives here and a file is resent as a
// whole on every change.
type breakpoint struct {
	number   int
	file     string // absolute path
	line     int
	enabled  bool
	cond     string
	verified bool
}

// syncFile sends the enabled breakpoints of file and records which ones the
// adapter verified. Caller holds c.bmu.
func (c *Client) syncFile(file string) error {
	var rows []*breakpoint // nil row = the stop-on-entry breakpoint
	for _, b := range c.breaks {
		if b.file == file && b.enabled {
			rows = append(rows, b)
		}
	}
	if e := c.entryFor(file); e != nil {
		rows = append(rows, nil)
	}
	line := func(b *breakpoint) int {
		if b == nil {
			return c.entryFor(file).line
		}
		return b.line
	}
	sort.Slice(rows, func(i, j int) bool { return line(rows[i]) < line(rows[j]) })
	type srcBP struct {
		Line      int    `json:"line"`
		Condition string `json:"condition,omitempty"`
	}
	bps := make([]srcBP, len(rows))
	for i, b := range rows {
		bps[i] = srcBP{Line: line(b)}
		if b != nil {
			bps[i].Condition = b.cond
		}
	}
	d := c.conn()
	if d == nil {
		return fmt.Errorf("not connected")
	}
	body, err := d.call("setBreakpoints", map[string]any{
		"source":      dapSource{Path: file},
		"breakpoints": bps,
	}, requestTimeout)
	if err != nil {
		return err
	}
	var resp struct {
		Breakpoints []struct {
			Verified bool `json:"verified"`
		} `json:"breakpoints"`
	}
	_ = json.Unmarshal(body, &resp)
	for i, b := range rows {
		if b != nil {
			b.verified = i < len(resp.Breakpoints) && resp.Breakpoints[i].Verified
		}
	}
	return nil
}

// syncAll resends every file in the table (after a restart).
func (c *Client) syncAll() error {
	c.bmu.Lock()
	defer c.bmu.Unlock()
	files := map[string]bool{}
	c.mu.Lock()
	if c.entry != nil {
		files[c.entry.file] = true
	}
	c.mu.Unlock()
	for _, b := range c.breaks {
		files[b.file] = true
	}
	for f := range files {
		if err := c.syncFile(f); err != nil {
			return err
		}
	}
	return nil
}

// SetBreak adds a breakpoint (or returns the one already on that line).
func (c *Client) SetBreak(file string, line int) (int, error) {
	if abs, err := filepath.Abs(file); err == nil {
		file = abs
	}
	c.bmu.Lock()
	for _, b := range c.breaks {
		if b.file == file && b.line == line {
			n := b.number
			c.bmu.Unlock()
			return n, nil
		}
	}
	c.nextBreak++
	b := &breakpoint{number: c.nextBreak, file: file, line: line, enabled: true}
	c.breaks = append(c.breaks, b)
	err := c.syncFile(file)
	if err != nil {
		c.removeLocked(b.number)
	}
	c.bmu.Unlock()
	if err != nil {
		return 0, err
	}
	c.emit(Event{Kind: BreaksChanged})
	return b.number, nil
}

// ClearBreak removes a breakpoint by number, or by file and line when number is 0.
func (c *Client) ClearBreak(file string, line, number int) error {
	if abs, err := filepath.Abs(file); err == nil {
		file = abs
	}
	c.bmu.Lock()
	var hit *breakpoint
	for _, b := range c.breaks {
		if (number > 0 && b.number == number) || (number == 0 && b.file == file && b.line == line) {
			hit = b
			break
		}
	}
	if hit == nil {
		c.bmu.Unlock()
		return fmt.Errorf("no breakpoint at %s:%d", file, line)
	}
	c.removeLocked(hit.number)
	err := c.syncFile(hit.file)
	c.bmu.Unlock()
	c.emit(Event{Kind: BreaksChanged})
	return err
}

func (c *Client) removeLocked(number int) {
	for i, b := range c.breaks {
		if b.number == number {
			c.breaks = append(c.breaks[:i], c.breaks[i+1:]...)
			return
		}
	}
}

// change applies fn to breakpoint number and resends its file.
func (c *Client) change(number int, fn func(*breakpoint)) error {
	c.bmu.Lock()
	var hit *breakpoint
	for _, b := range c.breaks {
		if b.number == number {
			hit = b
		}
	}
	if hit == nil {
		c.bmu.Unlock()
		return fmt.Errorf("no breakpoint %d", number)
	}
	fn(hit)
	err := c.syncFile(hit.file)
	c.bmu.Unlock()
	c.emit(Event{Kind: BreaksChanged})
	return err
}

func (c *Client) EnableBreak(number int, on bool) error {
	return c.change(number, func(b *breakpoint) { b.enabled = on })
}

func (c *Client) SetCondition(number int, cond string) error {
	return c.change(number, func(b *breakpoint) { b.cond = cond })
}

// Breakpoints returns the table ordered by number.
func (c *Client) Breakpoints() []models.BreakInfo {
	c.bmu.Lock()
	defer c.bmu.Unlock()
	out := make([]models.BreakInfo, 0, len(c.breaks))
	for _, b := range c.breaks {
		out = append(out, breakInfoFromEntry(b))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

const requestTimeout = 10 * time.Second

// entry is a hidden breakpoint on the first statement of the script:
// debugpy ignores stopOnEntry when attaching, so this stands in for it.
type entry struct {
	file string
	line int
}

// entryPoint finds the first statement of a plain `script.py` target; module
// targets (`-m x`) have none and run until the first breakpoint.
func entryPoint(python string, args []string) *entry {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return nil
	}
	file, err := filepath.Abs(args[0])
	if err != nil {
		return nil
	}
	out, err := exec.Command(python, "-c",
		"import ast,sys;print(ast.parse(open(sys.argv[1]).read()).body[0].lineno)", file).Output()
	if err != nil {
		return nil
	}
	line, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return nil
	}
	return &entry{file, line}
}

// entryFor returns the pending entry breakpoint when it lives in file and no
// enabled user breakpoint already sits on that line.
func (c *Client) entryFor(file string) *entry {
	c.mu.Lock()
	e := c.entry
	c.mu.Unlock()
	if e == nil || e.file != file {
		return nil
	}
	for _, b := range c.breaks {
		if b.file == file && b.enabled && b.line == e.line {
			return nil
		}
	}
	return e
}

// entryHit turns the stop at the entry breakpoint into a Reason "entry" stop
// and removes the breakpoint from the adapter.
func (c *Client) entryHit(ev *Event) {
	c.mu.Lock()
	e := c.entry
	hit := e != nil && ev.File == e.file && ev.Line == e.line
	if hit {
		c.entry = nil
	}
	c.mu.Unlock()
	if !hit {
		return
	}
	ev.Reason = "entry"
	c.bmu.Lock()
	_ = c.syncFile(e.file)
	c.bmu.Unlock()
}
