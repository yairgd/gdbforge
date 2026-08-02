package termui

import (
	"reflect"
	"testing"
)

func TestMemoryHistoryAddConsecutiveDedupe(t *testing.T) {
	h := NewMemoryHistory()
	h.Add(":q")
	h.Add(":q")
	h.Add(":q")
	h.Add(":clear")
	h.Add(":q")
	got := h.Items()
	want := []string{":q", ":clear", ":q"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestMemoryHistoryLoadConsecutiveDedupe(t *testing.T) {
	h := NewMemoryHistory()
	h.Load([]string{":q", ":q", "", ":help ", ":help ", ":close"})
	got := h.Items()
	want := []string{":q", ":help ", ":close"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}
