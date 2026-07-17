package widgets

import "testing"

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
