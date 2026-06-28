package convert

import (
	"encoding/json"
	"fmt"
	"time"
)

// ConvertOpenAIReqToClaude converts OpenAI chat completion request format to Claude messages format.
func ConvertOpenAIReqToClaude(openaiReq map[string]interface{}) ([]byte, error) {
	model, maxTokens, err := getModelInfo(openaiReq)
	if err != nil {
		return nil, err
	}
	messages, err := getMessages(openaiReq)
	if err != nil {
		return nil, err
	}
	converted, systemPrompt, err := convertMessages(messages)
	if err != nil {
		return nil, err
	}
	claudeReq := buildClaudeRequest(model, maxTokens, converted, systemPrompt)
	copyOptionalFields(openaiReq, claudeReq)
	return json.Marshal(claudeReq)
}

func getModelInfo(req map[string]interface{}) (model string, maxTokens int, err error) {
	model, ok := req["model"].(string)
	if !ok || model == "" {
		return "", 0, fmt.Errorf("model is required")
	}
	maxTokens = 4096
	switch mt := req["max_tokens"].(type) {
	case float64:
		maxTokens = int(mt)
	case int:
		maxTokens = mt
	}
	return model, maxTokens, nil
}

func getMessages(req map[string]interface{}) ([]interface{}, error) {
	msgs, ok := req["messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("messages is required")
	}
	return msgs, nil
}

func convertMessages(messages []interface{}) (converted []map[string]interface{}, systemPrompt string, err error) {
	converted = make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			return nil, "", fmt.Errorf("invalid message format")
		}
		role, _ := msgMap["role"].(string)
		if role == "system" {
			if s, _ := msgMap["content"].(string); s != "" {
				systemPrompt = s
			}
			continue
		}
		cm := map[string]interface{}{"role": role}
		if content, ok := msgMap["content"].(string); ok {
			cm["content"] = content
		}
		converted = append(converted, cm)
	}
	return converted, systemPrompt, nil
}

func buildClaudeRequest(model string, maxTokens int, messages []map[string]interface{}, systemPrompt string) map[string]interface{} {
	req := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   messages,
	}
	if systemPrompt != "" {
		req["system"] = systemPrompt
	}
	return req
}

func copyOptionalFields(src, dst map[string]interface{}) {
	for _, key := range []string{"temperature", "top_p"} {
		if val, ok := src[key]; ok {
			switch v := val.(type) {
			case float64:
				dst[key] = v
			case int:
				dst[key] = float64(v)
			}
		}
	}
	if stream, ok := src["stream"].(bool); ok && stream {
		dst["stream"] = true
	}
}

// ConvertClaudeRespToOpenAI converts Claude messages response format to OpenAI chat completion format.
func ConvertClaudeRespToOpenAI(claudeResp []byte) ([]byte, error) {
	var claudeRespMap map[string]interface{}
	if err := json.Unmarshal(claudeResp, &claudeRespMap); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %v", err)
	}
	resp := buildOpenAIResponse(claudeRespMap)
	return json.Marshal(resp)
}

func buildOpenAIResponse(claudeRespMap map[string]interface{}) map[string]interface{} {
	id, _ := claudeRespMap["id"].(string)
	model, _ := claudeRespMap["model"].(string)
	content := extractClaudeContent(claudeRespMap)
	usage := buildUsage(claudeRespMap)
	finishReason := resolveFinishReason(claudeRespMap)

	return map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{{
			"message":       map[string]interface{}{"role": "assistant", "content": content},
			"finish_reason": finishReason,
			"index":         0,
		}},
		"usage": usage,
	}
}

func extractClaudeContent(m map[string]interface{}) string {
	contentArray, ok := m["content"].([]interface{})
	if !ok {
		return ""
	}
	var sb string
	for _, block := range contentArray {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if blockType, _ := blockMap["type"].(string); blockType != "text" {
			continue
		}
		if text, ok := blockMap["text"].(string); ok {
			sb += text
		}
	}
	return sb
}

func buildUsage(m map[string]interface{}) map[string]interface{} {
	usage := map[string]interface{}{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
	usageMap, ok := m["usage"].(map[string]interface{})
	if !ok {
		return usage
	}
	if v, ok := usageMap["input_tokens"].(float64); ok {
		usage["prompt_tokens"] = int(v)
	}
	if v, ok := usageMap["output_tokens"].(float64); ok {
		usage["completion_tokens"] = int(v)
	}
	pt := usage["prompt_tokens"].(int)
	ct := usage["completion_tokens"].(int)
	usage["total_tokens"] = pt + ct
	return usage
}

func resolveFinishReason(m map[string]interface{}) string {
	if stopReason, ok := m["stop_reason"].(string); ok {
		return MapStopReason(stopReason)
	}
	return "stop"
}

// MapStopReason converts Claude stop_reason to OpenAI finish_reason.
func MapStopReason(claudeReason string) string {
	switch claudeReason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}
