package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestVisibleANSIWidthStripsControls(t *testing.T) {
	in := "\x1b]0;title\x07\x1b[?2004h\x1b[01;32muser\x1b[01;34m ~/path $\x1b[00m \x1b[?2004l"
	got := VisibleANSIWidth(in)
	want := VisibleANSIWidth("user ~/path $ ")
	if got != want {
		t.Fatalf("VisibleANSIWidth = %d, want %d", got, want)
	}
	if got != len([]rune("user ~/path $ ")) {
		t.Fatalf("visible runes = %d, want %d", got, len([]rune("user ~/path $ ")))
	}
}

func TestConsumeEscapeSGR(t *testing.T) {
	base := tcell.StyleDefault
	style := base
	text := "\x1b[01;32mhi\x1b[0m"
	next, style, ok := consumeEscape(text, 0, style, base)
	if !ok || next != 8 {
		t.Fatalf("SGR consume next=%d ok=%v", next, ok)
	}
	fg, _, _ := style.Decompose()
	if fg != tcell.ColorGreen {
		t.Fatalf("fg = %v, want ColorGreen", fg)
	}
}

func TestConsumeEscapeOSC(t *testing.T) {
	text := "\x1b]0;yair@localhost cgdb-go\x07rest"
	next, _, ok := consumeEscape(text, 0, tcell.StyleDefault, tcell.StyleDefault)
	if !ok {
		t.Fatal("expected OSC consumed")
	}
	if text[next:] != "rest" {
		t.Fatalf("remaining = %q", text[next:])
	}
}

func TestConsumeEscapePrivateMode(t *testing.T) {
	text := "\x1b[?2004h>"
	next, _, ok := consumeEscape(text, 0, tcell.StyleDefault, tcell.StyleDefault)
	if !ok || text[next:] != ">" {
		t.Fatalf("next remaining %q ok=%v", text[next:], ok)
	}
}
