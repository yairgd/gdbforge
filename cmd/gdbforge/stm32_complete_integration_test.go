package main

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestSTM32StlinkScriptCompletion(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "lua", "stm32", "stm32-stlink", "stm32-stlink.lua")
	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	a := &DebuggerApp{}
	a.lua.host = a
	a.lua.pending = map[string]luahost.ResolvedScript{
		"stm32-stlink": {Cmd: "stm32-stlink", Path: path},
	}

	// Arg 1: all boards/MCUs
	got := a.lua.completions("stm32-stlink ", false)
	if len(got) < 10 {
		t.Fatalf("board completion: got %d %v", len(got), got)
	}
	for _, want := range []string{"nucleo_f429zi", "f429zi", "stm32f405"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("board list missing %q in %v", want, got)
		}
	}

	// Arg 2: profiles after board + space
	got = a.lua.completions("stm32-stlink nucleo_f429zi ", false)
	for _, want := range []string{"baremetal", "zephyr", "freertos"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("profile list missing %q in %v", want, got)
		}
	}

	// Arg 2: profile prefix rtos → freertos
	got = a.lua.completions("stm32-stlink nucleo_f429zi rtos", false)
	if !contains(got, "freertos") {
		t.Fatalf("rtos prefix: %v want freertos", got)
	}

	// Arg 2: z → zephyr
	got = a.lua.completions("stm32-stlink nucleo_f429zi z", false)
	if !reflect.DeepEqual(got, []string{"zephyr"}) {
		t.Fatalf("profile prefix z: %v want [zephyr]", got)
	}

	// MCU alias then profile (parser path)
	root, _ := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	t.Chdir(root)
	a.commandReg = commands.NewCommandRegistry()
	a.ExapData()
	p := commands.NewCommandParser(a.commandReg)
	line := "lua stm32-stlink f429zi "
	p.Sync(line, len([]rune(line))-1)
	got = a.lua.completions(p.CurrentToken(), p.RestTrailingSpace())
	if !contains(got, "zephyr") || !contains(got, "baremetal") {
		t.Fatalf("MCU alias + space profiles: %v", got)
	}
}
