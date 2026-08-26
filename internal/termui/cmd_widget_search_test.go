package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/commands"
)

func TestCmdWidgetActivateSearch(t *testing.T) {
	reg := commands.NewCommandRegistry()
	w := NewCmdWidget(reg)
	var changes []string
	var submitted string
	w.SetPostInterrupt(func(ev any) {
		switch msg := ev.(type) {
		case SearchTextChangedMsg:
			changes = append(changes, msg.Text)
		case SearchSubmittedMsg:
			submitted = msg.Pattern
		}
	})

	w.ActivateSearch()
	if !w.Active() || w.Kind() != CmdKindSearch || w.Text() != "/" {
		t.Fatalf("ActivateSearch got active=%v kind=%v text=%q", w.Active(), w.Kind(), w.Text())
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'f', 0))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'o', 0))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'o', 0))
	if w.Text() != "/foo" || w.Pattern() != "foo" {
		t.Fatalf("typed search got text=%q pattern=%q", w.Text(), w.Pattern())
	}
	if len(changes) != 3 || changes[2] != "/foo" {
		t.Fatalf("SearchTextChangedMsg = %#v", changes)
	}

	// Tab must not run command completion in search mode.
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTAB, 0, 0))
	if w.Text() != "/foo" {
		t.Fatalf("Tab mutated search line: %q", w.Text())
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if submitted != "foo" {
		t.Fatalf("SearchSubmittedMsg got %q", submitted)
	}
	if w.Active() {
		t.Fatal("Enter should deactivate")
	}
}
