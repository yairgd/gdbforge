package persist

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSaveLoadCmdlineHistory(t *testing.T) {
	dir := t.TempDir()
	cmds := []string{":b io", ":lua remotegdb help"}
	search := []string{"/foo", "/bar"}
	if err := SaveCmdlineHistory(dir, cmds, search); err != nil {
		t.Fatal(err)
	}
	path := CmdlineHistoryPath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	gotCmds, gotSearch, err := LoadCmdlineHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotCmds) != 2 || gotCmds[0] != ":b io" || gotCmds[1] != ":lua remotegdb help" {
		t.Fatalf("commands=%v", gotCmds)
	}
	if len(gotSearch) != 2 || gotSearch[0] != "/foo" || gotSearch[1] != "/bar" {
		t.Fatalf("search=%v", gotSearch)
	}
}

func TestSaveCmdlineHistoryEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := SaveCmdlineHistory(dir, nil, nil); err != nil {
		t.Fatal(err)
	}
	cmds, search, err := LoadCmdlineHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 0 || len(search) != 0 {
		t.Fatalf("cmds=%v search=%v", cmds, search)
	}
	raw, err := os.ReadFile(CmdlineHistoryPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "commands:") || !strings.Contains(s, "search:") {
		t.Fatalf("raw=%q", s)
	}
}

func TestLoadCmdlineHistoryMissing(t *testing.T) {
	cmds, search, err := LoadCmdlineHistory(t.TempDir())
	if err != nil || cmds != nil || search != nil {
		t.Fatalf("cmds=%v search=%v err=%v", cmds, search, err)
	}
}

func TestCmdlineHistoryCap(t *testing.T) {
	dir := t.TempDir()
	n := CmdlineHistoryMax + 50
	cmds := make([]string, n)
	for i := range cmds {
		cmds[i] = ":c" + strconv.Itoa(i)
	}
	if err := SaveCmdlineHistory(dir, cmds, nil); err != nil {
		t.Fatal(err)
	}
	got, _, err := LoadCmdlineHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != CmdlineHistoryMax {
		t.Fatalf("len=%d want %d", len(got), CmdlineHistoryMax)
	}
	wantFirst := ":c" + strconv.Itoa(n-CmdlineHistoryMax)
	wantLast := ":c" + strconv.Itoa(n-1)
	if got[0] != wantFirst || got[len(got)-1] != wantLast {
		t.Fatalf("got first=%q last=%q want %q … %q", got[0], got[len(got)-1], wantFirst, wantLast)
	}
}

func TestCmdlineHistoryPath(t *testing.T) {
	want := filepath.Join("build", DirName, CmdlineHistoryFile)
	if got := CmdlineHistoryPath("build"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSaveCmdlineHistorySkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := SaveCmdlineHistory(dir, []string{":a", "", ":b"}, []string{"", "/x"}); err != nil {
		t.Fatal(err)
	}
	cmds, search, err := LoadCmdlineHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 2 || cmds[0] != ":a" || cmds[1] != ":b" {
		t.Fatalf("cmds=%v", cmds)
	}
	if len(search) != 1 || search[0] != "/x" {
		t.Fatalf("search=%v", search)
	}
}

func TestSaveCmdlineHistoryConsecutiveDedupe(t *testing.T) {
	dir := t.TempDir()
	if err := SaveCmdlineHistory(dir, []string{":q", ":q", ":clear", ":q"}, nil); err != nil {
		t.Fatal(err)
	}
	cmds, _, err := LoadCmdlineHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 3 || cmds[0] != ":q" || cmds[1] != ":clear" || cmds[2] != ":q" {
		t.Fatalf("cmds=%v", cmds)
	}
}
