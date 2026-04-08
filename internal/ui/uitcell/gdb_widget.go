package uitcell

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
	"github.com/yairgd/promptcore/internal/gdb"
	"github.com/yairgd/promptcore/internal/termui"
	"log"
	"strconv"
	"strings"
	"unicode/utf8"
)

//////////////////////////
// GDB WIDGET
//////////////////////////

type GDBWidget struct {
	termui.BaseWidget
	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf string
	Cursor   int

	Debugger core.Debugger
}

func NewGDBWidget(uiContext termui.UIContext) *GDBWidget {
	buf := core.NewBuffer()
	client, outputChan, err := gdb.NewGDBClient()
	if err != nil {
		log.Fatal(err)
	}
	//	defer client.Close()

	widget := &GDBWidget{
		BaseWidget: termui.NewBaseWidget(uiContext.Emit),
		Buffer:     buf,
		Viewport:   core.Viewport{Height: 10},
		Debugger:   client,
		Cursor:     0,
	}
	widget.StartGdbUIBridge(uiContext.Screen(), widget, outputChan)
	return widget
}

func (m *GDBWidget) StartGdbUIBridge(
	screen tcell.Screen,
	widget *GDBWidget,
	outputChan <-chan core.GdbOutputMsg,
) {
	go func() {
		for msg := range outputChan {
			widget.OnGDBOutput(msg.Data)
			screen.PostEvent(tcell.NewEventInterrupt(msg))
			widget.Emit(msg)
		}

		screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	}()
}

// ////////////////////////
// PUBLIC API
// ////////////////////////

