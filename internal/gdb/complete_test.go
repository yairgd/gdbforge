package gdb

import (
	"reflect"
	"testing"
)

func TestParseCompleteResultMatches(t *testing.T) {
	raw := "-complete br\r\n" +
		`^done,completion="break",matches=["break","break-range"],max_completions_reached="0"` + "\r\n(gdb) \r\n"
	got := ParseCompleteResult(raw)
	if got.Completion != "break" {
		t.Fatalf("completion=%q", got.Completion)
	}
	want := []string{"break", "break-range"}
	if !reflect.DeepEqual(got.Matches, want) {
		t.Fatalf("matches=%v want %v", got.Matches, want)
	}
}

func TestParseCompleteResultEmptyMatches(t *testing.T) {
	raw := `^done,matches=[],max_completions_reached="0"` + "\n(gdb)\n"
	got := ParseCompleteResult(raw)
	if len(got.Matches) != 0 {
		t.Fatalf("matches=%v", got.Matches)
	}
}

func TestParseCompleteResultMultiWord(t *testing.T) {
	raw := `^done,completion="info breakpoints",matches=["info breakpoints"],max_completions_reached="0"` + "\n"
	got := ParseCompleteResult(raw)
	if got.Completion != "info breakpoints" {
		t.Fatalf("completion=%q", got.Completion)
	}
	if len(got.Matches) != 1 || got.Matches[0] != "info breakpoints" {
		t.Fatalf("matches=%v", got.Matches)
	}
}

func TestQuoteCompleteArg(t *testing.T) {
	if got := QuoteCompleteArg("break"); got != "break" {
		t.Fatalf("got %q", got)
	}
	if got := QuoteCompleteArg("info b"); got != `"info b"` {
		t.Fatalf("got %q", got)
	}
	if got := QuoteCompleteArg(`a"b`); got != `"a\"b"` {
		t.Fatalf("got %q", got)
	}
}

func TestMenuNamesStripsPriorWords(t *testing.T) {
	matches := []string{"delete bookmark", "delete breakpoints", "delete display"}
	got := MenuNames("delete ", matches)
	want := []string{"bookmark", "breakpoints", "display"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = MenuNames("info b", []string{"info bookmarks", "info breakpoints"})
	want = []string{"bookmarks", "breakpoints"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partial word: got %v want %v", got, want)
	}
	got = MenuNames("br", []string{"break", "break-range"})
	want = []string{"break", "break-range"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("first word: got %v want %v", got, want)
	}
}

func TestApplyMenuChoice(t *testing.T) {
	if got := ApplyMenuChoice("delete ", "bookmark"); got != "delete bookmark" {
		t.Fatalf("got %q", got)
	}
	if got := ApplyMenuChoice("info b", "breakpoints"); got != "info breakpoints" {
		t.Fatalf("got %q", got)
	}
	if got := ApplyMenuChoice("br", "break"); got != "break" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMIListFieldEscapes(t *testing.T) {
	line := `^done,matches=["a\"b","c\\d"]`
	got := ExtractMIListField(line, "matches")
	want := []string{`a"b`, `c\d`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
