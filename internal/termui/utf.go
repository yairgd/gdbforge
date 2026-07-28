package termui

import (
	"github.com/yairgd/gdbforge/internal/platform"
	"strconv"
	"strings"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
)

func (c Canvas) ClearLineRange(localY, x1, x2 int, style tcell.Style) {
	if x1 < 0 {
		x1 = 0
	}
	if x2 > c.W() {
		x2 = c.W()
	}

	for x := x1; x < x2; x++ {
		c.SetContent(x, localY, ' ', style)
	}
}

// DrawANSIText draws text interpreting ANSI SGR colors and skipping OSC/CSI
// control sequences. skipCols skips leading visible cells (horizontal scroll).
// If selected is non-nil, it is called with the byte offset of each printable
// rune; a true result draws that cell in reverse.
// If decorate is non-nil, it may adjust the style for each absolute visible
// column (before skipCols), e.g. search-match backgrounds.
// Returns the number of visible cells written.
func (c Canvas) DrawANSIText(localX, localY, skipCols int, text string, baseStyle tcell.Style, selected func(bufByte int) bool, decorate func(absVisCol int, st tcell.Style) tcell.Style) int {
	style := baseStyle
	col := localX
	visible := 0
	i := 0

	for i < len(text) {
		if text[i] == 0x1b {
			next, newStyle, ok := consumeEscape(text, i, style, baseStyle)
			if ok {
				style = newStyle
				i = next
				continue
			}
			// Incomplete escape at end of buffer — wait for more data.
			break
		}

		byteCol := i
		ch, size := utf8.DecodeRuneInString(text[i:])
		if ch == utf8.RuneError && size == 1 {
			i++
			continue
		}
		i += size

		absVis := visible
		if visible < skipCols {
			visible++
			continue
		}
		if col < c.W() {
			st := style
			if decorate != nil {
				st = decorate(absVis, st)
			}
			if selected != nil && selected(byteCol) {
				st = st.Reverse(true)
			}
			c.SetContent(col, localY, ch, st)
			col++
		}
		visible++
	}
	return col - localX
}

// StripANSI removes OSC/CSI/SGR sequences, leaving printable text only.
func StripANSI(text string) string {
	return platform.StripANSI(text)
}

