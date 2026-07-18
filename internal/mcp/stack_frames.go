package mcp

import (
	"regexp"
	"strconv"
)

var frameChunkRe = regexp.MustCompile(`frame=\{`)

// StackFrame is one row from -stack-list-frames.
type StackFrame struct {
	Level int
	Func  string
	File  string
	Line  int
	Addr  string
}

// ParseStackListFrames extracts frames from -stack-list-frames MI output.
func ParseStackListFrames(raw string) []StackFrame {
	var out []StackFrame
	idxs := frameChunkRe.FindAllStringIndex(raw, -1)
	for i, loc := range idxs {
		start := loc[0]
		end := len(raw)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		chunk := raw[start:end]
		level := 0
		if lv := extractQuotedField(chunk, "level"); lv != "" {
			if n, err := strconv.Atoi(lv); err == nil {
				level = n
			}
		}
		file := extractQuotedField(chunk, "fullname")
		if file == "" {
			file = extractQuotedField(chunk, "file")
		}
		line := 0
		if lm := lineFieldRe.FindStringSubmatch(chunk); len(lm) >= 2 {
			line, _ = strconv.Atoi(lm[1])
		}
		out = append(out, StackFrame{
			Level: level,
			Func:  extractQuotedField(chunk, "func"),
			File:  unescapeMI(file),
			Line:  line,
			Addr:  extractQuotedField(chunk, "addr"),
		})
	}
	return out
}
