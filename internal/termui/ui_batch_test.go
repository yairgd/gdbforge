package termui

import (
	"fmt"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

type batchTestAPI struct {
	events []string
}

func (m *batchTestAPI) HandleMouse(*tcell.EventMouse)  {}
func (m *batchTestAPI) HandleResize()                  {}
func (m *batchTestAPI) HandleTTYResume()               {}
func (m *batchTestAPI) HandleInterrupt(ev *tcell.EventInterrupt) {
	m.events = append(m.events, "i:"+fmt.Sprint(ev.Data()))
}

func TestPollEventBatchCaps(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()

	app := &TermApp{screen: scr}
	for i := 0; i < 5; i++ {
		if err := scr.PostEvent(tcell.NewEventInterrupt(i)); err != nil {
			t.Fatalf("PostEvent: %v", err)
		}
	}

	batch := app.pollEventBatch(2)
	if len(batch) != 2 {
		t.Fatalf("first len=%d want 2", len(batch))
	}
	batch = app.pollEventBatch(3)
	if len(batch) != 3 {
		t.Fatalf("second len=%d want 3", len(batch))
	}
}

func TestHandleUIEventBatchKeysBeforeInterrupts(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()

	api := &batchTestAPI{}
	app := &TermApp{
		screen:       scr,
		appState:     platform.NewAppState(),
		Api:          api,
		modeHandlers: make(ModeKeyHandlers),
	}
	app.RegisterModeHandler(platform.ModeNormal, func(ev *tcell.EventKey) bool {
		api.events = append(api.events, "k")
		return true
	})

	batch := []tcell.Event{
		tcell.NewEventInterrupt("x"),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone),
		tcell.NewEventInterrupt("y"),
	}
	if urgent := app.handleUIEventBatch(batch); !urgent {
		t.Fatal("expected urgent batch")
	}
	want := []string{"k", "i:x", "i:y"}
	if len(api.events) != len(want) {
		t.Fatalf("events=%v want %v", api.events, want)
	}
	for i := range want {
		if api.events[i] != want[i] {
			t.Fatalf("events=%v want %v", api.events, want)
		}
	}
}
