package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

		// Construct target URL
		targetURL := config.BaseURL
		if targetURL == "" {
			if config.Provider == "openai" {
				targetURL = "https://api.openai.com/v1/chat/completions"
			} else {
				http.Error(w, "Unsupported provider", http.StatusBadRequest)
				return
			}
		}

		// Re-create request body for forwarding
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal request body: %v", err), http.StatusInternalServerError)
			return
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

		// Set authorization header
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Key))

		// Make the request
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to make request: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
			return
		}

		// Parse response to extract usage
		var responseBody map[string]interface{}
		usage := map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		}

		if err := json.Unmarshal(body, &responseBody); err == nil {
			if usageData, ok := responseBody["usage"].(map[string]interface{}); ok {
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

		// Generate request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
		}

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

		// Write response body
		w.Write(body)
	}
}