package convert

import (
	"encoding/json"
	"fmt"
)

// ConvertOpenAIReqToClaude converts OpenAI chat completion request format to Claude messages format
func ConvertOpenAIReqToClaude(openaiReq map[string]interface{}) ([]byte, error) {
	// Extract model
	model, ok := openaiReq["model"].(string)
	if !ok || model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Extract max_tokens
	maxTokens := 4096 // default
	if mt, ok := openaiReq["max_tokens"].(float64); ok {
		maxTokens = int(mt)
	}

	// Extract messages
	messages, ok := openaiReq["messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("messages is required")
	}

	// Convert messages
	convertedMessages := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid message format")
		}

		convertedMsg := map[string]interface{}{}
		if role, ok := msgMap["role"].(string); ok {
			convertedMsg["role"] = role
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

	// Copy optional fields
	if temp, ok := openaiReq["temperature"].(float64); ok {
		claudeReq["temperature"] = temp
	}
	if topP, ok := openaiReq["top_p"].(float64); ok {
		claudeReq["top_p"] = topP
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

	// Extract content
	content := ""
	if contentArray, ok := claudeRespMap["content"].([]interface{}); ok && len(contentArray) > 0 {
		if firstContent, ok := contentArray[0].(map[string]interface{}); ok {
			if text, ok := firstContent["text"].(string); ok {
				content = text
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
		usage["total_tokens"] = usage["prompt_tokens"].(int) + usage["completion_tokens"].(int)
	}

	// Map finish reason
	finishReason := "stop"
	if stopReason, ok := claudeRespMap["stop_reason"].(string); ok {
		finishReason = mapStopReason(stopReason)
	}

	// Build OpenAI response
	openaiResp := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": 0, // Will be set to current time in proxy
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

// mapStopReason converts Claude stop_reason to OpenAI finish_reason
func mapStopReason(claudeReason string) string {
	mapping := map[string]string{
		"end_turn":           "stop",
		"max_tokens":         "length",
		"stop_sequence":      "stop",
		"tool_use":           "tool_calls",
	}
	if mapped, ok := mapping[claudeReason]; ok {
		return mapped
	}
	return "stop"
}