package convert

import (
	"encoding/json"
	"fmt"
	"time"
)

// ConvertOpenAIReqToClaude converts OpenAI chat completion request format to Claude messages format
func ConvertOpenAIReqToClaude(openaiReq map[string]interface{}) ([]byte, error) {
	// Extract model
	model, ok := openaiReq["model"].(string)
	if !ok || model == "" {
		return nil, fmt.Errorf("model is required")
	}

// Extract max_tokens (Claude requires this field)
maxTokens := 4096 // default
switch mt := openaiReq["max_tokens"].(type) {
case float64:
	maxTokens = int(mt)
case int:
	maxTokens = mt
}

	// Extract messages
	messages, ok := openaiReq["messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("messages is required")
	}

	// Convert messages, also extracting system prompt if present
	convertedMessages := make([]map[string]interface{}, 0, len(messages))
	var systemPrompt string
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid message format")
		}

		role, _ := msgMap["role"].(string)
		if role == "system" {
			// Claude uses a top-level "system" field instead of a system message
			if s, ok := msgMap["content"].(string); ok {
				systemPrompt = s
			}
			continue
		}

		convertedMsg := map[string]interface{}{
			"role": role,
		}
		if content, ok := msgMap["content"].(string); ok {
			convertedMsg["content"] = content
		}

		convertedMessages = append(convertedMessages, convertedMsg)
	}

	// Build Claude request
	claudeReq := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   convertedMessages,
	}

	// Add system prompt if extracted
	if systemPrompt != "" {
		claudeReq["system"] = systemPrompt
	}

	// Copy optional fields
	if temp, ok := openaiReq["temperature"]; ok {
		switch v := temp.(type) {
		case float64:
			claudeReq["temperature"] = v
		case int:
			claudeReq["temperature"] = float64(v)
		}
	}
	if topP, ok := openaiReq["top_p"]; ok {
		switch v := topP.(type) {
		case float64:
			claudeReq["top_p"] = v
		case int:
			claudeReq["top_p"] = float64(v)
		}
	}
	// Pass stream flag through
	if stream, ok := openaiReq["stream"].(bool); ok && stream {
		claudeReq["stream"] = true
	}

	return json.Marshal(claudeReq)
}

// ConvertClaudeRespToOpenAI converts Claude messages response format to OpenAI chat completion format
func ConvertClaudeRespToOpenAI(claudeResp []byte) ([]byte, error) {
	// Parse Claude response
	var claudeRespMap map[string]interface{}
	if err := json.Unmarshal(claudeResp, &claudeRespMap); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %v", err)
	}

	// Extract basic fields
	id, _ := claudeRespMap["id"].(string)
	model, _ := claudeRespMap["model"].(string)

	// Extract content (Claude returns content as an array of blocks)
	content := ""
	if contentArray, ok := claudeRespMap["content"].([]interface{}); ok {
		// Concatenate all text blocks (there may be multiple)
		for _, block := range contentArray {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, _ := blockMap["type"].(string); blockType == "text" {
					if text, ok := blockMap["text"].(string); ok {
						content += text
					}
				}
			}
		}
	}

	// Extract usage
	usage := map[string]interface{}{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
	if usageMap, ok := claudeRespMap["usage"].(map[string]interface{}); ok {
		if inputTokens, ok := usageMap["input_tokens"].(float64); ok {
			usage["prompt_tokens"] = int(inputTokens)
		}
		if outputTokens, ok := usageMap["output_tokens"].(float64); ok {
			usage["completion_tokens"] = int(outputTokens)
		}
		pt := usage["prompt_tokens"].(int)
		ct := usage["completion_tokens"].(int)
		usage["total_tokens"] = pt + ct
	}

	// Map finish reason
	finishReason := "stop"
	if stopReason, ok := claudeRespMap["stop_reason"].(string); ok {
		finishReason = MapStopReason(stopReason)
	}

	// Build OpenAI response with real timestamp
	openaiResp := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": finishReason,
				"index":         0,
			},
		},
		"usage": usage,
	}

	return json.Marshal(openaiResp)
}

// MapStopReason converts Claude stop_reason to OpenAI finish_reason
func MapStopReason(claudeReason string) string {
	mapping := map[string]string{
		"end_turn":      "stop",
		"max_tokens":    "length",
		"stop_sequence": "stop",
		"tool_use":      "tool_calls",
	}
	if mapped, ok := mapping[claudeReason]; ok {
		return mapped
	}
	return "stop"
}
