package termui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

type AppApi interface {
	HandleMouse(ev *tcell.EventMouse)
	HandleResize()
	HandleInterrupt(ev *tcell.EventInterrupt)
	// HandleTTYResume runs after Suspend/Resume or RunForeground; reset in-pane terminals.
	HandleTTYResume()
}

type WidgetNode struct {
	widget Widget
	rect   Rect
}

func (w *WidgetNode) SetRect(r Rect) {
	w.rect = r
}
func (w *WidgetNode) Rect() Rect     { return w.rect }
func (w *WidgetNode) Widget() Widget { return w.widget }

type TermApp struct {
	Api     AppApi
	widgets []WidgetNode
	screen  tcell.Screen
	exit    bool
	// widgets draw here all the time
	// last frame that was actually displayed
	frontBuffer *Grid
	canvas      Canvas

	mouseActive bool
	mouseX      int
	mouseY      int

	// ttyReleases counts completed Suspend/RunForeground cycles so the event
	// loop can tell that a key handler gave the tty away and came back.
	ttyReleases  int
	layoutDirty  bool
	appState     *platform.AppState
	modeHandlers ModeKeyHandlers
	closeOnce    sync.Once
	suspendMu    sync.Mutex
}

func NewTermApp() *TermApp {

	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}

	screen.EnableMouse(tcell.MouseMotionEvents)
	screen.EnablePaste()

	return &TermApp{
		screen:       screen,
		exit:         false,
		modeHandlers: make(ModeKeyHandlers),
		appState:     platform.NewAppState(),
	}
}

func (app *TermApp) Widgets() []WidgetNode { return app.widgets }
func (app *TermApp) Exit()                 { app.exit = true }

func (app *TermApp) Mode() platform.Mode {
	return app.appState.Mode()
}

func (app *TermApp) SetMode(mode platform.Mode) {
	app.appState.SetMode(mode)
}

// State returns the process-global AppState (modes, PTY owner, layout policy).
func (app *TermApp) State() *platform.AppState {
	return app.appState
}

func (app *TermApp) RegisterModeHandler(mode platform.Mode, h KeyHandler) {
	app.modeHandlers[mode] = h
}

func (app *TermApp) HandleKey(ev *tcell.EventKey) {
	if h, ok := app.modeHandlers[app.appState.Mode()]; ok {
		if h(ev) {
			return
		}
	}
}

func (app *TermApp) Close() {
	if app == nil {
		return
	}
	app.closeOnce.Do(func() {
		app.exit = true
		if app.screen == nil {
			return
		}
		// Give terminal enough time to disable mouse reporting.
		// Without this delay, pending mouse escape sequences may
		// leak to the shell after Fini().
		time.Sleep(100 * time.Millisecond)
		app.screen.DisableMouse()
		app.screen.Fini()
	})
}

func (app *TermApp) drainTcellEventQueue() {
	if app == nil || app.screen == nil {
		return
	}
	for app.screen.HasPendingEvent() {
		if ev := app.screen.PollEvent(); ev == nil {
			return
		}
	}
}

func isAlreadyEngaged(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already engaged")
}

func (app *TermApp) syncTerminalDimensions() (w, h int) {
	if app == nil || app.screen == nil {
		return 80, 24
	}
	app.screen.Sync()
	w, h = app.screen.Size()
	if tty, ok := app.screen.Tty(); ok {
		if ws, err := tty.WindowSize(); err == nil && ws.Width > 0 && ws.Height > 0 {
			w, h = ws.Width, ws.Height
		}
	}
	if w < 8 || h < 4 {
		time.Sleep(10 * time.Millisecond)
		app.screen.Sync()
		w, h = app.screen.Size()
	}
	if w < 8 {
		w = 80
	}
	if h < 4 {
		h = 24
	}
	return w, h
}

func (app *TermApp) restoreAfterResume() {
	if app == nil || app.screen == nil {
		return
	}
	flushControllingTTYInput()
	app.screen.EnableMouse(tcell.MouseMotionEvents)
	app.screen.EnablePaste()
	app.screen.Clear()
	app.screen.Sync()
	_, _ = app.syncTerminalDimensions()
	_ = app.UpdateCanvas()
	app.layoutDirty = true
	if app.Api != nil {
		app.Api.HandleResize()
		app.Api.HandleTTYResume()
	}
	app.drainTcellEventQueue()
	app.present()
}

func (app *TermApp) resumeAfterSuspend() error {
	if err := app.screen.Resume(); err != nil {
		if isAlreadyEngaged(err) {
			// Stopped mid-disengage on a prior cycle — force clean re-engage.
			_ = app.screen.Suspend()
			if err2 := app.screen.Resume(); err2 != nil && !isAlreadyEngaged(err2) {
				return err2
			}
		} else {
			return err
		}
	}
	app.restoreAfterResume()
	return nil
}

