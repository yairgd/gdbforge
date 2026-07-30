package widgets

import "github.com/yairgd/gdbforge/internal/gdbforge/models"

// SameSourceLoc reports whether two file:line pairs refer to the same source
// location (full path or basename match), matching breakpoint list lookup.
func SameSourceLoc(aFile string, aLine int, bFile string, bLine int) bool {
	return models.SameSourceLoc(aFile, aLine, bFile, bLine)
}

// sameSourceLoc is the package-local alias used by list widgets.
func sameSourceLoc(aFile string, aLine int, bFile string, bLine int) bool {
	return models.SameSourceLoc(aFile, aLine, bFile, bLine)
}
