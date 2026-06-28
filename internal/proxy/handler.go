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
	"github.com/yourusername/ais/internal/keymanager"
	"github.com/yourusername/ais/internal/logger"
)

// ── Config ──────────────────────────────────────────────────────────────

// Config holds proxy configuration.
type Config struct {
	Provider string
	Key      string
	Model    string
	BaseURL  string
	Verbose  bool
	KeyMgr   *keymanager.Manager
}

// resolveKey returns the API key. Priority: KeyMgr > config.Key.
func (c Config) resolveKey() (string, error) {
	if c.KeyMgr != nil {
		if key, err := c.KeyMgr.GetKey(c.Provider); err == nil {
			return key, nil
		}
	}
	if c.Key != "" {
		return c.Key, nil
	}
	return "", fmt.Errorf("no API key provided for %s", c.Provider)
}

func defaultTargetURL(provider string) (string, bool) {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1/chat/completions", true
	case "anthropic":
		return "https://api.anthropic.com/v1/messages", true
	case "deepseek":
		return "https://api.deepseek.com/v1/chat/completions", true
	default:
		return "", false
	}
}

// ── Handler ─────────────────────────────────────────────────────────────

// Handler returns an http.HandlerFunc that proxies chat completion requests.
func Handler(config Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		if config.Verbose {
			fmt.Printf("[VERBOSE] %s %s %s → %s (model=%s)\n",
				r.Method, r.URL.Path, r.RemoteAddr, config.Provider, config.Model)
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, model, stream, err := parseRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if model == "" {
			model = config.Model
		}

		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
		}

		upstreamBody, err := buildUpstreamBody(config, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if config.Verbose {
			fmt.Printf("[VERBOSE] → upstream: %s\n", limitString(string(upstreamBody), 500))
		}

		resp, err := sendUpstream(config, r, upstreamBody, stream)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			forwardError(w, resp)
			return
		}

		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			handleStreamingResponse(w, resp, config, model, startTime, requestID)
			return
		}

		handleNonStreaming(w, resp, config, model, startTime, requestID)
	}
}

// ── Request helpers ─────────────────────────────────────────────────────

func parseRequest(r *http.Request) (body map[string]interface{}, model string, stream bool, err error) {
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, "", false, fmt.Errorf("invalid request body: %v", err)
	}
	defer r.Body.Close()
	if v, ok := body["stream"].(bool); ok {
		stream = v
	}
	if v, ok := body["model"].(string); ok {
		model = v
	}
	return body, model, stream, nil
}

func buildUpstreamBody(config Config, body map[string]interface{}) ([]byte, error) {
	if config.Provider == "anthropic" {
		return convert.ConvertOpenAIReqToClaude(body)
	}
	return json.Marshal(body)
}

func sendUpstream(config Config, r *http.Request, body []byte, stream bool) (*http.Response, error) {
	targetURL := config.BaseURL
	if targetURL == "" {
		url, ok := defaultTargetURL(config.Provider)
		if !ok {
			return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
		}
		targetURL = url
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}
	copyRelevantHeaders(r, req)

	apiKey, err := config.resolveKey()
	if err != nil {
		return nil, err
	}
	setAuthHeaders(req, config.Provider, apiKey)

	client := &http.Client{}
	if !stream {
		client.Timeout = 120 * time.Second
	}
	return client.Do(req)
}

func copyRelevantHeaders(src *http.Request, dst *http.Request) {
	for key, values := range src.Header {
		if strings.EqualFold(key, "Content-Type") || strings.EqualFold(key, "Accept") {
			for _, v := range values {
				dst.Header.Set(key, v)
			}
		}
	}
}

func setAuthHeaders(req *http.Request, provider, apiKey string) {
	if provider == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}
}

// ── Non‑streaming response ──────────────────────────────────────────────

