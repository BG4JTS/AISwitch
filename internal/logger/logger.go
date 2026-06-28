package logger

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// LogEntry represents a single log entry for AI API requests
type LogEntry struct {
	Timestamp        string  `json:"timestamp"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	DurationMS       int64   `json:"duration_ms"`
	Stream           bool    `json:"stream"`
	Status           int     `json:"status"`
	RequestID        string  `json:"request_id"`
}

// priceTable maps model name -> [prompt_price_per_1k_tokens, completion_price_per_1k_tokens]
// Prices in USD per 1,000 tokens (updated as of mid-2025).
var priceTable = map[string][2]float64{
	// OpenAI
	"gpt-4":            {0.03, 0.06},
	"gpt-4-turbo":      {0.01, 0.03},
	"gpt-4o":           {0.0025, 0.01},
	"gpt-4o-mini":      {0.00015, 0.0006},
	"gpt-3.5-turbo":    {0.0005, 0.0015},
	"gpt-3.5-turbo-16k": {0.003, 0.004},
	"o1-preview":       {0.015, 0.06},
	"o1-mini":          {0.003, 0.012},

	// Anthropic
	"claude-3-opus-20240229":  {0.015, 0.075},
	"claude-3-sonnet-20240229": {0.003, 0.015},
	"claude-3-haiku-20240307": {0.00025, 0.00125},
	"claude-3-5-sonnet-20241022": {0.003, 0.015},
	"claude-3-5-haiku-20241022": {0.001, 0.005},

	// DeepSeek
	"deepseek-chat":     {0.00014, 0.00028},
	"deepseek-reasoner": {0.00055, 0.00219},

	// Other common models
	"gemini-1.5-pro":  {0.00125, 0.005},
	"gemini-1.5-flash": {0.000075, 0.0003},
}

// defaultPrice is used when the model is not found in the table.
var defaultPrice = [2]float64{0.001, 0.002}

// CalculateCost computes the USD cost for a request based on token usage.
// Result is rounded to 8 decimal places.
func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	// Try exact match first
	if prices, ok := priceTable[model]; ok {
		cost := float64(promptTokens)/1000*prices[0] + float64(completionTokens)/1000*prices[1]
		return math.Round(cost*1e8) / 1e8
	}

	// Sort keys by length descending so longer (more specific) prefixes match first
	keys := make([]string, 0, len(priceTable))
	for k := range priceTable {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	// Try prefix match
	lowerModel := strings.ToLower(model)
	for _, key := range keys {
		if strings.HasPrefix(lowerModel, key) {
			prices := priceTable[key]
			cost := float64(promptTokens)/1000*prices[0] + float64(completionTokens)/1000*prices[1]
			return math.Round(cost*1e8) / 1e8
		}
	}

	// Fallback to default price
	cost := float64(promptTokens)/1000*defaultPrice[0] + float64(completionTokens)/1000*defaultPrice[1]
	return math.Round(cost*1e8) / 1e8
}

// PrintLog prints a log entry in JSON format to stdout
func PrintLog(entry LogEntry) {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Calculate cost if it hasn't been set yet
	if entry.CostUSD == 0 && (entry.PromptTokens > 0 || entry.CompletionTokens > 0) {
		entry.CostUSD = CalculateCost(entry.Model, entry.PromptTokens, entry.CompletionTokens)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf(`{"timestamp":"%s","error":"Failed to marshal log entry: %v"}`+"\n",
			time.Now().UTC().Format(time.RFC3339), err)
		return
	}

	fmt.Println(string(data))
}
