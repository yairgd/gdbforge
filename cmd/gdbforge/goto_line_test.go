package main

import "testing"

func TestParseGotoLineCmd(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{":42", 42, true},
		{"42", 42, true},
		{":0", 1, true},
		{"0", 1, true},
		{": 7", 7, true},
		{":12 ", 12, true},
		{":b", 0, false},
		{":edit", 0, false},
		{"", 0, false},
		{":", 0, false},
		{"12x", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseGotoLineCmd(tc.in)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("%q → (%d,%v) want (%d,%v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}
