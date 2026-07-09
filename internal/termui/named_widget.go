package termui

// NamedWidget is implemented by workspace panes that have a display name for
// the per-pane status line shown when the pane has focus.
type NamedWidget interface {
	Widget
	WindowName() string
}
