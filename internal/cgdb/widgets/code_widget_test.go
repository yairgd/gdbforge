package widgets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if !strings.HasPrefix(lines[1], "-->") {
		t.Fatalf("want --> on line 2, got %q", lines[1])
	}
	if !strings.HasPrefix(lines[0], "   ") {
		t.Fatalf("want blank gutter on line 1, got %q", lines[0])
	}
}
