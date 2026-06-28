package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yourusername/ais/internal/convert"
	"github.com/yourusername/ais/internal/logger"
)

// Config holds proxy configuration
type Config struct {
	Provider string
	Key      string
	Model    string
	BaseURL  string
}

// Handler handles HTTP requests and forwards them to the target AI provider
func Handler(config Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		
		// Only allow POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var requestBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Extract stream flag for logging
		stream := false
		if streamVal, ok := requestBody["stream"].(bool); ok {
			stream = streamVal
		}

			// Extract model from request body if not in config
			model := config.Model
			if modelVal, ok := requestBody["model"].(string); ok && modelVal != "" {
				model = modelVal
			}

			// Generate request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
			}

			// Construct target URL based on provider
		targetURL := config.BaseURL
		if targetURL == "" {
			if config.Provider == "openai" {
				targetURL = "https://api.openai.com/v1/chat/completions"
			} else if config.Provider == "anthropic" {
				targetURL = "https://api.anthropic.com/v1/messages"
			} else {
				http.Error(w, "Unsupported provider", http.StatusBadRequest)
				return
			}
		}

		// Convert request body based on provider
		var jsonBody []byte
		var err error
		
		if config.Provider == "anthropic" {
			jsonBody, err = convert.ConvertOpenAIReqToClaude(requestBody)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to convert request for Anthropic: %v", err), http.StatusBadRequest)
				return
			}
		} else {
			jsonBody, err = json.Marshal(requestBody)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to marshal request body: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Create new request
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			if strings.EqualFold(key, "Content-Type") || strings.EqualFold(key, "Accept") {
				for _, value := range values {
					req.Header.Set(key, value)
				}
			}
		}

		// Set provider-specific headers
		if config.Provider == "anthropic" {
			req.Header.Set("x-api-key", config.Key)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Key))
		}

		// Make the request
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to make request: %v", err), http.StatusBadGateway)
			return
		}
			defer resp.Body.Close()

			// Handle streaming response
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				
				handleStreamingResponse(w, resp, config, model, startTime, requestID)
				return
			}

			// Read response body for non-streaming
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
				return
			}

			// Read response body for non-streaming
			body, err = io.ReadAll(resp.Body)

			// Convert response based on provider for non-streaming
			var responseBody []byte
			if config.Provider == "anthropic" {
				responseBody, err = convert.ConvertClaudeRespToOpenAI(body)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to convert Anthropic response: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			responseBody = body
		}

		// Parse response to extract usage
		var responseBodyMap map[string]interface{}
		usage := map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		}

		if err := json.Unmarshal(responseBody, &responseBodyMap); err == nil {
			if usageData, ok := responseBodyMap["usage"].(map[string]interface{}); ok {
				usage = usageData
			}
		}

		// Extract token counts
		promptTokens := 0
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			promptTokens = int(pt)
		}
		completionTokens := 0
		if ct, ok := usage["completion_tokens"].(float64); ok {
			completionTokens = int(ct)
		}
		totalTokens := 0
		if tt, ok := usage["total_tokens"].(float64); ok {
			totalTokens = int(tt)
		}
	
			// Calculate duration
			duration := time.Since(startTime).Milliseconds()

			// Log the request (only for non-streaming or completion)
		if !stream {
			logEntry := logger.LogEntry{
				Timestamp:        startTime.UTC().Format(time.RFC3339),
				Provider:         config.Provider,
				Model:            model,
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
				CostUSD:          0.0, // Will be calculated in T5
				DurationMS:       duration,
				Stream:           stream,
				Status:           resp.StatusCode,
				RequestID:        requestID,
			}
			logger.PrintLog(logEntry)
		}

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Set status code
		w.WriteHeader(resp.StatusCode)

		// Write response body (converted if Anthropic, original if OpenAI)
		w.Write(responseBody)
	}
}

