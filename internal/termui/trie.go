package termui

import (
	tcell "github.com/gdamore/tcell/v2"
)

type Callback func(args ...any)

type Key struct {
	Key  tcell.Key
	Rune rune
}

type TrieNode struct {
	Children     map[Key]*TrieNode
	IsTerminal   bool
	OnEnter      func()
	OnExactMatch Callback
}

type Trie struct {
	root    TrieNode
	current *TrieNode
	seq     string
	keySeq  []Key
}

func NewTrie() *Trie {
	return &Trie{}

}

var keyMap = map[string]tcell.Key{
	// Control
	"C-a": tcell.KeyCtrlA,
	"C-b": tcell.KeyCtrlB,
	"C-c": tcell.KeyCtrlC,
	"C-d": tcell.KeyCtrlD,
	"C-e": tcell.KeyCtrlE,
	"C-f": tcell.KeyCtrlF,
	"C-g": tcell.KeyCtrlG,
	"C-h": tcell.KeyCtrlH,
	"C-i": tcell.KeyCtrlI,
	"C-j": tcell.KeyCtrlJ,
	"C-k": tcell.KeyCtrlK,
	"C-l": tcell.KeyCtrlL,
	"C-m": tcell.KeyCtrlM,
	"C-n": tcell.KeyCtrlN,
	"C-o": tcell.KeyCtrlO,
	"C-p": tcell.KeyCtrlP,
	"C-q": tcell.KeyCtrlQ,
	"C-r": tcell.KeyCtrlR,
	"C-s": tcell.KeyCtrlS,
	"C-t": tcell.KeyCtrlT,
	"C-u": tcell.KeyCtrlU,
	"C-v": tcell.KeyCtrlV,
	"C-w": tcell.KeyCtrlW,
	"C-x": tcell.KeyCtrlX,
	"C-y": tcell.KeyCtrlY,
	"C-z": tcell.KeyCtrlZ,

	// Navigation
	"Up":    tcell.KeyUp,
	"Down":  tcell.KeyDown,
	"Left":  tcell.KeyLeft,
	"Right": tcell.KeyRight,
	"Home":  tcell.KeyHome,
	"End":   tcell.KeyEnd,
	"PgUp":  tcell.KeyPgUp,
	"PgDn":  tcell.KeyPgDn,

	// Editing
	"Tab":       tcell.KeyTab,
	"Enter":     tcell.KeyEnter,
	"Esc":       tcell.KeyEscape,
	"Backspace": tcell.KeyBackspace2,
	"Delete":    tcell.KeyDelete,
	"Insert":    tcell.KeyInsert,

	// Function keys
	"F1":  tcell.KeyF1,
	"F2":  tcell.KeyF2,
	"F3":  tcell.KeyF3,
	"F4":  tcell.KeyF4,
	"F5":  tcell.KeyF5,
	"F6":  tcell.KeyF6,
	"F7":  tcell.KeyF7,
	"F8":  tcell.KeyF8,
	"F9":  tcell.KeyF9,
	"F10": tcell.KeyF10,
	"F11": tcell.KeyF11,
	"F12": tcell.KeyF12,
}

