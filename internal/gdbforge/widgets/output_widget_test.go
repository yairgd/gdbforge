package widgets

import "testing"

func TestOutputWidgetHostLine(t *testing.T) {
	w := NewOutputWidget()
	w.AppendHostLine("hello")
	w.AppendInferior("world\r\n")
}

func TestOutputWidgetClear(t *testing.T) {
	w := NewOutputWidget()
	w.AppendHostLine("x")
	w.Clear()
	w.AppendHostLine("y")
}
