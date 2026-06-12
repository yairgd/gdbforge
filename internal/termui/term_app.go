package termui

import (
	"log"

	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
)

type TermApp struct {
	widgets []Widget
	screen  tcell.Screen
	events  chan core.Event
	// widgets draw here all the time
	backBuffer *Grid
	// last frame that was actually displayed
	frontBuffer *Grid
}

func NewTermApp() *TermApp {
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}

	return &TermApp{
		screen: screen,
		events: make(chan core.Event, 100),
	}
}

func (app *TermApp) Close() {
	app.screen.Fini()
}

func (app *TermApp) AddWidget(w Widget) {
	app.widgets = append(app.widgets, w)
}

func (app *TermApp) Run() {
	for {
		app.Draw(Canvas{app.screen, Rect{0, 0, app.frontBuffer.W, app.frontBuffer.H}, app.frontBuffer})
		app.frontBuffer.Draw(app.screen, tcell.StyleDefault)

		app.screen.Show()

		ev := app.screen.PollEvent()
		app.HandleEvent(ev)

		for _, w := range app.widgets {
			w.HandleEvent(ev)
		}

	}
}
func (app *TermApp) UpdateCanvas() Canvas {
	w, h := app.screen.Size()
	app.backBuffer = NewGrid(w, h)
	app.frontBuffer = NewGrid(w, h)
	return Canvas{app.screen, Rect{0, 0, w, h}, app.frontBuffer}

}

func (app *TermApp) Draw(c Canvas) {
	for _, w := range app.widgets {
		w.Draw(c)
	}

}

func (app *TermApp) HandleUIEvent(ev tcell.Event) {
	for _, w := range app.widgets {
		w.HandleEvent(ev)
	}
}

func (app *TermApp) Screen() tcell.Screen {
	return app.screen
}

func (a *TermApp) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {
	case *tcell.EventKey:
		switch e.Key() {
		case tcell.KeyCtrlD:
			return

		default:
			//	tab.HandleEvent(e)
		}
	case *tcell.EventResize:
		a.screen.Sync()
		_ = a.UpdateCanvas()

	case *tcell.EventInterrupt:
		switch data := e.Data().(type) {
		//	case core.Event:
		//		for _, w := range app.widgets {
		//			w.HandleEvent(data)
		//		}
		case string:
			if data == "gdb-exit" {
				return
			}
		}
	}

}

func (app *TermApp) EventLoop() {
	for {

		ev := app.screen.PollEvent()

		switch e := ev.(type) {

		case *tcell.EventInterrupt:

			switch data := e.Data().(type) {
			//	case core.Event:
			//		for _, w := range app.widgets {
			//			w.HandleEvent(data)
			//		}
			case string:
				if data == "gdb-exit" {
					return
				}
			}

		case *tcell.EventResize:
			app.screen.Sync()
			_ = app.UpdateCanvas()

		case *tcell.EventKey:
			if e.Key() == tcell.KeyCtrlE {
				return
			}
		}

		// 👇 All events (UI + core) are funneled through the same dispatch path
		for _, w := range app.widgets {
			w.HandleEvent(ev)
		}
		app.screen.Show()

	}
}
func (app *TermApp) Emit(e core.Event) {
	app.events <- e
}
