package termui

import (
	"strings"
	"testing"

	xterm "github.com/gitpod-io/xterm-go"
)

func TestTerminalNewlines(t *testing.T) {
	got := TerminalNewlines("a\nb\nc")
	want := "a\r\nb\r\nc"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := TerminalNewlines("a\r\nb"); got != "a\r\nb" {
		t.Fatalf("crlf normalize: %q", got)
	}
	if got := TerminalNewlines(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
}

func TestTerminalNewlinesMultilineErrorAtColumnZero(t *testing.T) {
	c := NewTerminalController(80, 24, 100)
	defer c.Close()

	msg := "<string>:1: attempt to call a non-function object\nstack traceback:\n\t<string>:1: in main chunk\n\t[G]: ?"
	if err := c.WriteString(TerminalNewlines(msg) + "\r\n"); err != nil {
		t.Fatal(err)
	}
	c.WithTerminal(func(term *xterm.Terminal) {
		buf := term.Buffer()
		lineText := func(row int) string {
			line := buf.Lines.Get(buf.YDisp + row)
			if line == nil {
				t.Fatalf("missing row %d", row)
			}
			var b strings.Builder
			cell := xterm.NewCellData()
			for x := 0; x < term.Cols(); x++ {
				line.LoadCell(x, cell)
				chars := cell.GetChars()
				if chars == "" {
					continue
				}
				for _, r := range chars {
					if r == 0 || r == ' ' {
						continue
					}
					b.WriteRune(r)
					break
				}
			}
			return b.String()
		}
		if !strings.HasPrefix(lineText(0), "<string>") {
			t.Fatalf("row0=%q", lineText(0))
		}
		if !strings.HasPrefix(lineText(1), "stack") {
			t.Fatalf("row1=%q want stack...", lineText(1))
		}
		if !strings.Contains(lineText(2), "main") {
			t.Fatalf("row2=%q want main chunk", lineText(2))
		}
		if !strings.Contains(lineText(3), "[G]") {
			t.Fatalf("row3=%q want [G]", lineText(3))
		}
	})
}
