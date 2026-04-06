package uitcell

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
)

func StartGdbUIBridge(
	screen tcell.Screen,
	widget *GDBWidget,
	outputChan <-chan core.GdbOutputMsg,
) {
	go func() {
		for msg := range outputChan {
			widget.OnGDBOutput(msg.Data)
			screen.PostEvent(tcell.NewEventInterrupt(msg))
		}

		screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	}()
}
