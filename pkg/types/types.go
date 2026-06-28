// Package types 定义了 AI Switch 中共享的核心数据结构。
//
// 本包被 pkg/、internal/、modules/ 等多个子包引用，
// 提供统一的配置模型、日志模型和 API 用量模型。
package types

// Usage 表示单次 API 调用的 token 用量统计。
//
// 字段对应 OpenAI 响应中 usage 对象的语义：
//
//	PromptTokens     输入（prompt）消耗的 token 数量
//	CompletionTokens 输出（completion）消耗的 token 数量
//	TotalTokens      两者之和
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LogEntry 表示每次代理请求的结构化日志条目。
//
// 每个请求在完成后会写入一行 JSON 到标准输出，
// 包含请求元数据、token 用量、费用和耗时。
//
// 字段说明：
//
//	Timestamp        请求开始时的 UTC 时间戳（RFC 3339）
//	Provider         上游提供商名称（openai / anthropic / deepseek）
//	Model            实际使用的模型名称
//	PromptTokens     输入 token 数
//	CompletionTokens 输出 token 数
//	TotalTokens      合计 token 数
//	CostUSD          本次请求的美元费用（由价格表计算并四舍五入到 8 位小数）
//	DurationMS       从收到请求到返回响应的耗时（毫秒）
//	Stream           是否为流式请求
//	Status           上游 API 返回的 HTTP 状态码
//	RequestID        请求唯一标识（从 X-Request-ID 头部或自动生成）
//	Error            如果请求失败，包含错误信息（可选）
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
	Error            string  `json:"error,omitempty"`
}

// Config 是应用层的顶层配置结构。
//
// 涵盖服务器监听参数、可选模块开关和提供商凭据。
type Config struct {
	Server   ServerConfig   `json:"server"`
	Modules  ModuleConfigs  `json:"modules,omitempty"`
	Provider ProviderConfig `json:"provider,omitempty"`
}

// ServerConfig 定义 HTTP 服务器的监听参数。
type ServerConfig struct {
	Port int    `json:"port"` // 监听端口，默认 8080
	Host string `json:"host"` // 监听地址，默认 localhost
}

// ModuleConfigs 聚合所有可选模块的配置。
//
// 每个模块通过 build tag 控制是否编译，
// 因此对应字段在未编译时会被忽略。
type ModuleConfigs struct {
	WebUI   WebUIConfig   `json:"webui,omitempty"`
	Cost    CostConfig    `json:"cost,omitempty"`
	KeyMgr  KeyMgrConfig  `json:"keymgr,omitempty"`
	Monitor MonitorConfig `json:"monitor,omitempty"`
}

// WebUIConfig 是 Web 仪表板模块的配置。
//
// 编译标签：webui
type WebUIConfig struct {
	Enabled bool `json:"enabled"` // 是否启用 WebUI
	Port    int  `json:"port"`    // WebUI 监听端口
}

// CostConfig 是费用控制模块的配置。
//
// 编译标签：cost
type CostConfig struct {
	Enabled        bool    `json:"enabled"`         // 是否启用费用控制
	BudgetDaily    float64 `json:"budget_daily"`    // 每日预算上限（美元）
	BudgetMonthly  float64 `json:"budget_monthly"`  // 每月预算上限（美元）
	AlertThreshold float64 `json:"alert_threshold"` // 告警阈值（0~1），默认 0.8
	BlockOnExceed  bool    `json:"block_on_exceed"` // 超预算时是否拒绝请求（返回 429）
}

// KeyMgrConfig 是加密 Key 管理模块的配置。
//
// 编译标签：keymgr
type KeyMgrConfig struct {
	Enabled bool   `json:"enabled"` // 是否启用
	Storage string `json:"storage"` // Key 文件存储路径
}

// MonitorConfig 是监控模块的配置。
type MonitorConfig struct {
	Enabled bool `json:"enabled"` // 是否启用
	Port    int  `json:"port"`    // 监控 HTTP 端口
}

// ProviderConfig 保存所有上游提供商的凭据和偏好设置。
type ProviderConfig struct {
	Default string               `json:"default"`          // 默认提供商名称
	Keys    map[string]string    `json:"keys"`             // 提供商 → API Key 映射
	Models  map[string]string    `json:"models"`           // 提供商 → 默认模型映射
	Prices  map[string][2]float64 `json:"prices,omitempty"` // 模型 → [输入价, 输出价] 映射
}
