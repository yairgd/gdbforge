package dlv

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
	"regexp"
	"strconv"
	"strings"
)

var (
	// User breakpoints from `breakpoints` / set notify:
	//   Breakpoint 1 (enabled) at 0x… for main.main() ./hello.go:24 (0)
	//   Breakpoint 1 set at 0x… for main.main() ./hello.go:24
	//   Breakpoint 1 at 0x… for main.main() ./hello.go:5 (1)
	// Named runtime BPs (runtime-fatal-throw, unrecovered-panic) are skipped
	// because the id is not numeric-only after "Breakpoint ".
	bpUserRe = regexp.MustCompile(
		`(?i)^Breakpoint\s+(\d+)\b(?:\s+\((?:enabled|disabled)\))?(?:\s+set)?\s+at\s+\S+\s+for\s+(\S+)\(\)\s+(\S+):(\d+)`,
	)
	stackFrameRe = regexp.MustCompile(`^\s*(\d+)\s+0x[0-9a-fA-F]+\s+in\s+(\S+)`)
	stackAtRe    = regexp.MustCompile(`^\s+at\s+(\S+):(\d+)`)
	goroutineRe  = regexp.MustCompile(`(?i)^(\*?)\s*Goroutine\s+(\d+)\s+-\s+(?:User:\s+)?(\S+):(\d+)\s+(\S+)`)
)

func plainLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		out = append(out, strings.TrimRight(platform.StripANSI(line), "\r"))
	}
	return out
}

// ParseBreakpoints extracts user breakpoint rows from Delve `breakpoints` output.
func ParseBreakpoints(raw string) []models.BreakInfo {
	var out []models.BreakInfo
	for _, line := range plainLines(raw) {
		line = strings.TrimSpace(line)
		if line == "" || line == PromptToken {
			continue
		}
		m := bpUserRe.FindStringSubmatch(line)
		if len(m) != 5 {
			continue
		}
		fn := m[2]
		file := m[3]
		if fn == "" || strings.Contains(fn, "multiple") ||
			file == "" || strings.Contains(file, "<") {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		ln, _ := strconv.Atoi(m[4])
		if num < 1 || ln < 1 {
			continue
		}
		enabled := !strings.Contains(strings.ToLower(line), "(disabled)")
		out = append(out, models.BreakInfo{
			Number:  num,
			Enabled: enabled,
			File:    ResolveSourcePath(file),
			Line:    ln,
		})
	}
	return out
}

// ParseStack extracts frames from `stack` / `bt` CLI output.
func ParseStack(raw string) []models.StackFrame {
	var out []models.StackFrame
	lines := plainLines(raw)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := stackFrameRe.FindStringSubmatch(line); len(m) == 3 {
			level, _ := strconv.Atoi(m[1])
			fr := models.StackFrame{Level: level, Func: m[2]}
			if i+1 < len(lines) {
				if am := stackAtRe.FindStringSubmatch(lines[i+1]); len(am) == 3 {
					fr.File = ResolveSourcePath(am[1])
					fr.Line, _ = strconv.Atoi(am[2])
					i++
				}
			}
			out = append(out, fr)
		}
	}
	return out
}

// ParseGoroutines extracts goroutine rows from `goroutines` CLI output.
func ParseGoroutines(raw string) []models.ThreadInfo {
	var out []models.ThreadInfo
	for _, line := range plainLines(raw) {
		line = strings.TrimSpace(line)
		if line == "" || line == PromptToken {
			continue
		}
		m := goroutineRe.FindStringSubmatch(line)
		if len(m) < 6 {
			continue
		}
		ln, _ := strconv.Atoi(m[4])
		out = append(out, models.ThreadInfo{
			ID:      m[2],
			State:   "stopped",
			Name:    m[5],
			File:    ResolveSourcePath(m[3]),
			Line:    ln,
			Func:    m[5],
			Current: m[1] == "*" || strings.HasPrefix(line, "*"),
		})
	}
	if len(out) == 0 {
		out = parseGoroutinesLoose(raw)
	}
	return out
}

func parseGoroutinesLoose(raw string) []models.ThreadInfo {
	var out []models.ThreadInfo
	re := regexp.MustCompile(`(?i)(\*?)\s*Goroutine\s+(\d+)\b`)
	for _, line := range plainLines(raw) {
		trim := strings.TrimSpace(line)
		m := re.FindStringSubmatch(trim)
		if len(m) < 3 {
			continue
		}
		th := models.ThreadInfo{
			ID:      m[2],
			State:   "stopped",
			Current: m[1] == "*" || strings.HasPrefix(trim, "*"),
		}
		if fm := regexp.MustCompile(`(\S+\.go):(\d+)`).FindStringSubmatch(trim); len(fm) == 3 {
			th.File = ResolveSourcePath(fm[1])
			th.Line, _ = strconv.Atoi(fm[2])
		}
		out = append(out, th)
	}
	return out
}

// ParseStackInfoFrame returns the innermost (level 0) frame from stack output.
// Prefer frameAtLevel / call-stack selection when syncing after `frame N`.
func ParseStackInfoFrame(raw string) (models.StackFrame, bool) {
	frames := ParseStack(raw)
	if len(frames) == 0 {
		return models.StackFrame{}, false
	}
	return frames[0], true
}