// ANSIByteIndexAtVisible maps a visible cell column to a byte offset in text.
func ANSIByteIndexAtVisible(text string, visCol int) int {
	if visCol <= 0 {
		return 0
	}
	style := tcell.StyleDefault
	visible := 0
	for i := 0; i < len(text); {
		if text[i] == 0x1b {
			next, _, ok := consumeEscape(text, i, style, style)
			if ok {
				i = next
				continue
			}
		}
		if visible >= visCol {
			return i
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
		visible++
	}
	return len(text)
}

// VisibleANSIColAtByte returns the visible cell column for a byte offset in text.
func VisibleANSIColAtByte(text string, byteIdx int) int {
	if byteIdx <= 0 {
		return 0
	}
	if byteIdx > len(text) {
		byteIdx = len(text)
	}
	return VisibleANSIWidth(text[:byteIdx])
}

// ANSIRuneAtVisible returns the printable rune at visible cell visCol, or ' '.
func ANSIRuneAtVisible(text string, visCol int) rune {
	if visCol < 0 {
		return ' '
	}
	style := tcell.StyleDefault
	visible := 0
	for i := 0; i < len(text); {
		if text[i] == 0x1b {
			next, _, ok := consumeEscape(text, i, style, style)
			if ok {
				i = next
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		if visible == visCol {
			if r == utf8.RuneError && size == 1 {
				return ' '
			}
			return r
		}
		i += size
		visible++
	}
	return ' '
}

// VisibleANSIWidth returns how many terminal cells text occupies after stripping
// ANSI/OSC control sequences.
func VisibleANSIWidth(text string) int {
	n := 0
	style := tcell.StyleDefault
	for i := 0; i < len(text); {
		if text[i] == 0x1b {
			next, _, ok := consumeEscape(text, i, style, style)
			if ok {
				i = next
				continue
			}
			// Incomplete escape at end: stop counting.
			break
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
		n++
	}
	return n
}

// HasIncompleteANSI reports whether text ends with an unfinished ESC sequence.
func HasIncompleteANSI(text string) bool {
	style := tcell.StyleDefault
	for i := 0; i < len(text); {
		if text[i] == 0x1b {
			next, _, ok := consumeEscape(text, i, style, style)
			if !ok {
				return true
			}
			i = next
			continue
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}
	return false
}

// consumeEscape handles ESC sequences starting at i. ok is false if this is not
// a recognized escape (caller should treat ESC as a normal character).
func consumeEscape(text string, i int, style, baseStyle tcell.Style) (next int, newStyle tcell.Style, ok bool) {
	if i >= len(text) || text[i] != 0x1b {
		return i, style, false
	}
	if i+1 >= len(text) {
		return i, style, false // incomplete; leave for later
	}

	switch text[i+1] {
	case '[': // CSI
		j := i + 2
		for j < len(text) && !isCSIFinal(text[j]) {
			j++
		}
		if j >= len(text) {
			return i, style, false // incomplete
		}
		final := text[j]
		if final == 'm' {
			style = applySGR(style, baseStyle, text[i+2:j])
		}
		return j + 1, style, true

	case ']': // OSC … BEL or ST (ESC \)
		j := i + 2
		for j < len(text) {
			if text[j] == 0x07 { // BEL
				return j + 1, style, true
			}
			if text[j] == 0x1b && j+1 < len(text) && text[j+1] == '\\' {
				return j + 2, style, true
			}
			j++
		}
		return i, style, false // incomplete

	default:
		// Other ESC+byte sequences (e.g. ESC c): skip both bytes.
		return i + 2, style, true
	}
}

func isCSIFinal(b byte) bool {
	return b >= 0x40 && b <= 0x7e
}

func applySGR(style, baseStyle tcell.Style, seq string) tcell.Style {
	if seq == "" {
		return baseStyle
	}
	parts := strings.Split(seq, ";")
	codes := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			codes = append(codes, 0)
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		codes = append(codes, n)
	}
	if len(codes) == 0 {
		return style
	}

	for i := 0; i < len(codes); i++ {
		switch c := codes[i]; c {
		case 0:
			style = baseStyle
		case 1:
			style = style.Bold(true)
		case 2:
			style = style.Dim(true)
		case 3:
			style = style.Italic(true)
		case 4:
			style = style.Underline(true)
		case 7:
			style = style.Reverse(true)
		case 22:
			style = style.Bold(false).Dim(false)
		case 23:
			style = style.Italic(false)
		case 24:
			style = style.Underline(false)
		case 27:
			style = style.Reverse(false)
		case 30, 31, 32, 33, 34, 35, 36, 37:
			style = style.Foreground(ansiBasicColor(c - 30))
		case 39:
			fg, _, _ := baseStyle.Decompose()
			style = style.Foreground(fg)
		case 40, 41, 42, 43, 44, 45, 46, 47:
			style = style.Background(ansiBasicColor(c - 40))
		case 49:
			_, bg, _ := baseStyle.Decompose()
			style = style.Background(bg)
		case 90, 91, 92, 93, 94, 95, 96, 97:
			style = style.Foreground(ansiBrightColor(c - 90))
		case 100, 101, 102, 103, 104, 105, 106, 107:
			style = style.Background(ansiBrightColor(c - 100))
		case 38:
			if col, n := parseExtendedColor(codes[i+1:]); n > 0 {
				style = style.Foreground(col)
				i += n
			}
		case 48:
			if col, n := parseExtendedColor(codes[i+1:]); n > 0 {
				style = style.Background(col)
				i += n
			}
		}
	}
	return style
}

func parseExtendedColor(codes []int) (tcell.Color, int) {
	if len(codes) < 1 {
		return tcell.ColorDefault, 0
	}
	switch codes[0] {
	case 5: // 256-color palette index
		if len(codes) < 2 {
			return tcell.ColorDefault, 0
		}
		idx := codes[1]
		if idx < 0 {
			idx = 0
		}
		if idx > 255 {
			idx = 255
		}
		return tcell.PaletteColor(idx), 2
	case 2: // truecolor
		if len(codes) < 4 {
			return tcell.ColorDefault, 0
		}
		return tcell.NewRGBColor(int32(codes[1]), int32(codes[2]), int32(codes[3])), 4
	default:
		return tcell.ColorDefault, 0
	}
}

func ansiBasicColor(n int) tcell.Color {
	switch n {
	case 0:
		return tcell.ColorBlack
	case 1:
		return tcell.ColorMaroon
	case 2:
		return tcell.ColorGreen
	case 3:
		return tcell.ColorOlive
	case 4:
		return tcell.ColorNavy
	case 5:
		return tcell.ColorPurple
	case 6:
		return tcell.ColorTeal
	case 7:
		return tcell.ColorSilver
	default:
		return tcell.ColorDefault
	}
}

func ansiBrightColor(n int) tcell.Color {
	switch n {
	case 0:
		return tcell.ColorGray
	case 1:
		return tcell.ColorRed
	case 2:
		return tcell.ColorLime
	case 3:
		return tcell.ColorYellow
	case 4:
		return tcell.ColorBlue
	case 5:
		return tcell.ColorFuchsia
	case 6:
		return tcell.ColorAqua
	case 7:
		return tcell.ColorWhite
	default:
		return tcell.ColorDefault
	}
}
