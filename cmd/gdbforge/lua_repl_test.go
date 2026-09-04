package main

import "testing"

func TestSanitizeReplLine(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"print(1)", "print(1)"},
		{"lua>", ""},
		{"lua> ", ""},
		{"lua> lua> ", ""},
		{"lua> print(1)", "print(1)"},
		{"  lua>  x  ", "x"},
	}
	for _, tc := range tests {
		if got := sanitizeReplLine(tc.in); got != tc.want {
			t.Errorf("sanitizeReplLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
