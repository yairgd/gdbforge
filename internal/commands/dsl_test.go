package commands

import "testing"

func TestDSLHierarchy(t *testing.T) {
	var called string

	registry := NewCommandRegistry()
	registry.Root.
		Group("window",
			Cmd("left", func(args ...any) { called = "left" }),
			Group("split",
				Cmd("horizontal", func(args ...any) { called = "horizontal" }),
			),
		).
		Group("break",
			Group("file",
				Cmd("line", func(args ...any) { called = "line" }),
			),
		)

	window, ok := registry.Root.Children.SearchFull("window")
	if !ok {
		t.Fatal("missing window group")
	}
	if window.Action != nil {
		t.Fatal("window should be a container")
	}

	left, ok := window.Children.SearchFull("left")
	if !ok || left.Parent != window {
		t.Fatal("missing left command")
	}
	left.Action()
	if called != "left" {
		t.Fatalf("left action = %q", called)
	}

	split, ok := window.Children.SearchFull("split")
	if !ok {
		t.Fatal("missing split group")
	}
	line, ok := registry.Root.Children.SearchFull("break")
	if !ok {
		t.Fatal("missing break group")
	}
	file, ok := line.Children.SearchFull("file")
	if !ok || file.Parent != line {
		t.Fatal("missing file group")
	}
	leaf, ok := file.Children.SearchFull("line")
	if !ok || leaf.Parent != file {
		t.Fatal("missing third-level line command")
	}
	_ = split
}
