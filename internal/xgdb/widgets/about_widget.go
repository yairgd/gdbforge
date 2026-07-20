package widgets

import (
	"fmt"
	"runtime/debug"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

const (
	aboutProductVersion = "0.1.0"
	// AboutEmail is the public contact shown on the About page.
	AboutEmail = "yairgd@gmail.com"
)

// AboutWidget is a passive, scrollable built-in page (Logger-style Viewport).
// No goroutines, timers, or background work.
type AboutWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer
}

// NewAboutWidget caches build info once into a read-only buffer.
func NewAboutWidget() *AboutWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)
	vp.LineStyle = aboutLineStyle

	w := &AboutWidget{
		BaseWidget: termui.BaseWidget{PaneName: "About"},
		viewport:   vp,
		buf:        buf,
	}
	for _, line := range buildAboutLines(readVCSBuildInfo()) {
		buf.AppendLine(line)
	}
	vp.Home()
	w.initKeyBindings()
	return w
}

func aboutLineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if line == "xgdb" {
		return st.Foreground(tcell.ColorYellow).Bold(true)
	}
	if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, " ") {
		return st.Foreground(tcell.ColorGray)
	}
	return st
}

type vcsBuildInfo struct {
	Revision string
	Time     string
	Modified string
}

func readVCSBuildInfo() vcsBuildInfo {
	info := vcsBuildInfo{
		Revision: "unknown",
		Time:     "unknown",
		Modified: "unknown",
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi == nil {
		return info
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if s.Value != "" {
				info.Revision = s.Value
			}
		case "vcs.time":
			if s.Value != "" {
				info.Time = s.Value
			}
		case "vcs.modified":
			switch s.Value {
			case "true":
				info.Modified = "dirty"
			case "false":
				info.Modified = "clean"
			default:
				if s.Value != "" {
					info.Modified = s.Value
				}
			}
		}
	}
	return info
}

func buildAboutLines(b vcsBuildInfo) []string {
	return []string{
		"xgdb",
		"",
		"xGDB: Extreme Tooling Suite — AI-assisted terminal debugger",
		"for Embedded Linux and C/C++ developers.",
		"",
		"Version:",
		"    " + aboutProductVersion,
		"",
		"Build:",
		"    Git SHA: " + b.Revision,
		"    Commit time: " + b.Time,
		"    Dirty state: " + b.Modified,
		"",
		"Author:",
		"    Yair Gadelov",
		"",
		"Email:",
		"    " + AboutEmail,
		"",
		"GitHub:",
		"    https://github.com/yairgd/newcgdb",
		"",
		"Inspired by:",
		"    cgdb",
		"    gdb",
		"    Vim",
		"",
		"Features:",
		"    • Terminal UI debugger",
		"    • Multi-pane interface",
		"    • Command mode",
		"    • AI-assisted workflows (experimental)",
		"",
		"License:",
		"    MIT License",
		"",
		"Copyright (c) 2026 Yair Gadelov",
	}
}

func (w *AboutWidget) initKeyBindings() {
	w.BindKeyFunc("scroll-up", func(args ...any) { w.viewport.ScrollLineUp() }, "<Up>", "k")
	w.BindKeyFunc("scroll-down", func(args ...any) { w.viewport.ScrollLineDown() }, "<Down>", "j")
	w.BindKeyFunc("page-up", func(args ...any) { w.viewport.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.viewport.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.viewport.ScrollHome() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.viewport.ScrollEnd() }, "<End>", "G")
}

// HandleFocusKey enables scroll in normal mode when About is focused.
func (w *AboutWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *AboutWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		w.viewport.HandleEvent(e)
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.viewport.HandleEvent(e)
	}
}

func (w *AboutWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(focused)
}

func (w *AboutWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *AboutWidget) Viewport() *termui.Viewport {
	return w.viewport
}

// LinesForTest exposes buffer lines for unit tests.
func (w *AboutWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}

// FormatBuildLine is a tiny helper kept for tests of missing-info rendering.
func FormatBuildLine(key, value string) string {
	if value == "" {
		value = "unknown"
	}
	return fmt.Sprintf("    %s: %s", key, value)
}
