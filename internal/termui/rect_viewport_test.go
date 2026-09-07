package termui

import "testing"

func TestRectViewportClamp(t *testing.T) {
	rv := NewRectViewport()
	rv.SetContentSize(100, 50)
	rv.SetOrigin(200, 200)
	rv.Clamp(20, 10)
	if rv.Origin.X != 80 || rv.Origin.Y != 40 {
		t.Fatalf("origin=(%d,%d) want (80,40)", rv.Origin.X, rv.Origin.Y)
	}

	rv.SetContentSize(10, 5)
	rv.SetOrigin(3, 2)
	rv.Clamp(20, 10)
	if rv.Origin.X != 0 || rv.Origin.Y != 0 {
		t.Fatalf("small content origin=(%d,%d) want (0,0)", rv.Origin.X, rv.Origin.Y)
	}
}

func TestRectViewportPanAndScrollEnd(t *testing.T) {
	rv := NewRectViewport()
	rv.SetContentSize(30, 20)
	rv.SetOrigin(10, 0)
	rv.ScrollEnd(8)
	if rv.Origin.Y != 12 {
		t.Fatalf("ScrollEnd y=%d want 12", rv.Origin.Y)
	}
	rv.Pan(5, -1)
	rv.Clamp(10, 8)
	if rv.Origin.X != 15 || rv.Origin.Y != 11 {
		t.Fatalf("after pan origin=(%d,%d) want (15,11)", rv.Origin.X, rv.Origin.Y)
	}
}

func TestRectViewportVisibleContentRect(t *testing.T) {
	rv := NewRectViewport()
	rv.SetContentSize(5, 4)
	rv.SetOrigin(2, 1)
	x, y, w, h := rv.VisibleContentRect(10, 10)
	if x != 2 || y != 1 || w != 3 || h != 3 {
		t.Fatalf("visible=(%d,%d,%d,%d) want (2,1,3,3)", x, y, w, h)
	}
}

func TestRectViewportEnsureContentVisible(t *testing.T) {
	rv := NewRectViewport()
	rv.SetContentSize(40, 40)
	rv.SetOrigin(0, 0)
	rv.EnsureContentVisible(15, 20, 10, 10)
	if rv.Origin.X != 6 || rv.Origin.Y != 11 {
		t.Fatalf("origin=(%d,%d) want (6,11)", rv.Origin.X, rv.Origin.Y)
	}
}
