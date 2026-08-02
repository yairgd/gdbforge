package commands

import (
	"github.com/yairgd/gdbforge/internal/collections"
	"github.com/yairgd/gdbforge/internal/platform"
)

type CommandRegistry struct {
	Root *CommandNode
	Keys *collections.Trie[*CommandNode]
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		Root: NewCommandNode("/"),
		Keys: collections.NewTrie[*CommandNode](),
	}
}

func (r *CommandRegistry) Insert(cmd *CommandNode, bindings ...string) {
	r.Root.Insert(cmd)

	for _, b := range bindings {
		r.Keys.Insert(cmd, b)
	}
}
func (r *CommandRegistry) ResetPartial() {
	r.Keys.ResetPartial()
}
func (r *CommandRegistry) SearchPartial(key platform.Key) (*CommandNode, bool) {
	return r.Keys.SearchPartial(key)
}

type CommandNode struct {
	Parent   *CommandNode
	Children *collections.Trie[*CommandNode]
	Name     string
	Action   func(args ...any)
	// Handled if set is used instead of Action for key dispatch. Return false
	// to decline the binding (fall through to the next input layer).
	Handled func() bool
	// RestArgs means tokens after this node are passed to Action, not walked as children.
	RestArgs bool
	// CompleteArgs optionally supplies dynamic rest-arg completions (e.g. :b buffers).
	// Same Completer shape as GDB Tab (-complete → []string names).
	CompleteArgs Completer
}

func NewCommandNode(name string) *CommandNode {
	return &CommandNode{
		Name:     name,
		Children: collections.NewTrie[*CommandNode](),
	}
}

func NewCommand(name string, action func(args ...any)) *CommandNode {
	return &CommandNode{
		Name:   name,
		Action: action,
	}
}

// NewHandledCommand binds a key action that may fall through (return false).
func NewHandledCommand(name string, handled func() bool) *CommandNode {
	return &CommandNode{
		Name:    name,
		Handled: handled,
	}
}
func (n *CommandNode) Complete(prefix string) ([]*CommandNode, bool) {
	if n == nil || n.Children == nil {
		return nil, false
	}

	return n.Children.Complete(prefix)
}
func (n *CommandNode) Insert(child *CommandNode) {
	if n.Children == nil {
		n.Children = collections.NewTrie[*CommandNode]()
	}
	child.Parent = n
	n.Children.Insert(child, child.Name)
}

func (n *CommandNode) InsertName(name string) *CommandNode {
	child := NewCommandNode(name)
	n.Insert(child)
	return child
}
