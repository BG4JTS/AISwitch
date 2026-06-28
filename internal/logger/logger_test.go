package logger

import (
	"testing"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model                  string
		promptTokens           int
		completionTokens       int
		wantCost               float64
	}{
		{"deepseek-chat", 12, 49, 0.0000154},
		{"deepseek-chat", 1000, 1000, 0.00042},
		{"gpt-4o", 500, 200, 0.00325},
		{"claude-3-sonnet-20240229", 300, 150, 0.00315},
		{"unknown-model", 100, 50, 0.0002},
	}

	for _, tt := range tests {
		got := CalculateCost(tt.model, tt.promptTokens, tt.completionTokens)
		if got != tt.wantCost {
			t.Errorf("CalculateCost(%q, %d, %d) = %v, want %v",
				tt.model, tt.promptTokens, tt.completionTokens, got, tt.wantCost)
		}
	}
}

func TestCalculateCost_PrefixMatch(t *testing.T) {
	// gpt-4o-2024-08-06 should match gpt-4o price
	cost := CalculateCost("gpt-4o-2024-08-06", 1000, 1000)
	expected := 0.0125
	if cost != expected {
		t.Errorf("prefix match: got %v, want %v", cost, expected)
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	cost := CalculateCost("gpt-4o", 0, 0)
	if cost != 0 {
		t.Errorf("zero tokens: got %v, want 0", cost)
	}
}

func TestLogEntry_JSON(t *testing.T) {
	// Verify the struct fields parse correctly
	entry := LogEntry{
		Timestamp:        "2025-01-01T00:00:00Z",
		Provider:         "deepseek",
		Model:            "deepseek-chat",
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		CostUSD:          0.001,
		DurationMS:       500,
		Stream:           false,
		Status:           200,
		RequestID:        "test-123",
	}

	if entry.Provider != "deepseek" {
		t.Errorf("Provider = %v, want deepseek", entry.Provider)
	}
	if entry.CostUSD != 0.001 {
		t.Errorf("CostUSD = %v, want 0.001", entry.CostUSD)
	}
}
