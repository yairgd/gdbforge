package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestExecPushRawKeepsLivePrompt(t *testing.T) {
	w := NewExecWidget(nil)
	w.pushRaw("prompt $ ")
	if !w.console.LivePrompt() {
		t.Fatal("expected live prompt after incomplete line")
	}
	if n := w.console.Buffer().NumLines(); n != 1 {
		t.Fatalf("lines=%d", n)
	}

	w.pushRaw("\rbash: wed: command not found\n")
	w.pushRaw("prompt $ ")
	if !w.console.LivePrompt() {
		t.Fatal("expected live prompt after new prompt")
	}
	last := w.console.Buffer().Line(w.console.Buffer().NumLines() - 1)
	if last != "prompt $ " {
		t.Fatalf("last=%q", last)
	}
}

func TestExecCROverwritesPending(t *testing.T) {
	w := NewExecWidget(nil)
	w.pushRaw("old prompt $ ")
	w.pushRaw("\rnew prompt $ ")
	if w.console.Buffer().NumLines() != 1 {
		t.Fatalf("lines=%d want 1", w.console.Buffer().NumLines())
	}
	if got := w.console.Buffer().Line(0); got != "new prompt $ " {
		t.Fatalf("got %q", got)
	}
	if !w.console.LivePrompt() {
		t.Fatal("expected live prompt")
	}
}

func TestExecDismissAfterSessionEnded(t *testing.T) {
	w := NewExecWidget(nil)
	dismissed := false
	w.SetOnDismiss(func() { dismissed = true })

	w.HandleEvent(tcell.NewEventInterrupt(execSessionEnded))
	if !w.Ended() {
		t.Fatal("expected ended after session-ended interrupt")
	}
	last := w.console.Buffer().Line(w.console.Buffer().NumLines() - 1)
	if last == "" || last[:6] != "[exec]" {
		t.Fatalf("hint line=%q", last)
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if !dismissed {
		t.Fatal("expected dismiss on key after exit")
	}
}
