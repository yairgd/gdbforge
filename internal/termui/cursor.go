package termui

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
)

// pointerShape values for OSC 22 (system mouse pointer, not a drawn cell).
const (
	PointerDefault = "default" // arrow
	PointerText    = "text"    // I-beam over text
)

// writeOSC22 sets the OS/terminal mouse pointer shape (OSC 22).
// Unsupported terminals ignore this safely.
func writeOSC22(shape string) {
	if !shapeKnownTTY() {
		return
	}
	if shape == "" {
		fmt.Print("\033]22;\007")
		return
	}
	fmt.Printf("\033]22;>%s\007", shape)
}

func shapeKnownTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ApplySystemCursor configures the real terminal cursors via escape codes.
// This never draws a fake cursor character into the grid.
func (g *Grid) ApplySystemCursor(screen tcell.Screen) {
	switch g.pointerShape {
	case PointerDefault:
		writeOSC22(PointerDefault)
	case PointerText:
		writeOSC22(PointerText)
	default:
		writeOSC22("")
	}

	if g.cursorVisible {
		screen.SetCursorStyle(g.cursorStyle)
		screen.ShowCursor(g.cursorX, g.cursorY)
	} else {
		screen.HideCursor()
	}
}
