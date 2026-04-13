package gdb

import (
	"strconv"
	"strings"
)

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
