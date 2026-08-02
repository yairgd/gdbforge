package termui

import (
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestViewScrollColClampsAndDraw(t *testing.T) {
	buf := platform.NewBuffer()
	wide := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 2) // 52 runes
	buf.AppendLine(wide)
	v := NewViewport(buf)
	v.width = 10
	v.height = 3

	if got := v.maxLeft(); got != 42 {
		t.Fatalf("maxLeft=%d want 42", got)
	}

	v.ViewScrollColLeft()
	if v.Left != 0 {
		t.Fatalf("Left=%d want 0 at start", v.Left)
	}

	v.ViewScrollColRight()
	if v.Left != 1 {
		t.Fatalf("Left=%d want 1", v.Left)
	}

	for i := 0; i < 100; i++ {
		v.ViewScrollColRight()
	}
	if v.Left != 42 {
		t.Fatalf("Left=%d want clamp 42", v.Left)
	}

	v.ViewScrollColLeft()
	if v.Left != 41 {
		t.Fatalf("Left=%d want 41", v.Left)
	}

	g := NewGrid(10, 3)
	c := Canvas{rect: NewRect(0, 0, 10, 3), grid: g}
	v.Draw(c)
	// After Left=41, first visible rune of "abc...xyzabc..." is wide[41] = 'p' (0-based: a=0 ... z=25, a=26 ... p=41)
	if g.Cells[0][0].Rune != []rune(wide)[41] {
		t.Fatalf("cell0=%q want %q", string(g.Cells[0][0].Rune), string([]rune(wide)[41]))
	}
}

func TestViewScrollColANSIUsesVisibleWidth(t *testing.T) {
	buf := platform.NewBuffer()
	line := "\x1b[31m" + strings.Repeat("x", 30) + "\x1b[0m"
	buf.AppendLine(line)
	v := NewViewport(buf)
	v.ANSI = true
	v.width = 10
	v.height = 2

	if got := v.maxLeft(); got != 20 {
		t.Fatalf("maxLeft=%d want 20 (visible 30 - width 10)", got)
	}
	v.ViewScrollColRight()
	if v.Left != 1 {
		t.Fatalf("Left=%d", v.Left)
	}
}
