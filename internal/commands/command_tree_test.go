package commands

import "testing"

func TestCommandTreeHierarchy(t *testing.T) {
	reg := NewCommandRegistry()
	root := reg.Root

	if root.Name != "/" {
		t.Fatalf("root name = %q, want %q", root.Name, "/")
	}
	if root.Parent != nil {
		t.Fatal("root parent should be nil")
	}
	if root.Children == nil {
		t.Fatal("root children trie should be allocated")
	}

	var (
		onFocusLeft  = func(args ...any) {}
		onFocusRight = func(args ...any) {}
		onFocusUp    = func(args ...any) {}
		onFocusDown  = func(args ...any) {}
		breakFile    = func(args ...any) {}
		deleteBP     = func(args ...any) {}
		showRegs     = func(args ...any) {}
		showThreads  = func(args ...any) {}
	)

	window := root.InsertName("window")
	left := window.InsertName("left")
	left.Action = onFocusLeft
	right := window.InsertName("right")
	right.Action = onFocusRight
	up := window.InsertName("up")
	up.Action = onFocusUp
	down := window.InsertName("down")
	down.Action = onFocusDown

	breakCmd := root.InsertName("break")
	breakFileNode := breakCmd.InsertName("file")
	breakFileNode.Action = breakFile
	deleteNode := breakCmd.InsertName("delete")
	deleteNode.Action = deleteBP

	info := root.InsertName("info")
	registers := info.InsertName("registers")
	registers.Action = showRegs
	threads := info.InsertName("threads")
	threads.Action = showThreads

	assertNode(t, window, root, "window", nil)
	assertNode(t, left, window, "left", onFocusLeft)
	assertNode(t, right, window, "right", onFocusRight)
	assertNode(t, up, window, "up", onFocusUp)
	assertNode(t, down, window, "down", onFocusDown)

	assertNode(t, breakCmd, root, "break", nil)
	assertNode(t, breakFileNode, breakCmd, "file", breakFile)
	assertNode(t, deleteNode, breakCmd, "delete", deleteBP)

	assertNode(t, info, root, "info", nil)
	assertNode(t, registers, info, "registers", showRegs)
	assertNode(t, threads, info, "threads", showThreads)

	if node, ok := root.Children.SearchFull("window"); !ok || node != window {
		t.Fatal("root should find window child")
	}
	if node, ok := window.Children.SearchFull("left"); !ok || node != left {
		t.Fatal("window should find left child")
	}
	if node, ok := breakCmd.Children.SearchFull("file"); !ok || node != breakFileNode {
		t.Fatal("break should find file child")
	}
	if node, ok := info.Children.SearchFull("threads"); !ok || node != threads {
		t.Fatal("info should find threads child")
	}
}

func assertNode(
	t *testing.T,
	node, wantParent *CommandNode,
	wantName string,
	wantAction func(args ...any),
) {
	t.Helper()

	if node.Name != wantName {
		t.Fatalf("name = %q, want %q", node.Name, wantName)
	}
	if node.Parent != wantParent {
		t.Fatalf("%q parent = %p, want %p", wantName, node.Parent, wantParent)
	}
	if node.Children == nil {
		t.Fatalf("%q children trie should be allocated", wantName)
	}
	if (node.Action == nil) != (wantAction == nil) {
		t.Fatalf("%q action nil = %v, want nil = %v", wantName, node.Action == nil, wantAction == nil)
	}
}