func extractMIField(line, key string) string {
	// מחפש pattern: key="value"

	prefix := key + "=\""
	start := strings.Index(line, prefix)
	if start == -1 {
		return ""
	}

	start += len(prefix)

	// מצא סוף string (quote שלא escaped)
	var value strings.Builder
	escaped := false

	for i := start; i < len(line); i++ {
		c := line[i]

		if escaped {
			value.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		if c == '"' {
			break
		}

		value.WriteByte(c)
	}

	// unescape כמו MI
	unescaped, err := strconv.Unquote(`"` + value.String() + `"`)
	if err != nil {
		return value.String()
	}

	return unescaped
}
func expandTabs(s string, tabSize int) string {
	var result strings.Builder
	col := 0

	for _, r := range s {
		if r == '\t' {
			spaces := tabSize - (col % tabSize)
			result.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			result.WriteRune(r)
			col++
		}
	}

	return result.String()
}

func decodeMIString(raw string) string {
	// Pre-allocate a byte slice to store the decoded result
	var result []byte

	for i := 0; i < len(raw); {
		// Look for the start of an escape sequence
		if raw[i] == '\\' && i+1 < len(raw) {

			// 1. Handle Octal Escapes (e.g., \342\235\214)
			// Octal escapes in GDB MI always follow the \NNN format (3 digits)
			if i+3 < len(raw) && isOctalDigit(raw[i+1]) && isOctalDigit(raw[i+2]) && isOctalDigit(raw[i+3]) {
				// Parse the 3 digits following the backslash as base 8
				val, err := strconv.ParseInt(raw[i+1:i+4], 8, 16)
				if err == nil {
					result = append(result, byte(val))
					i += 4 // Move past \NNN
					continue
				}
			}

			// 2. Handle Standard C-style Escapes
			// GDB often escapes quotes and backslashes in its output strings
			switch raw[i+1] {
			case '\\':
				result = append(result, '\\')
				i += 2
			case '"':
				result = append(result, '"')
				i += 2
			case 'n':
				result = append(result, '\n')
				i += 2
			case 't':
				result = append(result, '\t')
				i += 2
			case 'r':
				result = append(result, '\r')
				i += 2
			default:
				// If it's an unknown escape, just append the character after the backslash
				result = append(result, raw[i+1])
				i += 2
			}
			continue
		}

		// 3. Handle Regular Characters
		// If no backslash is found, treat it as a literal byte
		result = append(result, raw[i])
		i++
	}

	return string(result)
}

// Helper function to validate octal digits (0-7)
func isOctalDigit(ch byte) bool {
	return ch >= '0' && ch <= '7'
}

func decodeMIString11(raw string) string {
	raw = normalizeMI(raw)

	s, err := strconv.Unquote(`"` + raw + `"`)
	if err == nil {
		return s
	}

	return raw
}
func decodeMIString22(raw string) string {
	var result []byte

	for i := 0; i < len(raw); {
		if raw[i] == '\\' && i+3 < len(raw) {
			// octal \NNN
			val, err := strconv.ParseInt(raw[i+1:i+4], 8, 8)
			if err == nil {
				result = append(result, byte(val))
				i += 4
				continue
			}
		}

		// רגיל
		result = append(result, raw[i])
		i++
	}

	return string(result)
}

func extractQuoted(line string) string {
	start := strings.Index(line, "\"")
	end := strings.LastIndex(line, "\"")

	if start == -1 || end <= start {
		return ""
	}

	return line[start+1 : end]
}

func decodeMIString1(raw string) string {
	var result []byte
	raw = normalizeMI(raw)

	for i := 0; i < len(raw); {
		if raw[i] == '\\' {
			// --- octal \NNN ---
			if i+3 < len(raw) &&
				raw[i+1] >= '0' && raw[i+1] <= '7' &&
				raw[i+2] >= '0' && raw[i+2] <= '7' &&
				raw[i+3] >= '0' && raw[i+3] <= '7' {

				val, err := strconv.ParseInt(raw[i+1:i+4], 8, 8)
				if err == nil {
					result = append(result, byte(val))
					i += 4
					continue
				}
			}

			// --- standard escapes ---
			if i+1 < len(raw) {
				switch raw[i+1] {
				case 'n':
					result = append(result, '\n')
				case 't':
					result = append(result, '\t')
				case '\\':
					result = append(result, '\\')
				case '"':
					result = append(result, '"')
				default:
					// fallback
					result = append(result, raw[i+1])
				}
				i += 2
				continue
			}
		}

		result = append(result, raw[i])
		i++
	}

	return string(result)
}

func normalizeMI(raw string) string {
	// \\342 → \342
	return strings.ReplaceAll(raw, "\\\\", "\\")
}

func isOctal(c byte) bool {
	return c >= '0' && c <= '7'
}
func (m *GDBWidget) handleAsyncRecord(line string) {
	// דוגמאות:
	// *stopped,reason="breakpoint-hit",frame={...}
	// =breakpoint-created,...

	if strings.HasPrefix(line, "*stopped") {
		reason := extractMIField(line, "reason")

		switch reason {
		//		case "breakpoint-hit":
		//			m.Buffer.AppendText("\n🛑 Breakpoint hit\n")

		case "end-stepping-range":
			// step
		case "exited-normally":
			//			m.Buffer.AppendText("\nProgram exited\n")
		}
	}

	// אפשר להרחיב:
	// =breakpoint-created → לעדכן state
}
func (m *GDBWidget) handlePrompt() {
	// אופציונלי — תלוי UI

	// אם אתה רוצה להציג:
	// m.Buffer.AppendText("(gdb) ")

	// או להתעלם לגמרי (רוב הכלים לא מציגים)
}
func (m *GDBWidget) handleResultRecord(line string) {
	// דוגמאות:
	// ^done
	// ^error,msg="..."
	// ^running

	if strings.HasPrefix(line, "^error") {
		msg := extractMIField(line, "msg")
		if msg != "" {
			m.Buffer.AppendText("ERROR: " + msg + "\n")
		} else {
			m.Buffer.AppendText("ERROR\n")
		}
		return
	}

	// לרוב ^done לא צריך להציג
}

func (m *GDBWidget) OnGDBOutput(data string) {
	lines := strings.Split(data, "\n")

	var consoleBuf strings.Builder
	var targetBuf strings.Builder
	//	var logBuf strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		// --- Console stream (~"...") ---
		case strings.HasPrefix(line, "~\"") && strings.HasSuffix(line, "\""):
			text := decodeMIString(line[2 : len(line)-1])
			text = expandTabs(text, 8)
			consoleBuf.WriteString(text)

		// --- Target output (@"...") ---
		case strings.HasPrefix(line, "@\"") && strings.HasSuffix(line, "\""):
			text := decodeMIString(line[2 : len(line)-1])
			text = expandTabs(text, 8)
			targetBuf.WriteString(text)

		// --- Log stream (&"...") ---
		case strings.HasPrefix(line, "&\"") && strings.HasSuffix(line, "\""):
			text := decodeMIString(line[2 : len(line)-1])
			//	bytes := []byte{0xe2, 0x9d, 0x8c, 0xef, 0xb8, 0x8f, 0x20, 0x51, 0x75, 0x69, 0x74, 0x0a}

			// Converting the byte slice directly to a string
			//	result := string(bytes)

			//	fmt.Print(result)

			//	raw := extractQuoted(line)
			m.Buffer.AppendText(text)
		//	fmt.Printf("RAW LINE: %q\n", line)

		// --- Result record (^done, ^error...) ---
		case strings.HasPrefix(line, "^"):
			m.handleResultRecord(line)

		// --- Async record (*stopped, =breakpoint...) ---
		case strings.HasPrefix(line, "*") || strings.HasPrefix(line, "="):
			m.handleAsyncRecord(line)

		// --- Prompt ---
		case line == "(gdb)":
			m.handlePrompt()

		default:
			// fallback (sometimes garbage / partial lines)
			consoleBuf.WriteString(line + "\n")
		}
	}

	// --- Update UI (only what you want visible) ---
	if consoleBuf.Len() > 0 {
		m.Buffer.AppendText(consoleBuf.String())
	}

	if targetBuf.Len() > 0 {
		// optional: separate pane later
		m.Buffer.AppendText(targetBuf.String())
	}

	// logs usually hidden (debug only)
	// if logBuf.Len() > 0 { ... }

	m.Viewport.FollowBottom(m.Buffer)
}

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

