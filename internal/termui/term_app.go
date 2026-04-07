package termui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
	"log"
)

type TermApp struct {
	widgets []Widget
	screen  tcell.Screen
	events  chan core.Event
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

func (app *TermApp) Draw() {
	for _, w := range app.widgets {
		w.Draw(app.screen)
	}
}

func (app *TermApp) HandleUIEvent(ev tcell.Event) {
	for _, w := range app.widgets {
		w.HandleEvent(ev)
	}
}

func (app *TermApp) SetSize(w int, h int) {
	for _, widget := range app.widgets {
		widget.SetSize(w, h)
	}
}

func (app *TermApp) Screen() tcell.Screen {
	return app.screen
}

func (app *TermApp) EventLoop() {

	w, h := app.screen.Size()
	app.SetSize(w, h)

	for {
		app.Draw()

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
			w, h := e.Size()
			app.SetSize(w, h)

		case *tcell.EventKey:
			if e.Key() == tcell.KeyCtrlE {
				return
			}
		}

		// 👇 All events (UI + core) are funneled through the same dispatch path
		for _, w := range app.widgets {
			w.HandleEvent(ev)
		}
	}
}
func (app *TermApp) Emit(e core.Event) {
	app.events <- e
}
