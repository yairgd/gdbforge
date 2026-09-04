package termui

import "unicode/utf8"

// VisibleColAtByte maps a byte offset to a visible column (rune index).
func VisibleColAtByte(text string, byteIdx int) int {
	if byteIdx <= 0 {
		return 0
	}
	if byteIdx > len(text) {
		byteIdx = len(text)
	}
	return utf8.RuneCountInString(text[:byteIdx])
}

// ByteIndexAtVisibleCol maps a visible column to a byte offset in text.
func ByteIndexAtVisibleCol(text string, visCol int) int {
	if visCol <= 0 {
		return 0
	}
	i := 0
	for col := 0; col < visCol && i < len(text); col++ {
		_, size := utf8.DecodeRuneInString(text[i:])
		if size <= 0 {
			break
		}
		i += size
	}
	return i
}