func handleNonStreaming(w http.ResponseWriter, resp *http.Response, config Config,
	model string, startTime time.Time, requestID string) {

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	responseBody := body
	if config.Provider == "anthropic" {
		if converted, err := convert.ConvertClaudeRespToOpenAI(body); err == nil {
			responseBody = converted
		}
	}

	u := extractUsage(responseBody)
	p, c, t := readTokenCounts(u)
	dur := time.Since(startTime).Milliseconds()

	logger.PrintLog(logger.LogEntry{
		Timestamp:        startTime.UTC().Format(time.RFC3339),
		Provider:         config.Provider,
		Model:            model,
		PromptTokens:     p,
		CompletionTokens: c,
		TotalTokens:      t,
		DurationMS:       dur,
		Stream:           false,
		Status:           resp.StatusCode,
		RequestID:        requestID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(responseBody)

	if config.Verbose {
		fmt.Printf("[VERBOSE] ← status=%d bytes=%d tokens(p=%d,c=%d)\n",
			resp.StatusCode, len(responseBody), p, c)
	}
}

// ── Error forwarding ─────────────────────────────────────────────────────

func forwardError(w http.ResponseWriter, resp *http.Response) {
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ── Usage extraction ────────────────────────────────────────────────────

func extractUsage(body []byte) map[string]interface{} {
	var m map[string]interface{}
	if json.Unmarshal(body, &m) != nil {
		return zeroUsage()
	}
	if usage, ok := m["usage"].(map[string]interface{}); ok {
		return usage
	}
	return zeroUsage()
}

func zeroUsage() map[string]interface{} {
	return map[string]interface{}{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
}

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

// ── Streaming ───────────────────────────────────────────────────────────

func handleStreamingResponse(w http.ResponseWriter, resp *http.Response, config Config,
	model string, startTime time.Time, requestID string) {

	flusher, _ := w.(http.Flusher)
	if flusher == nil {
		return
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var cachedUsage map[string]interface{}
	seenDone := false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			fmt.Fprint(w, "\n")
			flusher.Flush()
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}
		dataLine := line[6:] // strip "data: "
		if dataLine == "[DONE]" {
			seenDone = true
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}
		var chunk map[string]interface{}
		if json.Unmarshal([]byte(dataLine), &chunk) != nil {
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
			continue
		}
		if ud, ok := chunk["usage"].(map[string]interface{}); ok {
			cachedUsage = ud
		}
		output := streamOutput(config, chunk, &cachedUsage)
		fmt.Fprintf(w, "%s", output)
		flusher.Flush()
	}

	if !seenDone {
		fmt.Fprint(w, "data: [DONE]\n")
		flusher.Flush()
	}

	p, c, t := readTokenCounts(cachedUsage)
	dur := time.Since(startTime).Milliseconds()
	logger.PrintLog(logger.LogEntry{
		Timestamp:        startTime.UTC().Format(time.RFC3339),
		Provider:         config.Provider,
		Model:            model,
		PromptTokens:     p,
		CompletionTokens: c,
		TotalTokens:      t,
		DurationMS:       dur,
		Stream:           true,
		Status:           resp.StatusCode,
		RequestID:        requestID,
	})
}

func streamOutput(config Config, chunk map[string]interface{}, cached *map[string]interface{}) string {
	if config.Provider != "anthropic" {
		data, _ := json.Marshal(chunk)
		return fmt.Sprintf("data: %s\n", data)
	}
	converted, err := convertChunk(chunk)
	if err != nil {
		data, _ := json.Marshal(chunk)
		return fmt.Sprintf("data: %s\n", data)
	}
	if ud, ok := converted["usage"].(map[string]interface{}); ok {
		*cached = ud
	}
	data, _ := json.Marshal(converted)
	return fmt.Sprintf("data: %s\n", data)
}

// ── Anthropic chunk conversion ──────────────────────────────────────────

func convertChunk(chunk map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
	}
	enrichChunkMeta(chunk, result)

	eventType, _ := chunk["type"].(string)
	switch eventType {
	case "ping", "error":
		result["choices"] = []map[string]interface{}{}
		return result, nil
	default:
		result["choices"] = []map[string]interface{}{chunkChoice(chunk, eventType)}
		return result, nil
	}
}

func enrichChunkMeta(chunk, result map[string]interface{}) {
	if id, ok := chunk["id"].(string); ok {
		result["id"] = id
	}
	if model, ok := chunk["model"].(string); ok {
		result["model"] = model
	} else if msg, ok := chunk["message"].(map[string]interface{}); ok {
		if model, ok := msg["model"].(string); ok {
			result["model"] = model
		}
		if id, ok := msg["id"].(string); ok {
			if _, exists := result["id"]; !exists {
				result["id"] = id
			}
		}
	}
}

func chunkChoice(chunk map[string]interface{}, eventType string) map[string]interface{} {
	choice := map[string]interface{}{
		"index":         0,
		"delta":         map[string]interface{}{},
		"finish_reason": nil,
	}
	switch eventType {
	case "message_start":
		choice["delta"] = map[string]interface{}{"role": "assistant", "content": ""}
	case "content_block_start":
		if cb, ok := chunk["content_block"].(map[string]interface{}); ok {
			if text, ok := cb["text"].(string); ok && text != "" {
				choice["delta"] = map[string]interface{}{"content": text}
			}
		}
	case "content_block_delta":
		if delta, ok := chunk["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				choice["delta"] = map[string]interface{}{"content": text}
			}
		}
	case "message_delta":
		if delta, ok := chunk["delta"].(map[string]interface{}); ok {
			if sr, ok := delta["stop_reason"].(string); ok {
				choice["finish_reason"] = convert.MapStopReason(sr)
			}
		}
	case "message_stop":
		choice["finish_reason"] = "stop"
	}
	return choice
}

// ── Utilities ───────────────────────────────────────────────────────────

func limitString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
