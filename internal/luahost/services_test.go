package luahost

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yairgd/termforge/service"
)

func svcRuntime(t *testing.T) (*Runtime, *memPane, *service.Registry) {
	t.Helper()
	p := &memPane{w: 40, h: 10}
	rt := New(p, nil)
	t.Cleanup(rt.Close)
	reg := &service.Registry{}
	InstallServices(rt, reg) // no services yet: must be a no-op
	return rt, p, reg
}

func TestInstallServices(t *testing.T) {
	rt, p, reg := svcRuntime(t)
	var fromLua bool
	reg.Add(service.Service{
		Name: "echo", Help: "echo args",
		Params: []service.Param{
			{Name: "word", Type: service.String},
			{Name: "n", Type: service.Integer, Optional: true},
			{Name: "rest", Type: service.String, Variadic: true},
		},
		Run: func(ctx context.Context, a service.Args) (any, error) {
			fromLua = FromLua(ctx)
			return map[string]any{
				"word": a.String("word"), "n": a["n"], "rest": a.Strings("rest"),
			}, nil
		},
	})
	reg.Add(service.Service{Name: "fail", Run: func(context.Context, service.Args) (any, error) {
		return nil, errors.New("boom")
	}})
	reg.Add(service.Service{Name: "none", Run: func(context.Context, service.Args) (any, error) { return nil, nil }})
	reg.Add(service.Service{Name: "name", Run: func(context.Context, service.Args) (any, error) { return "gdb", nil }})
	reg.Add(service.Service{Name: "rows", Run: func(context.Context, service.Args) (any, error) {
		return []map[string]any{{"id": 1, "ok": true}, {"id": 2, "ok": false}}, nil
	}})
	InstallServices(rt, reg)

	// positional, optional number given as a numeric string, number coerced to string, variadic rest
	err := rt.LoadString(`
local r = gdbforge.echo("a", "7", 5, "x")
gdbforge.print(r.word .. " " .. r.n .. " " .. #r.rest .. " " .. r.rest[1] .. " " .. r.rest[2])
local rows = gdbforge.rows()
gdbforge.print(#rows .. " " .. tostring(rows[1].ok) .. " " .. rows[2].id)
gdbforge.print(gdbforge.name())
local nv = gdbforge.none()
gdbforge.print(tostring(nv))
local ok, e = pcall(gdbforge.fail)
gdbforge.print(tostring(ok) .. " " .. e)
ok, e = pcall(gdbforge.echo)
gdbforge.print(tostring(ok) .. " " .. e)
`, "t")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a 7 2 5 x", "2 true 2", "gdb", "nil"}
	for i, w := range want {
		if i >= len(p.lines) || p.lines[i] != w {
			t.Fatalf("line %d: got %q want %q (all: %q)", i, p.lines[i], w, p.lines)
		}
	}
	if !strings.Contains(p.lines[4], "false") || !strings.Contains(p.lines[4], "boom") {
		t.Fatalf("service error not raised in Lua: %q", p.lines[4])
	}
	if !strings.Contains(p.lines[5], `missing "word"`) {
		t.Fatalf("missing param not reported: %q", p.lines[5])
	}
	if !fromLua {
		t.Fatal("service ctx not marked FromLua")
	}
	if FromLua(context.Background()) {
		t.Fatal("plain ctx must not be FromLua")
	}
}
