package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestBreakGutterColorPriority(t *testing.T) {
	st := debugstate.New(platform.NewAppState())
	st.SetBreakColor(tcell.ColorRed)
	st.SetBreakDisabledColor(tcell.ColorYellow)
	st.SetBreakCondColor(tcell.ColorOrange)

	if c := breakGutterColor(models.BreakGutter{Enabled: false}, st); c != tcell.ColorYellow {
		t.Fatalf("disabled=%v", c)
	}
	if c := breakGutterColor(models.BreakGutter{Enabled: true, Condition: "i>0"}, st); c != tcell.ColorOrange {
		t.Fatalf("cond=%v", c)
	}
	if c := breakGutterColor(models.BreakGutter{Enabled: true}, st); c != tcell.ColorRed {
		t.Fatalf("enabled=%v", c)
	}
}
