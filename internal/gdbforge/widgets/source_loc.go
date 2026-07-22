package widgets

import "path/filepath"

// sameSourceLoc reports whether two file:line pairs refer to the same source
// location (full path or basename match), matching breakpoint list lookup.
func sameSourceLoc(aFile string, aLine int, bFile string, bLine int) bool {
	if aLine == 0 || bLine == 0 || aLine != bLine {
		return false
	}
	if aFile == "" || bFile == "" {
		return false
	}
	if aFile == bFile {
		return true
	}
	return filepath.Base(aFile) == filepath.Base(bFile)
}
