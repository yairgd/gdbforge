package models

import "testing"

func TestGuttersByLine(t *testing.T) {
	got := GuttersByLine([]BreakInfo{
		{Number: 1, Enabled: true, Line: 10},
		{Number: 2, Enabled: false, Line: 10},
		{Number: 3, Enabled: true, Condition: "i==1", Line: 20},
		{Number: 4, Enabled: true, Line: 20},
	})
	g10 := got[10]
	if !g10.Enabled || g10.Conditional() || len(g10.Numbers) != 2 {
		t.Fatalf("line10=%+v", g10)
	}
	g20 := got[20]
	if !g20.Enabled || g20.Condition != "i==1" || len(g20.Numbers) != 2 {
		t.Fatalf("line20=%+v", g20)
	}
}

func TestGuttersByAddr(t *testing.T) {
	got := GuttersByAddr([]BreakInfo{
		{Number: 1, Enabled: true, Addr: "0x401126", Condition: "x>0"},
		{Number: 2, Enabled: false, Addr: "0x0000000000401126"},
	})
	g := got["0x401126"]
	if !g.Enabled || g.Condition != "x>0" || len(g.Numbers) != 2 {
		t.Fatalf("gutter=%+v keys=%v", g, got)
	}
}

func TestSameSourceLoc(t *testing.T) {
	if !SameSourceLoc("/a/b.c", 3, "b.c", 3) {
		t.Fatal("basename")
	}
	if SameSourceLoc("/a/b.c", 3, "b.c", 4) {
		t.Fatal("line")
	}
}

func TestNormalizeAddr(t *testing.T) {
	if NormalizeAddr("0x0000000000401126") != "0x401126" {
		t.Fatal(NormalizeAddr("0x0000000000401126"))
	}
}
