package widgets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yairgd/cgdb-go/internal/termui"
)

func TestCodeWidgetShowLocationMarksPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	src := "int main(void) {\n  return 0;\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	w := NewCodeWidget()
	if err := w.ShowLocation(path, 2); err != nil {
		t.Fatal(err)
	}
	lines := w.LinesForTest()
	if len(lines) < 2 {
		t.Fatalf("lines=%v", lines)
	}
	plain1 := termui.StripANSI(lines[1])
	if !strings.Contains(plain1, "━━▶") {
		t.Fatalf("want ━━▶ on line 2, got %q", plain1)
	}
	if !strings.Contains(plain1, "│") {
		t.Fatalf("want box-drawing │ gutter, got %q", plain1)
	}
	plain0 := termui.StripANSI(lines[0])
	if !strings.Contains(plain0, "1") {
		t.Fatalf("want line 1 gutter, got %q", plain0)
	}
}
