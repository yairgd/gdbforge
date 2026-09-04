package termui

import (
	"sync/atomic"
	"testing"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

type paintCountWidget struct{ draws atomic.Int64 }

func (w *paintCountWidget) HandleEvent(tcell.Event)     {}
func (w *paintCountWidget) Draw(Canvas)                 { w.draws.Add(1) }
func (w *paintCountWidget) DrawStatusLine(Canvas, bool) {}

type paintTestAPI struct{ app *TermApp }

func (m *paintTestAPI) HandleMouse(*tcell.EventMouse) {}
func (m *paintTestAPI) HandleResize()                 {}
func (m *paintTestAPI) HandleTTYResume()              {}
func (m *paintTestAPI) HandleInterrupt(ev *tcell.EventInterrupt) {
	if s, ok := ev.Data().(string); ok && s == "quit" {
		m.app.Exit()
	}
}

func TestMustWaitForPaintTick(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()

	app := &TermApp{screen: scr}
	if app.mustWaitForPaintTick(false) {
		t.Fatal("clean frame must not wait on the paint tick")
	}
	if !app.mustWaitForPaintTick(true) {
		t.Fatal("dirty frame with an empty queue must wait on the paint tick")
	}
	if err := scr.PostEvent(tcell.NewEventInterrupt("x")); err != nil {
		t.Fatal(err)
	}
	if app.mustWaitForPaintTick(true) {
		t.Fatal("queued events must be drained instead of waiting on the tick")
	}
}

// Async output (Lua REPL result, GDB chunk) must reach the screen on its own.
//
// A lone interrupt after an idle period always painted, because the tick that
// buffered while Run sat in PollEvent is consumed right after the batch. The
// failing sequence is a keystroke first: it paints immediately and then burns
// that buffered tick, so the async interrupt it triggers dirties a frame with
// both the queue and the ticker empty. Run used to fall through to a blocking
// PollEvent there, leaving the output unpainted until the next keypress —
// which is why every Lua REPL line appeared to need a second Enter.
//
// A long paint interval widens the window so the keystroke and the interrupt
// land in the same tick period; the repeats make a tick boundary sneaking into
// that window harmless.
func TestRunPaintsAsyncFrameWithoutFurtherInput(t *testing.T) {
	const interval = 200 * time.Millisecond

	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}

	w := &paintCountWidget{}
	api := &paintTestAPI{}
	app := &TermApp{
		screen:        scr,
		appState:      platform.NewAppState(),
		Api:           api,
		modeHandlers:  make(ModeKeyHandlers),
		paintInterval: interval,
	}
	api.app = app
	app.AddWidget(w)
	app.UpdateCanvas()
	app.RegisterModeHandler(platform.ModeNormal, func(*tcell.EventKey) bool { return true })

	done := make(chan struct{})
	go func() {
		app.Run()
		close(done)
	}()
	t.Cleanup(func() {
		_ = scr.PostEvent(tcell.NewEventInterrupt("quit"))
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Run did not exit")
		}
	})

	// Let Run block in PollEvent long enough to buffer a tick.
	time.Sleep(2 * interval)

	for i := 0; i < 3; i++ {
		// Keystroke: paints at once, then burns the buffered tick.
		if err := scr.PostEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)

		before := w.draws.Load()
		if err := scr.PostEvent(tcell.NewEventInterrupt("async-output")); err != nil {
			t.Fatal(err)
		}

		deadline := time.Now().Add(2 * interval)
		for time.Now().Before(deadline) && w.draws.Load() <= before {
			time.Sleep(2 * time.Millisecond)
		}
		if w.draws.Load() <= before {
			t.Fatalf("round %d: async interrupt never painted; frame waited for the next input event", i)
		}
	}
}