// handleStreamingResponse handles streaming SSE responses
func handleStreamingResponse(w http.ResponseWriter, resp *http.Response, config Config, model string, startTime time.Time, requestID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	var cachedUsage map[string]interface{} = nil
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Skip empty lines and non-data lines
		if line == "" || !strings.HasPrefix(line, "data: ") {
			if line != "" {
				fmt.Fprintf(w, "%s\n", line)
				flusher.Flush()
			}
			continue
		}
		
		dataLine := strings.TrimPrefix(line, "data: ")
		
		// Check for [DONE] marker
		if dataLine == "[DONE]" {
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}
		
		// Parse the SSE data
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataLine), &chunk); err != nil {
			// Invalid JSON, forward as-is
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}
		
		// Convert chunk based on provider
		var outputLine string
		if config.Provider == "anthropic" {
			convertedChunk, err := convertAnthropicStreamChunk(chunk)
			if err != nil {
				// Conversion failed, forward as-is
				fmt.Fprintf(w, "%s\n", line)
				flusher.Flush()
				continue
			}
			convertedData, _ := json.Marshal(convertedChunk)
			outputLine = fmt.Sprintf("data: %s\n", string(convertedData))
		} else {
			outputLine = line + "\n"
		}
		
		// Cache usage information
		if usageData, ok := chunk["usage"].(map[string]interface{}); ok {
			cachedUsage = usageData
		}
		
		fmt.Fprintf(w, "%s", outputLine)
		flusher.Flush()
	}
	
	// Log the streaming request
	duration := time.Since(startTime).Milliseconds()
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	
	if cachedUsage != nil {
		if pt, ok := cachedUsage["prompt_tokens"].(float64); ok {
			promptTokens = int(pt)
		}
		if ct, ok := cachedUsage["completion_tokens"].(float64); ok {
			completionTokens = int(ct)
		}
		if tt, ok := cachedUsage["total_tokens"].(float64); ok {
			totalTokens = int(tt)
		}
	}
	
	logEntry := logger.LogEntry{
		Timestamp:        startTime.UTC().Format(time.RFC3339),
		Provider:         config.Provider,
		Model:           model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		CostUSD:          0.0, // Will be calculated in T5
		DurationMS:       duration,
		Stream:          true,
		Status:          resp.StatusCode,
		RequestID:       requestID,
	}
	logger.PrintLog(logEntry)
}

// convertAnthropicStreamChunk converts an Anthropic streaming chunk to OpenAI format
func convertAnthropicStreamChunk(claudeChunk map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"id":      "",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   "",
		"choices": []map[string]interface{}{},
	}
	
	// Extract basic fields
	if id, ok := claudeChunk["id"].(string); ok {
		result["id"] = id
	}
	if model, ok := claudeChunk["model"].(string); ok {
		result["model"] = model
	}
	
	// Handle different Anthropic event types
	eventType, _ := claudeChunk["type"].(string)
	
	choices := make([]map[string]interface{}, 0, 1)
	choice := map[string]interface{}{
		"index": 0,
		"delta": map[string]interface{}{
			"role":    "assistant",
			"content": "",
		},
		"finish_reason": nil,
	}
	
	switch eventType {
	case "message_start":
		// Start of message - just update id/model
	case "content_block_start":
		// Start of content block
		if contentBlock, ok := claudeChunk["content_block"].(map[string]interface{}); ok {
			if text, ok := contentBlock["text"].(string); ok {
				choice["delta"].(map[string]interface{})["content"] = text
			}
		}
	case "content_block_delta":
		// Content delta
		if delta, ok := claudeChunk["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				choice["delta"].(map[string]interface{})["content"] = text
			}
		}
	case "content_block_stop":
		// Content block stopped
		choice["finish_reason"] = nil
	case "message_delta":
		// End of message with usage
		if delta, ok := claudeChunk["delta"].(map[string]interface{}); ok {
			if stopReason, ok := delta["stop_reason"].(string); ok {
				choice["finish_reason"] = mapStopReason(stopReason)
			}
		}
		// Copy usage information
		if usage, ok := claudeChunk["usage"].(map[string]interface{}); ok {
			result["usage"] = convertUsageFormat(usage)
		}
	case "message_stop":
		// End of message
		choice["finish_reason"] = "stop"
	case "ping", "error":
		// Skip ping and error events
		return result, nil
	}
	
	choices = append(choices, choice)
	result["choices"] = choices
	
	return result, nil
}

// convertUsageFormat converts Anthropic usage format to OpenAI format
func convertUsageFormat(anthropicUsage map[string]interface{}) map[string]interface{} {
	openaiUsage := map[string]interface{}{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
	
	if inputTokens, ok := anthropicUsage["input_tokens"].(float64); ok {
		openaiUsage["prompt_tokens"] = int(inputTokens)
	}
	if outputTokens, ok := anthropicUsage["output_tokens"].(float64); ok {
		openaiUsage["completion_tokens"] = int(outputTokens)
	}
	
	openaiUsage["total_tokens"] = openaiUsage["prompt_tokens"].(int) + openaiUsage["completion_tokens"].(int)
	
	return openaiUsage
}

// mapStopReason maps Anthropic stop_reason to OpenAI finish_reason
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