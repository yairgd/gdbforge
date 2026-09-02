package widgets

import "testing"

func TestOutputWidgetHostLine(t *testing.T) {
	w := NewOutputWidget()
	w.AppendHostLine("hello")
	w.AppendHostLine("world")
}

func TestOutputWidgetClear(t *testing.T) {
	w := NewOutputWidget()
	w.AppendHostLine("x")
	w.Clear()
	w.AppendHostLine("y")
}
