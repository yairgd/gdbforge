package termui

import (
	"log"
	"time"

	"github.com/gdamore/tcell/v2"
)

type AppApi interface {
	HandleCoreEvents(ev Event)
	HandleKey(ev *tcell.EventKey)
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
}

func NewTermApp() *TermApp {

	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}

	screen.EnableMouse()

	return &TermApp{
		screen:   screen,
		exit:     false,
		events:   make(chan Event, 100),
		uiEvents: make(chan tcell.Event, 100),
	}
}

func (app *TermApp) Widgets() []WidgetNode { return app.widgets }
func (app *TermApp) Exit()                 { app.exit = true }

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
			if app.Api != nil {
				app.Api.HandleCoreEvents(ev)
			}
		// event from tcell
		case ev := <-app.uiEvents:
			app.HandleEvent(ev)
		}

		app.Draw(Canvas{
			rect: app.canvas.Rect(),
			grid: app.frontBuffer,
		})

		app.frontBuffer.Draw(app.screen)
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

func (app *TermApp) Draw(c Canvas) {
	for _, w := range app.widgets {
		w.widget.Draw(Canvas{rect: w.rect, grid: c.grid})
	}

}

func (app *TermApp) Screen() tcell.Screen {
	return app.screen
}

const redrawInterrupt = "termui-redraw"

func (app *TermApp) RequestRedraw() {
	app.screen.PostEvent(tcell.NewEventInterrupt(redrawInterrupt))
}

func (a *TermApp) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventKey:
		if a.Api != nil {
			a.Api.HandleKey(e)
		}

		switch e.Key() {
		case tcell.KeyCtrlD:
			a.exit = true
			return

		default:
			//	tab.HandleEvent(e)
		}
	case *tcell.EventResize:
		_ = a.UpdateCanvas()
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
			if data == "gdb-exit" {
				return
			}
		}
	}

}

func (app *TermApp) Events() chan Event {
	return app.events
}
