package main

import (
	"context"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/gdb"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type codeRefreshMsg struct {
	widget *widgets.CodeWidget
}

// onGdbStopped updates AppState / CodeWidget when GDB hits a breakpoint or steps.
func (a *DebuggerApp) onGdbStopped(stop *gdb.MiStopMsg) {
	if stop == nil || stop.File == "" {
		return
	}
	file := stop.File
	line := stop.Line

	a.State().SetCurrentLocation(file, line)

	go func() {
		a.ensureSourceFiles()
		w := a.ensureCodeBuffer(file)
		if w != nil {
			_ = w.ShowLocation(file, line)
		}
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{widget: w}))
		}
	}()
}

// applyCodeStop refreshes the source view for a stop without stealing focus from
// the GDB console. If the focused pane is already a CodeWidget, switch that pane
// to the stop file; otherwise update another visible CodeWidget leaf in place.
func (a *DebuggerApp) applyCodeStop(w *widgets.CodeWidget) {
	if w == nil || a.tab == nil {
		return
	}
	focused := a.tab.FocusedWidget()
	if cw, ok := focused.(*widgets.CodeWidget); ok {
		if cw != w {
			_ = a.tab.ReplaceFocusedWidget(w)
		}
		return
	}
	_ = a.tab.ReplaceMatchingLeafWidget(w, func(x termui.Widget) bool {
		_, ok := x.(*widgets.CodeWidget)
		return ok
	})
}

func (a *DebuggerApp) ensureSourceFiles() {
	if len(a.State().SourceFiles()) > 0 || a.gdbMcp == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := a.gdbMcp.Query(ctx, "-file-list-exec-source-files")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("code").Error(err.Error())
		}
		return
	}
	files := mcp.ParseSourceFileList(raw)
	if len(files) > 0 {
		a.State().SetSourceFiles(files)
	}
}
