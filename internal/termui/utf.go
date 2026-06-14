package termui

import (
	"strings"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
)

func (c Canvas) DrawANSIText(localX, localY int, text string, baseStyle tcell.Style) {
	style := baseStyle
	col := localX

	for i := 0; i < len(text); {

		if false {
			if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '[' {
				j := i + 2
				for j < len(text) && (text[j] < '@' || text[j] > '~') {
					j++
				}

				if j < len(text) {
					finalChar := text[j]

					if finalChar == 'm' {
						seq := text[i+2 : j]
						codes := strings.Split(seq, ";")

						for _, code := range codes {
							switch code {
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
							}
						}
					}

					i = j + 1
					continue
				}
			}
		}

		ch, size := utf8.DecodeRuneInString(text[i:])

		if col < c.W() {
			c.SetContent(col, localY, ch, style)
			col++
		}

		i += size
	}
}
