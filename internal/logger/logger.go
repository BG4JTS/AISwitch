// Package logger 提供 AI Switch 的结构化日志输出。
//
// 通过 Logger 结构体将每次请求记录为单行 JSON 到 stdout。
// 日志包含 token 用量、费用（美元）、耗时、状态码等字段。
//
// PrintLog() 是便捷入口，内部委托给全局 defaultLogger。
// 费用计算委托给 pkg/price 包，本包仅负责日志格式化。
package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/yourusername/ais/pkg/price"
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
		entry.CostUSD = price.Global().Calculate(entry.Model, entry.PromptTokens, entry.CompletionTokens)
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

// CalculateCost is a convenience wrapper around price.Global().Calculate().
// Kept for backward compatibility.
func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	return price.Global().Calculate(model, promptTokens, completionTokens)
}
