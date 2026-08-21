package luahost

import (
	"reflect"
	"testing"
)

func TestParseArgCompletion(t *testing.T) {
	cases := []struct {
		args          []string
		trailingSpace bool
		wantIndex     int
		wantToken     string
		wantPrior     []string
	}{
		{nil, true, 1, "", nil},
		{[]string{"z"}, false, 1, "z", nil},
		{[]string{"board1"}, true, 2, "", []string{"board1"}},
		{[]string{"board1", "z"}, false, 2, "z", []string{"board1"}},
	}
	for _, tc := range cases {
		got := ParseArgCompletion(tc.args, tc.trailingSpace)
		if got.ArgIndex != tc.wantIndex || got.Token != tc.wantToken {
			t.Fatalf("args=%v space=%v got index=%d token=%q want index=%d token=%q",
				tc.args, tc.trailingSpace, got.ArgIndex, got.Token, tc.wantIndex, tc.wantToken)
		}
		if !reflect.DeepEqual(got.Prior, tc.wantPrior) {
			t.Fatalf("args=%v prior=%v want %v", tc.args, got.Prior, tc.wantPrior)
		}
	}
}

func TestCompleteScriptArgs(t *testing.T) {
	rt := New(nil, nil)
	defer rt.Close()
	if err := rt.LoadString(`
gdbforge.complete_args(function(token, index)
  if index == 1 then
    local all = {"baremetal", "zephyr", "help"}
    if token == "" then return all end
    local out = {}
    for _, v in ipairs(all) do
      if v:sub(1, #token) == token then out[#out+1] = v end
    end
    return out
  end
  return {}
end)
`, "test.lua"); err != nil {
		t.Fatal(err)
	}
	got, ok := rt.CompleteScriptArgs(1, "z", nil)
	if !ok || !reflect.DeepEqual(got, []string{"zephyr"}) {
		t.Fatalf("got %v ok=%v want [zephyr] true", got, ok)
	}
	gotAll, ok := rt.CompleteScriptArgs(1, "", nil)
	if !ok || !reflect.DeepEqual(gotAll, []string{"baremetal", "help", "zephyr"}) {
		t.Fatalf("all: got %v ok=%v", gotAll, ok)
	}
	gotEmpty, ok := rt.CompleteScriptArgs(2, "", []string{"board"})
	if !ok || len(gotEmpty) != 0 {
		t.Fatalf("arg2: got %v ok=%v", gotEmpty, ok)
	}
}

func TestCompleteScriptArgsLegacyGlobal(t *testing.T) {
	rt := New(nil, nil)
	defer rt.Close()
	if err := rt.LoadString(`
function complete_arg(token, index)
  if index == 1 then return {"help"} end
  return {}
end
`, "legacy.lua"); err != nil {
		t.Fatal(err)
	}
	got, ok := rt.CompleteScriptArgs(1, "", nil)
	if !ok || !reflect.DeepEqual(got, []string{"help"}) {
		t.Fatalf("legacy complete_arg: %v ok=%v", got, ok)
	}
}

func TestCompleteScriptArgsMissing(t *testing.T) {
	rt := New(nil, nil)
	defer rt.Close()
	if err := rt.LoadString(`function main() end`, "test.lua"); err != nil {
		t.Fatal(err)
	}
	got, ok := rt.CompleteScriptArgs(1, "", nil)
	if ok || got != nil {
		t.Fatalf("want false,nil without complete_args, got ok=%v %v", ok, got)
	}
}

func TestFilterPrefix(t *testing.T) {
	got := FilterPrefix("Z", []string{"zephyr", "baremetal"})
	if !reflect.DeepEqual(got, []string{"zephyr"}) {
		t.Fatalf("FilterPrefix: %v", got)
	}
}
