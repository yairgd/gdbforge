package core

import (
	tcell "github.com/gdamore/tcell/v2"
	"strings"
	"unicode/utf8"
)

func DrawANSIText(screen tcell.Screen, x, y int, text string, baseStyle tcell.Style, maxWidth int) {
	style := baseStyle
	posX := x

	// Use a manual loop so we can control the index 'i' based on character size
	for i := 0; i < len(text); {

		// --- Check for ANSI ESC sequences ---
		if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '[' {
			j := i + 2
			// Look for the terminating character of the ANSI sequence (usually 'm')
			for j < len(text) && (text[j] < '@' || text[j] > '~') {
				j++
			}

			if j < len(text) {
				finalChar := text[j]

				// Only handle SGR (Select Graphic Rendition) codes ending in 'm'
				if finalChar == 'm' {
					seq := text[i+2 : j]
					codes := strings.Split(seq, ";")

					for _, c := range codes {
						switch c {
						case "0":
							style = baseStyle
						case "1":
							style = style.Bold(true)
						case "22":
							style = style.Bold(false)
						case "31":
							style = style.Foreground(tcell.ColorMaroon)
						case "32":
							style = style.Foreground(tcell.ColorGreen)
							// Add other colors/styles as needed for your Zynq/GDB logs
						}
					}
				}

				// Skip the entire escape sequence
				i = j + 1
				continue
			}
		}

		// --- CRITICAL FIX: Handle multi-byte UTF-8 characters ---
		// DecodeRuneInString returns the full Unicode character (ch)
		// and how many bytes it occupied (size).
		ch, size := utf8.DecodeRuneInString(text[i:])

		if posX < maxWidth {
			// SetContent expects a 'rune'. Passing a full ❌ (U+274C)
			// instead of just 0xE2 fixes the rendering.
			screen.SetContent(posX, y, ch, nil, style)

			// Note: Some emojis take 2 terminal cells (Double-width).
			// If characters overlap, you may need 'runewidth.RuneWidth(ch)'.
			posX++
		}

		// Advance index by the byte size of the character (1 for ASCII, 3-4 for Emojis)
		i += size
	}
}
