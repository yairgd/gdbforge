package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Cell struct {
	Up    bool
	Down  bool
	Left  bool
	Right bool
	Rune  rune
	Bold  bool
	Style tcell.Style
}

func NewCell() Cell {
	return Cell{
		Up:    false,
		Down:  false,
		Left:  false,
		Right: false,
	}
}

func (c *Cell) IsBorder() bool {
	return c.Up || c.Down || c.Left || c.Right
}

// The ideal solution is to use mixed-weight corner/separator glyphs,
// but this becomes significantly more complex at this stage.
//
// For example, we would need to know whether the bold window
// is connected from the top, bottom, left, or right side.
func (c *Cell) EdgesToRune() {

	switch {

	// Cross
	case c.Up && c.Down && c.Left && c.Right:

		if c.Bold {
			c.Rune = '╋'
		} else {
			c.Rune = '┼'
		}

	// T junctions
	case c.Up && c.Down && c.Right && !c.Left:

		if c.Bold {
			c.Rune = '┣'
		} else {
			c.Rune = '├'
		}

	case c.Up && c.Down && c.Left && !c.Right:

		if c.Bold {
			c.Rune = '┫'
		} else {
			c.Rune = '┤'
		}

	case c.Left && c.Right && c.Down && !c.Up:

		if c.Bold {
			c.Rune = '┳'
		} else {
			c.Rune = '┬'
		}

	case c.Left && c.Right && c.Up && !c.Down:

		if c.Bold {
			c.Rune = '┻'
		} else {
			c.Rune = '┴'
		}

	// Corners
	case c.Left && c.Down && !c.Up && !c.Right:

		if c.Bold {
			c.Rune = '┓'
		} else {
			c.Rune = '┐'
		}

	case c.Right && c.Down && !c.Up && !c.Left:

		if c.Bold {
			c.Rune = '┏'
		} else {
			c.Rune = '┌'
		}

	case c.Right && c.Up && !c.Down && !c.Left:

		if c.Bold {
			c.Rune = '┗'
		} else {
			c.Rune = '└'
		}

	case c.Left && c.Up && !c.Down && !c.Right:

		if c.Bold {
			c.Rune = '┛'
		} else {
			c.Rune = '┘'
		}

	// Simple lines
	case c.Up && c.Down && !c.Left && !c.Right:

		if c.Bold {
			c.Rune = '┃'
		} else {
			c.Rune = '│'
		}

	case c.Left && c.Right && !c.Up && !c.Down:

		if c.Bold {
			c.Rune = '━'
		} else {
			c.Rune = '─'
		}

	default:
		c.Rune = ' '
	}
}
