package commands

import (
	"github.com/yairgd/cgdb-go/internal/collections"
	"github.com/yairgd/cgdb-go/internal/platform"
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
	n.Children.Insert(child, child.Name)
}

func (n *CommandNode) InsertName(name string) *CommandNode {
	child := NewCommandNode(name)
	n.Insert(child)
	return child
}
