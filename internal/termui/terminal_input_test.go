package termui

import (
	"strings"
	"testing"
)

func TestInputLineTextStripsDelvePrompt(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	if err := c.WriteString("(dlv) b main."); err != nil {
		t.Fatal(err)
	}
	got := InputLineText(c)
	if got != "b main." {
		t.Fatalf("got %q want b main.", got)
	}
}

func TestInputLineTextStripsGdbPrompt(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	if err := c.WriteString("(gdb) info b"); err != nil {
		t.Fatal(err)
	}
	got := InputLineText(c)
	if got != "info b" {
		t.Fatalf("got %q want info b", got)
	}
}

func TestApplyCompletionSuffixOnly(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	var sent strings.Builder
	c.SetInputHandler(func(b []byte) error {
		_, err := sent.Write(b)
		return err
	})

	if err := c.WriteString("(gdb) ju"); err != nil {
		t.Fatal(err)
	}
	ApplyCompletion(c, "ju", "jump")
	if got := sent.String(); got != "mp" {
		t.Fatalf("sent %q want mp", got)
	}

	sent.Reset()
	ApplyCompletion(c, "jump", "jump ")
	if got := sent.String(); got != " " {
		t.Fatalf("trailing space sent %q want space", got)
	}
}

func TestApplyCompletionFullReplaceWhenNotPrefix(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	var sent strings.Builder
	c.SetInputHandler(func(b []byte) error {
		_, err := sent.Write(b)
		return err
	})

	if err := c.WriteString("(gdb) br"); err != nil {
		t.Fatal(err)
	}
	ApplyCompletion(c, "br", "info breakpoints")
	want := strings.Repeat("\x7f", len("br")) + "info breakpoints"
	if sent.String() != want {
		t.Fatalf("sent %q want %q", sent.String(), want)
	}
}

func TestReplaceInputLinePrefixExpand(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	var sent strings.Builder
	c.SetInputHandler(func(b []byte) error {
		_, err := sent.Write(b)
		return err
	})

	if err := c.WriteString("(dlv) b main."); err != nil {
		t.Fatal(err)
	}
	ReplaceInputLine(c, "b main.m")
	want := strings.Repeat("\x7f", len("b main.")) + "b main.m"
	if sent.String() != want {
		t.Fatalf("sent %q want %q", sent.String(), want)
	}
}

func TestReplaceInputLineFullReplace(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	var sent strings.Builder
	c.SetInputHandler(func(b []byte) error {
		_, err := sent.Write(b)
		return err
	})

	if err := c.WriteString("(dlv) br"); err != nil {
		t.Fatal(err)
	}
	ReplaceInputLine(c, "break")
	want := "\x7f\x7f" + "break"
	if sent.String() != want {
		t.Fatalf("sent %q want %q", sent.String(), want)
	}
}
