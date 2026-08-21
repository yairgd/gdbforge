package main

import (
	"path/filepath"
	"testing"

	"github.com/yairgd/gdbforge/internal/commands"
)

func TestLuaTabCompletionViaCommandParser(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	a := &DebuggerApp{}
	a.commandReg = commands.NewCommandRegistry()
	a.ExapData()
	a.lua.app = a
	a.lua.loadScripts()

	if !a.lua.scriptKnown("stm32-stlink") {
		t.Fatal("stm32-stlink not indexed")
	}

	rt, err := a.lua.ensureRuntime("stm32-stlink")
	if err != nil {
		t.Fatalf("ensureRuntime: %v", err)
	}
	if rt == nil {
		t.Fatal("ensureRuntime returned nil")
	}

	p := commands.NewCommandParser(a.commandReg)
	cases := []struct {
		line string
		want int
		check func([]string) bool
	}{
		{"lua stm32-stlink ", 10, nil},
		{"lua stm32-stlink nucleo_f429zi ", 3, func(got []string) bool {
			return contains(got, "baremetal") && contains(got, "zephyr") && contains(got, "freertos")
		}},
		{"lua stm32-stlink f429zi ", 3, func(got []string) bool {
			return contains(got, "baremetal") && contains(got, "freertos")
		}},
		{"lua stm32-stlink nucleo_f429zi rtos", 1, func(got []string) bool {
			return contains(got, "freertos")
		}},
	}
	for _, tc := range cases {
		p.Reset()
		runes := []rune(tc.line)
		p.Sync(tc.line, len(runes)-1)
		if !p.CurrentIsRestArgs() {
			t.Fatalf("line %q: expected rest-args", tc.line)
		}
		token := p.CurrentToken()
		got := a.lua.completions(token, p.RestTrailingSpace())
		if len(got) < tc.want {
			t.Fatalf("line %q token %q trail=%v: got %d %v, want >= %d",
				tc.line, token, p.RestTrailingSpace(), len(got), got, tc.want)
		}
		if tc.check != nil && !tc.check(got) {
			t.Fatalf("line %q: check failed on %v", tc.line, got)
		}
		t.Logf("line %q -> %d candidates", tc.line, len(got))
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
