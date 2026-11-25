package gdb

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/mitext"
	"strconv"
	"strings"
)

type GdbState int

const (
	Done GdbState = iota
	Error
	Running
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

func DecodeMIString(raw string) string {
	return mitext.DecodeMIString(raw)
}
