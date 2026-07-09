package commands

// Cmd declares a leaf command node with the given name and action.
// The node is not inserted into a tree until passed to Group or CommandNode.Group.
func Cmd(name string, action func(args ...any)) *CommandNode {
	return NewCommand(name, action)
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
