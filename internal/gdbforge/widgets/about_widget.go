package widgets

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

const (
	// AboutEmail is the public contact shown on the About page.
	AboutEmail = "yairgd@gmail.com"
	// AboutNotForRelease is shown when the binary was not stamped from a release tag.
	AboutNotForRelease = "not for release"
)

// AboutWidget is a passive, scrollable built-in page (Logger-style Viewport).
// No goroutines, timers, or background work.
type AboutWidget struct {
	termui.BaseWidget
	doc *termui.ScrollDocument
	buf *platform.Buffer
}

// NewAboutWidget caches build info once into a read-only buffer.
// version should be the link-time stamp (main.version), typically a tag like
// "v1.0.0". Non-release builds show AboutNotForRelease.
func NewAboutWidget(version string) *AboutWidget {
	buf := platform.NewBuffer()
	vp := termui.NewScrollDocument(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)
	vp.LineStyle = aboutLineStyle

	w := &AboutWidget{
		BaseWidget: termui.BaseWidget{PaneName: "About"},
		doc:   vp,
		buf:        buf,
	}
	for _, line := range buildAboutLines(FormatAboutVersion(version), readVCSBuildInfo()) {
		buf.AppendLine(line)
	}
	vp.Home()
	w.initKeyBindings()
	return w
}

func aboutLineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if line == "gdbforge" {
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

var (
	// releaseTagRe matches v1.0.0 / 1.0.0 / v1.0.0-rc.1 (not git-describe -N-gSHA).
	releaseTagRe     = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
	gitDescribeExtra = regexp.MustCompile(`-\d+-g[0-9a-f]+$`)
)

// FormatAboutVersion maps the link-time version stamp to the About display.
// Release tags look like "v1.0.0" or "v1.0.0-rc.1". Anything else (empty,
// "dev", git-describe with -N-gSHA, …) is not a release build.
func FormatAboutVersion(version string) string {
	v := strings.TrimSpace(version)
	if !isReleaseVersionTag(v) {
		return AboutNotForRelease
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

func isReleaseVersionTag(v string) bool {
	if v == "" || v == "dev" {
		return false
	}
	if gitDescribeExtra.MatchString(v) {
		return false
	}
	return releaseTagRe.MatchString(v)
}

func buildAboutLines(version string, b vcsBuildInfo) []string {
	return []string{
		"gdbforge",
		"",
		"gdbforge: Extreme Tooling Suite — AI-assisted terminal debugger",
		"for Embedded Linux and C/C++ developers.",
		"",
		"Version:",
		"    " + version,
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
		"    https://github.com/yairgd/gdbforge",
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
	w.BindKeyFunc("scroll-up", func(args ...any) { w.doc.ScrollLineUp() }, "<Up>", "k")
	w.BindKeyFunc("scroll-down", func(args ...any) { w.doc.ScrollLineDown() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.doc.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.doc.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("page-up", func(args ...any) { w.doc.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.doc.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.doc.ScrollHome() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.doc.ScrollEnd() }, "<End>", "G")
}

// HandleFocusKey enables scroll in normal mode when About is focused.
func (w *AboutWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *AboutWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		w.doc.HandleEvent(e)
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.doc.HandleEvent(e)
	}
}

func (w *AboutWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.doc.SetCursorVisible(focused)
}

func (w *AboutWidget) Draw(c termui.Canvas) {
	w.doc.Draw(c)
}

func (w *AboutWidget) Viewport() *termui.ScrollDocument {
	return w.doc
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
