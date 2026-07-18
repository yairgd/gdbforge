package mcp

import "testing"

func TestParseSourceFileList(t *testing.T) {
	raw := `^done,files=[{file="a.c",fullname="/src/a.c"},{file="b.c",fullname="/src/b.c"}]`
	got := ParseSourceFileList(raw)
	if len(got) != 2 || got[0] != "/src/a.c" || got[1] != "/src/b.c" {
		t.Fatalf("got=%v", got)
	}
}
