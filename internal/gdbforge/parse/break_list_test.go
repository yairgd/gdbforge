package parse

import "testing"

func TestParseBreakList(t *testing.T) {
	raw := `^done,BreakpointTable={body=[bkpt={number="1",enabled="y",fullname="/tmp/hello.c",line="22"},bkpt={number="2",enabled="n",fullname="/tmp/hello.c",line="10"},bkpt={number="3",enabled="y",file="util.c",line="5"}]}`
	got := ParseBreakList(raw)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	if got[0].Number != 1 || !got[0].Enabled || got[0].Line != 22 {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].Number != 2 || got[1].Enabled {
		t.Fatalf("second should be disabled: %+v", got[1])
	}
	locs := EnabledBreakMarks(got)
	if len(locs) != 2 {
		t.Fatalf("enabled locs=%v", locs)
	}
}

func TestParseBreakListAddrOnly(t *testing.T) {
	raw := `^done,BreakpointTable={body=[bkpt={number="4",enabled="y",addr="0x0000000000401126",at="<main+22>"}]}`
	got := ParseBreakList(raw)
	if len(got) != 1 {
		t.Fatalf("got %d, want 1: %+v", len(got), got)
	}
	if got[0].Number != 4 || got[0].Addr != "0x401126" || got[0].File != "" {
		t.Fatalf("addr-only=%+v", got[0])
	}
}
