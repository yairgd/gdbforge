package termui

import "testing"

func TestInputLineInsertBackspace(t *testing.T) {
	l := NewInputLine()
	l.InsertRune('a')
	l.InsertRune('b')
	l.InsertRune('c')
	if l.Text() != "abc" || l.CursorCol() != 3 {
		t.Fatalf("got %q col=%d", l.Text(), l.CursorCol())
	}
	l.MoveLeft()
	l.Backspace()
	if l.Text() != "ac" || l.CursorCol() != 1 {
		t.Fatalf("after backspace: %q col=%d", l.Text(), l.CursorCol())
	}
}

func TestInputLineKillWord(t *testing.T) {
	l := NewInputLine()
	l.SetText("foo bar baz")
	l.KillWord()
	if l.Text() != "foo bar " {
		t.Fatalf("got %q", l.Text())
	}
}

func TestInputLineHistoryDraft(t *testing.T) {
	l := NewInputLine()
	l.PushHistory("first")
	l.PushHistory("second")
	l.SetText("draft")
	l.HistoryPrev() // second
	if l.Text() != "second" {
		t.Fatalf("prev: %q", l.Text())
	}
	l.HistoryPrev() // first
	if l.Text() != "first" {
		t.Fatalf("prev2: %q", l.Text())
	}
	l.HistoryNext()
	l.HistoryNext() // back to draft
	if l.Text() != "draft" {
		t.Fatalf("draft restore: %q", l.Text())
	}
}

func TestInputLinePushHistoryDedupe(t *testing.T) {
	l := NewInputLine()
	l.PushHistory("x")
	l.PushHistory("x")
	l.PushHistory("")
	if l.LastHistory() != "x" {
		t.Fatalf("last=%q", l.LastHistory())
	}
	l.HistoryPrev()
	if l.Text() != "x" {
		t.Fatalf("hist: %q", l.Text())
	}
	l.HistoryPrev()
	if l.Text() != "x" {
		t.Fatalf("still one entry: %q", l.Text())
	}
}
