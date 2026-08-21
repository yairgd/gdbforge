package luahost_test

import (
	"path/filepath"
	"testing"

	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestSTM32CompleteBoardThenProfile(t *testing.T) {
	path, _ := filepath.Abs("../../lua/stm32/stm32-stlink/stm32-stlink.lua")
	rt := luahost.New(nil, nil)
	defer rt.Close()
	if err := rt.LoadScriptFile(path, "stm32-stlink"); err != nil {
		t.Fatal(err)
	}
	out, ok := rt.CompleteScriptArgs(1, "nucleo_f429zi", nil)
	t.Logf("arg1 board token ok=%v out=%v", ok, out)
	if !ok || len(out) == 0 {
		t.Fatal("expected profiles after full board at arg1")
	}
}