var keyToStringMap = map[tcell.Key]string{
	tcell.KeyCtrlA: "C-a",
	tcell.KeyCtrlB: "C-b",
	tcell.KeyCtrlC: "C-c",
	tcell.KeyCtrlD: "C-d",
	tcell.KeyCtrlE: "C-e",
	tcell.KeyCtrlF: "C-f",
	tcell.KeyCtrlG: "C-g",
	tcell.KeyCtrlH: "C-h",
	tcell.KeyCtrlI: "C-i",
	tcell.KeyCtrlJ: "C-j",
	tcell.KeyCtrlK: "C-k",
	tcell.KeyCtrlL: "C-l",
	tcell.KeyCtrlM: "C-m",
	tcell.KeyCtrlN: "C-n",
	tcell.KeyCtrlO: "C-o",
	tcell.KeyCtrlP: "C-p",
	tcell.KeyCtrlQ: "C-q",
	tcell.KeyCtrlR: "C-r",
	tcell.KeyCtrlS: "C-s",
	tcell.KeyCtrlT: "C-t",
	tcell.KeyCtrlU: "C-u",
	tcell.KeyCtrlV: "C-v",
	tcell.KeyCtrlW: "C-w",
	tcell.KeyCtrlX: "C-x",
	tcell.KeyCtrlY: "C-y",
	tcell.KeyCtrlZ: "C-z",

	tcell.KeyUp:    "Up",
	tcell.KeyDown:  "Down",
	tcell.KeyLeft:  "Left",
	tcell.KeyRight: "Right",
	tcell.KeyHome:  "Home",
	tcell.KeyEnd:   "End",
	tcell.KeyPgUp:  "PgUp",
	tcell.KeyPgDn:  "PgDn",

	tcell.KeyTab:        "Tab",
	tcell.KeyEnter:      "Enter",
	tcell.KeyEscape:     "Esc",
	tcell.KeyBackspace2: "Backspace",
	tcell.KeyDelete:     "Delete",
	tcell.KeyInsert:     "Insert",

	tcell.KeyF1:  "F1",
	tcell.KeyF2:  "F2",
	tcell.KeyF3:  "F3",
	tcell.KeyF4:  "F4",
	tcell.KeyF5:  "F5",
	tcell.KeyF6:  "F6",
	tcell.KeyF7:  "F7",
	tcell.KeyF8:  "F8",
	tcell.KeyF9:  "F9",
	tcell.KeyF10: "F10",
	tcell.KeyF11: "F11",
	tcell.KeyF12: "F12",
}

func (t *Trie) EventToKey(ev tcell.Event) (Key, bool) {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return Key{}, false
	}

	return Key{
		Key:  kev.Key(),
		Rune: kev.Rune(),
	}, true
}

func (t *Trie) normalize(k Key) Key {
	if k.Key != tcell.KeyRune {
		k.Rune = 0
	}
	return k
}

func (t *Trie) parseToken(str string) Key {
	return Key{
		Key:  keyMap[str],
		Rune: 0,
	}
}

func (t *Trie) keyToString(k Key) string {

	if k.Key == tcell.KeyRune {
		return string(k.Rune)
	}

	return "<" + keyToStringMap[k.Key] + ">"
}

func (t *Trie) ParseSequence(str string) ([]Key, bool) {
	var tmp string
	var isCtl bool
	var pending []Key

	for _, v := range str {
		if v == '<' {
			tmp = ""
			isCtl = true
			continue
		} else if v == '>' {
			pending = append(pending, t.parseToken(tmp))
			isCtl = false
		} else if isCtl {
			tmp += string(v)
		} else {
			pending = append(pending, Key{
				Key:  tcell.KeyRune,
				Rune: v,
			})
		}
	}
	if isCtl == true {
		return nil, false
	} // error
	return pending, true
}

func (t *Trie) Bind(str string, fn Callback) bool {

	if pending, ok := t.ParseSequence(str); ok {
		t.insert(pending, fn)
		return true
	}
	return false

}

func (t *Trie) insert(pending []Key, onExactMatch Callback) {
	root := &t.root

	for _, key := range pending {
		if root.Children == nil {
			root.Children = make(map[Key]*TrieNode)
		}

		if _, ok := root.Children[key]; !ok {
			root.Children[key] = &TrieNode{
				IsTerminal: false,
			}
		}
		root = root.Children[key]
	}
	root.IsTerminal = true
	root.OnExactMatch = onExactMatch
}

func (t *Trie) SearchFull(str string) bool {
	root := &t.root

	if pending, ok := t.ParseSequence(str); ok {
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

func (t *Trie) SearchPartial(ev tcell.Event) bool {

	var key Key
	if k, ok := ev.(*tcell.EventKey); ok {
		key = Key{
			Key:  k.Key(),
			Rune: k.Rune(),
		}

		if k.Key() != tcell.KeyRune {
			key.Key = k.Key()
			key.Rune = 0
		}
	} else {
		t.current = nil
		t.seq = ""
		t.keySeq = t.keySeq[:0]
		return false
	}

	if t.current == nil {
		t.current = &t.root
	}

	if child, ok := t.current.Children[key]; ok {
		t.current = child
		t.seq += t.keyToString(key)
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
