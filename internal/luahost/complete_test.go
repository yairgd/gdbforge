package luahost

import "testing"

func TestCompleteGdbforgeMembers(t *testing.T) {
	rt := New(nil, nil)
	members := rt.GdbforgeMembers()
	if len(members) == 0 {
		t.Fatal("expected gdbforge members")
	}
	found := false
	for _, m := range members {
		if m == "print" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing print in %v", members)
	}
}

func TestCompleteGdbforgePrefix(t *testing.T) {
	members := []string{"print", "program", "pane"}
	line, matches := CompleteGdbforge("gdbforge.pr", members)
	if line != "gdbforge.pr" || len(matches) != 2 {
		t.Fatalf("line=%q matches=%v", line, matches)
	}
	line, matches = CompleteGdbforge("gdbforge.p", members)
	if line != "gdbforge.p" || len(matches) != 3 {
		t.Fatalf("lcp line=%q matches=%v", line, matches)
	}
	line, matches = CompleteGdbforge("gdbforge.print", members)
	if line != "gdbforge.print" || len(matches) != 1 {
		t.Fatalf("unique line=%q matches=%v", line, matches)
	}
}

func TestCompleteGdbforgeWord(t *testing.T) {
	line, matches := CompleteGdbforge("gdbforge", []string{"print"})
	if line != "gdbforge." || len(matches) != 1 || matches[0] != "." {
		t.Fatalf("line=%q matches=%v", line, matches)
	}
}

func TestApplyGdbforgeChoice(t *testing.T) {
	if got := ApplyGdbforgeChoice("gdbforge.pr", "print"); got != "gdbforge.print" {
		t.Fatalf("got %q", got)
	}
}
