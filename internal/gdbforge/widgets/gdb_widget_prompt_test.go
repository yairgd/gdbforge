package widgets

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/termui"
)

func TestGDBWidgetDrawSmoke(t *testing.T) {
	w := NewGDBWidget()
	g := termui.NewGrid(40, 10)
	c := termui.NewCanvas(g).WithRect(termui.NewRect(0, 0, 40, 10))
	w.Draw(c)
	w.WriteBoot("(gdb) \n")
	w.AppendLines([]string{">>> AI: ping"})
}
