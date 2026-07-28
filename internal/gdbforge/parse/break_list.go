package parse

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"regexp"
	"strconv"
	"strings"
)

var (
	bkptChunkRe = regexp.MustCompile(`bkpt=\{`)
	lineFieldRe = regexp.MustCompile(`\bline="(\d+)"`)
	numberRe    = regexp.MustCompile(`\bnumber="(\d+)"`)
	fileLineRe  = regexp.MustCompile(`([^:]+):(\d+)\s*$`)
)

// ParseBreakList extracts breakpoint rows from -break-list output.
// Includes pending breakpoints (original-location / pending="file:line").
func ParseBreakList(raw string) []models.BreakInfo {
	var out []models.BreakInfo
	idxs := bkptChunkRe.FindAllStringIndex(raw, -1)
	for i, loc := range idxs {
		start := loc[0]
		end := len(raw)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		chunk := raw[start:end]
		nm := numberRe.FindStringSubmatch(chunk)
		if len(nm) < 2 {
			continue
		}
		num, err := strconv.Atoi(nm[1])
		if err != nil || num < 1 {
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
		if line < 1 || file == "" {
			// Pending / unresolved: pending="hello.c:23" or original-location.
			for _, key := range []string{"pending", "original-location"} {
				if f, ln, ok := parseFileLineLoc(extractQuotedField(chunk, key)); ok {
					if file == "" {
						file = f
					}
					if line < 1 {
						line = ln
					}
					break
				}
			}
		}
		addr := extractQuotedField(chunk, "addr")
		if addr == "<PENDING>" || addr == "0x0" || addr == "0" {
			addr = ""
		}
		if addr != "" {
			addr = NormalizeAddr(addr)
		}
		if (file == "" || line < 1) && addr == "" {
			continue
		}
		out = append(out, models.BreakInfo{
			Number:  num,
			Enabled: !strings.Contains(chunk, `enabled="n"`),
			File:    unescapeMI(file),
			Line:    line,
			Addr:    addr,
		})
	}
	return out
}

func parseFileLineLoc(s string) (file string, line int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, false
	}
	m := fileLineRe.FindStringSubmatch(s)
	if len(m) < 3 {
		return "", 0, false
	}
	ln, err := strconv.Atoi(m[2])
	if err != nil || ln < 1 {
		return "", 0, false
	}
	return m[1], ln, true
}

// EnabledBreakMarks returns file:line for enabled breakpoints only.
func EnabledBreakMarks(items []models.BreakInfo) []models.BreakMark {
	return models.EnabledBreakMarks(items)
}

func extractQuotedField(s, key string) string {
	re := regexp.MustCompile(key + `="((?:\\.|[^"\\])*)"`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func unescapeMI(s string) string {
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}
