package app

import "testing"

func TestParseFlagsConnect(t *testing.T) {
	cfg, err := ParseFlags([]string{"-g", "dlv", "-connect", "192.168.20.50:2345"})
	if err != nil || cfg.Connect != "192.168.20.50:2345" || len(cfg.GDBArgs) != 0 {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	if _, err := ParseFlags([]string{"-connect", ":2345"}); err == nil {
		t.Fatal("-connect without -g dlv must fail")
	}
}
