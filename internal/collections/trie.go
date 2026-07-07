package collections

import (
	"github.com/yairgd/cgdb-go/internal/platform"
)

type Callback func(args ...any)

type TrieNode[T any] struct {
	Children     map[platform.Key]*TrieNode[T]
	IsTerminal   bool
	OnEnter      func()
	OnExactMatch Callback
	data         T
}

type Trie[T any] struct {
	root    TrieNode[T]
	current *TrieNode[T]
	seq     string
	keySeq  []platform.Key
}

func NewTrie[T any]() *Trie[T] {
	return &Trie[T]{}
}

func (t *Trie[T]) Bind(str string, fn Callback) bool {
	if pending, ok := platform.ParseKeySequence(str); ok {
		t.insert(pending, fn)
		return true
	}
	return false
}

func (t *Trie[T]) insert(pending []platform.Key, onExactMatch Callback) {
	root := &t.root

	for _, key := range pending {
		if root.Children == nil {
			root.Children = make(map[platform.Key]*TrieNode[T])
		}

		if _, ok := root.Children[key]; !ok {
			root.Children[key] = &TrieNode[T]{
				IsTerminal: false,
			}
		}
		root = root.Children[key]
	}
	root.IsTerminal = true
	root.OnExactMatch = onExactMatch
}

func (t *Trie[T]) Insert(str string, data T) bool {
	pending, ok := platform.ParseKeySequence(str)
	if !ok {
		return false
	}

	root := &t.root
	for _, key := range pending {
		if root.Children == nil {
			root.Children = make(map[platform.Key]*TrieNode[T])
		}
		if _, exists := root.Children[key]; !exists {
			root.Children[key] = &TrieNode[T]{}
		}
		root = root.Children[key]
	}
	root.IsTerminal = true
	root.data = data
	return true
}

func (t *Trie[T]) SearchFull(str string) bool {
	root := &t.root

	if pending, ok := platform.ParseKeySequence(str); ok {
		for _, key := range pending {
			if child, ok := root.Children[key]; ok {
				root = child
			} else {
				return false
			}
		}

		if !root.IsTerminal {
			return false
		}

		if root.OnExactMatch != nil {
			root.OnExactMatch()
		}

		return true
	}
	return false
}

func (t *Trie[T]) SearchPartial(key platform.Key) bool {
	if t.current == nil {
		t.current = &t.root
	}

	if child, ok := t.current.Children[key]; ok {
		t.current = child
		t.seq += key.String()
		t.keySeq = append(t.keySeq, key)
	} else {
		t.current = nil
		t.seq = ""
		t.keySeq = t.keySeq[:0]
		return false
	}

	if !t.current.IsTerminal {
		return false
	}

	if t.current.OnExactMatch != nil {
		t.current.OnExactMatch(t.seq, t.keySeq)
		t.current = nil
		t.seq = ""
		t.keySeq = t.keySeq[:0]
	}

	return true
}

func (t *Trie[T]) ResetPartial() {
	t.current = nil
	t.seq = ""
	t.keySeq = t.keySeq[:0]
}

func (t *Trie[T]) Completer(str string) ([]T, bool) {
	root := &t.root

	keys, ok := platform.ParseKeySequence(str)
	if !ok {
		var zero []T
		return zero, false
	}
	for _, key := range keys {
		if root.Children == nil {
			var zero []T
			return zero, false
		}
		child, ok := root.Children[key]
		if !ok {
			return nil, false
		}
		root = child
	}

	var list []T

	var collect func(*TrieNode[T])
	collect = func(n *TrieNode[T]) {
		if n == nil {
			return
		}

		if n.IsTerminal {
			list = append(list, n.data)
		}

		for _, child := range n.Children {
			collect(child)
		}
	}

	collect(root)

	return list, len(list) > 0
}
