package termui

// MouseOriginLeaf maps screen-space mouse coordinates to pane-local space.
// WidgetTree calls this before delivering mouse events so hit testing works
// even when Paint has not run yet this frame.
type MouseOriginLeaf interface {
	SetMouseOrigin(screenX, screenY int)
}
