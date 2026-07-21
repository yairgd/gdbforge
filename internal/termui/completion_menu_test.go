package termui

import "testing"

func TestCompletionMenuIgnoresSingle(t *testing.T) {
	var m CompletionMenu
	m.Set([]string{"only"})
	if m.Active() {
		t.Fatal("single match should not activate menu")
	}
}

func TestCompletionMenuMoveWraps(t *testing.T) {
	var m CompletionMenu
	m.Set([]string{"about", "logger", "gdb"})
	if m.Selected() != "about" {
		t.Fatalf("selected=%q", m.Selected())
	}
	m.Move(1)
	if m.Selected() != "logger" {
		t.Fatalf("after next selected=%q", m.Selected())
	}
	m.Move(-1)
	if m.Selected() != "about" {
		t.Fatalf("after prev selected=%q", m.Selected())
	}
	m.Move(-1)
	if m.Selected() != "gdb" {
		t.Fatalf("wrap prev selected=%q", m.Selected())
	}
	m.Clear()
	if m.Active() {
		t.Fatal("cleared menu still active")
	}
}

func TestCompletionMenuSetReplaces(t *testing.T) {
	var m CompletionMenu
	m.Set([]string{"about", "logger", "gdb"})
	m.Move(2)
	m.Set([]string{"foo", "bar", "baz"})
	if m.Selected() != "foo" || len(m.Visible()) != 3 {
		t.Fatalf("selected=%q names=%v", m.Selected(), m.Visible())
	}
}
