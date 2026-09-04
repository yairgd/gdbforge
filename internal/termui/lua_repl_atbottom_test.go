package termui

import "testing"

func TestLuaReplAtBottomAfterPrompt(t *testing.T) {
	c := NewCompositeTerminalWithPrefix(80, 24, 100, "")
	defer c.Close()
	ctl := c.Controller()
	ctl.SetInputHandler(func(b []byte) error { return ctl.Write(b) })
	RewritePromptInput(ctl, "lua> ", "")
	if !c.AtBottom() {
		t.Fatal("expected at bottom after prompt")
	}
	_ = ctl.SendString("help")
	if !c.AtBottom() {
		t.Fatal("expected at bottom after typing help")
	}
}

func TestLuaReplAtBottomAfterLongOutput(t *testing.T) {
	c := NewCompositeTerminalWithPrefix(80, 10, 100, "")
	defer c.Close()
	ctl := c.Controller()
	ctl.SetInputHandler(func(b []byte) error { return ctl.Write(b) })
	for i := 0; i < 50; i++ {
		c.WriteRaw("line of help output\r\n")
	}
	RewritePromptInput(ctl, "lua> ", "")
	if !c.AtBottom() {
		t.Fatalf("expected at bottom after output ydisp check")
	}
	_ = ctl.SendString("help")
	if !c.AtBottom() {
		t.Fatal("expected at bottom after typing help on long scrollback")
	}
}
