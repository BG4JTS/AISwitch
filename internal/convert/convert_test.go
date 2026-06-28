package convert

import (
	"encoding/json"
	"testing"
)

func TestConvertOpenAIReqToClaude(t *testing.T) {
	openaiReq := map[string]interface{}{
		"model":      "claude-3-sonnet-20240229",
		"max_tokens": 100,
		"temperature": 0.7,
		"top_p":      0.9,
		"stream":     true,
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "You are helpful."},
			map[string]interface{}{"role": "user", "content": "Hello"},
		},
	}

	data, err := ConvertOpenAIReqToClaude(openaiReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var claude map[string]interface{}
	if err := json.Unmarshal(data, &claude); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify model preserved
	if claude["model"] != "claude-3-sonnet-20240229" {
		t.Errorf("model = %v, want claude-3-sonnet-20240229", claude["model"])
	}
	// Verify max_tokens preserved
	if claude["max_tokens"] != 100.0 {
		t.Errorf("max_tokens = %v, want 100", claude["max_tokens"])
	}
	// Verify temperature preserved
	if claude["temperature"] != 0.7 {
		t.Errorf("temperature = %v, want 0.7", claude["temperature"])
	}
	// Verify top_p preserved
	if claude["top_p"] != 0.9 {
		t.Errorf("top_p = %v, want 0.9", claude["top_p"])
	}
	// Verify stream preserved
	if claude["stream"] != true {
		t.Errorf("stream = %v, want true", claude["stream"])
	}
	// Verify system message extracted to top-level "system" field
	if claude["system"] != "You are helpful." {
		t.Errorf("system = %v, want 'You are helpful.'", claude["system"])
	}
	// Verify only user message remains in messages array (system was extracted)
	msgs := claude["messages"].([]interface{})
	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	msg := msgs[0].(map[string]interface{})
	if msg["role"] != "user" {
		t.Errorf("message role = %v, want user", msg["role"])
	}
}

func TestConvertOpenAIReqToClaude_Defaults(t *testing.T) {
	openaiReq := map[string]interface{}{
		"model":    "claude-3-sonnet-20240229",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello"},
		},
	}

	data, err := ConvertOpenAIReqToClaude(openaiReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var claude map[string]interface{}
	json.Unmarshal(data, &claude)

	if claude["max_tokens"] != 4096.0 {
		t.Errorf("default max_tokens = %v, want 4096", claude["max_tokens"])
	}
	if _, ok := claude["system"]; ok {
		t.Error("system should not be present when no system message")
	}
}

func TestConvertOpenAIReqToClaude_NoModel(t *testing.T) {
	openaiReq := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello"},
		},
	}

	_, err := ConvertOpenAIReqToClaude(openaiReq)
	if err == nil {
		t.Error("expected error when model is missing")
	}
}

func TestConvertClaudeRespToOpenAI(t *testing.T) {
	claudeResp := `{
		"id": "msg_abc123",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-sonnet-20240229",
		"content": [
			{"type": "text", "text": "Hello!"},
			{"type": "text", "text": " How can I help?"}
		],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 25,
			"output_tokens": 15
		}
	}`

	data, err := ConvertClaudeRespToOpenAI([]byte(claudeResp))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var openai map[string]interface{}
	json.Unmarshal(data, &openai)

	// Verify structure
	if openai["id"] != "msg_abc123" {
		t.Errorf("id = %v, want msg_abc123", openai["id"])
	}
	if openai["object"] != "chat.completion" {
		t.Errorf("object = %v, want chat.completion", openai["object"])
	}
	if openai["model"] != "claude-3-sonnet-20240229" {
		t.Errorf("model = %v, want claude-3-sonnet-20240229", openai["model"])
	}
	// Verify created is a real timestamp (non-zero)
	if created, ok := openai["created"].(float64); !ok || created == 0 {
		t.Errorf("created = %v, want non-zero timestamp", openai["created"])
	}
	// Verify content concatenation from multiple blocks
	choices := openai["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	if message["content"] != "Hello! How can I help?" {
		t.Errorf("content = %v, want 'Hello! How can I help?'", message["content"])
	}
	if message["role"] != "assistant" {
		t.Errorf("role = %v, want assistant", message["role"])
	}
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", choice["finish_reason"])
	}
	// Verify usage conversion
	usage := openai["usage"].(map[string]interface{})
	if usage["prompt_tokens"] != 25.0 {
		t.Errorf("prompt_tokens = %v, want 25", usage["prompt_tokens"])
	}
	if usage["completion_tokens"] != 15.0 {
		t.Errorf("completion_tokens = %v, want 15", usage["completion_tokens"])
	}
	if usage["total_tokens"] != 40.0 {
		t.Errorf("total_tokens = %v, want 40", usage["total_tokens"])
	}
}

func TestConvertClaudeRespToOpenAI_MaxTokens(t *testing.T) {
	claudeResp := `{
		"id": "msg_test",
		"model": "claude-3-sonnet-20240229",
		"content": [{"type": "text", "text": "partial"}],
		"stop_reason": "max_tokens",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	data, _ := ConvertClaudeRespToOpenAI([]byte(claudeResp))
	var openai map[string]interface{}
	json.Unmarshal(data, &openai)

	choices := openai["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	if choice["finish_reason"] != "length" {
		t.Errorf("finish_reason = %v, want length (max_tokens mapping)", choice["finish_reason"])
	}
}

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		claude string
		openai string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"tool_use", "tool_calls"},
		{"unknown_reason", "stop"},
	}
	for _, tt := range tests {
		got := MapStopReason(tt.claude)
		if got != tt.openai {
			t.Errorf("MapStopReason(%q) = %q, want %q", tt.claude, got, tt.openai)
		}
	}
}
