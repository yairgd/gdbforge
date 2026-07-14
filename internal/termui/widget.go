package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Widget interface {
	HandleEvent(ev tcell.Event)
	Draw(c Canvas)
	DrawStatusLine(c Canvas, active bool)
}

// Clearable is implemented by widgets that can clear their content via :clear.
type Clearable interface {
	Clear()
}
