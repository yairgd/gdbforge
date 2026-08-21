package luahost_test

import (
	"path/filepath"
	"testing"

	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestLoadSTM32StlinkFromGdbforgeDir(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".gdbforge", "lua", "stm32-stlink", "stm32-stlink.lua")
	rt := luahost.New(nil, nil)
	defer rt.Close()
	if err := rt.LoadScriptFile(path, "stm32-stlink"); err != nil {
		t.Fatalf("load: %v", err)
	}
	out, ok := rt.CompleteScriptArgs(1, "", nil)
	if !ok {
		t.Fatal("CompleteScriptArgs not registered")
	}
	if len(out) == 0 {
		t.Fatal("empty completion list")
	}
	t.Logf("got %d candidates, sample: %v", len(out), out[:min(5, len(out))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
