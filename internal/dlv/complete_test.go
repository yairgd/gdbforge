package dlv

import (
	"context"
	"testing"

	"github.com/yairgd/gdbforge/internal/core"
)

// fakeSession records any PTY write. Completion must stay off the CLI PTY: it
// carries the user's half-typed line, so a `funcs …` query would be appended
// to it (e.g. "b main." + Tab producing "b main.funcs ^main\.").
type fakeSession struct {
	funcs   []string
	written []string
}

func (f *fakeSession) Send(cmd string) error    { f.written = append(f.written, cmd); return nil }
func (f *fakeSession) SendRaw(raw string) error { f.written = append(f.written, raw); return nil }
func (f *fakeSession) Close()                   {}

func (f *fakeSession) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	ch := make(chan core.PtyOutputMsg)
	return ch, func() {}
}

func (f *fakeSession) WithWrite(_ context.Context, fn func(w core.PTYWriter) error) error {
	return fn(f)
}

func (f *fakeSession) ListFunctionsFilter(string) ([]string, error) { return f.funcs, nil }

func TestCompleteLocspecUsesRPCNotPTY(t *testing.T) {
	sess := &fakeSession{funcs: []string{"main.main", "main.init", "runtime.main"}}
	res := Complete(sess, nil, "b main.")
	if len(sess.written) != 0 {
		t.Fatalf("completion wrote to the CLI PTY: %q", sess.written)
	}
	if !hasMatch(res.Matches, "b main.init") || !hasMatch(res.Matches, "b main.main") {
		t.Fatalf("matches: %+v", res.Matches)
	}
	if hasMatch(res.Matches, "b runtime.main") {
		t.Fatalf("prefix filter leaked: %+v", res.Matches)
	}
	if res.Completion != "b main." {
		t.Fatalf("completion: %q", res.Completion)
	}
}

func TestCompleteLocspecWithoutRPC(t *testing.T) {
	// No FuncLister (e.g. CLI-only session): return nothing rather than
	// corrupting the input line.
	res := Complete(nil, nil, "b main.")
	if len(res.Matches) != 0 || res.Completion != "" {
		t.Fatalf("expected empty result: %+v", res)
	}
}

func TestCleanFuncNames(t *testing.T) {
	got := CleanFuncNames([]string{"main.main", "", "main.init", "main.main", "not a name"})
	if len(got) != 2 || got[0] != "main.init" || got[1] != "main.main" {
		t.Fatalf("got %v", got)
	}
}

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
