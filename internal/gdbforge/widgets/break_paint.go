package widgets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
)

// themeFrom reads nil-safe theme colors from debugstate (or platform defaults).
type themeFrom struct {
	st *debugstate.State
}

func (t themeFrom) Break() tcell.Color {
	if t.st != nil {
		return t.st.BreakColor()
	}
	return platform.DefaultBreakColor
}

func (t themeFrom) BreakDisabled() tcell.Color {
	if t.st != nil {
		return t.st.BreakDisabledColor()
	}
	return platform.DefaultBreakDisabledColor
}

func (t themeFrom) BreakCond() tcell.Color {
	if t.st != nil {
		return t.st.BreakCondColor()
	}
	return platform.DefaultBreakCondColor
}

func (t themeFrom) PC() tcell.Color {
	if t.st != nil {
		return t.st.PCColor()
	}
	return platform.DefaultPCColor
}

func (t themeFrom) StackBreak() tcell.Color {
	if t.st != nil {
		return t.st.StackBreakColor()
	}
	return platform.DefaultStackBreakColor
}

func (t themeFrom) Mark() tcell.Color {
	if t.st != nil {
		return t.st.MarkColor()
	}
	return platform.DefaultMarkColor
}

func (t themeFrom) MarkDim() tcell.Color {
	if t.st != nil {
		return t.st.MarkDimColor()
	}
	return platform.DefaultMarkDimColor
}

func (t themeFrom) CodeSel() tcell.Color {
	if t.st != nil {
		return t.st.CodeSelColor()
	}
	return platform.DefaultCodeSelColor
}

func (t themeFrom) Muted() tcell.Color {
	if t.st != nil {
		return t.st.MutedColor()
	}
	return platform.DefaultMutedColor
}

func (t themeFrom) Search() tcell.Color {
	if t.st != nil {
		return t.st.SearchColor()
	}
	return platform.DefaultSearchColor
}

// breakGutterColor maps BreakGutter state to theme color.
// Priority: disabled → conditional → enabled. Callers handle at-$pc separately.
func breakGutterColor(g models.BreakGutter, st *debugstate.State) tcell.Color {
	th := themeFrom{st}
	if !g.Enabled {
		return th.BreakDisabled()
	}
	if g.Conditional() {
		return th.BreakCond()
	}
	return th.Break()
}
