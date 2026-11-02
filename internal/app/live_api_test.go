package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/mcp"
)

// A hot loop: every line of it is reached millions of times a second.
const hotSrc = `int counter;
int main(void)
{
	for (;;) {
		counter++;
	}
	return 0;
}
`

const hotLoopLine = "hot.c:5"

type liveHost struct {
	luaHost
	m *mcp.GdbMcpService
	b backend.Backend
}

func (h liveHost) GdbMcp() *mcp.GdbMcpService { return h.m }
func (h liveHost) Backend() backend.Backend   { return h.b }

type liveAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu    sync.Mutex
	trace []string // timestamped raw MI traffic and API calls, dumped when a check fails
}

func (l *liveAPI) note(format string, a ...any) {
	l.mu.Lock()
	l.trace = append(l.trace, time.Now().Format("05.000")+" "+fmt.Sprintf(format, a...))
	l.mu.Unlock()
}

func (l *liveAPI) dump() {
	l.mu.Lock()
	defer l.mu.Unlock()
	var tail []string
	for _, e := range l.trace {
		if strings.Contains(e, " CALL ") || strings.Contains(e, " REPLY ") || strings.Contains(e, "*") || strings.Contains(e, "^") {
			tail = append(tail, e)
		}
	}
	if len(tail) > 50 {
		tail = tail[len(tail)-50:]
	}
	l.t.Logf("trace:\n%s", strings.Join(tail, "\n"))
}

