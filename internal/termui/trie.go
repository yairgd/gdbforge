package termui

import (
	tcell "github.com/gdamore/tcell/v2"
)

/*

ל־Trie קלאסי יש בדרך כלל 3 פעולות מרכזיות:

    Insert

        הכנסת מילה/רצף לעץ.

    Search (Exact Match)

        בדיקה האם מילה/רצף קיים במלואו.

    Prefix Search (StartsWith)

        בדיקה האם קיים לפחות איבר אחד שמתחיל בפרפיקס נתון.

*/

type Callback func()

type Key struct {
	Key  tcell.Key
	Rune rune
}

type TrieNode struct {
	Children     map[Key]*TrieNode
	IsTerminal   bool
	OnEnter      func()
	OnExactMatch func()
}

type Trie struct {
	root TrieNode
}

func NewTrie() *Trie {
	return &Trie{}

}
func (t *Trie) parseToken(str string) Key {
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
	return Key{
		Key: keyMap[str],
	}
}

func (t *Trie) Bind(str string, fn Callback) bool {
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
			pending = append(pending, Key{Rune: v})
		}
	}
	if isCtl == true {
		return false
	} // error
	t.insert(pending, fn)
	return true

}

func (t *Trie) insert(pending []Key, onExactMatch Callback) {
	root := &t.root

	for _, key := range pending {
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
