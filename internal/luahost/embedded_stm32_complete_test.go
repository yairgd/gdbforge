package luahost_test
import (
  "path/filepath"
  "testing"
  luacatalog "github.com/yairgd/gdbforge/lua"
  "github.com/yairgd/gdbforge/internal/luahost"
)
func TestEmbeddedSTM32Complete(t *testing.T) {
  cache, err := luahost.MaterializeEmbeddedLua(luacatalog.FS)
  if err != nil { t.Fatal(err) }
  path := filepath.Join(cache, "stm32", "stm32-stlink", "stm32-stlink.lua")
  rt := luahost.New(nil, nil)
  defer rt.Close()
  if err := rt.LoadScriptFile(path, "stm32-stlink"); err != nil {
    t.Fatalf("load: %v", err)
  }
  out, ok := rt.CompleteScriptArgs(1, "", nil)
  t.Logf("ok=%v n=%d sample=%v", ok, len(out), out)
  if !ok || len(out) < 3 { t.Fatalf("bad completion") }
}
