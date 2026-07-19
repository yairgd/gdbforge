package platform

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
)

// ColorANSI256 returns the nearest xterm-256 palette index for c.
func ColorANSI256(c tcell.Color) int {
	if c == tcell.ColorDefault {
		return 0
	}
	r, g, b := c.RGB()
	return rgbToXterm256(int(r), int(g), int(b))
}

// ContrastANSI256 returns black (16) or white (231) for text on background c.
func ContrastANSI256(c tcell.Color) int {
	r, g, b := c.RGB()
	if (int(r)+int(g)+int(b))/3 > 140 {
		return 16
	}
	return 231
}

// ContrastColor returns black or white for text on background c.
func ContrastColor(c tcell.Color) tcell.Color {
	if ContrastANSI256(c) == 16 {
		return tcell.ColorBlack
	}
	return tcell.ColorWhite
}

// BreakNumberANSI styles a gutter line number with background c (bold + contrast fg).
func BreakNumberANSI(num string, bg tcell.Color) string {
	return fmt.Sprintf("\x1b[48;5;%d;38;5;%d;1m%s\x1b[0m",
		ColorANSI256(bg), ContrastANSI256(bg), num)
}

func rgbToXterm256(r, g, b int) int {
	// Greyscale ramp 232–255 when near grey.
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 248 {
			return 231
		}
		return 232 + (r-8)/10
	}
	return 16 + 36*(r*5/255) + 6*(g*5/255) + (b * 5 / 255)
}
