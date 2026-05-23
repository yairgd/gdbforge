package termui

type Cell struct {
	Up    bool
	Down  bool
	Left  bool
	Right bool
	Rune  rune
}

func NewCell() Cell {
	return Cell{
		Up:    false,
		Down:  false,
		Left:  false,
		Right: false,
	}
}

/*
Convert edge combination to Unicode rune.
*/
func (c *Cell) EdgesToRune() {

	switch {

	// Cross
	case c.Up && c.Down && c.Left && c.Right:
		c.Rune = '┼'

	// T junctions
	case c.Up && c.Down && c.Right && !c.Left:
		c.Rune = '├'

	case c.Up && c.Down && c.Left && !c.Right:
		c.Rune = '┤'

	case c.Left && c.Right && c.Down && !c.Up:
		c.Rune = '┬'

	case c.Left && c.Right && c.Up && !c.Down:
		c.Rune = '┴'

	// Corners
	case c.Left && c.Down && !c.Up && !c.Right:
		c.Rune = '┐'

	case c.Right && c.Down && !c.Up && !c.Left:
		c.Rune = '┌'

	case c.Right && c.Up && !c.Down && !c.Left:
		c.Rune = '└'

	case c.Left && c.Up && !c.Down && !c.Right:
		c.Rune = '┘'

	// Simple lines
	case c.Up && c.Down && !c.Left && !c.Right:
		c.Rune = '│'

	case c.Left && c.Right && !c.Up && !c.Down:
		c.Rune = '─'

	default:
		c.Rune = ' '
	}
}
