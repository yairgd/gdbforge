package parse

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"regexp"
	"strconv"
)

var threadIDChunkRe = regexp.MustCompile(`\{id="`)

// ParseThreadInfo extracts threads from -thread-info MI output.
func ParseThreadInfo(raw string) []models.ThreadInfo {
	currentID := extractQuotedField(raw, "current-thread-id")
	var out []models.ThreadInfo
	idxs := threadIDChunkRe.FindAllStringIndex(raw, -1)
	for i, loc := range idxs {
		start := loc[0]
		end := len(raw)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		chunk := raw[start:end]
		id := extractQuotedField(chunk, "id")
		if id == "" {
			continue
		}
		file := extractQuotedField(chunk, "fullname")
		if file == "" {
			file = extractQuotedField(chunk, "file")
		}
		line := 0
		if lm := lineFieldRe.FindStringSubmatch(chunk); len(lm) >= 2 {
			line, _ = strconv.Atoi(lm[1])
		}
		out = append(out, models.ThreadInfo{
			ID:      id,
			State:   extractQuotedField(chunk, "state"),
			Name:    extractQuotedField(chunk, "name"),
			File:    unescapeMI(file),
			Line:    line,
			Func:    extractQuotedField(chunk, "func"),
			Current: id == currentID || (currentID == "" && i == 0),
		})
	}
	return out
}
