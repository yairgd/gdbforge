package termui

import (
	"strings"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestCompletionBarCyclesAndIgnoresSingle(t *testing.T) {
	ctx := platform.NewAppContext()
	bar := NewCompletionBarWidget(ctx)

	bar.onCompletion(CompletionMsg{Names: []string{"only"}})
	if bar.Active() {
		t.Fatal("single match should not activate wildmenu")
	}

	bar.onCompletion(CompletionMsg{Names: []string{"about", "logger", "gdb"}})
	if !bar.Active() || bar.Selected() != "about" {
		t.Fatalf("selected=%q active=%v", bar.Selected(), bar.Active())
	}
	bar.move(1)
	if bar.Selected() != "logger" {
		t.Fatalf("after next selected=%q", bar.Selected())
	}
	bar.move(-1)
	if bar.Selected() != "about" {
		t.Fatalf("after prev selected=%q", bar.Selected())
	}
	bar.move(-1)
	if bar.Selected() != "gdb" {
		t.Fatalf("wrap prev selected=%q", bar.Selected())
	}
	bar.Clear()
	if bar.Active() {
		t.Fatal("cleared bar still active")
	}
}

func completionBarLine(g *Grid) string {
	var b strings.Builder
	for x := 0; x < g.W; x++ {
		r := g.Cells[x][0].Rune
		if r == 0 {
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " ")
}

func TestCompletionBarRollsLeftRight(t *testing.T) {
	ctx := platform.NewAppContext()
	bar := NewCompletionBarWidget(ctx)
	// Narrow width: only one full name + partial next fits at a time.
	// "aaaa" (4) + space + "bbbb" needs 9; width 6 forces a roll.
	bar.onCompletion(CompletionMsg{Names: []string{"aaaa", "bbbb", "cccc"}})

	g := NewGrid(6, 1)
	c := Canvas{rect: NewRect(0, 0, 6, 1), grid: g}
	bar.Draw(c)
	if got := completionBarLine(g); !strings.HasPrefix(got, "aaaa") {
		t.Fatalf("start draw=%q want prefix aaaa", got)
	}
	if bar.start != 0 {
		t.Fatalf("start=%d want 0", bar.start)
	}

	bar.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if bar.Selected() != "bbbb" {
		t.Fatalf("selected=%q want bbbb", bar.Selected())
	}
	bar.Draw(c)
	if bar.start != 1 {
		t.Fatalf("after Right start=%d want 1 (rolled left)", bar.start)
	}
	if got := completionBarLine(g); !strings.HasPrefix(got, "bbbb") {
		t.Fatalf("after Right draw=%q want prefix bbbb", got)
	}

	bar.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	bar.Draw(c)
	if bar.Selected() != "cccc" || bar.start != 2 {
		t.Fatalf("selected=%q start=%d want cccc/2", bar.Selected(), bar.start)
	}
	if got := completionBarLine(g); !strings.HasPrefix(got, "cccc") {
		t.Fatalf("after Right draw=%q want prefix cccc", got)
	}

	bar.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	bar.Draw(c)
	if bar.Selected() != "bbbb" || bar.start != 1 {
		t.Fatalf("after Left selected=%q start=%d want bbbb/1", bar.Selected(), bar.start)
	}
	if got := completionBarLine(g); !strings.HasPrefix(got, "bbbb") {
		t.Fatalf("after Left draw=%q want prefix bbbb", got)
	}
}
