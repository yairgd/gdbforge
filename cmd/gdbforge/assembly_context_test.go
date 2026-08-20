package main

import (
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

func TestFormatAsmFrameContextOnlyRelevantFrame(t *testing.T) {
	lines := formatAsmFrameContext(models.StackFrame{
		Level: 2,
		Addr:  "0x7ffff7ec56ea",
		Func:  "write",
		From:  "/usr/lib64/libc.so.6",
	})
	if len(lines) != 1 {
		t.Fatalf("want 1 line (relevant frame only), got %v", lines)
	}
	if !strings.Contains(lines[0], "#2  ") || !strings.Contains(lines[0], "in write () from ") {
		t.Fatalf("frame=%q", lines[0])
	}
	if strings.Contains(lines[0], "main") || strings.Contains(lines[0], "#0") {
		t.Fatalf("must not include other frames: %q", lines[0])
	}
}

func TestFormatAsmFrameContextUnknownEmpty(t *testing.T) {
	if lines := formatAsmFrameContext(models.StackFrame{
		Level: 0, Addr: "0x7ffff7e5d03a", Func: "", From: "/usr/lib64/libc.so.6",
	}); lines != nil {
		t.Fatalf("?? frame should have no preamble, got %v", lines)
	}
}

func TestFormatAsmFrameContextWithSource(t *testing.T) {
	lines := formatAsmFrameContext(models.StackFrame{
		Level: 8, Addr: "0x5555555551bc", Func: "main", File: "/tmp/hello.c", Line: 12,
	})
	if len(lines) < 1 || !strings.Contains(lines[0], "in main () at hello.c:12") {
		t.Fatalf("main frame=%v", lines)
	}
}
