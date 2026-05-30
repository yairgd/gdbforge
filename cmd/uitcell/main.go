package main

import (
	"log"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/promptcore/internal/core"
	"github.com/yairgd/promptcore/internal/termui"
)

// drived from UIContext
type App struct {
	screen tcell.Screen
}

func (a *App) Screen() tcell.Screen {
	return a.screen
}

func (a *App) Emit(event core.Event) {
	a.screen.PostEvent(tcell.NewEventInterrupt(event))
}

func main() {
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}

	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}
	defer screen.Fini()

	app := &App{
		screen: screen,
	}

	//gdbWidget := termui.NewGDBWidget(app)
	codeWidget := termui.NewCodeWidget(app)
	codeWidget1 := termui.NewCodeWidget(app)

	//w, h := screen.Size()
	//gdbWidget.SetSize(w, h)

	tab := termui.NewTabTwoHozSplitWins(app, "basic debuger", codeWidget1, codeWidget)

	tab.Draw()

	for {
		ev := screen.PollEvent()

		switch e := ev.(type) {
		case *tcell.EventKey:
			switch e.Key() {
			case tcell.KeyCtrlD:
				return

			default:
				tab.HandleEvent(e)
				// Ctrl-C also goes here, so GDBWidget sends SIGINT to GDB.
				//	gdbWidget.HandleEvent(e)
			}

		case *tcell.EventResize:
			screen.Sync()
			tab.HandleEvent(e)
			//		gdbWidget.HandleEvent(e)

		case *tcell.EventInterrupt:
			tab.HandleEvent(e)
			//gdbWidget.HandleEvent(e)
		}
		tab.Draw()

		//	gdbWidget.Draw(screen)
	}
}
