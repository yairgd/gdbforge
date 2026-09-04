package termui

import (
	"fmt"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestLoggerWidgetBindKeyMovesCursorBeforeScroll(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewLoggerWidget(ctx)
	for i := 0; i < 40; i++ {
		_ = w.Write(platform.LogEntry{Text: fmt.Sprintf("line %d", i)})
	}
	w.doc.width = 80
	w.doc.height = 10
	w.doc.ScrollToBottom()
	startTop := w.doc.Top
	startCur := w.doc.CursorLine

	if !w.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)) {
		t.Fatal("expected 'k' binding to handle")
	}
	if w.doc.FollowTail() {
		t.Fatal("scroll-up should leave follow-tail")
	}
	// First Up: caret moves within the view; Top stays put.
	if w.doc.Top != startTop {
		t.Fatalf("Top should stay %d until caret hits top edge; got %d", startTop, w.doc.Top)
	}
	if w.doc.CursorLine != startCur-1 {
		t.Fatalf("CursorLine: got %d want %d", w.doc.CursorLine, startCur-1)
	}

	// Walk cursor to the top edge of the view, then one more to scroll.
	for w.doc.CursorLine > w.doc.Top {
		w.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	}
	w.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if w.doc.Top >= startTop {
		t.Fatalf("expected Top to decrease after caret hits edge: start=%d now=%d", startTop, w.doc.Top)
	}
}

func TestLoggerWidgetArrowMovesCursorBeforeScroll(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewLoggerWidget(ctx)
	for i := 0; i < 40; i++ {
		_ = w.Write(platform.LogEntry{Text: fmt.Sprintf("line %d", i)})
	}
	w.doc.width = 80
	w.doc.height = 10
	w.doc.ScrollToBottom()
	startTop := w.doc.Top
	startCur := w.doc.CursorLine

	w.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if w.doc.FollowTail() {
		t.Fatal("arrow up should leave follow-tail")
	}
	if w.doc.Top != startTop {
		t.Fatalf("Top should stay %d on first arrow; got %d", startTop, w.doc.Top)
	}
	if w.doc.CursorLine != startCur-1 {
		t.Fatalf("CursorLine: got %d want %d", w.doc.CursorLine, startCur-1)
	}
}
