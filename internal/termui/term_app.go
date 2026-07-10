package termui

import (
	"log"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
)

type AppApi interface {
	HandleMouse(ev *tcell.EventMouse)
	HandleResize()
}

type WidgetNode struct {
	widget Widget
	rect   Rect
}

func (w *WidgetNode) SetRect(r Rect) {
	w.rect = r
}
func (w *WidgetNode) Widget() Widget { return w.widget }

type TermApp struct {
	Api     AppApi
	widgets []WidgetNode
	screen  tcell.Screen
	events  chan Event
	exit    bool
	// widgets draw here all the time
	// last frame that was actually displayed
	frontBuffer *Grid
	uiEvents    chan tcell.Event
	canvas      Canvas

	mouseActive bool
	mouseX      int
	mouseY      int

	layoutDirty     bool
	appState        platform.AppState
	modeHandlers    ModeKeyHandlers
	commandHandlers CommandHandlers
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

	return &TermApp{
		screen:          screen,
		exit:            false,
		events:          make(chan Event, 100),
		uiEvents:        make(chan tcell.Event, 100),
		modeHandlers:    make(ModeKeyHandlers),
		commandHandlers: make(CommandHandlers),
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

func (app *TermApp) RegisterModeHandler(mode platform.Mode, h KeyHandler) {
	app.modeHandlers[mode] = h
}

func (app *TermApp) RegisterCommandHandler(id CommandID, h CommandHandler) {
	app.commandHandlers[id] = h
}

func (app *TermApp) HandleCoreEvents(ev Event) {
	msg, ok := ev.(CommandEvent)
	if !ok {
		return
	}
	if h, ok := app.commandHandlers[msg.CommandID()]; ok {
		h(msg)
	}
}

func (app *TermApp) HandleKey(ev *tcell.EventKey) {
	if h, ok := app.modeHandlers[app.appState.Mode()]; ok {
		if h(ev) {
			return
		}
	}
}

func (app *TermApp) Close() {
	app.screen.Fini()
}

func (app *TermApp) AddWidget(w Widget) {
	app.widgets = append(app.widgets, WidgetNode{
		widget: w,
	})
}

func (app *TermApp) Run() {
	defer func() {
		// Give terminal enough time to disable mouse reporting.
		// Without this delay, pending mouse escape sequences may
		// leak to the shell after Fini().
		time.Sleep(100 * time.Millisecond)
		app.screen.DisableMouse()
		app.screen.Fini()
	}()
	// UI event source — PollEvent blocks; run off the main loop goroutine.
	go func() {
		for {
			app.uiEvents <- app.screen.PollEvent()
		}
	}()
	for !app.exit {
		select {
		// events from termui events
		case ev := <-app.events:
			app.HandleCoreEvents(ev)
		// event from tcell
		case ev := <-app.uiEvents:
			app.HandleEvent(ev)
		}

		app.Draw(Canvas{
			rect: app.canvas.Rect(),
			grid: app.frontBuffer,
		})

		app.frontBuffer.Draw(app.screen)
		app.frontBuffer.ApplySystemCursor(app.screen)
		app.screen.Show()
	}

}

func (app *TermApp) UpdateCanvas() Canvas {
	app.screen.Sync()
	w, h := app.screen.Size()
	//	app.backBuffer = NewGrid(w, h)
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

func (a *TermApp) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventKey:
		a.mouseActive = false
		a.HandleKey(e)

		switch e.Key() {
		case tcell.KeyCtrlD:
			a.exit = true
			return

		default:
			//	tab.HandleEvent(e)
		}

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
			if data == "gdb-exit" {
				return
			}
		}
	}

}

func (app *TermApp) CopyToClipboard(text string) {
	if text == "" {
		return
	}
	app.screen.SetClipboard([]byte(text))
	platform.SetClipboardText(text)
}

func (app *TermApp) Events() chan Event {
	return app.events
}
