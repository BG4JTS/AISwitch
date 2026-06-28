package logger

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/yourusername/ais/pkg/types"
)

// LogEntry is an alias for the shared type defined in pkg/types.
type LogEntry = types.LogEntry

// Logger writes structured JSON log entries.
type Logger struct {
	encoder *json.Encoder
}

// New creates a Logger that writes to stdout.
func New() *Logger {
	return &Logger{encoder: json.NewEncoder(os.Stdout)}
}

// Log writes a single log entry as JSON.
func (l *Logger) Log(entry LogEntry) error {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if entry.CostUSD == 0 && (entry.PromptTokens > 0 || entry.CompletionTokens > 0) {
		entry.CostUSD = CalculateCost(entry.Model, entry.PromptTokens, entry.CompletionTokens)
	}
	return l.encoder.Encode(entry)
}

// defaultLogger is the global singleton used by the convenience PrintLog function.
var defaultLogger = New()

// PrintLog is a convenience wrapper that writes a log entry to stdout.
// It is kept for backward compatibility with existing callers.
func PrintLog(entry LogEntry) {
	if err := defaultLogger.Log(entry); err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
	}
}

// ---------------------------------------------------------------------------
// Pricing tables and cost calculation (unchanged)
// ---------------------------------------------------------------------------

var priceTable = map[string][2]float64{
	// OpenAI
	"gpt-4":              {0.03, 0.06},
	"gpt-4-turbo":        {0.01, 0.03},
	"gpt-4o":             {0.0025, 0.01},
	"gpt-4o-mini":        {0.00015, 0.0006},
	"gpt-3.5-turbo":      {0.0005, 0.0015},
	"gpt-3.5-turbo-16k":  {0.003, 0.004},
	"o1-preview":         {0.015, 0.06},
	"o1-mini":            {0.003, 0.012},

	// Anthropic
	"claude-3-opus-20240229":    {0.015, 0.075},
	"claude-3-sonnet-20240229":  {0.003, 0.015},
	"claude-3-haiku-20240307":   {0.00025, 0.00125},
	"claude-3-5-sonnet-20241022": {0.003, 0.015},
	"claude-3-5-haiku-20241022":  {0.001, 0.005},

	// DeepSeek
	"deepseek-chat":     {0.00014, 0.00028},
	"deepseek-reasoner": {0.00055, 0.00219},

	// Other
	"gemini-1.5-pro":   {0.00125, 0.005},
	"gemini-1.5-flash": {0.000075, 0.0003},
}

var defaultPrice = [2]float64{0.001, 0.002}

func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	if prices, ok := priceTable[model]; ok {
		return round(promptTokens, completionTokens, prices)
	}
	// Prefix match (longest keys first)
	keys := make([]string, 0, len(priceTable))
	for k := range priceTable {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	lowerModel := strings.ToLower(model)
	for _, key := range keys {
		if strings.HasPrefix(lowerModel, key) {
			return round(promptTokens, completionTokens, priceTable[key])
		}
	}
	return round(promptTokens, completionTokens, defaultPrice)
}

func round(promptTokens, completionTokens int, prices [2]float64) float64 {
	cost := float64(promptTokens)/1000*prices[0] + float64(completionTokens)/1000*prices[1]
	return math.Round(cost*1e8) / 1e8
}
