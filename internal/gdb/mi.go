package gdb

import (
	"strconv"
	"strings"
)

type GdbState int

const (
	Done GdbState = iota
	Error
	Running
)

var state GdbState = Done

func ExtractMIField(line, key string) string {
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
func ExpandTabs(s string, tabSize int) string {
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

// Helper function to validate octal digits (0-7)
func isOctalDigit(ch byte) bool {
	return ch >= '0' && ch <= '7'
}
func DecodeMIString(raw string) string {
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

var consoleBuf strings.Builder

func OnGDBOutput1111(line string, lastCmd string) strings.Builder {
	//	lines := strings.Split(data, "\n")

	var targetBuf strings.Builder
	//	var logBuf strings.Builder

	//	lines := strings.Split(data, "\n")
	//	for _, line := range lines {
	//	line = strings.TrimSpace(line)
	//	if line == "" {
	//		continue
	//	}

	switch {
	// --- Console stream (~"...") ---
	case strings.HasPrefix(line, "~\"") && strings.HasSuffix(line, "\""):
		text := DecodeMIString(line[2 : len(line)-1])
		text = ExpandTabs(text, 8)
		consoleBuf.Reset()
		if state == Done {
			consoleBuf.WriteString(text)
		} else if state == Running {
			consoleBuf.WriteString(text)
		}
	//	return consoleBuf
	//	if text != lastCommand {
	//		consoleBuf.WriteString(text)
	//	}

	// --- Target output (@"...") ---
	case strings.HasPrefix(line, "@\"") && strings.HasSuffix(line, "\""):
		text := DecodeMIString(line[2 : len(line)-1])
		text = ExpandTabs(text, 8)
		targetBuf.WriteString(text)

	// --- Log stream (&"...") ---
	case strings.HasPrefix(line, "&\"") && strings.HasSuffix(line, "\""):
		//text := DecodeMIString(line[2 : len(line)-1])
		consoleBuf.WriteString("\n")
		//msg := ExtractMIField(text, "msg")
		//if text != "\n" && text != "" && lastCmd != text {
		//	if state == Error {
		//		consoleBuf.WriteString("\n" + text)
		//	} else {
		//		consoleBuf.WriteString(text)
		//	}
		//}
		//m.Buffer.AppendText(text)

	// --- Result record (^done, ^error...) ---
	case strings.HasPrefix(line, "^"):
		if strings.HasPrefix(line, "^error") {
			state = Error
			//	msg := ExtractMIField(line, "msg")
			consoleBuf.Reset()
			//	consoleBuf.WriteString("\n" + msg + "\n")
			//	consoleBuf.WriteString("\n" + msg + "\n")
		} else if strings.HasPrefix(line, "^running") {
			consoleBuf.Reset()
			//consoleBuf.WriteString("\n")
			state = Running
		} else if strings.HasPrefix(line, "^done") {
			state = Done
		}

	//	m.handleResultRecord(line)

	// --- Async record (*stopped, =breakpoint...) ---
	case strings.HasPrefix(line, "*") || strings.HasPrefix(line, "="):
		consoleBuf.Reset()
		//print(line + "\n")

	// --- Prompt ---
	case line == "(gdb)" && state != Running:
	//	consoleBuf.Reset()
	//	consoleBuf.WriteString("(gdb) ")

	//	m.handlePrompt()

	default:
		consoleBuf.Reset()
		// fallback (sometimes garbage / partial lines)
		//	consoleBuf.WriteString(line + "\n")
	}
	//	}

	// --- Update UI (only what you want visible) ---
	//	if consoleBuf.Len() > 0 {
	//		m.Buffer.AppendText(consoleBuf.String())
	//	}

	if targetBuf.Len() > 0 {
		// optional: separate pane later
		//		m.Buffer.AppendText(targetBuf.String())
	}

	return consoleBuf
	// logs usually hidden (debug only)
	// if logBuf.Len() > 0 { ... }

	// m.Viewport.FollowBottom(m.Buffer)
}
