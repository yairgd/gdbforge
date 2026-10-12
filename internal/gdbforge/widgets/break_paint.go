package widgets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

// breakGutterColor maps BreakGutter state to theme color.
// Priority: disabled → conditional → enabled. Callers handle at-$pc separately.
func breakGutterColor(g models.BreakGutter, st *debugstate.State) tcell.Color {
	if !g.Enabled {
		return st.BreakDisabledColor()
	}
	if g.Conditional() {
		return st.BreakCondColor()
	}
	return st.BreakColor()
}
