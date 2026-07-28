package main

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestIsCtrlZ(t *testing.T) {
	cases := []struct {
		name string
		ev   *tcell.EventKey
		want bool
	}{
		{"nil", nil, false},
		{"KeyCtrlZ", tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone), true},
		{"Key26", tcell.NewEventKey(tcell.Key(0x1a), 0, tcell.ModNone), true},
		{"runeSUB", tcell.NewEventKey(tcell.KeyRune, 0x1a, tcell.ModNone), true},
		{"ModCtrlZ", tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModCtrl), true},
		{"plainZ", tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone), false},
		{"CtrlC", tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone), false},
	}
	for _, tc := range cases {
		if got := isCtrlZ(tc.ev); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
