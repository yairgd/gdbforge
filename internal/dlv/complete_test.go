package dlv

import "testing"

func TestCompleteCommands(t *testing.T) {
	res := CompleteCommands("br")
	if res.Completion != "break" && !hasMatch(res.Matches, "break") {
		t.Fatalf("br: %+v", res)
	}
	res = CompleteCommands("s")
	if len(res.Matches) < 2 {
		t.Fatalf("s should be ambiguous: %+v", res)
	}
	res = CompleteCommands("locals")
	if len(res.Matches) != 1 || res.Matches[0] != "locals" {
		t.Fatalf("locals: %+v", res)
	}
	res = CompleteCommands("b")
	if !hasMatch(res.Matches, "b") && !hasMatch(res.Matches, "break") {
		t.Fatalf("b: %+v", res)
	}
	// Second word: command completer alone returns empty.
	res = CompleteCommands("break main")
	if len(res.Matches) != 0 {
		t.Fatalf("expected no matches for args: %+v", res)
	}
}

func TestParseFuncs(t *testing.T) {
	raw := "main.main\nmain.init\nruntime.main\n(dlv)\n"
	got := ParseFuncs(raw)
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
	if got[0] != "main.init" || got[1] != "main.main" {
		t.Fatalf("sorted: %v", got)
	}
}

func TestFuncsRegex(t *testing.T) {
	if got := funcsRegex("main."); got != `^main\.` {
		t.Fatalf("got %q", got)
	}
}

func TestLooksLikeFileLine(t *testing.T) {
	if !looksLikeFileLine("main.go:12") {
		t.Fatal("file:line")
	}
	if !looksLikeFileLine("main.go:") {
		t.Fatal("file:")
	}
	if looksLikeFileLine("main.main") {
		t.Fatal("package.func is not file:line")
	}
	if looksLikeFileLine("main.") {
		t.Fatal("main. is package prefix")
	}
}

func TestFilterPrefix(t *testing.T) {
	names := []string{"main.init", "main.main", "runtime.main"}
	got := filterPrefix(names, "main.")
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func hasMatch(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
