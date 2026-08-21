package luahost

import (
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// ArgCompletionRequest describes one :lua <script> Tab completion query.
type ArgCompletionRequest struct {
	ArgIndex int
	Token    string
	Prior    []string
}

// ParseArgCompletion derives arg index/token/prior from script args only (not the script name).
// args is strings.Fields(rest) after the script token; trailingSpace is true when rest ends with space/tab.
func ParseArgCompletion(args []string, trailingSpace bool) ArgCompletionRequest {
	if trailingSpace {
		idx := len(args) + 1
		if idx < 1 {
			idx = 1
		}
		return ArgCompletionRequest{
			ArgIndex: idx,
			Token:    "",
			Prior:    append([]string(nil), args...),
		}
	}
	if len(args) == 0 {
		return ArgCompletionRequest{ArgIndex: 1}
	}
	req := ArgCompletionRequest{
		ArgIndex: len(args),
		Token:    args[len(args)-1],
	}
	if len(args) > 1 {
		req.Prior = append([]string(nil), args[:len(args)-1]...)
	}
	return req
}

// CompleteScriptArgs returns Tab candidates for :lua <script> args.
// The bool is true when a handler ran (even if the slice is empty).
func (rt *Runtime) CompleteScriptArgs(argIndex int, token string, prior []string) ([]string, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil || argIndex < 1 {
		return nil, false
	}
	if rt.completeArgFn != nil {
		return rt.callCompleteFn(rt.completeArgFn, argIndex, token, prior)
	}
	fn := rt.L.GetGlobal("complete_arg")
	if fn.Type() != lua.LTFunction {
		return nil, false
	}
	return rt.callCompleteFn(fn.(*lua.LFunction), argIndex, token, prior)
}

func (rt *Runtime) callCompleteFn(fn *lua.LFunction, argIndex int, token string, prior []string) ([]string, bool) {
	L := rt.L
	L.Push(fn)
	L.Push(lua.LString(token))
	L.Push(lua.LNumber(argIndex))
	for _, p := range prior {
		L.Push(lua.LString(p))
	}
	if err := L.PCall(2+len(prior), 1, nil); err != nil {
		return nil, false
	}
	defer L.Pop(1)
	return luaStringList(L, -1), true
}

func luaStringList(L *lua.LState, idx int) []string {
	v := L.Get(idx)
	switch v.Type() {
	case lua.LTTable:
		var out []string
		t := v.(*lua.LTable)
		t.ForEach(func(_, val lua.LValue) {
			if val.Type() != lua.LTString {
				return
			}
			s := strings.TrimSpace(val.String())
			if s != "" {
				out = append(out, s)
			}
		})
		sort.Strings(out)
		return out
	case lua.LTString:
		s := strings.TrimSpace(v.String())
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

// FilterPrefix returns opts whose lowercase form starts with token (case-insensitive).
func FilterPrefix(token string, opts []string) []string {
	token = strings.ToLower(strings.TrimSpace(token))
	var out []string
	for _, o := range opts {
		if token == "" || strings.HasPrefix(strings.ToLower(o), token) {
			out = append(out, o)
		}
	}
	return out
}
