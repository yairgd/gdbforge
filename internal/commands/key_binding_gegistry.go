package commands

import (
	"github.com/yairgd/cgdb-go/internal/collections"
	"github.com/yairgd/cgdb-go/internal/platform"
)

type KeyBindingRegistry struct {
	trie *collections.Trie[*CommandNode]
}

func NewKeyBindingRegistry() *KeyBindingRegistry {
	return &KeyBindingRegistry{
		trie: collections.NewTrie[*CommandNode](),
	}
}

func (k *KeyBindingRegistry) Bind(cmd *CommandNode, bindings ...string) {
	for _, b := range bindings {
		k.trie.Insert(cmd, b)
	}
}

func (k *KeyBindingRegistry) SearchPartial(key platform.Key) (*CommandNode, bool) {
	return k.trie.SearchPartial(key)
}

func (k *KeyBindingRegistry) ResetPartial() {
	k.trie.ResetPartial()
}

// InPartial reports whether a multi-key chord is in progress.
func (k *KeyBindingRegistry) InPartial() bool {
	return k.trie.InPartial()
}

// HandleKey advances the binding trie. handled is true when the key matched a
// binding leaf or is part of an unfinished chord. completed is true when the
// binding's action ran successfully (Handled returned true, or Action ran).
// If Handled returns false, completed is false so the caller may fall through
// even though handled is true (trie advanced / leaf matched).
func (k *KeyBindingRegistry) HandleKey(key platform.Key) (completed, handled bool) {
	cmd, ok := k.trie.SearchPartial(key)
	if ok {
		if cmd != nil {
			if cmd.Handled != nil {
				return cmd.Handled(), true
			}
			if cmd.Action != nil {
				cmd.Action()
				return true, true
			}
		}
		return true, true
	}
	if k.trie.InPartial() {
		return false, true
	}
	return false, false
}
