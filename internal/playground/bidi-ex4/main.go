package main

import (
	"fmt"
	"unicode"
)

func isRTL(r rune) bool {
	return unicode.Is(unicode.Hebrew, r) || unicode.Is(unicode.Arabic, r)
}

func VisualString(s string) string {
	var result []rune
	var segment []rune
	var rtl bool

	flush := func() {
		if len(segment) == 0 {
			return
		}

		if rtl {
			for i := len(segment) - 1; i >= 0; i-- {
				result = append(result, segment[i])
			}
		} else {
			result = append(result, segment...)
		}

		segment = nil
	}

	for _, r := range s {
		dir := isRTL(r)

		if len(segment) == 0 {
			rtl = dir
		}

		if dir != rtl {
			flush()
			rtl = dir
		}

		segment = append(segment, r)
	}

	flush()

	return string(result)
}

func main() {

	tests := []string{
		"hello world",
		"שלום עולם",
		"hello שלום",
		"abc שלום 123",
		"123 שלום abc",
		"mix עברית and English 456",
	}

	fmt.Println("==== BIDI DEMO ====\n")

	for _, t := range tests {
		fmt.Println("Original:", t)
		fmt.Println("Visual:  ", VisualString(t))
		fmt.Println()
	}
}
