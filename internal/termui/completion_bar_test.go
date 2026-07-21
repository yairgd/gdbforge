package termui

import (
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestCompletionBarSetItemsAndClear(t *testing.T) {
	ctx := platform.NewAppContext()
	bar := NewCompletionBarWidget(ctx)

	bar.SetItems([]string{"about", "logger", "gdb"}, 0)
	if !bar.Active() {
		t.Fatal("expected active after SetItems")
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

func TestCompletionBarRollsWithSelection(t *testing.T) {
	ctx := platform.NewAppContext()
	bar := NewCompletionBarWidget(ctx)
	names := []string{"aaaa", "bbbb", "cccc"}
	bar.SetItems(names, 0)

	g := NewGrid(6, 1)
	c := Canvas{rect: NewRect(0, 0, 6, 1), grid: g}
	bar.Draw(c)
	if got := completionBarLine(g); !strings.HasPrefix(got, "aaaa") {
		t.Fatalf("start draw=%q want prefix aaaa", got)
	}
	if bar.start != 0 {
		t.Fatalf("start=%d want 0", bar.start)
	}

	bar.SetItems(names, 1)
	bar.Draw(c)
	if bar.start != 1 {
		t.Fatalf("after select 1 start=%d want 1 (rolled left)", bar.start)
	}
	if got := completionBarLine(g); !strings.HasPrefix(got, "bbbb") {
		t.Fatalf("after select 1 draw=%q want prefix bbbb", got)
	}

	bar.SetItems(names, 2)
	bar.Draw(c)
	if bar.selected != 2 || bar.start != 2 {
		t.Fatalf("selected=%d start=%d want 2/2", bar.selected, bar.start)
	}
	if got := completionBarLine(g); !strings.HasPrefix(got, "cccc") {
		t.Fatalf("after select 2 draw=%q want prefix cccc", got)
	}

	bar.SetItems(names, 1)
	bar.Draw(c)
	if bar.selected != 1 || bar.start != 1 {
		t.Fatalf("after select 1 again selected=%d start=%d want 1/1", bar.selected, bar.start)
	}
	if got := completionBarLine(g); !strings.HasPrefix(got, "bbbb") {
		t.Fatalf("after select 1 draw=%q want prefix bbbb", got)
	}
}
