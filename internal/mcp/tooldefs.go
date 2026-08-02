package mcp

// toolDef is the canonical tool schema; Anthropic/OpenAI adapters map it.
type toolDef struct {
	Name        string
	Description string
	// empty props when Properties is nil
	Properties map[string]any
	Required   []string
}

func agentToolDefs() []toolDef {
	fileLine := map[string]any{
		"file": map[string]any{"type": "string", "description": "Source path or basename"},
		"line": map[string]any{"type": "integer", "description": "1-based line"},
	}
	return []toolDef{
		{Name: "list_breakpoints", Description: "List breakpoints from the shared model (same as the Breakpoints pane)."},
		{Name: "list_threads", Description: "List threads from the shared model (same as the Threads pane)."},
		{Name: "list_frames", Description: "List call-stack frames from the shared model (same as the Call Stack pane)."},
		{Name: "set_breakpoint", Description: "Set a breakpoint at file:line using the same path as the GUI (Space).", Properties: fileLine, Required: []string{"file", "line"}},
		{Name: "clear_breakpoint", Description: "Clear a breakpoint at file:line using the same path as the GUI (Space).", Properties: fileLine, Required: []string{"file", "line"}},
		{Name: "gdb_command", Description: "Run a raw GDB/MI or Delve command when no domain tool fits (continue, next, print, …).", Properties: map[string]any{
			"command": map[string]any{"type": "string", "description": "Debugger command line"},
		}, Required: []string{"command"}},
	}
}

func anthropicTools() []any {
	out := make([]any, 0, len(agentToolDefs()))
	for _, t := range agentToolDefs() {
		props := t.Properties
		if props == nil {
			props = map[string]any{}
		}
		schema := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(t.Required) > 0 {
			schema["required"] = t.Required
		}
		out = append(out, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": schema,
		})
	}
	return out
}

func openaiTools() []map[string]any {
	out := make([]map[string]any, 0, len(agentToolDefs()))
	for _, t := range agentToolDefs() {
		props := t.Properties
		if props == nil {
			props = map[string]any{}
		}
		params := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(t.Required) > 0 {
			params["required"] = t.Required
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  params,
			},
		})
	}
	return out
}
