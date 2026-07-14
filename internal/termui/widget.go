package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Widget interface {
	HandleEvent(ev tcell.Event)
	Draw(c Canvas)
	DrawStatusLine(c Canvas, active bool)
}

// Focusable receives focus notifications from the layout before Draw.
type Focusable interface {
	SetFocused(focused bool)
}

// Clearable is implemented by widgets that can clear their content via :clear.
type Clearable interface {
	Clear()
}

// FocusKeyHandler receives keys while focused in normal mode (before insert).
// Used by scrollable panes like LoggerWidget.
type FocusKeyHandler interface {
	HandleFocusKey(ev *tcell.EventKey) bool
}
