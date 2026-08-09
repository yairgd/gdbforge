package parse

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"regexp"
	"strconv"
)

var frameChunkRe = regexp.MustCompile(`frame=\{`)

// ParseStackListFrames extracts frames from -stack-list-frames MI output.
func ParseStackListFrames(raw string) []models.StackFrame {
	var out []models.StackFrame
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
func ParseStackInfoFrame(raw string) (models.StackFrame, bool) {
	idxs := frameChunkRe.FindAllStringIndex(raw, -1)
	if len(idxs) == 0 {
		return models.StackFrame{}, false
	}
	start := idxs[0][0]
	end := len(raw)
	if len(idxs) > 1 {
		end = idxs[1][0]
	}
	return parseFrameChunk(raw[start:end])
}

func parseFrameChunk(chunk string) (models.StackFrame, bool) {
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
	fr := models.StackFrame{
		Level: level,
		Func:  extractQuotedField(chunk, "func"),
		File:  unescapeMI(file),
		Line:  line,
		Addr:  extractQuotedField(chunk, "addr"),
		From:  unescapeMI(extractQuotedField(chunk, "from")),
	}
	if fr.File == "" && fr.Func == "" && fr.Addr == "" {
		return models.StackFrame{}, false
	}
	return fr, true
}
