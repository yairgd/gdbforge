package main

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestLuaPaneInstanceName(t *testing.T) {
	p := &discardPane{}
	rt := luahost.New(p, nil)
	defer rt.Close()

	if got := luaPaneInstanceName(rt, []string{"snake1"}); got != "" {
		t.Fatalf("no hooks: got %q", got)
	}
	if err := rt.LoadString(`function on_key(k) end`, "pane"); err != nil {
		t.Fatal(err)
	}
	if got := luaPaneInstanceName(rt, nil); got != "" {
		t.Fatalf("no args: got %q", got)
	}
	if got := luaPaneInstanceName(rt, []string{"snake1"}); got != "snake1" {
		t.Fatalf("got %q", got)
	}
	if got := luaPaneInstanceName(nil, []string{"x"}); got != "" {
		t.Fatalf("nil rt: got %q", got)
	}
}

type discardPane struct{}

func (discardPane) AppendPrint(string)             {}
func (discardPane) ClearAll()                      {}
func (discardPane) ClearCells()                    {}
func (discardPane) SetCell(int, int, rune, string) {}
func (discardPane) Size() (int, int)               { return 0, 0 }
