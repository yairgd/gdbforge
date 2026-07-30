package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxToolRounds = 8

const agentSystemPrompt = `You are a debugging assistant inside gdbforge.
Prefer domain tools that read/write the shared debugger models (same truth as the GUI):
list_breakpoints, list_threads, list_frames, set_breakpoint, clear_breakpoint.
Use gdb_command only for GDB/MI that has no domain tool (e.g. continue, next, print).
Explain findings clearly.`

// Ask sends a natural-language debug question to an LLM (Anthropic or OpenAI)
// with domain + gdb_command tools on the shared GdbMcpService session.
func (s *GdbMcpService) Ask(ctx context.Context, question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", fmt.Errorf("empty question")
	}
	if s == nil || s.sess == nil {
		return "", fmt.Errorf("no gdb session")
	}

	if key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		return s.askAnthropic(ctx, key, question)
	}
	if key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); key != "" {
		return s.askOpenAI(ctx, key, question)
	}
	return "", fmt.Errorf("set ANTHROPIC_API_KEY or OPENAI_API_KEY for :AI")
}

func (s *GdbMcpService) askAnthropic(ctx context.Context, apiKey, question string) (string, error) {
	model := os.Getenv("GDBFORGE_AI_MODEL")
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	tools := anthropicTools()
	messages := []map[string]any{
		{"role": "user", "content": []map[string]any{{"type": "text", "text": question}}},
	}

	client := &http.Client{Timeout: 120 * time.Second}
	for round := 0; round < maxToolRounds; round++ {
		body, _ := json.Marshal(map[string]any{
			"model":      model,
			"max_tokens": 2048,
			"system":     agentSystemPrompt,
			"tools":      tools,
			"messages":   messages,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("anthropic: %s: %s", resp.Status, truncate(string(raw), 400))
		}

		var parsed struct {
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", err
		}
		if parsed.Error != nil {
			return "", fmt.Errorf("anthropic: %s", parsed.Error.Message)
		}

		asst := make([]map[string]any, 0, len(parsed.Content))
		var textParts []string
		var toolResults []map[string]any
		for _, b := range parsed.Content {
			block := map[string]any{"type": b.Type}
			if b.Text != "" {
				block["text"] = b.Text
				textParts = append(textParts, b.Text)
			}
			if b.ID != "" {
				block["id"] = b.ID
			}
			if b.Name != "" {
				block["name"] = b.Name
			}
			if len(b.Input) > 0 {
				var in any
				_ = json.Unmarshal(b.Input, &in)
				block["input"] = in
			}
			asst = append(asst, block)

			if b.Type != "tool_use" {
				continue
			}
			out := s.runTool(ctx, b.Name, b.Input)
			toolResults = append(toolResults, map[string]any{
				"type":        "tool_result",
				"tool_use_id": b.ID,
				"content":     out,
			})
		}

		messages = append(messages, map[string]any{"role": "assistant", "content": asst})
		if len(toolResults) == 0 {
			if len(textParts) == 0 {
				return "(no response)", nil
			}
			return strings.Join(textParts, "\n"), nil
		}
		messages = append(messages, map[string]any{"role": "user", "content": toolResults})
	}
	return "", fmt.Errorf("too many tool rounds")
}

func (s *GdbMcpService) askOpenAI(ctx context.Context, apiKey, question string) (string, error) {
	model := os.Getenv("GDBFORGE_AI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	messages := []map[string]any{
		{"role": "system", "content": agentSystemPrompt},
		{"role": "user", "content": question},
	}
	tools := openaiTools()

	client := &http.Client{Timeout: 120 * time.Second}
	for round := 0; round < maxToolRounds; round++ {
		body, _ := json.Marshal(map[string]any{
			"model":       model,
			"messages":    messages,
			"tools":       tools,
			"tool_choice": "auto",
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("openai: %s: %s", resp.Status, truncate(string(raw), 400))
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", err
		}
		if len(parsed.Choices) == 0 {
			return "", fmt.Errorf("openai: empty choices")
		}
		msg := parsed.Choices[0].Message
		asst := map[string]any{"role": "assistant", "content": msg.Content}
		if len(msg.ToolCalls) > 0 {
			asst["tool_calls"] = msg.ToolCalls
		}
		messages = append(messages, asst)
		if len(msg.ToolCalls) == 0 {
			if msg.Content == "" {
				return "(no response)", nil
			}
			return msg.Content, nil
		}
		for _, tc := range msg.ToolCalls {
			name := tc.Function.Name
			out := s.runTool(ctx, name, json.RawMessage(tc.Function.Arguments))
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      out,
			})
		}
	}
	return "", fmt.Errorf("too many tool rounds")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
