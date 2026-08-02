package platform

import "testing"

func TestBufferTrimTo(t *testing.T) {
	b := NewBuffer()
	for i := 0; i < 10; i++ {
		b.AppendLine(string(rune('a' + i)))
	}
	b.TrimTo(4)
	if b.NumLines() != 4 {
		t.Fatalf("lines=%d", b.NumLines())
	}
	if b.Line(0) != "g" || b.Line(3) != "j" {
		t.Fatalf("got %q %q", b.Line(0), b.Line(3))
	}
	b.TrimTo(0)
	if b.NumLines() != 4 {
		t.Fatal("TrimTo(0) should no-op")
	}
}
