package gdb

import (
	"reflect"
	"testing"
)

func TestCompletingLinespec(t *testing.T) {
	if !CompletingLinespec("break hello.c:") {
		t.Fatal("expected file: linespec")
	}
	if !CompletingLinespec("break hello.c:ma") {
		t.Fatal("expected file:func linespec")
	}
	if CompletingLinespec("break ") {
		t.Fatal("not a linespec yet")
	}
	if CompletingLinespec("info breakpoints") {
		t.Fatal("command completion is not linespec")
	}
	if CompletingLinespec("print NS::foo") {
		t.Fatal("C++ scope must not count as file:")
	}
}

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
	if got := ApplyMenuChoice("break banner.c:f", "func(int x)"); got != "break banner.c:func" {
		t.Fatalf("linespec apply strips sig: got %q", got)
	}
	if got := ApplyMenuChoice("display banner.c:", "main()"); got != "display banner.c:main" {
		t.Fatalf("linespec apply empty after colon: got %q", got)
	}
	if got := ApplyMenuChoice("break f.c:", "int foo(int, char *)"); got != "break f.c:foo" {
		t.Fatalf("linespec apply strips return type: got %q", got)
	}
}

func TestMenuNamesLinespecShowsFuncWithSignature(t *testing.T) {
	matches := []string{
		"break banner.c:func(int x)",
		"break banner.c:func2(char *s)",
	}
	got := MenuNames("break banner.c:f", matches)
	want := []string{"func(int x)", "func2(char *s)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = MenuNames("display banner.c:", []string{"display banner.c:main()"})
	want = []string{"main()"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty after colon: got %v want %v", got, want)
	}
	// C++ scope must not be treated as file:
	got = MenuNames("break NS::", []string{"break NS::foo", "break NS::bar"})
	want = []string{"NS::foo", "NS::bar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cxx scope: got %v want %v", got, want)
	}
	// file.cc:NS::method → show NS::method
	got = MenuNames("break file.cc:N", []string{"break file.cc:NS::method(int)"})
	want = []string{"NS::method(int)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("file + cxx: got %v want %v", got, want)
	}
}

func TestParseSymbolInfoFunctionsAndEnrich(t *testing.T) {
	raw := `^done,symbols={debug=[{filename="/tmp/sigtest.c",fullname="/tmp/sigtest.c",symbols=[{line="2",name="bar",type="void (void)",description="void bar(void);"},{line="1",name="foo",type="int (int, char *)",description="int foo(int, char *);"},{line="3",name="main",type="int (void)",description="int main(void);"}]}]}`
	sigs := ParseSymbolInfoFunctions(raw)
	if sigs["foo"] != "foo(int, char *)" {
		t.Fatalf("foo=%q", sigs["foo"])
	}
	if sigs["bar"] != "bar(void)" {
		t.Fatalf("bar=%q", sigs["bar"])
	}
	if sigs["main"] != "main(void)" {
		t.Fatalf("main=%q", sigs["main"])
	}

	menu := []string{"foo", "bar", "main"}
	got := EnrichLinespecMenuNames("break /tmp/sigtest.c:", menu, sigs)
	want := []string{"foo(int, char *)", "bar(void)", "main(void)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enrich=%v want %v", got, want)
	}
	// Non-linespec: unchanged
	got = EnrichLinespecMenuNames("break f", menu, sigs)
	if !reflect.DeepEqual(got, menu) {
		t.Fatalf("non-linespec enrich=%v", got)
	}
}

func TestLinespecFuncName(t *testing.T) {
	if got := LinespecFuncName("foo(int, char *)"); got != "foo" {
		t.Fatalf("got %q", got)
	}
	if got := LinespecFuncName("int foo(int)"); got != "foo" {
		t.Fatalf("got %q", got)
	}
	if got := LinespecFuncName("main"); got != "main" {
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
