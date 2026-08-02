// Package mitext holds GDB MI string helpers shared by the gdb backend and
// debugger widgets without importing the GDB process client.
// Debugger-domain only — not part of the reusable TUI framework.
package mitext

import (
	"strconv"
	"strings"
)

// MI prompt record GDB emits over MI (exact line token after TrimSpace).
const (
	MIPromptToken = "(gdb)"
	// MIPromptLiveHost is MIPromptToken plus one trailing space for the caret.
	MIPromptLiveHost = MIPromptToken + " "
)

// IsMIPromptRecord reports whether line is GDB's MI prompt record.
func IsMIPromptRecord(line string) bool {
	return line == MIPromptToken
}

// IsBareMIPromptHost reports a scrollback line that is only the MI prompt
// token (optional trailing space from a console-stream echo).
func IsBareMIPromptHost(line string) bool {
	return strings.TrimSpace(line) == MIPromptToken
}

// LivePromptHost returns fromGDB with exactly one trailing space for input.
// Empty fromGDB yields empty (do not invent a token).
func LivePromptHost(fromGDB string) string {
	if fromGDB == "" {
		return ""
	}
	return strings.TrimRight(fromGDB, " ") + " "
}

// IsCtrlCQuitLog reports GDB log-stream Ctrl-C feedback ("Quit" / "❌️ Quit").
func IsCtrlCQuitLog(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	return t == "Quit" || strings.HasSuffix(t, " Quit") || strings.Contains(t, "Quit")
}

func isOctalDigit(ch byte) bool {
	return ch >= '0' && ch <= '7'
}

// DecodeMIString unescapes a GDB MI quoted payload (octal + C escapes).
func DecodeMIString(raw string) string {
	var result []byte
	for i := 0; i < len(raw); {
		if raw[i] == '\\' && i+1 < len(raw) {
			if i+3 < len(raw) && isOctalDigit(raw[i+1]) && isOctalDigit(raw[i+2]) && isOctalDigit(raw[i+3]) {
				val, err := strconv.ParseInt(raw[i+1:i+4], 8, 16)
				if err == nil {
					result = append(result, byte(val))
					i += 4
					continue
				}
			}
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
				result = append(result, raw[i+1])
				i += 2
			}
			continue
		}
		result = append(result, raw[i])
		i++
	}
	return string(result)
}
