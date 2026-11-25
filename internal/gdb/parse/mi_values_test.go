package parse

import "testing"

func TestMIEvaluateAndError(t *testing.T) {
	raw := "-data-evaluate-expression x\r\n^done,value=\"42\"\r\n(gdb)\r\n"
	if v, ok := MIEvaluateValue(raw); !ok || v != "42" {
		t.Fatalf("%q %v", v, ok)
	}
	raw = "^error,msg=\"No symbol \\\"y\\\" in current context.\"\r\n"
	if m, ok := MIError(raw); !ok || m != `No symbol "y" in current context.` {
		t.Fatalf("%q %v", m, ok)
	}
	if _, ok := MIEvaluateValue(raw); ok {
		t.Fatal("error reply must not parse as a value")
	}
}

func TestMIMemoryHex(t *testing.T) {
	raw := `^done,memory=[{begin="0x1000",offset="0x0000",end="0x1004",contents="deadbeef"},{begin="0x1004",offset="0x0",end="0x1006",contents="0102"}]`
	if h, ok := MIMemoryHex(raw); !ok || h != "deadbeef0102" {
		t.Fatalf("%q %v", h, ok)
	}
	if _, ok := MIMemoryHex(`^error,msg="Cannot access memory at address 0x0"`); ok {
		t.Fatal("error reply")
	}
}

func TestMIRegisters(t *testing.T) {
	names := MIRegisterNames(`^done,register-names=["rax","rbx","","rcx"]`)
	if len(names) != 4 || names[3] != "rcx" {
		t.Fatalf("%v", names)
	}
	vals := MIRegisterValues(`^done,register-values=[{number="0",value="0x1"},{number="3",value="0xff"}]`)
	if vals[0] != "0x1" || vals[3] != "0xff" || len(vals) != 2 {
		t.Fatalf("%v", vals)
	}
}

func TestInfoRegisters(t *testing.T) {
	raw := "info registers\r\n&\"info registers\\n\"\r\n" +
		"~\"rax            0x1c                28\\n\"\r\n" +
		"~\"eflags         0x246               [ IF ZF PF ]\\n\"\r\n" +
		"~\"cpsr           0x60000010          1610612752\\n\"\r\n^done\r\n(gdb) \r\n"
	got := InfoRegisters(MIConsoleText(raw))
	if len(got) != 3 || got["rax"] != "0x1c" || got["eflags"] != "0x246" || got["cpsr"] != "0x60000010" {
		t.Fatalf("%v", got)
	}
}
