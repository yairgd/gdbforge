package widgets

import (
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/termforge"
)

// LogoWidget shows the gdbforge banner in the code leaf until source is opened.
type LogoWidget struct {
	termforge.BaseWidget
}

// NewLogoWidget returns the startup splash for the code pane.
func NewLogoWidget() *LogoWidget {
	return &LogoWidget{
		BaseWidget: termforge.BaseWidget{PaneName: "gdbforge"},
	}
}

// banner is one size variant of the splash: block art plus an optional tagline.
type banner struct {
	art     []string
	tagline string
}

func wideBanner() banner {
	return banner{
		art: []string{
			" ██████╗ ██████╗ ██████╗ ███████╗ ██████╗ ██████╗  ██████╗ ███████╗",
			"██╔════╝ ██╔══██╗██╔══██╗██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝",
			"██║  ███╗██║  ██║██████╔╝█████╗  ██║   ██║██████╔╝██║  ███╗█████╗",
			"██║   ██║██║  ██║██╔══██╗██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝",
			"╚██████╔╝██████╔╝██████╔╝██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗",
			" ╚═════╝ ╚═════╝ ╚═════╝ ╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝",
		},
		tagline: ">> gdbforge: Extreme Tooling Suite <<",
	}
}

func narrowBanner() banner {
	return banner{
		art: []string{
			"┌─┐┌┬┐┌┐ ┌─┐┌─┐┬─┐┌─┐┌─┐",
			"│ ┬ ││├┴┐├┤ │ │├┬┘│ ┬├┤",
			"└─┘─┴┘└─┘└  └─┘┴└─└─┘└─┘",
		},
		tagline: "Extreme Tooling Suite",
	}
}

func plainBanner() banner {
	return banner{art: []string{"gdbforge"}}
}

// width is the widest line, which is what the block is centred on.
func (b banner) width() int {
	w := utf8.RuneCountInString(b.tagline)
	for _, line := range b.art {
		if n := utf8.RuneCountInString(line); n > w {
			w = n
		}
	}
	return w
}

func (b banner) lines() []string {
	if b.tagline == "" {
		return b.art
	}
	return append(append([]string{}, b.art...), "", b.tagline)
}

// bannerFor picks the largest variant that fits, so the art is never clipped
// mid-glyph in a narrow code pane.
func bannerFor(width int) banner {
	for _, b := range []banner{wideBanner(), narrowBanner()} {
		if b.width() <= width {
			return b
		}
	}
	return plainBanner()
}

func logoLines() []string {
	return wideBanner().lines()
}

func (w *LogoWidget) HandleEvent(ev tcell.Event) {}

func (w *LogoWidget) Draw(c termforge.Canvas) {
	style := tcell.StyleDefault
	title := style.Foreground(tcell.ColorYellow).Bold(true)
	tag := style.Foreground(tcell.ColorWhite)

	for y := 0; y < c.H(); y++ {
		c.ClearLine(y, style)
	}

	b := bannerFor(c.W())
	lines := b.lines()
	maxW := b.width()
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
		x := startX
		if b.tagline != "" && i == len(lines)-1 {
			// The tagline is shorter than the art, so centre it within the block.
			st = tag
			x += (maxW - utf8.RuneCountInString(line)) / 2
		}
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

// LogoLinesForWidthForTest exposes the variant chosen for a pane width.
func LogoLinesForWidthForTest(width int) []string {
	return bannerFor(width).lines()
}
