package termui

import (
	"fmt"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
)

func TestLoggerWidgetBindKeyMovesCursorBeforeScroll(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewLoggerWidget(ctx)
	for i := 0; i < 40; i++ {
		_ = w.Write(platform.LogEntry{Text: fmt.Sprintf("line %d", i)})
	}
	w.viewport.width = 80
	w.viewport.height = 10
	w.viewport.ScrollToBottom()
	startTop := w.viewport.Top
	startCur := w.viewport.CursorLine

	if !w.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)) {
		t.Fatal("expected 'k' binding to handle")
	}
	if w.viewport.FollowTail() {
		t.Fatal("scroll-up should leave follow-tail")
	}
	// First Up: caret moves within the view; Top stays put.
	if w.viewport.Top != startTop {
		t.Fatalf("Top should stay %d until caret hits top edge; got %d", startTop, w.viewport.Top)
	}
	if w.viewport.CursorLine != startCur-1 {
		t.Fatalf("CursorLine: got %d want %d", w.viewport.CursorLine, startCur-1)
	}

	// Walk cursor to the top edge of the view, then one more to scroll.
	for w.viewport.CursorLine > w.viewport.Top {
		w.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	}
	w.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if w.viewport.Top >= startTop {
		t.Fatalf("expected Top to decrease after caret hits edge: start=%d now=%d", startTop, w.viewport.Top)
	}
}

func TestLoggerWidgetArrowMovesCursorBeforeScroll(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewLoggerWidget(ctx)
	for i := 0; i < 40; i++ {
		_ = w.Write(platform.LogEntry{Text: fmt.Sprintf("line %d", i)})
	}
	w.viewport.width = 80
	w.viewport.height = 10
	w.viewport.ScrollToBottom()
	startTop := w.viewport.Top
	startCur := w.viewport.CursorLine

	w.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if w.viewport.FollowTail() {
		t.Fatal("arrow up should leave follow-tail")
	}
	if w.viewport.Top != startTop {
		t.Fatalf("Top should stay %d on first arrow; got %d", startTop, w.viewport.Top)
	}
	if w.viewport.CursorLine != startCur-1 {
		t.Fatalf("CursorLine: got %d want %d", w.viewport.CursorLine, startCur-1)
	}
}
