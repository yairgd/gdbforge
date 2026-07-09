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
