package termui

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestCenterClampsTopToViewport(t *testing.T) {
	buf := platform.NewBuffer()
	for i := 0; i < 5; i++ {
		buf.AppendLine("x")
	}
	v := NewScrollDocument(buf)
	v.Center(4, 20)
	if v.Top != 0 {
		t.Fatalf("Top=%d want 0 for short buffer", v.Top)
	}
}
