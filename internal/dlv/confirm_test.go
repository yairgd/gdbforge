package dlv

import "testing"

func TestLooksLikeConfirmPrompt(t *testing.T) {
	kind, ok := LooksLikeConfirmPrompt("Set a suspended breakpoint … [Y/n]?")
	if !ok || kind != ConfirmYesNo {
		t.Fatalf("y/n: ok=%v kind=%v", ok, kind)
	}
	kind, ok = LooksLikeConfirmPrompt("Would you like to [p]ause … [p/q]?")
	if !ok || kind != ConfirmPauseQuit {
		t.Fatalf("p/q: ok=%v kind=%v", ok, kind)
	}
	if _, ok := LooksLikeConfirmPrompt("only p or q allowed"); ok {
		t.Fatal("error line should not match")
	}
}