// startLiveAPI starts a real gdb on a hot-loop program and serves the REST handler over it.
func startLiveAPI(t *testing.T) *liveAPI {
	t.Helper()
	if _, err := exec.LookPath("gdb"); err != nil {
		t.Skip("no gdb in PATH")
	}
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("no cc in PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "hot.c")
	if err := os.WriteFile(src, []byte(hotSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "hot")
	if out, err := exec.Command(cc, "-g", "-O0", "-o", bin, src).CombinedOutput(); err != nil {
		t.Skipf("cc: %v: %s", err, out)
	}
	cl, err := gdb.NewGDBClient("gdb", []string{bin})
	if err != nil {
		t.Skipf("start gdb: %v", err)
	}
	t.Cleanup(cl.Close)
	var c luaCtl
	c.host = liveHost{m: mcp.NewGdbMcpService(cl, nil), b: backend.NewGDB(cl)}
	srv := httptest.NewServer(c.restHandler())
	t.Cleanup(srv.Close)
	l := &liveAPI{t: t, srv: srv}
	ch, cancel := cl.MITTY().Subscribe()
	t.Cleanup(cancel)
	go func() {
		for m := range ch {
			l.note("MI %q", m.Data)
		}
	}()
	return l
}

// call POSTs /api/<tool> and returns the status and decoded reply.
func (l *liveAPI) call(tool string, args map[string]any) (int, map[string]any) {
	l.t.Helper()
	b, _ := json.Marshal(args)
	l.note("CALL %s %s", tool, b)
	resp, err := http.Post(l.srv.URL+"/api/"+tool, "application/json", bytes.NewReader(b))
	if err != nil {
		l.t.Fatalf("%s: %v", tool, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		l.t.Fatalf("%s: bad reply: %v", tool, err)
	}
	l.note("REPLY %s -> %d state=%v", tool, resp.StatusCode, out["state"])
	return resp.StatusCode, out
}

func (l *liveAPI) ok(tool string, args map[string]any) map[string]any {
	l.t.Helper()
	code, out := l.call(tool, args)
	if code != 200 {
		l.t.Fatalf("%s %v -> %d %v", tool, args, code, out)
	}
	return out
}

// startRunning sets a temporary breakpoint at main, runs to it and returns the process id.
func (l *liveAPI) startRunning() int {
	l.t.Helper()
	l.ok("gdb_command", map[string]any{"command": "-break-insert -t main"})
	st := l.ok("run_control", map[string]any{"action": "run", "wait": 15})
	if st["state"] != "stopped" {
		l.t.Skipf("could not start the program under gdb here: %v", st)
	}
	info := l.ok("gdb_command", map[string]any{"command": "info proc"})["result"].(string)
	m := regexp.MustCompile(`process (\d+)`).FindStringSubmatch(info)
	if m == nil {
		l.t.Skipf("no pid in %q", info)
	}
	pid, _ := strconv.Atoi(m[1])
	return pid
}

func procState(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "?"
	}
	s := string(b)
	return string(s[strings.LastIndexByte(s, ')')+2])
}

func TestLiveInterruptTwentyTimes(t *testing.T) { // acceptance 1
	l := startLiveAPI(t)
	l.startRunning()
	for i := 0; i < 20; i++ {
		if st := l.ok("continue", map[string]any{"wait_ms": 0}); st["state"] != "running" {
			t.Fatalf("trial %d: continue -> %v", i, st)
		}
		time.Sleep(time.Duration(10+rand.Intn(60)) * time.Millisecond)
		st := l.ok("interrupt", nil)
		if st["state"] != "stopped" || st["pc"] == nil || st["pc"] == "" {
			t.Fatalf("trial %d: interrupt -> %v", i, st)
		}
	}
}

func TestLiveContinueHitsBreakpoint(t *testing.T) { // acceptance 2, and breakpoint visibility
	l := startLiveAPI(t)
	l.startRunning()
	l.ok("gdb_command", map[string]any{"command": "-break-insert " + hotLoopLine})
	st := l.ok("continue", map[string]any{"wait_ms": 5000})
	if st["state"] != "stopped" || st["stop_reason"] != "breakpoint-hit" || st["bkpt_number"] == nil || st["line"] != float64(5) {
		t.Fatalf("continue -> %v", st)
	}
	res, _ := l.ok("list_breakpoints", nil)["result"].([]any)
	if len(res) == 0 {
		t.Fatal("list_breakpoints is empty")
	}
	row := res[len(res)-1].(map[string]any)
	if row["number"] != st["bkpt_number"] || row["hits"].(float64) < 1 {
		t.Fatalf("breakpoint row %v does not match the stop %v", row, st)
	}
}

func TestLiveTargetRunningAndConsole(t *testing.T) { // acceptance 3 and 4
	l := startLiveAPI(t)
	l.startRunning()
	out := l.ok("gdb_command", map[string]any{"command": "info registers pc"})["result"].(string)
	if strings.Count(out, "\n") != 0 || !strings.HasPrefix(out, "pc") {
		t.Fatalf("console reply is not one clean line: %q", out)
	}
	l.ok("continue", map[string]any{"wait_ms": 0})
	code, e := l.call("read_memory", map[string]any{"address": "$sp", "length": 8})
	if code != http.StatusConflict || e["error"] != "target_running" || e["state"] != "running" {
		t.Fatalf("read_memory while running -> %d %v", code, e)
	}
	code, e = l.call("gdb_command", map[string]any{"command": "-exec-next"})
	if code != http.StatusConflict || e["error"] != "target_running" {
		t.Fatalf("evaluate while running -> %d %v", code, e)
	}
	l.ok("interrupt", nil)
}

func TestLiveBatchHaltsAndResumes(t *testing.T) { // acceptance 5
	l := startLiveAPI(t)
	l.startRunning()
	l.ok("continue", map[string]any{"wait_ms": 0})
	ops := []any{}
	for i := 0; i < 5; i++ {
		ops = append(ops, map[string]any{"tool": "read_memory", "args": map[string]any{"address": "&counter", "length": 4}})
	}
	out := l.ok("batch", map[string]any{"ops": ops, "halt": true, "resume": true})
	if out["state"] != "running" {
		t.Fatalf("target must be running after the batch: %v", out)
	}
	r := out["result"].(map[string]any)
	results := r["results"].([]any)
	if len(results) != 5 || r["halted"] != true || r["resumed"] != true {
		t.Fatalf("batch -> %v", r)
	}
	for i, it := range results {
		m := it.(map[string]any)
		rm, _ := m["result"].(map[string]any)
		if m["error"] != nil || rm["words32"] == nil || rm["endian"] != "little" {
			t.Fatalf("op %d -> %v", i, m)
		}
	}
	l.ok("interrupt", nil)
}

func TestLiveRunTo(t *testing.T) {
	l := startLiveAPI(t)
	l.startRunning()
	out := l.ok("run_to", map[string]any{"location": hotLoopLine, "timeout_ms": 5000})["result"].(map[string]any)
	if out["state"] != "stopped" || out["reached"] != true || out["line"] != float64(5) {
		t.Fatalf("run_to -> %v", out)
	}
}

// Acceptance 6: the reported state must match what the kernel says. GDBFORGE_STRESS_SECS sets
// the length (default 3 s; 600 for the 10-minute run).
func TestLiveStateMatchesTarget(t *testing.T) {
	secs := 3
	if v, err := strconv.Atoi(os.Getenv("GDBFORGE_STRESS_SECS")); err == nil && v > 0 {
		secs = v
	}
	l := startLiveAPI(t)
	pid := l.startRunning()
	l.ok("gdb_command", map[string]any{"command": "-break-insert " + hotLoopLine})
	traced := func(s string) bool { return s == "t" || s == "T" }
	check := func(step string, st map[string]any) {
		t.Helper()
		switch st["state"] {
		case "stopped":
			if p := procState(pid); !traced(p) {
				t.Fatalf("%s: reported stopped but /proc says %q", step, p)
			}
		case "running":
			// The breakpoint is in the hot loop, so the target may stop again a moment after the
			// reply. "Running" is right if the process was seen running, or if the reported
			// state catches up with a stop; it is wrong only if the process sits stopped while
			// the reported state stays running.
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				if !traced(procState(pid)) {
					return
				}
				if l.ok("wait_for_stop", map[string]any{"timeout_ms": 0})["state"] == "stopped" {
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
			l.dump()
			t.Fatalf("%s: reported running but the process stayed stopped for 500 ms", step)
		default:
			t.Fatalf("%s: state %v", step, st["state"])
		}
	}
	end := time.Now().Add(time.Duration(secs) * time.Second)
	for n := 0; time.Now().Before(end); n++ {
		switch rand.Intn(4) {
		case 0:
			check("continue(wait)", l.ok("continue", map[string]any{"wait_ms": 300}))
		case 1:
			check("continue(0)", l.ok("continue", map[string]any{"wait_ms": 0}))
		case 2:
			check("interrupt", l.ok("interrupt", nil))
		case 3:
			check("wait_for_stop", l.ok("wait_for_stop", map[string]any{"timeout_ms": 100}))
		}
		time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
	}
}
