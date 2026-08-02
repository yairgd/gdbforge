package widgets

import (
	"strings"
	"testing"
)

func TestOutputWidgetTargetStream(t *testing.T) {
	w := NewOutputWidget()
	w.AppendPty("@\"yair\\n\"\n")
	lines := w.LinesForTest()
	if len(lines) != 1 || lines[0] != "yair" {
		t.Fatalf("target stream: %v", lines)
	}
}

func TestOutputWidgetRawWhileRunning(t *testing.T) {
	w := NewOutputWidget()
	w.AppendPty("~\"Starting program: /tmp/out\\n\"\n")
	w.AppendPty("^running\n")
	w.AppendPty("*running,thread-id=\"all\"\n")
	w.AppendPty("here is the value 0\n")
	w.AppendPty("*stopped,reason=\"end-stepping-range\",frame={}\n")
	w.AppendPty("^running\n")
	w.AppendPty("here is the value 1\n")
	w.AppendPty("*stopped,reason=\"end-stepping-range\"\n")
	w.AppendPty("*stopped,reason=\"exited-normally\"\n")
	w.AppendPty("should not appear\n")

	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "here is the value 0" || lines[1] != "here is the value 1" {
		t.Fatalf("raw across steps: %v", lines)
	}
}

func TestOutputWidgetSeparateTTYIgnoresGdbRaw(t *testing.T) {
	w := NewOutputWidget()
	w.EnableInput(true)
	w.AppendPty("^running\n")
	w.AppendPty("should not appear from gdb pty\n")
	w.AppendPty("@\"should not appear from mi\\n\"\n")
	w.AppendInferior("from inferior\n")
	lines := w.LinesForTest()
	if len(lines) != 1 || lines[0] != "from inferior" {
		t.Fatalf("separate tty path: %v", lines)
	}
}

func TestOutputWidgetAppendInferior(t *testing.T) {
	w := NewOutputWidget()
	w.AppendInferior("hello\nworld\n")
	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("inferior: %v", lines)
	}
}

func TestOutputWidgetIgnoresMINoise(t *testing.T) {
	w := NewOutputWidget()
	w.AppendPty("^running\n")
	w.AppendPty("~\"Breakpoint 1\\n\"\n")
	w.AppendPty("=thread-created,id=\"1\"\n")
	w.AppendPty("(gdb)\n")
	if lines := w.LinesForTest(); len(lines) != 0 {
		t.Fatalf("expected empty, got %v", lines)
	}
}

func TestOutputWidgetCRAndTab(t *testing.T) {
	w := NewOutputWidget()
	w.AppendPty("@\"hello\\rworld\\n\"\n")
	lines := w.LinesForTest()
	if len(lines) != 1 || lines[0] != "world" {
		t.Fatalf("CR overwrite: %v", lines)
	}

	w.Clear()
	w.AppendPty("@\"a\\tb\\n\"\n")
	lines = w.LinesForTest()
	want := "a" + stringsRepeat(" ", 7) + "b"
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("tab: got %v want %q", lines, want)
	}
}

func TestOutputWidgetSubmitClearsInputKeepsLivePrompt(t *testing.T) {
	w := NewOutputWidget()
	w.EnableInput(true)
	if !w.console.LivePrompt() {
		t.Fatal("expected live prompt attach when IO stdin is enabled")
	}
	w.console.Input().SetText("hello stdin")
	w.handleSubmit("hello stdin")
	if got := w.console.Input().Text(); got != "" {
		t.Fatalf("input should clear after submit, got %q", got)
	}
	if !w.console.LivePrompt() {
		t.Fatal("live prompt attach should remain after submit")
	}
	// No local echo — scrollback stays empty until inferior PTY output arrives.
	if lines := w.LinesForTest(); len(lines) != 0 {
		t.Fatalf("submit must not local-echo (avoids doubling PTY echo): %q", lines)
	}
}

func TestOutputWidgetClearKeepsRunning(t *testing.T) {
	w := NewOutputWidget()
	w.AppendPty("^running\n")
	w.Clear()
	w.AppendPty("after clear\n")
	lines := w.LinesForTest()
	if len(lines) != 1 || lines[0] != "after clear" {
		t.Fatalf("after clear while running: %v", lines)
	}
}

func TestOutputWidgetKeepsANSIEsc(t *testing.T) {
	w := NewOutputWidget()
	// MI target stream with octal ESC (033) for ANSI red.
	w.AppendPty("@\"\\033[31mred\\033[0m\\n\"\n")
	lines := w.LinesForTest()
	if len(lines) != 1 || !strings.Contains(lines[0], "\x1b[31m") || !strings.Contains(lines[0], "red") {
		t.Fatalf("want ANSI kept in buffer: %q", lines)
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
