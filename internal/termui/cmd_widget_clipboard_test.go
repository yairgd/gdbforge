package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/commands"
)

func TestCmdWidgetPasteCtrlV(t *testing.T) {
	reg := commands.NewCommandRegistry()
	w := NewCmdWidget(reg)
	var clip string
	w.SetClipboard(ClipboardIO{
		Copy:  func(s string) { clip = s },
		Paste: func() string { return "edit foo" },
	})
	w.Activate()
	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	if w.Text() != ":edit foo" {
		t.Fatalf("after paste got %q", w.Text())
	}
	_ = clip
}

func TestCmdWidgetPasteMiddleClick(t *testing.T) {
	resetMiddlePasteState()
	reg := commands.NewCommandRegistry()
	w := NewCmdWidget(reg)
	w.SetClipboard(ClipboardIO{
		Paste: func() string { return "layout panels" },
	})
	w.Activate()
	w.HandleEvent(tcell.NewEventMouse(0, 0, tcell.ButtonMiddle, 0))
	if w.Text() != ":layout panels" {
		t.Fatalf("after middle paste got %q", w.Text())
	}
	// Motion / repeat while held must not paste again.
	w.HandleEvent(tcell.NewEventMouse(1, 0, tcell.ButtonMiddle, 0))
	w.HandleEvent(tcell.NewEventMouse(2, 0, tcell.ButtonMiddle, 0))
	if w.Text() != ":layout panels" {
		t.Fatalf("middle drag pasted again: %q", w.Text())
	}
}

func TestMiddlePasteRisingEdgeOnly(t *testing.T) {
	resetMiddlePasteState()
	evDown := tcell.NewEventMouse(0, 0, tcell.ButtonMiddle, 0)
	if !isMiddlePaste(evDown) {
		t.Fatal("first press should paste")
	}
	if isMiddlePaste(tcell.NewEventMouse(1, 0, tcell.ButtonMiddle, 0)) {
		t.Fatal("held motion must not paste")
	}
	if isMiddlePaste(tcell.NewEventMouse(0, 0, tcell.ButtonNone, 0)) {
		t.Fatal("release must not paste")
	}
}

func TestCmdWidgetPasteFirstLineOnly(t *testing.T) {
	reg := commands.NewCommandRegistry()
	w := NewCmdWidget(reg)
	w.SetClipboard(ClipboardIO{
		Paste: func() string { return "one\ntwo\nthree" },
	})
	w.Activate()
	w.pasteAtCursor()
	if w.Text() != ":one" {
		t.Fatalf("multi-line paste should keep first line, got %q", w.Text())
	}
}

func TestCmdWidgetCopyCut(t *testing.T) {
	reg := commands.NewCommandRegistry()
	w := NewCmdWidget(reg)
	var clip string
	w.SetClipboard(ClipboardIO{
		Copy:  func(s string) { clip = s },
		Paste: func() string { return "" },
	})
	w.Activate()
	w.pasteText("hello")
	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))
	if clip != "hello" {
		t.Fatalf("copy got %q", clip)
	}
	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	if clip != "hello" {
		t.Fatalf("cut copy got %q", clip)
	}
	if w.Text() != ":" {
		t.Fatalf("cut should clear editable text, got %q", w.Text())
	}
}

func TestCmdWidgetEditKeys(t *testing.T) {
	reg := commands.NewCommandRegistry()
	w := NewCmdWidget(reg)
	w.Activate()
	w.pasteText("edit foo bar")
	w.SetCursorAtLocalX(8) // after "edit f"

	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if w.cursor != 1 {
		t.Fatalf("Ctrl-A cursor=%d want 1", w.cursor)
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlE, 0, tcell.ModCtrl))
	if w.cursor != len([]rune(w.Text())) {
		t.Fatalf("Ctrl-E cursor=%d want end", w.cursor)
	}

	w.SetCursorAtLocalX(8)
	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModCtrl))
	if w.Active() {
		t.Fatal("Ctrl-U should exit command mode")
	}
	if w.Text() != "" {
		t.Fatalf("Ctrl-U should clear cmdline, got %q", w.Text())
	}

	w.ActivateSearch()
	w.pasteText("pattern")
	w.SetCursorAtLocalX(5)
	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModCtrl))
	if w.Active() {
		t.Fatal("Ctrl-U should exit search mode")
	}
	if w.Text() != "" {
		t.Fatalf("Ctrl-U should clear search cmdline, got %q", w.Text())
	}
}

func TestCmdWidgetSetCursorAtLocalX(t *testing.T) {
	reg := commands.NewCommandRegistry()
	w := NewCmdWidget(reg)
	w.Activate()
	w.pasteText("edit")
	// ":edit" — click on 'e' (index 1) stays at 1; on 'd' (index 4) → 4
	w.SetCursorAtLocalX(4)
	if w.cursor != 4 {
		t.Fatalf("cursor=%d want 4", w.cursor)
	}
	w.SetCursorAtLocalX(0) // cannot sit on ':'
	if w.cursor != 1 {
		t.Fatalf("cursor=%d want 1", w.cursor)
	}
	w.SetCursorAtLocalX(99)
	if w.cursor != len([]rune(w.Text())) {
		t.Fatalf("cursor=%d want end", w.cursor)
	}
}
