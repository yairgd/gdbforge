package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

var asmInsnChunkRe = regexp.MustCompile(`\{address="`)

// maxInsnBytes is a conservative upper bound for one instruction (x86/ARM).
const maxInsnBytes = 15

// DefaultAsmRows is used before the viewport has been painted.
const DefaultAsmRows = 20

// ParseDataDisassemble extracts asm_insns from -data-disassemble MI output.
func ParseDataDisassemble(raw string) []models.AsmLine {
	var out []models.AsmLine
	idxs := asmInsnChunkRe.FindAllStringIndex(raw, -1)
	for i, loc := range idxs {
		start := loc[0]
		end := len(raw)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		chunk := raw[start:end]
		addr := extractQuotedField(chunk, "address")
		if addr == "" {
			continue
		}
		out = append(out, models.AsmLine{
			Addr:    NormalizeAddr(addr),
			Opcodes: extractQuotedField(chunk, "opcodes"),
			Inst:    unescapeMI(extractQuotedField(chunk, "inst")),
			Func:    extractQuotedField(chunk, "func-name"),
			Offset:  extractQuotedField(chunk, "offset"),
		})
	}
	return out
}

// ParseDataEvaluateAddr extracts a hex address from -data-evaluate-expression
// (e.g. $pc → ^done,value="0x401126").
func ParseDataEvaluateAddr(raw string) (string, bool) {
	v := extractQuotedField(raw, "value")
	if v == "" {
		return "", false
	}
	// GDB may return "0x401126" or "(void *) 0x401126".
	if i := strings.LastIndex(v, "0x"); i >= 0 {
		v = v[i:]
		// Trim trailing junk (e.g. ")").
		end := 0
		for end < len(v) {
			c := v[end]
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == 'x' {
				end++
				continue
			}
			break
		}
		v = v[:end]
	}
	if v == "" {
		return "", false
	}
	return NormalizeAddr(v), true
}

// NormalizeAddr parses a hex address and returns a stable "0x%x" form.
func NormalizeAddr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 64)
	if err != nil {
		return s
	}
	return fmt.Sprintf("0x%x", n)
}

// ParseAddrUint parses a hex address string.
func ParseAddrUint(s string) (uint64, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// DisassembleRange returns [start,end) byte addresses large enough for about
// rows instructions centered on centerAddr.
func DisassembleRange(centerAddr string, rows int) (start, end string, ok bool) {
	center, ok := ParseAddrUint(centerAddr)
	if !ok {
		return "", "", false
	}
	if rows < 1 {
		rows = DefaultAsmRows
	}
	half := rows / 2
	before := uint64(half * maxInsnBytes)
	after := uint64((rows - half + 1) * maxInsnBytes)
	// Over-fetch so edge navigation has headroom.
	before *= 2
	after *= 2
	var s uint64
	if center > before {
		s = center - before
	} else {
		s = 0
	}
	e := center + after
	if e < center { // overflow
		e = ^uint64(0)
	}
	return fmt.Sprintf("0x%x", s), fmt.Sprintf("0x%x", e), true
}

// WindowAround picks up to rows instructions centered on the line nearest to
// centerAddr. If centerAddr is missing, centers on the middle of items.
func WindowAround(items []models.AsmLine, centerAddr string, rows int) []models.AsmLine {
	if len(items) == 0 {
		return nil
	}
	if rows < 1 {
		rows = DefaultAsmRows
	}
	if rows >= len(items) {
		return append([]models.AsmLine(nil), items...)
	}
	idx := indexNearestAddr(items, centerAddr)
	if idx < 0 {
		idx = len(items) / 2
	}
	half := rows / 2
	start := idx - half
	if start < 0 {
		start = 0
	}
	end := start + rows
	if end > len(items) {
		end = len(items)
		start = end - rows
		if start < 0 {
			start = 0
		}
	}
	return append([]models.AsmLine(nil), items[start:end]...)
}

func indexNearestAddr(items []models.AsmLine, addr string) int {
	if addr == "" || len(items) == 0 {
		return -1
	}
	want, ok := ParseAddrUint(addr)
	if !ok {
		norm := NormalizeAddr(addr)
		for i, it := range items {
			if NormalizeAddr(it.Addr) == norm {
				return i
			}
		}
		return -1
	}
	best := -1
	var bestDist uint64
	for i, it := range items {
		got, ok := ParseAddrUint(it.Addr)
		if !ok {
			continue
		}
		var d uint64
		if got >= want {
			d = got - want
		} else {
			d = want - got
		}
		if best < 0 || d < bestDist {
			best = i
			bestDist = d
		}
		if d == 0 {
			return i
		}
	}
	return best
}

// IndexOfAddr returns the index of an exact (normalized) address, or -1.
func IndexOfAddr(items []models.AsmLine, addr string) int {
	norm := NormalizeAddr(addr)
	if norm == "" {
		return -1
	}
	for i, it := range items {
		if NormalizeAddr(it.Addr) == norm {
			return i
		}
	}
	return -1
}
