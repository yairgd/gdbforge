package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	xterm "github.com/gitpod-io/xterm-go"
)

func TestCompositeTerminalMouseScroll(t *testing.T) {
	c := NewCompositeTerminal(10, 5, 100)
	for i := 0; i < 20; i++ {
		_ = c.ctl.WriteString("line\r\n")
	}

	before := c.viewDisp()
	c.HandleMouse(tcell.NewEventMouse(0, 0, tcell.WheelUp, 0))
	if c.viewDisp() >= before {
		t.Fatalf("wheel up: YDisp %d want < %d", c.viewDisp(), before)
	}

	afterUp := c.viewDisp()
	c.HandleMouse(tcell.NewEventMouse(0, 0, tcell.WheelDown, 0))
	if c.viewDisp() <= afterUp {
		t.Fatalf("wheel down: YDisp %d want > %d", c.viewDisp(), afterUp)
	}
}

func TestCompositeTerminalMouseCopyWithOrigin(t *testing.T) {
	c := NewCompositeTerminal(20, 3, 100)
	c.SetMouseOrigin(10, 5)
	var copied string
	c.SetClipboard(ClipboardIO{Copy: func(s string) { copied = s }})
	_ = c.ctl.WriteString("hello world\r\n")

	c.HandleMouse(tcell.NewEventMouse(10, 5, tcell.ButtonPrimary, 0))
	c.HandleMouse(tcell.NewEventMouse(15, 5, tcell.ButtonPrimary, 0))
	c.HandleMouse(tcell.NewEventMouse(15, 5, tcell.ButtonNone, 0))

	if copied != "hello" {
		t.Fatalf("copied %q want %q", copied, "hello")
	}
}

func TestCompositeTerminalMiddleClickPaste(t *testing.T) {
	resetMiddlePasteState()
	c := NewCompositeTerminal(20, 3, 100)
	const paste = "pasted text"
	c.SetClipboard(ClipboardIO{
		PastePrimary: func() string { return paste },
	})
	var sent string
	c.ctl.SetInputHandler(func(b []byte) error {
		sent += string(b)
		return nil
	})

	c.HandleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonMiddle, 0))
	if sent != paste {
		t.Fatalf("sent %q want %q", sent, paste)
	}
}

func TestCompositeTerminalPasteBytes(t *testing.T) {
	c := NewCompositeTerminal(20, 3, 100)
	var sent string
	c.ctl.SetInputHandler(func(b []byte) error {
		sent += string(b)
		return nil
	})
	c.PasteBytes([]byte("clip"))
	if sent != "clip" {
		t.Fatalf("sent %q want clip", sent)
	}
}

func TestCompositeTerminalMouseCopy(t *testing.T) {
	c := NewCompositeTerminal(20, 3, 100)
	var copied string
	c.SetClipboard(ClipboardIO{Copy: func(s string) { copied = s }})
	_ = c.ctl.WriteString("hello world\r\n")

	c.HandleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonPrimary, 0))
	c.HandleMouse(tcell.NewEventMouse(5, 0, tcell.ButtonPrimary, 0))
	c.HandleMouse(tcell.NewEventMouse(5, 0, tcell.ButtonNone, 0))

	if copied != "hello" {
		t.Fatalf("copied %q want %q", copied, "hello")
	}
}

func TestCompositeTerminalCtrlCCopiesSelection(t *testing.T) {
	c := NewCompositeTerminal(20, 3, 100)
	var copied string
	c.SetClipboard(ClipboardIO{Copy: func(s string) { copied = s }})
	_ = c.ctl.WriteString("abort me\r\n")

	c.HandleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonPrimary, 0))
	c.HandleMouse(tcell.NewEventMouse(5, 0, tcell.ButtonPrimary, 0))
	c.hasSel = true

	var sent []byte
	c.ctl.SetInputHandler(func(b []byte) error {
		sent = append(sent, b...)
		return nil
	})

	c.HandleKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
	if copied != "abort" {
		t.Fatalf("copied %q want %q", copied, "abort")
	}
	if len(sent) != 0 {
		t.Fatalf("ctrl-c with selection sent %q to PTY", sent)
	}
}

func TestCompositeTerminalScrollKeys(t *testing.T) {
	c := NewCompositeTerminal(10, 5, 100)
	for i := 0; i < 30; i++ {
		_ = c.ctl.WriteString("line\r\n")
	}

	var sent []byte
	c.ctl.SetInputHandler(func(b []byte) error {
		sent = append(sent, b...)
		return nil
	})

	before := c.viewDisp()
	c.HandleKey(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	if c.viewDisp() >= before {
		t.Fatalf("PgUp: YDisp %d want < %d", c.viewDisp(), before)
	}
	if len(sent) != 0 {
		t.Fatalf("PgUp sent %q to PTY", sent)
	}

	sent = nil
	c.HandleKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if c.viewDisp() != 0 {
		t.Fatalf("Home: YDisp %d want 0", c.viewDisp())
	}
	if len(sent) != 0 {
		t.Fatalf("Home sent %q to PTY", sent)
	}

	sent = nil
	c.HandleKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if c.viewDisp() <= 0 {
		t.Fatal("End: expected scroll to bottom")
	}
	if len(sent) != 0 {
		t.Fatalf("End sent %q to PTY", sent)
	}
}

func TestCompositeTerminalEnterScrollsToBottom(t *testing.T) {
	c := NewCompositeTerminal(10, 5, 100)
	for i := 0; i < 30; i++ {
		_ = c.ctl.WriteString("line\r\n")
	}

	var sent []byte
	c.ctl.SetInputHandler(func(b []byte) error {
		sent = append(sent, b...)
		return nil
	})

	c.HandleKey(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	if c.atBottom() {
		t.Fatal("expected scrolled up before Enter test")
	}

	c.HandleKey(tcell.NewEventKey(tcell.KeyEnter, '\r', tcell.ModNone))
	if !c.atBottom() {
		t.Fatal("Enter while scrolled up should snap to bottom")
	}
	if len(sent) != 0 {
		t.Fatalf("Enter while scrolled up sent %q to PTY", sent)
	}

	sent = nil
	c.HandleKey(tcell.NewEventKey(tcell.KeyEnter, '\r', tcell.ModNone))
	if string(sent) != "\r" {
		t.Fatalf("Enter at bottom sent %q want \\r", sent)
	}
}

func TestCompositeTerminalSelectWordAt(t *testing.T) {
	c := NewCompositeTerminal(20, 3, 100)
	var copied string
	c.SetClipboard(ClipboardIO{Copy: func(s string) { copied = s }})
	_ = c.ctl.WriteString("foo_bar baz\r\n")

	var absLine int
	c.ctl.WithTerminal(func(term *xterm.Terminal) {
		absLine = term.Buffer().YDisp
	})
	if !c.selectWordAt(termPos{line: absLine, col: 4}) {
		t.Fatal("selectWordAt failed")
	}
	c.copySelection()
	if copied != "foo_bar" {
		t.Fatalf("copied %q want foo_bar", copied)
	}
}