// ////////////////////////
// EVENTS
// ////////////////////////
func (m *GDBWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		//switch data := e.Data().(type) {
		//case core.GdbOutputMsg:
		//	return
		//}
	case *tcell.EventResize:
		w, h := e.Size()

		m.SetSize(w, h)

		m.Viewport.Height = h - 1

	case *tcell.EventKey:

		switch e.Key() {

		case tcell.KeyCtrlC:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x03") // SIGINT
			}
			return
		case tcell.KeyCtrlD:
			if m.Debugger.Send != nil {
				m.Debugger.Send("q\n") // SIGINT
			}
		//	return

		case tcell.KeyEnter:
			if m.Debugger.Send != nil {
				m.Debugger.Send(m.InputBuf)
			}

			m.Buffer.AppendText("(gdb) " + m.InputBuf + "\n")
			m.Viewport.FollowBottom(m.Buffer)

			m.InputBuf = ""
			m.Cursor = 0

		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if m.Cursor > 0 {
				m.InputBuf = m.InputBuf[:m.Cursor-0-1] + m.InputBuf[m.Cursor-0:]
				m.Cursor--
			}

		case tcell.KeyLeft:
			if m.Cursor > 0 {
				m.Cursor--
			}

		case tcell.KeyRight:
			if m.Cursor < len(m.InputBuf) {
				m.Cursor++
			}

		case tcell.KeyUp:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x1b[A")
			}

		case tcell.KeyDown:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x1b[B")
			}

		case tcell.KeyRune:
			r := string(e.Rune())
			m.InputBuf =
				m.InputBuf[:m.Cursor-0] +
					r +
					m.InputBuf[m.Cursor-0:]
			m.Cursor += len(r)
		}
	}
}

// ////////////////////////
// DRAW
// ////////////////////////
func (m *GDBWidget) Draw(screen tcell.Screen) {
	screen.Clear()
	w, h := m.Size()

	// --- OUTPUT ---
	lines := m.Viewport.VisibleLines(m.Buffer)
	for y, line := range lines {
		// IMPORTANT: Reset to default style at the start of every line
		lineStyle := tcell.StyleDefault

		if strings.HasPrefix(line, ">>>") {
			// Apply special hardware status style
			lineStyle = lineStyle.Foreground(tcell.ColorTeal).Bold(true)
		} else if strings.HasPrefix(line, "(gdb)") {
			// Optional: Make the prompt stand out in a different color
			lineStyle = lineStyle.Foreground(tcell.ColorYellow)
		}

		// We call DrawANSIText ONCE per line.
		// No need for the 'for x := range line' loop here.
		DrawANSIText(screen, 0, y, line, lineStyle, w)
	}

	// --- INPUT LINE ---
	inputY := h - 2
	prompt := "(gdb) "
	promptLen := len(prompt)

	// Draw the static prompt
	DrawANSIText(screen, 0, inputY, prompt, tcell.StyleDefault.Foreground(tcell.ColorYellow), w)

	// Draw the user's current input
	for x, ch := range m.InputBuf {
		if x+promptLen >= w {
			break
		}
		screen.SetContent(x+promptLen, inputY, ch, nil, tcell.StyleDefault)
	}

	// --- CURSOR ---
	// Match the cursor position to the prompt offset
	screen.ShowCursor(m.Cursor+promptLen, inputY)

	screen.Show()
}
func (w *GDBWidget) HandleEvent1(ev tcell.Event) {
	w.App().RequestRedraw()

	w.Emit(core.GdbOutputMsg{Data: "test"})

}
