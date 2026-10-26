package mcp

import "github.com/yairgd/termforge/service"

// anthropicTools and openaiTools map the registry's tool schemas to each API's format.
func anthropicTools(r *service.Registry) []any {
	var out []any
	for _, t := range r.ToolSchemas() {
		out = append(out, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.InputSchema,
		})
	}
	return out
}

func openaiTools(r *service.Registry) []map[string]any {
	var out []map[string]any
	for _, t := range r.ToolSchemas() {
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		})
	}
	return out
}
