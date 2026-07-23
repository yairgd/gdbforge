package main

import "testing"

func TestWantsVersion(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"-version"}, true},
		{[]string{"--version"}, true},
		{[]string{"./hello"}, false},
		{[]string{"--", "-version"}, false},
		{[]string{"-g", "gdb", "-version"}, true},
	}
	for _, tc := range cases {
		if got := wantsVersion(tc.args); got != tc.want {
			t.Fatalf("wantsVersion(%v)=%v want %v", tc.args, got, tc.want)
		}
	}
}
