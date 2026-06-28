package price

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Entry holds per-model pricing (per 1k tokens).
type Entry struct {
	PromptPrice     float64
	CompletionPrice float64
}

// Table stores all model prices.
type Table struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// NewTable creates a Table with default prices loaded.
func NewTable() *Table {
	t := &Table{entries: make(map[string]Entry)}
	t.LoadDefaults()
	return t
}

// LoadDefaults fills the table with built-in prices.
func (t *Table) LoadDefaults() {
	defaults := map[string]Entry{
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
		// Gemini
		"gemini-1.5-pro":   {0.00125, 0.005},
		"gemini-1.5-flash": {0.000075, 0.0003},
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range defaults {
		t.entries[k] = v
	}
}

// Set adds or overwrites a model's price.
func (t *Table) Set(model string, promptPrice, completionPrice float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[model] = Entry{promptPrice, completionPrice}
}

// Get returns the price entry for a model, or the fallback (0.001, 0.002).
func (t *Table) Get(model string) Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if e, ok := t.entries[model]; ok {
		return e
	}
	// Prefix match (longest keys first)
	lower := strings.ToLower(model)
	best := ""
	for k := range t.entries {
		if strings.HasPrefix(lower, k) && len(k) > len(best) {
			best = k
		}
	}
	if best != "" {
		return t.entries[best]
	}
	return Entry{0.001, 0.002}
}

// Calculate computes USD cost and rounds to 8 decimal places.
func (t *Table) Calculate(model string, promptTokens, completionTokens int) float64 {
	e := t.Get(model)
	cost := float64(promptTokens)/1000*e.PromptPrice + float64(completionTokens)/1000*e.CompletionPrice
	return math.Round(cost*1e8) / 1e8
}

// LoadFromMap loads prices from a map of "model" → "prompt,completion" strings.
func (t *Table) LoadFromMap(m map[string]string) error {
	for model, priceStr := range m {
		parts := strings.FieldsFunc(priceStr, func(r rune) bool {
			return r == ',' || r == ' ' || r == '/'
		})
		if len(parts) != 2 {
			return fmt.Errorf("invalid price format for %s: %q", model, priceStr)
		}
		prompt, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return fmt.Errorf("bad prompt price for %s: %w", model, err)
		}
		completion, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return fmt.Errorf("bad completion price for %s: %w", model, err)
		}
		t.Set(model, prompt, completion)
	}
	return nil
}

// LoadFromEnv reads AIS_PRICE_<MODEL> environment variables.
// Example: AIS_PRICE_GPT4="0.03,0.06"
func (t *Table) LoadFromEnv() {
	prefix := "AIS_PRICE_"
	for _, e := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(e), prefix) {
			continue
		}
		kv := strings.SplitN(e, "=", 2)
		if len(kv) != 2 {
			continue
		}
		model := strings.TrimPrefix(strings.ToUpper(kv[0]), prefix)
		model = strings.ToLower(strings.ReplaceAll(model, "_", "-"))
		value := kv[1]
		parts := strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ' ' || r == '/'
		})
		if len(parts) != 2 {
			continue
		}
		prompt, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			continue
		}
		completion, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			continue
		}
		t.Set(model, prompt, completion)
	}
}

// ---------------------------------------------------------------------------
// Global singleton
// ---------------------------------------------------------------------------

var globalMu sync.Mutex
var globalTable *Table

// Global returns the global price table (lazy init with defaults + env).
func Global() *Table {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalTable == nil {
		globalTable = NewTable()
		globalTable.LoadFromEnv()
	}
	return globalTable
}

// OverrideGlobal replaces the global table (e.g. with config-loaded prices).
func OverrideGlobal(t *Table) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalTable = t
}
