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
	exit    bool
	// widgets draw here all the time
	backBuffer *Grid
	// last frame that was actually displayed
	frontBuffer *Grid

	canvas Canvas
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
		screen: screen,
		exit:   false,
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
	for !app.exit {

		ev := app.screen.PollEvent()
		app.HandleEvent(ev)

		for _, w := range app.widgets {
			w.HandleEvent(ev)
		}
		//	app.frontBuffer.Clear(app.screen, tcell.StyleDefault)

		app.Draw(Canvas{app.screen, app.canvas.Rect(), app.frontBuffer})

		// move grid to sceen
		app.frontBuffer.Draw(app.screen, tcell.StyleDefault)
		app.screen.Show()

	}
	app.screen.Fini()
}
func (app *TermApp) UpdateCanvas() Canvas {
	app.screen.Sync()
	w, h := app.screen.Size()
	app.backBuffer = NewGrid(w, h)
	app.frontBuffer = NewGrid(w, h)
	app.canvas = Canvas{app.screen, Rect{0, 0, w, h}, app.frontBuffer}
	return app.canvas

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
			a.exit = true

		default:
			//	tab.HandleEvent(e)
		}
	case *tcell.EventResize:
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

func (app *TermApp) Emit(e core.Event) {
	app.events <- e
}
