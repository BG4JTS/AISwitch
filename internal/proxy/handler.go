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
	Verbose  bool
}

// defaultTargetURL returns the upstream URL for a given provider.
func defaultTargetURL(provider string) (string, bool) {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1/chat/completions", true
	case "anthropic":
		return "https://api.anthropic.com/v1/messages", true
	case "deepseek":
		// DeepSeek is OpenAI-compatible
		return "https://api.deepseek.com/v1/chat/completions", true
	default:
		return "", false
	}
}

// Handler handles HTTP requests and forwards them to the target AI provider
func Handler(config Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		
		// Verbose: log incoming request summary
		if config.Verbose {
			fmt.Printf("[VERBOSE] %s %s %s → %s (model=%s)\n",
				r.Method, r.URL.Path, r.RemoteAddr, config.Provider, config.Model)
		}

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

		// Extract stream flag
		stream := false
		if streamVal, ok := requestBody["stream"].(bool); ok {
			stream = streamVal
		}

		// Extract model (request body takes precedence over config default)
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
			url, ok := defaultTargetURL(config.Provider)
			if !ok {
				http.Error(w, fmt.Sprintf("Unsupported provider: %s", config.Provider), http.StatusBadRequest)
				return
			}
			targetURL = url
		}

		// Build the upstream request body.
		// For Anthropic we translate the OpenAI-format request into Claude format.
		// For OpenAI-compatible providers (openai/deepseek/...) we forward as-is.
		var jsonBody []byte
		var err error
		if config.Provider == "anthropic" {
			jsonBody, err = convert.ConvertOpenAIReqToClaude(requestBody)
		} else {
			jsonBody, err = json.Marshal(requestBody)
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build request body: %v", err), http.StatusBadRequest)
			return
		}

		// Verbose: log the request body being sent to upstream
		if config.Verbose {
			if config.Provider == "anthropic" {
				fmt.Printf("[VERBOSE] → upstream (converted to Claude format): %s\n", limitString(string(jsonBody), 500))
			} else {
				fmt.Printf("[VERBOSE] → upstream (original): %s\n", limitString(string(jsonBody), 500))
			}
		}

		// Create upstream request
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
			return
		}

		// Forward Content-Type / Accept headers from the client
		for key, values := range r.Header {
			if strings.EqualFold(key, "Content-Type") || strings.EqualFold(key, "Accept") {
				for _, value := range values {
					req.Header.Set(key, value)
				}
			}
		}

		// Set provider-specific auth headers
		switch config.Provider {
		case "anthropic":
			req.Header.Set("x-api-key", config.Key)
			req.Header.Set("anthropic-version", "2023-06-01")
		default:
			// openai, deepseek, and any OpenAI-compatible provider use Bearer auth
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Key))
		}

		// Use a client WITHOUT a timeout for streaming (so long streams aren't killed),
		// but keep a sane timeout for non-streaming requests.
		client := &http.Client{}
		if !stream {
			client.Timeout = 120 * time.Second
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to make upstream request: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// ---------------------------------------------------------------
		// Error responses: forward the upstream body verbatim so the
		// client can see the real error message (e.g. 401 / 429 / 5xx).
		// ---------------------------------------------------------------
		if resp.StatusCode >= 400 {
			forwardError(w, resp)
			return
		}

		// ---------------------------------------------------------------
		// Streaming path
		// ---------------------------------------------------------------
		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			handleStreamingResponse(w, resp, config, model, startTime, requestID)
			return
		}

		// ---------------------------------------------------------------
		// Non-streaming path (read body ONCE)
		// ---------------------------------------------------------------
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
			return
		}

		// Convert Anthropic response -> OpenAI format if needed
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

		// Extract usage from the (possibly converted) response
		usage := extractUsage(responseBody)
		promptTokens, completionTokens, totalTokens := readTokenCounts(usage)

		duration := time.Since(startTime).Milliseconds()

		// Log the non-streaming request
		logger.PrintLog(logger.LogEntry{
			Timestamp:        startTime.UTC().Format(time.RFC3339),
			Provider:         config.Provider,
			Model:            model,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			CostUSD:          0.0, // Will be calculated in T5
			DurationMS:       duration,
			Stream:           false,
			Status:           resp.StatusCode,
			RequestID:        requestID,
		})

		// Write the response back (always JSON)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(responseBody)

		// Verbose: log the final response summary
		if config.Verbose {
			fmt.Printf("[VERBOSE] ← upstream status=%d bytes=%d tokens(p=%d,c=%d)\n",
				resp.StatusCode, len(responseBody), promptTokens, completionTokens)
		}
	}
}

// forwardError copies an upstream error response back to the client unchanged,
// preserving the status code and the original body so the real error is visible.
func forwardError(w http.ResponseWriter, resp *http.Response) {
	// Copy relevant headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// extractUsage parses a JSON body and returns its "usage" object, or a zeroed one.
func extractUsage(body []byte) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		}
	}
	if usage, ok := m["usage"].(map[string]interface{}); ok {
		return usage
	}
	return map[string]interface{}{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
}

// readTokenCounts safely extracts the token counts from a usage map.
func readTokenCounts(usage map[string]interface{}) (prompt, completion, total int) {
	if pt, ok := usage["prompt_tokens"].(float64); ok {
		prompt = int(pt)
	}
	if ct, ok := usage["completion_tokens"].(float64); ok {
		completion = int(ct)
	}
	if tt, ok := usage["total_tokens"].(float64); ok {
		total = int(tt)
	} else {
		total = prompt + completion
	}
	return
}

