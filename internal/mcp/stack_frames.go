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
		if fr, ok := parseFrameChunk(chunk); ok {
			out = append(out, fr)
		}
	}
	return out
}

// ParseStackInfoFrame extracts the current frame from -stack-info-frame output.
func ParseStackInfoFrame(raw string) (StackFrame, bool) {
	idxs := frameChunkRe.FindAllStringIndex(raw, -1)
	if len(idxs) == 0 {
		return StackFrame{}, false
	}
	start := idxs[0][0]
	end := len(raw)
	if len(idxs) > 1 {
		end = idxs[1][0]
	}
	return parseFrameChunk(raw[start:end])
}

func parseFrameChunk(chunk string) (StackFrame, bool) {
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
	fr := StackFrame{
		Level: level,
		Func:  extractQuotedField(chunk, "func"),
		File:  unescapeMI(file),
		Line:  line,
		Addr:  extractQuotedField(chunk, "addr"),
	}
	if fr.File == "" && fr.Func == "" && fr.Addr == "" {
		return StackFrame{}, false
	}
	return fr, true
}
