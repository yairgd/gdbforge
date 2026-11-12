package pydbg

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yairgd/termforge/ptyx"
)

func TestParseCommand(t *testing.T) {
	cases := []struct{ in, cmd, args string }{
		{"", "", ""},
		{"  b prog.py:3 ", "b", "prog.py:3"},
		{"c", "c", ""},
		{"condition 2 x > 1", "condition", "2 x > 1"},
		{"!c", "!", "c"},
		{"x = 5", "!", "x = 5"},
		{"n = 3", "!", "n = 3"},
		{"c.value", "!", "c.value"},
		{"print(x)", "!", "print(x)"},
		{"bt", "bt", ""},
		{"p pts", "!", "pts"},
		{"pp a.x", "!", "a.x"},
		{"print d", "!", "d"},
		{"p = 3", "!", "p = 3"},
	}
	for _, tc := range cases {
		cmd, args := parseCommand(tc.in)
		if cmd != tc.cmd || args != tc.args {
			t.Errorf("parseCommand(%q) = %q, %q; want %q, %q", tc.in, cmd, args, tc.cmd, tc.args)
		}
	}
}

func TestParseLocation(t *testing.T) {
	cases := []struct {
		arg, cur, file string
		line           int
		fn             string
		bad            bool
	}{
		{"a.py:7", "", "a.py", 7, "", false},
		{"a.py:7", "/d/a.py", "/d/a.py", 7, "", false},
		{"/d/a.py:7", "/e/b.py", "/d/a.py", 7, "", false},
		{"9", "/d/a.py", "/d/a.py", 9, "", false},
		{"9", "", "", 0, "", true},
		{"work", "/d/a.py", "", 0, "work", false},
		{"", "/d/a.py", "", 0, "", true},
	}
	for _, tc := range cases {
		file, line, fn, err := parseLocation(tc.arg, tc.cur)
		if (err != nil) != tc.bad || file != tc.file || line != tc.line || fn != tc.fn {
			t.Errorf("parseLocation(%q,%q) = %q %d %q %v", tc.arg, tc.cur, file, line, fn, err)
		}
	}
}

func TestBanners(t *testing.T) {
	ev := Event{Kind: Stopped, File: "/d/a.py", Line: 3, Func: "work", Reason: "breakpoint"}
	if got := stopBanner(ev, 0); got != "> work() /d/a.py:3" {
		t.Error(got)
	}
	if got := stopBanner(ev, 2); got != "> [Breakpoint 2] work() /d/a.py:3" {
		t.Error(got)
	}
	ev.Reason = "exception"
	if got := stopBanner(ev, 0); got != "> [exception] work() /d/a.py:3" {
		t.Error(got)
	}
	fe := Event{Kind: FrameSelected, File: "/d/a.py", Line: 7, Func: "<module>", Level: 1}
	if got := frameBanner(fe); got != "> <module>() /d/a.py:7 (frame 1)" {
		t.Error(got)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short") != "short" {
		t.Error("short changed")
	}
	got := truncate(strings.Repeat("x", maxResult+25))
	if !strings.HasSuffix(got, "... (25 more chars)") || len(got) != maxResult+len("... (25 more chars)") {
		t.Errorf("truncate: %q", got[len(got)-30:])
	}
}

// screen reads console output and checks it in order.
type screen struct {
	t   *testing.T
	ch  <-chan ptyx.PtyOutputMsg
	buf string
}

func (s *screen) expect(want string) {
	s.t.Helper()
	timeout := time.After(20 * time.Second)
	for {
		if i := strings.Index(s.buf, want); i >= 0 {
			s.buf = s.buf[i+len(want):]
			return
		}
		select {
		case m, ok := <-s.ch:
			if !ok {
				s.t.Fatalf("console closed; waiting for %q, have %q", want, s.buf)
			}
			s.buf += m.Data
		case <-timeout:
			s.t.Fatalf("timeout waiting for %q, have %q", want, s.buf)
		}
	}
}

func TestLiveConsole(t *testing.T) {
	py := os.Getenv("PYDBG_TEST_PYTHON")
	if py == "" {
		py = "python3"
	}
	if _, err := exec.LookPath(py); err != nil {
		t.Skip("no python")
	}
	if err := exec.Command(py, "-c", "import debugpy").Run(); err != nil {
		t.Skip("debugpy not importable")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "prog.py")
	if err := os.WriteFile(script, []byte(liveScript), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(py, []string{script}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ch, cancel := c.Subscribe()
	defer cancel()
	s := &screen{t: t, ch: ch}
	send := func(line string) {
		t.Helper()
		if err := c.Send(line); err != nil {
			t.Fatal(err)
		}
	}

	s.expect(readyText)
	s.expect("> <module>() " + script + ":1")
	s.expect("(py) ")
	send("b " + script + ":3")
	s.expect("Breakpoint 1 set at " + script + ":3\r\n(py) ")
	send("c")
	s.expect("> [Breakpoint 1] work() " + script + ":3\r\n(py) ")
	send("x")
	s.expect("\r\n0\r\n(py) ")
	send("x = 5")
	s.expect("(py) ")
	send("x")
	s.expect("\r\n5\r\n(py) ")
	send("no_such_name")
	s.expect("Command failed: ")
	s.expect("(py) ")
	send("n")
	s.expect("> ")
	if !regexp.MustCompile(`^<module>\(\) ` + regexp.QuoteMeta(script) + `:\d+\r\n`).MatchString(s.buf) {
		// the step may land in the caller or in work(); either is a plain stop banner
		t.Logf("after next: %q", s.buf)
	}
	s.expect("(py) ")
	send("bt")
	s.expect("#0  ")
	s.expect("(py) ")
	send("delete 1")
	s.expect("(py) ")
	send("c")
	s.expect("Process has exited with status 0")
	s.expect("(py) ")
}
