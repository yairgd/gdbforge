package backend

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

// BreakInsertAt formats a breakpoint insert at file:line.
func BreakInsertAt(kind Kind, file string, line int) string {
	loc := breakLocSpec(kind, file, line, "")
	return breakInsertPrefix(kind) + loc
}

// BreakClearAt formats a breakpoint clear at file:line or address.
func BreakClearAt(kind Kind, file string, line int, addr string) string {
	if addr != "" {
		return "clear *" + models.NormalizeAddr(addr)
	}
	loc := breakLocSpec(kind, file, line, "")
	if kind == DLV {
		return "clear " + loc
	}
	return "clear " + loc
}

// BreakDeleteNum deletes breakpoint by number.
func BreakDeleteNum(kind Kind, number int) string {
	if number < 1 {
		return ""
	}
	if kind == DLV {
		return fmt.Sprintf("clear %d", number)
	}
	return fmt.Sprintf("-break-delete %d", number)
}

// BreakDisableNum disables breakpoint by number (GDB MI / Delve CLI).
func BreakDisableNum(kind Kind, number int) string {
	if number < 1 {
		return ""
	}
	return fmt.Sprintf("disable %d", number)
}

// BreakConditionNum sets a conditional breakpoint.
func BreakConditionNum(kind Kind, number int, cond string) string {
	if number < 1 || strings.TrimSpace(cond) == "" {
		return ""
	}
	return fmt.Sprintf("condition %d %s", number, cond)
}

// BreakInsertAddr inserts an address breakpoint.
func BreakInsertAddr(kind Kind, addr string) string {
	addr = models.NormalizeAddr(addr)
	if addr == "" {
		return ""
	}
	return breakInsertPrefix(kind) + "*" + addr
}

func breakInsertPrefix(kind Kind) string {
	if kind == DLV {
		return "break "
	}
	return "break "
}

func breakLocSpec(kind Kind, file string, line int, addr string) string {
	if file != "" && line > 0 {
		base := filepath.Base(file)
		if strings.ContainsAny(file, " \t\"") {
			return fmt.Sprintf("%s:%d", base, line)
		}
		return fmt.Sprintf("%s:%d", file, line)
	}
	if addr != "" {
		return "*" + models.NormalizeAddr(addr)
	}
	return "?"
}