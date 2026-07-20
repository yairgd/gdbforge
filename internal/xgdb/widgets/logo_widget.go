package widgets

import (
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbx/internal/termui"
)

// LogoWidget shows the xgdb banner in the code leaf until source is opened.
type LogoWidget struct {
	termui.BaseWidget
}

// NewLogoWidget returns the startup splash for the code pane.
func NewLogoWidget() *LogoWidget {
	return &LogoWidget{
		BaseWidget: termui.BaseWidget{PaneName: "xgdb"},
	}
}

func logoLines() []string {
	return []string{
		"██╗  ██╗ ██████╗ ██████╗ ██████╗ ",
		"╚██╗██╔╝██╔════╝ ██╔══██╗██╔══██╗",
		" ╚███╔╝ ██║  ███╗██║  ██║██████╔╝",
		" ██╔██╗ ██║   ██║██║  ██║██╔══██╗",
		"██╔╝ ██╗╚██████╔╝██████╔╝██████╔╝",
		"╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═════╝ ",
		"",
		"    >> xGDB: Extreme Tooling Suite <<",
	}
}

func (w *LogoWidget) HandleEvent(ev tcell.Event) {}

func (w *LogoWidget) Draw(c termui.Canvas) {
	style := tcell.StyleDefault
	title := style.Foreground(tcell.ColorYellow).Bold(true)
	tag := style.Foreground(tcell.ColorWhite)

	for y := 0; y < c.H(); y++ {
		c.ClearLine(y, style)
	}

	lines := logoLines()
	maxW := 0
	for _, line := range lines {
		if n := utf8.RuneCountInString(line); n > maxW {
			maxW = n
		}
	}
	startY := (c.H() - len(lines)) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (c.W() - maxW) / 2
	if startX < 0 {
		startX = 0
	}

	for i, line := range lines {
		y := startY + i
		if y < 0 || y >= c.H() {
			continue
		}
		st := title
		if i >= len(lines)-1 {
			st = tag
		}
		x := startX
		for _, ch := range line {
			if x >= c.W() {
				break
			}
			if x >= 0 {
				c.SetContent(x, y, ch, st)
			}
			x++
		}
	}
}

// LogoLinesForTest exposes the banner for unit tests.
func LogoLinesForTest() []string {
	return logoLines()
}
