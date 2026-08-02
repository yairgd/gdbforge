package widgets

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestAdoptLuaWidgetBindsPane(t *testing.T) {
	rt := luahost.New(nil, nil)
	defer rt.Close()
	if err := rt.LoadString(`function on_key(k) end`, "t"); err != nil {
		t.Fatal(err)
	}
	w := AdoptLuaWidget("g1", rt)
	if w.Runtime() != rt {
		t.Fatal("widget should own adopted runtime")
	}
	if rt.Pane() != w {
		t.Fatal("runtime pane should be the widget")
	}
	if w.PaneName != "g1" {
		t.Fatalf("PaneName=%q", w.PaneName)
	}
}
