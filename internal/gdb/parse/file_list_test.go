package parse

import "testing"

func TestParseSourceFileList(t *testing.T) {
	raw := `^done,files=[{file="a.c",fullname="/src/a.c"},{file="b.c",fullname="/src/b.c"}]`
	got := ParseSourceFileList(raw)
	if len(got) != 2 || got[0] != "/src/a.c" || got[1] != "/src/b.c" {
		t.Fatalf("got=%v", got)
	}
}

func TestParseSourceFileListIgnoresStoppedFrame(t *testing.T) {
	// A shared PTY capture may include *stopped before ^done — must not
	// treat the frame fullname as the whole project file list.
	raw := `*stopped,frame={fullname="/proj/main.cpp",line="1"}
^done,files=[{file="main.cpp",fullname="/proj/main.cpp"},{file="util.hpp",fullname="/proj/util.hpp"},{file="util.cpp",fullname="/proj/util.cpp"}]
(gdb) `
	got := ParseSourceFileList(raw)
	if len(got) != 3 {
		t.Fatalf("got=%v want 3 paths", got)
	}
	if got[0] != "/proj/main.cpp" || got[1] != "/proj/util.hpp" || got[2] != "/proj/util.cpp" {
		t.Fatalf("got=%v", got)
	}
}

func TestParseSourceFileListFallsBackToFile(t *testing.T) {
	raw := `^done,files=[{file="only.c"}]`
	got := ParseSourceFileList(raw)
	if len(got) != 1 || got[0] != "only.c" {
		t.Fatalf("got=%v", got)
	}
}

func TestParseSourceFileListNoFilesKey(t *testing.T) {
	raw := `*stopped,frame={fullname="/proj/main.cpp",line="1"}`
	if got := ParseSourceFileList(raw); len(got) != 0 {
		t.Fatalf("got=%v want empty", got)
	}
}
