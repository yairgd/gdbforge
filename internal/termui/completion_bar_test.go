package termui

import (
	"testing"

	"github.com/yairgd/cgdb-go/internal/platform"
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
