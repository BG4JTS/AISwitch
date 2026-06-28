# AI Switch

<p align="center">
  <b>一个 API，打通所有大模型。</b><br>
  <i>通用 AI 对话代理 — OpenAI 格式进，任意厂商出。</i>
</p>

---

AI Switch 是一个轻量的 Go 语言代理服务器，接收 **OpenAI 兼容格式** 的聊天请求，并**实时翻译**到 **DeepSeek、Anthropic (Claude) 等任意厂商**。
客户端始终看到统一、标准的 OpenAI 风格 API，其余全部由代理处理。

## 功能亮点

- ✅ **OpenAI 平替** — `/v1/chat/completions`，标准请求/响应格式
- ✅ **多厂商** — OpenAI、Anthropic (Claude)、DeepSeek，以及任何 OpenAI 兼容服务
- ✅ **格式互转** — 请求和响应自动在 OpenAI ↔ Anthropic 之间转换
- ✅ **流式输出 (SSE)** — 完整支持 `stream=true`，实时逐块转换
- ✅ **费用追踪** — 内置价格表，自动计算每次请求的 `$cost_usd`
- ✅ **JSON 日志** — 每次请求一行 JSON 输出到 stdout：tokens、耗时、费用、状态
- ✅ **持久化配置** — API Key 存到 `~/.ais/config.json`，无需每次输入
- ✅ **开箱即用** — 单文件二进制，无运行时依赖，约 8 MB

## 快速上手

### 安装

```bash
go install github.com/yourusername/ais@latest
```

或者从源码构建：

```bash
git clone https://github.com/BG4JTS/AISwitch.git
cd AISwitch/ais
go build -o ais .
```

### 保存 Key（只需一次）

```bash
ais config set mykey --provider deepseek --key sk-xxx --model deepseek-chat
ais config use mykey
```

### 启动代理

```bash
ais serve
# → AI Switch started on port 8080
```

### 发送请求

```bash
curl http://localhost:8080/v1/chat/completions   \
  -H "Content-Type: application/json"            \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "你好！"}]
  }'
```

**流式请求：**

```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json"             \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "讲个故事"}],
    "stream": true
  }'
```

## 命令参考

### `ais serve`

启动代理服务器。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--provider` | `openai` | 厂商名 (`openai`, `anthropic`, `deepseek`) |
| `--key` | *(从配置文件)* | API 密钥 |
| `--model` | *(从配置文件)* | 模型名称 |
| `--port` | `8080` | 监听端口 |
| `--base-url` | *(厂商默认)* | 自定义上游 URL |
| `--profile` | *(默认)* | 使用的配置档案名称 |
| `--verbose` | `false` | 打印调试信息 |

### `ais config`

管理已保存的配置档案 (`~/.ais/config.json`)。

```bash
ais config set    <名称> --provider <厂商> --key <密钥> --model <模型>
ais config use    <名称>      # 设为默认
ais config list               # 列出所有档案（-t 表格模式）
ais config show   [名称]      # 查看全局配置或单个档案
ais config delete <名称>      # 删除档案
```

**Key 自动脱敏**：`list` 和 `show` 输出中 Key 显示为 `sk-abc123***`。

## 支持的厂商

| 厂商 | 默认 URL | 认证方式 | 状态 |
|------|----------|----------|------|
| **OpenAI** | `https://api.openai.com/v1/chat/completions` | `Bearer` | ✅ |
| **Anthropic** | `https://api.anthropic.com/v1/messages` | `x-api-key` | ✅ 自动格式转换 |
| **DeepSeek** | `https://api.deepseek.com/v1/chat/completions` | `Bearer` | ✅ |
| 任意 OpenAI 兼容 | 自定义 `--base-url` | `Bearer` | ✅ |

## 价格表

以下模型自动计算费用（更多模型可在 `internal/logger/logger.go` 中添加）：

```
gpt-4o           ·   $0.0025 / $0.01   每千 input / output token
gpt-4o-mini      ·   $0.00015 / $0.0006
claude-3-sonnet  ·   $0.003 / $0.015
claude-3-haiku   ·   $0.00025 / $0.00125
deepseek-chat    ·   $0.00014 / $0.00028
gemini-1.5-flash ·   $0.000075 / $0.0003
...
```

未知模型使用默认价格 `$0.001 / $0.002`。

## 日志

每次请求输出**一行 JSON** 到 stdout：

```json
{
  "timestamp": "2025-06-28T12:00:00Z",
  "provider": "deepseek",
  "model": "deepseek-chat",
  "prompt_tokens": 12,
  "completion_tokens": 49,
  "total_tokens": 61,
  "cost_usd": 0.0000154,
  "duration_ms": 1912,
  "stream": false,
  "status": 200,
  "request_id": "req_1782609522010379000"
}
```

可配合 `jq` 分析、聚合，或直接接入日志系统。

## 架构

```
客户端 (OpenAI SDK)          AI Switch               上游厂商
     │                          │                      │
     │  POST /v1/chat/          │                      │
     │  completions             │                      │
     │ ──────────────────────▶  │                      │
     │                          │ ① 解析 & 校验         │
     │                          │ ② 格式转换（如需要）    │
     │                          │ ③ 添加认证头          │
     │                          │ ────────────────────▶ │
     │                          │                      │ ④ 调用上游模型
     │                          │ ◀──────────────────── │
     │                          │ ⑤ 响应格式转换         │
     │ ◀──────────────────────  │ ⑥ 记录日志 & 计算费用  │
     │                          │                      │
```

```
ais/
├── main.go               # 程序入口
├── cmd/
│   ├── root.go           # CLI 根命令
│   ├── serve.go          # serve 子命令
│   └── config.go         # config 子命令
└── internal/
    ├── proxy/
    │   └── handler.go    # HTTP 代理 + SSE 流式处理
    ├── convert/
    │   └── convert.go    # OpenAI ↔ Anthropic 格式互转
    ├── logger/
    │   └── logger.go     # JSON 日志 + 费用计算
    └── config/
        └── config.go     # 持久化配置 (~/.ais/config.json)
```

## 测试

```bash
# 单元测试
go test ./internal/convert/ -v   # 6 个测试
go test ./internal/logger/ -v    # 4 个测试
go test ./internal/config/ -v    # 5 个测试

# 端到端测试（需要 API Key）
./test_e2e.sh <DEEPSEEK_API_KEY>
```

## 开发路线

| 阶段 | 内容 | 状态 |
|------|------|------|
| M1 | 基础代理：项目初始化 + 核心转发 | ✅ |
| M2 | 格式转换：Anthropic 互转 + 流式支持 | ✅ |
| M3 | 完善体验：成本计算 + 单元测试 | ✅ |
| M4 | 运维功能：配置持久化 + 文档 | ✅ |

## 开源协议

MIT

---

<p align="center">
  用 Go 构建 · 基于 Cobra<br>
  源于"一个 API 打通所有大模型"的想法
</p>
