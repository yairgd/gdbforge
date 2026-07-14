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

// HandleKey advances the binding trie. handled is true when the key
// completed a binding or is part of an unfinished chord.
func (k *KeyBindingRegistry) HandleKey(key platform.Key) (cmd *CommandNode, handled bool) {
	cmd, ok := k.trie.SearchPartial(key)
	if ok {
		return cmd, true
	}
	if k.trie.InPartial() {
		return nil, true
	}
	return nil, false
}
