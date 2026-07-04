package platform

type TextModel interface {
	NumLines() int
	Line(i int) string
}
