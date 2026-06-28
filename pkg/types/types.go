package types

// Usage represents token usage statistics from an API response.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LogEntry represents a single log entry for AI API requests.
type LogEntry struct {
	Timestamp        string  `json:"timestamp"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	DurationMs       int64   `json:"duration_ms"`
	Stream           bool    `json:"stream"`
	Status           int     `json:"status"`
	RequestID        string  `json:"request_id"`
	Error            string  `json:"error,omitempty"`
}

// Config is the top-level application configuration.
type Config struct {
	Server   ServerConfig   `json:"server"`
	Modules  ModuleConfigs  `json:"modules,omitempty"`
	Provider ProviderConfig `json:"provider,omitempty"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

// ModuleConfigs holds per-module configuration.
type ModuleConfigs struct {
	WebUI   WebUIConfig   `json:"webui,omitempty"`
	Cost    CostConfig    `json:"cost,omitempty"`
	KeyMgr  KeyMgrConfig  `json:"keymgr,omitempty"`
	Monitor MonitorConfig `json:"monitor,omitempty"`
}

// WebUIConfig holds WebUI module settings.
type WebUIConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

// CostConfig holds cost-control module settings.
type CostConfig struct {
	Enabled        bool    `json:"enabled"`
	BudgetDaily    float64 `json:"budget_daily"`
	BudgetMonthly  float64 `json:"budget_monthly"`
	AlertThreshold float64 `json:"alert_threshold"`
	BlockOnExceed  bool    `json:"block_on_exceed"`
}

// KeyMgrConfig holds key-manager module settings.
type KeyMgrConfig struct {
	Enabled bool   `json:"enabled"`
	Storage string `json:"storage"`
}

// MonitorConfig holds monitor module settings.
type MonitorConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

// ProviderConfig holds provider credentials and preferences.
type ProviderConfig struct {
	Default string               `json:"default"`
	Keys    map[string]string    `json:"keys"`
	Models  map[string]string    `json:"models"`
	Prices  map[string][2]float64 `json:"prices,omitempty"`
}
