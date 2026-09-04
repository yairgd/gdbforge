package termui

import (
	"strings"
	"testing"

	xterm "github.com/gitpod-io/xterm-go"
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

func TestInputLineTextStripsLuaPrompt(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	if err := c.WriteString("lua> print(1)"); err != nil {
		t.Fatal(err)
	}
	got := InputLineText(c)
	if got != "print(1)" {
		t.Fatalf("got %q want print(1)", got)
	}
}

func TestRewritePromptInputReplacesWholeLine(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	c.SetInputHandler(func(b []byte) error {
		return c.Write(b)
	})
	if err := c.WriteString("lua> short"); err != nil {
		t.Fatal(err)
	}
	RewritePromptInput(c, "lua> ", "a much longer history line")
	got := InputLineText(c)
	if got != "a much longer history line" {
		t.Fatalf("InputLineText=%q", got)
	}
	var line string
	c.WithTerminal(func(term *xterm.Terminal) {
		cx, cy := term.CursorX(), term.CursorY()
		line = readLineRunes(term, cx, cy)
	})
	if !strings.HasPrefix(line, "lua> a much longer history line") {
		t.Fatalf("line=%q", line)
	}
	if strings.Contains(line, "short") {
		t.Fatalf("stale input remained: %q", line)
	}

	RewritePromptInput(c, "lua> ", "tiny")
	got = InputLineText(c)
	if got != "tiny" {
		t.Fatalf("after shrink InputLineText=%q", got)
	}
	c.WithTerminal(func(term *xterm.Terminal) {
		cx, cy := term.CursorX(), term.CursorY()
		line = readLineRunes(term, cx, cy)
	})
	if strings.Contains(line, "longer") {
		t.Fatalf("stale tail after shrink: %q", line)
	}
}

func TestOnEmptyPromptLine(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	const prompt = "lua> "
	RewritePromptInput(c, prompt, "")
	if !OnEmptyPromptLine(c, prompt) {
		t.Fatalf("expected empty prompt line, line=%q", promptLineRaw(c))
	}
	if err := c.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if OnEmptyPromptLine(c, prompt) {
		t.Fatal("expected non-empty after typing")
	}
}

func TestPromptInputStateAndMoveCursor(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	c.SetInputHandler(func(b []byte) error {
		return c.Write(b)
	})
	const prompt = "lua> "
	RewritePromptInput(c, prompt, "hello")
	MovePromptCursor(c, prompt, 3)

	text, cur := PromptInputState(c, prompt)
	if text != "hello" || cur != 3 {
		t.Fatalf("state text=%q cur=%d", text, cur)
	}

	MovePromptCursor(c, prompt, 0)
	_, cur = PromptInputState(c, prompt)
	if cur != 0 {
		t.Fatalf("home cur=%d", cur)
	}

	MovePromptCursor(c, prompt, len(text))
	_, cur = PromptInputState(c, prompt)
	if cur != len("hello") {
		t.Fatalf("end cur=%d", cur)
	}

	RewritePromptInput(c, prompt, text[3:])
	MovePromptCursor(c, prompt, 0)
	text, cur = PromptInputState(c, prompt)
	if text != "lo" || cur != 0 {
		t.Fatalf("kill-bol text=%q cur=%d", text, cur)
	}
}

func TestPeelLeadingPrompt(t *testing.T) {
	const prompt = "lua> "
	if got := peelLeadingPrompt("lua> print(1)", prompt); got != "print(1)" {
		t.Fatalf("got %q", got)
	}
	if got := peelLeadingPrompt("lua> lua> ", prompt); got != "" {
		t.Fatalf("duplicate got %q", got)
	}
}

func TestPromptInputStateDuplicatePrompt(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	c.SetInputHandler(func(b []byte) error {
		return c.Write(b)
	})
	const prompt = "lua> "
	if err := c.WriteString("\r" + prompt + "lua> "); err != nil {
		t.Fatal(err)
	}
	text, _ := PromptInputState(c, prompt)
	if text != "" {
		t.Fatalf("duplicate prompt text=%q want empty", text)
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
