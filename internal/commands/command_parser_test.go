package commands

import "testing"

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
		t.Fatalf("root suggestions = %#v, want [window]", names(suggestions))
	}

	p.Sync("window l", 8)
	suggestions = p.Suggestions()
	if len(suggestions) != 1 || suggestions[0].Name != "left" {
		t.Fatalf("window suggestions = %#v, want [left]", names(suggestions))
	}
}

func names(nodes []*CommandNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}