// withTTYReleased disengages tcell, runs fn on the real tty, then re-engages.
// PollEvent runs only on the UI thread — no background poll goroutine — so
// Suspend/Resume cannot race with a second PollEvent caller.
func (app *TermApp) withTTYReleased(fn func() error) error {
	if app == nil || app.screen == nil {
		return fmt.Errorf("no screen")
	}
	app.drainTcellEventQueue()

	if err := app.screen.Suspend(); err != nil {
		return fmt.Errorf("suspend: %w", err)
	}
	app.ttyReleases++

	runErr := fn()
	if err := app.resumeAfterSuspend(); err != nil {
		app.restoreAfterResume()
		if runErr == nil {
			return err
		}
		return fmt.Errorf("%v (resume: %v)", runErr, err)
	}
	app.drainTcellEventQueue()
	return runErr
}

// Suspend restores the terminal and stops this process with SIGTSTP (job
// control), like Ctrl-Z in GDB/vim. Resumes on SIGCONT / shell `fg`.
//
// Must use Screen.Suspend/Resume — Fini() is once-only (finiOnce), so
// Fini+Init breaks on the second Ctrl-Z with "already engaged".
func (app *TermApp) Suspend() {
	if app == nil || app.screen == nil {
		return
	}
	app.suspendMu.Lock()
	defer app.suspendMu.Unlock()

	if err := app.withTTYReleased(stopForShellJobControl); err != nil {
		log.Printf("suspend: %v", err)
	}
}

