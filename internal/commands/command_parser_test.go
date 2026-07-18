package commands

import (
	"reflect"
	"testing"
)

func TestCommandParserBangRestArgs(t *testing.T) {
	reg := NewCommandRegistry()
	var got []string
	reg.Root.LeafRest("!", func(args ...any) {
		got = make([]string, len(args))
		for i, a := range args {
			got[i] = a.(string)
		}
	})

	p := NewCommandParser(reg)
	if err := p.Parse("!ssh root@192.168.20.50"); err != nil {
		t.Fatalf("Parse glued: %v", err)
	}
	wantArgs := []string{"ssh", "root@192.168.20.50"}
	if !reflect.DeepEqual(p.Args(), wantArgs) {
		t.Fatalf("Args = %#v, want %#v", p.Args(), wantArgs)
	}
	if err := p.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("action args = %#v, want %#v", got, wantArgs)
	}

	got = nil
	if err := p.Parse("! bash"); err != nil {
		t.Fatalf("Parse spaced: %v", err)
	}
	if !reflect.DeepEqual(p.Args(), []string{"bash"}) {
		t.Fatalf("Args = %#v", p.Args())
	}
}

func TestCommandParserBangLS(t *testing.T) {
	reg := NewCommandRegistry()
	reg.Root.LeafRest("!", func(args ...any) {})

	p := NewCommandParser(reg)
	if err := p.Parse("!ls"); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(p.Args(), []string{"ls"}) {
		t.Fatalf("Args = %#v, want [ls]", p.Args())
	}
}

func TestCommandParserRestArgsEmpty(t *testing.T) {
	reg := NewCommandRegistry()
	called := false
	reg.Root.LeafRest("!", func(args ...any) {
		called = true
		if len(args) != 0 {
			t.Fatalf("args = %#v, want empty", args)
		}
	})

	p := NewCommandParser(reg)
	if err := p.Parse("!"); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := p.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Fatal("action not called")
	}
}

func TestCommandParserSyncBang(t *testing.T) {
	reg := NewCommandRegistry()
	reg.Root.LeafRest("!", func(args ...any) {})

	p := NewCommandParser(reg)
	p.Sync("!ba", 3)
	if p.Current() == nil || p.Current().Name != "!" {
		t.Fatalf("current=%v", p.Current())
	}
	if p.CurrentToken() != "ba" {
		t.Fatalf("token=%q", p.CurrentToken())
	}
}

func TestCommandParserSyncSuggestions(t *testing.T) {
	reg := NewCommandRegistry()
	root := reg.Root
	window := root.InsertName("window")
	window.InsertName("left")
	window.InsertName("right")
	root.InsertName("break")

	p := NewCommandParser(reg)

	p.Sync("win", 3)
	suggestions := p.Suggestions()
	if len(suggestions) != 1 || suggestions[0].Name != "window" {
		t.Fatalf("root suggestions = %#v, want [window]", nodeNames(suggestions))
	}

	p.Sync("window l", 8)
	suggestions = p.Suggestions()
	if len(suggestions) != 1 || suggestions[0].Name != "left" {
		t.Fatalf("window suggestions = %#v, want [left]", nodeNames(suggestions))
	}
}

func TestSuggestionNamesRestArgsComplete(t *testing.T) {
	reg := NewCommandRegistry()
	reg.Root.LeafRestComplete("b", func(args ...any) {}, func(prefix string) []string {
		var out []string
		for _, n := range []string{"about", "logger", "gdb", "main.c"} {
			if prefix == "" || (len(n) >= len(prefix) && n[:len(prefix)] == prefix) {
				out = append(out, n)
			}
		}
		return out
	})

	p := NewCommandParser(reg)
	p.Sync("b ", 2)
	if !p.CurrentIsRestArgs() {
		t.Fatal("expected rest-args on b")
	}
	got := p.SuggestionNames()
	if len(got) != 4 {
		t.Fatalf("names=%v", got)
	}

	p.Sync("b lo", 4)
	got = p.SuggestionNames()
	if len(got) != 1 || got[0] != "logger" {
		t.Fatalf("names=%v, want [logger]", got)
	}
}

func nodeNames(nodes []*CommandNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}