// handleStreamingResponse handles streaming SSE responses.
// It reads from the upstream response line by line, converts Anthropic
// events to OpenAI chunks when needed, and flushes to the client in real time.
func handleStreamingResponse(w http.ResponseWriter, resp *http.Response, config Config, model string, startTime time.Time, requestID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Can't stream; nothing more we can do.
		return
	}

	// Use a scanner with a larger buffer so long SSE lines aren't truncated.
	const maxLineSize = 1 << 20 // 1 MiB
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var cachedUsage map[string]interface{}
	seenDone := false

	for scanner.Scan() {
		line := scanner.Text()

		// Blank line = SSE event separator, forward it to keep framing intact.
		if line == "" {
			fmt.Fprintf(w, "\n")
			flusher.Flush()
			continue
		}

		// Forward non-data lines (e.g. "event: ...") unchanged.
		if !strings.HasPrefix(line, "data: ") {
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}

		dataLine := strings.TrimPrefix(line, "data: ")

		// [DONE] terminates the stream.
		if dataLine == "[DONE]" {
			seenDone = true
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}

		// Parse the SSE data JSON.
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataLine), &chunk); err != nil {
			// Unparseable, forward as-is rather than dropping it.
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}

		// Cache usage if present (OpenAI sends it on the final chunk).
		if usageData, ok := chunk["usage"].(map[string]interface{}); ok {
			cachedUsage = usageData
		}

		// Convert Anthropic stream events to OpenAI chunks; pass OpenAI-style through.
		var outputData []byte
		if config.Provider == "anthropic" {
			converted, convErr := convertAnthropicStreamChunk(chunk)
			if convErr != nil {
				// On conversion error, forward the original line.
				fmt.Fprintf(w, "%s\n", line)
				flusher.Flush()
				continue
			}
			outputData, _ = json.Marshal(converted)
			// Anthropic carries usage on the message_delta event.
			if usage, ok := converted["usage"].(map[string]interface{}); ok {
				cachedUsage = usage
			}
		} else {
			outputData, _ = json.Marshal(chunk)
		}

		fmt.Fprintf(w, "data: %s\n", string(outputData))
		flusher.Flush()
	}

	// Some OpenAI-compatible servers don't emit [DONE] themselves; emit it
	// so clients relying on it (e.g. openai-python) close cleanly.
	if !seenDone {
		fmt.Fprintf(w, "data: [DONE]\n")
		flusher.Flush()
	}

	// Log the streaming request using the cached usage.
	promptTokens, completionTokens, totalTokens := readTokenCounts(cachedUsage)
	duration := time.Since(startTime).Milliseconds()

	logger.PrintLog(logger.LogEntry{
		Timestamp:        startTime.UTC().Format(time.RFC3339),
		Provider:         config.Provider,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		CostUSD:          0.0, // Will be calculated in T5
		DurationMS:       duration,
		Stream:           true,
		Status:           resp.StatusCode,
		RequestID:        requestID,
	})
}

// convertAnthropicStreamChunk converts an Anthropic streaming event to an
// OpenAI chat.completion.chunk object.
func convertAnthropicStreamChunk(claudeChunk map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
	}

	// Carry over id / model if present.
	if id, ok := claudeChunk["id"].(string); ok {
		result["id"] = id
	}
	// model may live at top level or inside the "message" object.
	if model, ok := claudeChunk["model"].(string); ok {
		result["model"] = model
	} else if msg, ok := claudeChunk["message"].(map[string]interface{}); ok {
		if model, ok := msg["model"].(string); ok {
			result["model"] = model
		}
		if id, ok := msg["id"].(string); ok {
			if _, exists := result["id"]; !exists {
				result["id"] = id
			}
		}
	}

	eventType, _ := claudeChunk["type"].(string)

	choice := map[string]interface{}{
		"index":         0,
		"delta":         map[string]interface{}{},
		"finish_reason": nil,
	}

	switch eventType {
	case "message_start":
		// First chunk: emit the assistant role.
		choice["delta"] = map[string]interface{}{"role": "assistant", "content": ""}
	case "content_block_start":
		if contentBlock, ok := claudeChunk["content_block"].(map[string]interface{}); ok {
			if text, ok := contentBlock["text"].(string); ok && text != "" {
				choice["delta"] = map[string]interface{}{"content": text}
			}
		}
	case "content_block_delta":
		if delta, ok := claudeChunk["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				choice["delta"] = map[string]interface{}{"content": text}
			}
		}
	case "message_delta":
		// Final chunk: carry the stop reason and usage.
		if delta, ok := claudeChunk["delta"].(map[string]interface{}); ok {
			if stopReason, ok := delta["stop_reason"].(string); ok {
				choice["finish_reason"] = convert.MapStopReason(stopReason)
			}
		}
		if usage, ok := claudeChunk["usage"].(map[string]interface{}); ok {
			result["usage"] = convertUsageFormat(usage)
		}
	case "message_stop":
		choice["finish_reason"] = "stop"
	case "ping", "error":
		// Skip these events entirely (return empty choices).
		result["choices"] = []map[string]interface{}{}
		return result, nil
	default:
		// Unknown event: emit empty delta.
	}

	result["choices"] = []map[string]interface{}{choice}
	return result, nil
}

// convertUsageFormat converts Anthropic usage format to OpenAI usage format.
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

	pt := openaiUsage["prompt_tokens"].(int)
	ct := openaiUsage["completion_tokens"].(int)
	openaiUsage["total_tokens"] = pt + ct

	return openaiUsage
}

// limitString truncates s to maxLen characters, appending "..." if truncated.
func limitString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
