package commands

// Cmd declares a leaf command node with the given name and action.
// The node is not inserted into a tree until passed to Group or CommandNode.Group.
func Cmd(name string, action func(args ...any)) *CommandNode {
	return NewCommand(name, action)
}

// CmdRest declares a leaf that consumes the remainder of the line as Action args
// (e.g. :!ssh root@host → Action("ssh", "root@host")).
func CmdRest(name string, action func(args ...any)) *CommandNode {
	n := NewCommand(name, action)
	n.RestArgs = true
	return n
}

// CmdRestComplete is CmdRest with a dynamic rest-arg completer (Tab after the command).
func CmdRestComplete(name string, action func(args ...any), complete Completer) *CommandNode {
	n := CmdRest(name, action)
	n.CompleteArgs = complete
	return n
}

// Group builds a container node and attaches the given children to it.
// The returned node is not inserted into a parent until passed to a parent Group
// or CommandNode.Group.
func Group(name string, children ...*CommandNode) *CommandNode {
	node := NewCommandNode(name)
	for _, child := range children {
		node.Insert(child)
	}
	return node
}

// Group inserts a named container with the given children into n and returns n
// so that sibling groups can be chained at the same level.
func (n *CommandNode) Group(name string, children ...*CommandNode) *CommandNode {
	n.Insert(Group(name, children...))
	return n
}

// Leaf inserts a leaf command into n and returns n so siblings can be chained.
func (n *CommandNode) Leaf(name string, action func(args ...any)) *CommandNode {
	n.Insert(Cmd(name, action))
	return n
}

// LeafRest inserts a rest-args leaf into n and returns n so siblings can be chained.
func (n *CommandNode) LeafRest(name string, action func(args ...any)) *CommandNode {
	n.Insert(CmdRest(name, action))
	return n
}

// LeafRestComplete inserts a rest-args leaf with dynamic Tab completions.
func (n *CommandNode) LeafRestComplete(name string, action func(args ...any), complete Completer) *CommandNode {
	n.Insert(CmdRestComplete(name, action, complete))
	return n
}
