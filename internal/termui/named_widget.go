package termui

// NamedWidget is implemented by workspace panes that have a display name for
// the per-pane status line.
type NamedWidget interface {
	Widget
	WindowName() string
}
