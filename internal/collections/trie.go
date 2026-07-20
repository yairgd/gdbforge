package collections

import (
	"github.com/yairgd/gdbx/internal/platform"
)

type Callback func(args ...any)

type TrieNode[T any] struct {
	Children   map[platform.Key]*TrieNode[T]
	IsTerminal bool
	data       T
}

type Trie[T any] struct {
	root    TrieNode[T]
	current *TrieNode[T]
	//seq     string
	//keySeq  []platform.Key
}

func NewTrie[T any]() *Trie[T] {
	return &Trie[T]{}
}

func (t *Trie[T]) insert(pending []platform.Key) { //, onExactMatch Callback) {
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
}

func (t *Trie[T]) Insert(data T, strs ...string) {

	for _, str := range strs {
		pending, ok := platform.ParseKeySequence(str)
		if !ok {
			return
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
	}
	return
}

func (t *Trie[T]) SearchFull(str string) (T, bool) {
	var zero T

	root := &t.root

	pending, ok := platform.ParseKeySequence(str)
	if !ok {
		return zero, false
	}

	for _, key := range pending {
		child, ok := root.Children[key]
		if !ok {
			return zero, false
		}
		root = child
	}

	if !root.IsTerminal {
		return zero, false
	}

	return root.data, true
}

func (t *Trie[T]) SearchPartial(key platform.Key) (T, bool) {
	var zero T

	if t.current == nil {
		t.current = &t.root
	}

	if child, ok := t.current.Children[key]; ok {
		t.current = child
		//		t.seq += key.String()
		//		t.keySeq = append(t.keySeq, key)
	} else {
		t.ResetPartial()
		return zero, false
	}

	if !t.current.IsTerminal {
		return zero, false
	}

	data := t.current.data
	t.ResetPartial()

	return data, true
}

func (t *Trie[T]) ResetPartial() {
	t.current = nil
	// t.seq = ""
	// t.keySeq = t.keySeq[:0]
}

// InPartial reports whether SearchPartial has advanced into a chord.
func (t *Trie[T]) InPartial() bool {
	return t.current != nil
}

func (t *Trie[T]) Complete(str string) ([]T, bool) {
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