// RunForeground suspends the tcell screen, runs argv on the real stdin/stdout
// (terminal vim, less, …), then resumes gdbforge. Must run on the UI thread.
func (app *TermApp) RunForeground(argv []string) error {
	if app == nil || app.screen == nil {
		return fmt.Errorf("no screen")
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return fmt.Errorf("empty command")
	}
	app.suspendMu.Lock()
	defer app.suspendMu.Unlock()

	return app.withTTYReleased(func() error {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

func (app *TermApp) AddWidget(w Widget) {
	app.widgets = append(app.widgets, WidgetNode{
		widget: w,
	})
}

func (app *TermApp) pollEventBatch(max int) []tcell.Event {
	if app == nil || app.screen == nil {
		return nil
	}
	if max < 1 {
		max = 1
	}
	if app.screen.HasPendingEvent() {
		return app.drainPollBatch(max)
	}
	ev := app.screen.PollEvent()
	if ev == nil {
		return nil
	}
	batch := []tcell.Event{ev}
	if len(batch) < max && app.screen.HasPendingEvent() {
		batch = append(batch, app.drainPollBatch(max-len(batch))...)
	}
	return batch
}

func (app *TermApp) drainPollBatch(max int) []tcell.Event {
	batch := make([]tcell.Event, 0, max)
	for len(batch) < max && app.screen.HasPendingEvent() {
		if ev := app.screen.PollEvent(); ev != nil {
			batch = append(batch, ev)
		} else {
			break
		}
	}
	return batch
}

func (app *TermApp) Run() {
	defer app.Close()

	paintTicker := time.NewTicker(16 * time.Millisecond)
	defer paintTicker.Stop()
	dirty := false
	for !app.exit {
		select {
		case <-paintTicker.C:
			if dirty {
				app.present()
				dirty = false
			}
		default:
		}

		if !app.screen.HasPendingEvent() {
			select {
			case <-paintTicker.C:
				if dirty {
					app.present()
					dirty = false
				}
				continue
			default:
			}
		}

		batch := app.pollEventBatch(96)
		if len(batch) == 0 {
			continue
		}
		urgent := app.handleUIEventBatch(batch)
		dirty = true
		if urgent {
			app.present()
			dirty = false
		}
	}
}

// handleUIEventBatch processes keys/mouse/resize before interrupts so Ctrl-C
// is not stuck behind a flood of output PostEvents.
// Returns true if the batch needs an immediate paint (input / resize).
func (app *TermApp) handleUIEventBatch(batch []tcell.Event) bool {
	urgent := false
	var keys, mice, resizes, interrupts, other []tcell.Event
	for _, ev := range batch {
		switch ev.(type) {
		case *tcell.EventKey:
			keys = append(keys, ev)
			urgent = true
		case *tcell.EventMouse:
			mice = append(mice, ev)
			urgent = true
		case *tcell.EventResize:
			resizes = append(resizes, ev)
			urgent = true
		case *tcell.EventInterrupt:
			interrupts = append(interrupts, ev)
		default:
			other = append(other, ev)
			urgent = true
		}
	}
	// A key that releases the tty (Ctrl-Z) blocks until the shell resumes us,
	// so the rest of the batch was typed before we stopped. Drop it, like
	// restoreAfterResume already drops input queued while stopped: a fast
	// double Ctrl-Z read in one go would otherwise suspend twice and need a
	// second fg.
	gen := app.ttyReleases
	for _, ev := range keys {
		app.HandleEvent(ev)
		if app.ttyReleases != gen {
			return true
		}
	}
	for _, ev := range mice {
		app.HandleEvent(ev)
	}
	for _, ev := range resizes {
		app.HandleEvent(ev)
	}
	for _, ev := range other {
		app.HandleEvent(ev)
	}
	for _, ev := range interrupts {
		app.HandleEvent(ev)
	}
	return urgent
}

func (app *TermApp) present() {
	app.Draw(Canvas{
		rect: app.canvas.Rect(),
		grid: app.frontBuffer,
	})
	app.frontBuffer.Draw(app.screen)
	app.frontBuffer.ApplySystemCursor(app.screen)
	app.screen.Show()
}

func (app *TermApp) UpdateCanvas() Canvas {
	app.screen.Sync()
	w, h := app.screen.Size()
	app.frontBuffer = NewGrid(w, h)
	app.canvas = Canvas{rect: NewRect(0, 0, w, h), grid: app.frontBuffer}
	return app.canvas
}

func (app *TermApp) MarkLayoutDirty() {
	app.layoutDirty = true
}

func (app *TermApp) Draw(c Canvas) {
	if app.layoutDirty {
		c.grid.Clear()
		app.layoutDirty = false
	}
	c.grid.HideCursor()

	for _, w := range app.widgets {
		w.widget.Draw(Canvas{rect: w.rect, grid: c.grid})
	}

	if app.mouseActive && !c.grid.nativeCursorSet {
		c.grid.ShowCursor(app.mouseX, app.mouseY)
	}

}

func (app *TermApp) Screen() tcell.Screen {
	return app.screen
}

const redrawInterrupt = "termui-redraw"
const frameInterrupt = "termui-frame"

func (app *TermApp) RequestRedraw() {
	app.screen.PostEvent(tcell.NewEventInterrupt(redrawInterrupt))
}

func (app *TermApp) RequestFrame() {
	app.layoutDirty = true
	app.screen.PostEvent(tcell.NewEventInterrupt(frameInterrupt))
}

// PostInterrupt queues payload on the UI thread (worker-safe via tcell).
func (app *TermApp) PostInterrupt(payload any) {
	if app == nil || app.screen == nil {
		return
	}
	_ = app.screen.PostEvent(tcell.NewEventInterrupt(payload))
}

func (a *TermApp) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventKey:
		a.mouseActive = false
		a.HandleKey(e)

	case *tcell.EventMouse:
		a.mouseX, a.mouseY = e.Position()
		if e.Buttons()&tcell.ButtonPrimary != 0 {
			a.mouseActive = false
		} else if e.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
			a.mouseActive = false
		} else if e.Buttons() == tcell.ButtonNone {
			a.mouseActive = true
		}
		if a.Api != nil {
			a.Api.HandleMouse(e)
		}

	case *tcell.EventResize:
		_ = a.UpdateCanvas()
		a.layoutDirty = true
		if a.Api != nil {
			a.Api.HandleResize()
		}

	case *tcell.EventInterrupt:
		switch data := e.Data().(type) {
		case string:
			if data == redrawInterrupt {
				_ = a.UpdateCanvas()
				return
			}
			if data == frameInterrupt {
				return
			}
		}
		if a.Api != nil {
			a.Api.HandleInterrupt(e)
		}

	case *tcell.EventClipboard, *tcell.EventPaste:
		// Command/completion mode: only the cmdline should receive paste
		// (GDB may still be the focused tab leaf).
		if a.Mode() == platform.ModeCommand || a.Mode() == platform.ModeCompletion {
			for i := range a.widgets {
				if _, ok := a.widgets[i].widget.(*CmdWidget); ok {
					a.widgets[i].widget.HandleEvent(e)
				}
			}
			return
		}
		for i := range a.widgets {
			a.widgets[i].widget.HandleEvent(e)
		}
	}

}

func (app *TermApp) CopyToClipboard(text string) {
	if text == "" {
		return
	}
	app.screen.SetClipboard([]byte(text))
	platform.SetClipboardText(text)
	// Middle-click outside gdbforge reads X11 PRIMARY; CLIPBOARD alone is not enough.
	platform.SetPrimaryText(text)
}

func (app *TermApp) PasteFromClipboard() string {
	if text, ok := platform.GetClipboardText(); ok {
		return text
	}
	return ""
}

func (app *TermApp) PasteFromPrimary() string {
	if text, ok := platform.GetPrimaryText(); ok {
		return text
	}
	// Fallback when PRIMARY is empty (e.g. copy from an app that only sets CLIPBOARD).
	return app.PasteFromClipboard()
}

// ClipboardIO returns the shared bridge for Viewport-backed widgets.
func (app *TermApp) ClipboardIO() ClipboardIO {
	return ClipboardIO{
		Copy:         app.CopyToClipboard,
		Paste:        app.PasteFromClipboard,
		PastePrimary: app.PasteFromPrimary,
	}
}
