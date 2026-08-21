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
	reg.Root.LeafRestComplete("b", func(args ...any) {}, func(prefix string, _ bool) []string {
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

func TestCommandParserExactPreferOverPrefix(t *testing.T) {
	reg := NewCommandRegistry()
	var got string
	reg.Root.Group("gdb",
		Cmd("next", func(args ...any) { got = "next" }),
		Cmd("nexti", func(args ...any) { got = "nexti" }),
		Cmd("step", func(args ...any) { got = "step" }),
		Cmd("stepi", func(args ...any) { got = "stepi" }),
	)
	p := NewCommandParser(reg)
	for _, tc := range []struct{ line, want string }{
		{"gdb next", "next"},
		{"gdb nexti", "nexti"},
		{"gdb step", "step"},
		{"gdb stepi", "stepi"},
	} {
		got = ""
		if err := p.Parse(tc.line); err != nil {
			t.Fatalf("Parse %q: %v", tc.line, err)
		}
		if err := p.Execute(); err != nil {
			t.Fatalf("Execute %q: %v", tc.line, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.line, got, tc.want)
		}
	}
	// Ambiguous short prefix still fails.
	if err := p.Parse("gdb nex"); err == nil {
		t.Fatal("gdb nex should be ambiguous")
	}
}

func TestCommandParserQuitBang(t *testing.T) {
	reg := NewCommandRegistry()
	var gotForce bool
	reg.Root.Leaf("quit", func(args ...any) {
		for _, a := range args {
			if s, ok := a.(string); ok && s == "!" {
				gotForce = true
			}
		}
	})
	p := NewCommandParser(reg)
	for _, line := range []string{"q!", "quit!"} {
		gotForce = false
		if err := p.Parse(line); err != nil {
			t.Fatalf("Parse %q: %v", line, err)
		}
		if p.Current() == nil || p.Current().Name != "quit" {
			t.Fatalf("%q current=%v", line, p.Current())
		}
		if err := p.Execute(); err != nil {
			t.Fatalf("Execute %q: %v", line, err)
		}
		if !gotForce {
			t.Fatalf("%q: want bang arg", line)
		}
	}
	gotForce = false
	if err := p.Parse("q"); err != nil {
		t.Fatal(err)
	}
	if err := p.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotForce {
		t.Fatal(":q must not force")
	}
}

func TestCommandParserUniquePrefixEdit(t *testing.T) {
	reg := NewCommandRegistry()
	var gotArgs []string
	reg.Root.LeafRestComplete("edit", func(args ...any) {
		gotArgs = make([]string, len(args))
		for i, a := range args {
			gotArgs[i] = a.(string)
		}
	}, nil)

	p := NewCommandParser(reg)
	if err := p.Parse("e"); err != nil {
		t.Fatalf("Parse e: %v", err)
	}
	if p.Current() == nil || p.Current().Name != "edit" {
		t.Fatalf("current=%v, want edit", p.Current())
	}
	if err := p.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(gotArgs) != 0 {
		t.Fatalf("args=%v, want empty", gotArgs)
	}

	gotArgs = nil
	if err := p.Parse("e main.c"); err != nil {
		t.Fatalf("Parse e main.c: %v", err)
	}
	if !reflect.DeepEqual(p.Args(), []string{"main.c"}) {
		t.Fatalf("Args=%v", p.Args())
	}
	if err := p.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !reflect.DeepEqual(gotArgs, []string{"main.c"}) {
		t.Fatalf("gotArgs=%v", gotArgs)
	}
}

func nodeNames(nodes []*CommandNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}
