package luahost

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/yairgd/termforge/service"
	lua "github.com/yuin/gopher-lua"
)

type fromLuaKey struct{}

// WithLua marks ctx as a call made from the Lua thread, so a service can tell it
// from a REST/MCP caller (which runs on an HTTP goroutine).
func WithLua(ctx context.Context) context.Context {
	return context.WithValue(ctx, fromLuaKey{}, true)
}

// FromLua reports whether ctx came from a Lua script or the console.
func FromLua(ctx context.Context) bool {
	v, _ := ctx.Value(fromLuaKey{}).(bool)
	return v
}

// InstallServices exposes every service of reg as gdbforge.<name>(...). Arguments are
// matched to the params in order, a variadic param takes the rest, a Go error is raised
// in Lua, and the result comes back as a string, number, boolean or table.
func InstallServices(rt *Runtime, reg *service.Registry) {
	if rt == nil || reg == nil {
		return
	}
	for _, sv := range reg.List() {
		sv := sv
		rt.SetGdbforgeFunc(sv.Name, func(L *lua.LState) int {
			in := service.Args{}
			for n, p := range sv.Params {
				if p.Variadic {
					rest := []any{}
					for i := n + 1; i <= L.GetTop(); i++ {
						if v, ok := luaToArg(L.Get(i), p); ok {
							rest = append(rest, v)
						}
					}
					in[p.Name] = rest
				} else if v, ok := luaToArg(L.Get(n+1), p); ok {
					in[p.Name] = v
				}
			}
			res, err := reg.Call(WithLua(rt.JobContext()), sv.Name, in)
			if err != nil {
				L.RaiseError("%s", err.Error())
				return 0
			}
			if res == nil {
				return 0
			}
			L.Push(goToLua(L, res))
			return 1
		})
	}
}

// luaToArg converts one Lua value to the JSON-style value the param expects. Numbers are
// accepted for string params and numeric strings for number params, as the old
// hand-written bindings did. ok is false for nil (the param is absent).
func luaToArg(v lua.LValue, p service.Param) (any, bool) {
	switch x := v.(type) {
	case *lua.LNilType:
		return nil, false
	case lua.LString:
		if p.Type == service.Integer || p.Type == service.Number {
			if f, err := strconv.ParseFloat(string(x), 64); err == nil {
				return f, true
			}
		}
		return string(x), true
	case lua.LNumber:
		if p.Type == service.String {
			return x.String(), true
		}
		return float64(x), true
	case lua.LBool:
		return bool(x), true
	}
	if p.Type == service.Boolean {
		return lua.LVAsBool(v), true
	}
	return v.String(), true // tables, functions: let the type check report it
}

// goToLua converts a service result (string, number, bool, slice, map, struct) to a Lua value.
func goToLua(L *lua.LState, res any) lua.LValue {
	if s, ok := res.(string); ok {
		return lua.LString(s)
	}
	b, err := json.Marshal(res)
	if err != nil {
		return lua.LString(err.Error())
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return lua.LString(err.Error())
	}
	return jsonToLua(L, v)
}

func jsonToLua(L *lua.LState, v any) lua.LValue {
	switch x := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(x)
	case float64:
		return lua.LNumber(x)
	case string:
		return lua.LString(x)
	case []any:
		t := L.CreateTable(len(x), 0)
		for _, e := range x {
			t.Append(jsonToLua(L, e))
		}
		return t
	case map[string]any:
		t := L.CreateTable(0, len(x))
		for k, e := range x {
			t.RawSetString(k, jsonToLua(L, e))
		}
		return t
	}
	return lua.LNil
}
