package termui

import (
	"testing"
	"time"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestWordBoundsAt(t *testing.T) {
	line := "foo_bar-baz 42"
	// click on 'b' of foo_bar
	s, e := wordBoundsAt(line, 4)
	if line[s:e] != "foo_bar" {
		t.Fatalf("got %q want foo_bar", line[s:e])
	}
	// click on '-'
	s, e = wordBoundsAt(line, 7)
	if line[s:e] != "-" {
		t.Fatalf("got %q want -", line[s:e])
	}
	// click on space → empty
	s, e = wordBoundsAt(line, 11)
	if s != e {
		t.Fatalf("space should be empty, got %q", line[s:e])
	}
}

func TestDoubleClickSelectsWordAndCopies(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("path/to/file.c:42")
	v := NewScrollDocument(buf)
	v.width = 40
	v.height = 5

	var got string
	v.SetClipboard(ClipboardIO{Copy: func(s string) { got = s }})

	now := time.Now()
	pos := bufferPos{line: 0, col: 10} // inside "file"
	v.noteClick(pos, now)
	v.noteClick(pos, now.Add(50*time.Millisecond))
	if v.clickCount != 2 {
		t.Fatalf("clickCount=%d want 2", v.clickCount)
	}
	if !v.selectWordAt(pos) {
		t.Fatal("selectWordAt failed")
	}
	v.CopySelection()
	if got != "file" {
		t.Fatalf("clipboard=%q want file (selected %q)", got, v.selectedText())
	}
}

func TestSelectWordAt(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("hello_world x")
	v := NewScrollDocument(buf)
	if !v.selectWordAt(bufferPos{line: 0, col: 3}) {
		t.Fatal("selectWordAt failed")
	}
	if got := v.selectedText(); got != "hello_world" {
		t.Fatalf("got %q", got)
	}
}

func TestSelectLineAt(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("entire line here")
	v := NewScrollDocument(buf)
	if !v.selectLineAt(bufferPos{line: 0, col: 5}) {
		t.Fatal("selectLineAt failed")
	}
	if got := v.selectedText(); got != "entire line here" {
		t.Fatalf("got %q", got)
	}
}
