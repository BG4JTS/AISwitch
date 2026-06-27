package logger

import (
	"encoding/json"
	"fmt"
	"time"
)

// LogEntry represents a single log entry for AI API requests
type LogEntry struct {
	Timestamp       string  `json:"timestamp"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	PromptTokens    int     `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	CostUSD         float64 `json:"cost_usd"`
	DurationMS      int64   `json:"duration_ms"`
	Stream          bool    `json:"stream"`
	Status          int     `json:"status"`
	RequestID       string  `json:"request_id"`
}

// PrintLog prints a log entry in JSON format to stdout
func PrintLog(entry LogEntry) {
	// Set timestamp if not provided
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	
	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf(`{"timestamp":"%s","error":"Failed to marshal log entry: %v"}`+"\n", 
			time.Now().UTC().Format(time.RFC3339), err)
		return
	}
	
	// Print JSON to stdout
	fmt.Println(string(data))
}