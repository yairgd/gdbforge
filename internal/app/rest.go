package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/termforge/service"
)

// RESTHandler is the -serve handler: termforge's REST and MCP endpoints, except that
// POST /api/<tool> also reports the run state beside the result, and a tool's errors carry a
// code (target_running, interrupt_timeout, …) instead of only text. Wrap it with
// service.WithToken.
func (a *DebuggerApp) RESTHandler() http.Handler { return a.lua.restHandler() }

func (c *luaCtl) restHandler() http.Handler {
	reg := c.services()
	base := reg.Handler()
	mux := http.NewServeMux()
	mux.Handle("/api", base) // GET: the tool list with schemas
	mux.Handle("/mcp", base)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, q *http.Request) {
		if q.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var args service.Args
		if err := json.NewDecoder(q.Body).Decode(&args); err != nil && !errors.Is(err, io.EOF) {
			writeAPIJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		name := strings.TrimPrefix(q.URL.Path, "/api/")
		if !isRemote(reg, name) {
			writeAPIJSON(w, http.StatusNotFound, map[string]any{"error": "no such service: " + name})
			return
		}
		res, err := reg.Call(q.Context(), name, args)
		body := c.stateFields(q.Context())
		code := http.StatusOK
		var ae *mcp.APIError
		switch {
		case errors.As(err, &ae):
			body["error"], body["message"] = ae.Code, ae.Message
			if ae.Hint != "" {
				body["hint"] = ae.Hint
			}
			code = http.StatusConflict
			if ae.Code == "timeout" || ae.Code == "interrupt_timeout" {
				code = http.StatusGatewayTimeout
			}
		case err != nil:
			body["error"] = err.Error()
			code = http.StatusBadRequest
		default:
			body["result"] = res
		}
		writeAPIJSON(w, code, body)
	})
	return mux
}

func isRemote(reg *service.Registry, name string) bool {
	for _, s := range reg.List() {
		if s.Name == name {
			return s.Remote
		}
	}
	return false
}

func writeAPIJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
