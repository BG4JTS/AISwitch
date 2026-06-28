# AI Switch

<p align="center">
  <b>One API to rule them all.</b><br>
  <i>Universal proxy for AI chat APIs — OpenAI format in, any provider out.</i>
</p>

---

AI Switch is a lightweight Go proxy that accepts **OpenAI-compatible** chat completion requests
and translates them on-the-fly to **Anthropic, DeepSeek, and other providers**. Clients see a single,
consistent OpenAI-style API — the proxy handles the rest.

## Features

- ✅ **Drop-in OpenAI replacement** — `/v1/chat/completions` with standard request/response format
- ✅ **Multi-provider** — OpenAI, Anthropic (Claude), DeepSeek, and any OpenAI-compatible service
- ✅ **Format translation** — automatic request/response conversion between OpenAI ↔ Anthropic
- ✅ **Streaming (SSE)** — full `stream=true` support with real-time chunk conversion
- ✅ **Cost tracking** — calculates `$cost_usd` per request from built-in pricing tables
- ✅ **JSON logging** — every request logged to stdout: tokens, duration, cost, status
- ✅ **Persistent config** — save API keys in `~/.ais/config.json`; no need to re-type them
- ✅ **Batteries included** — single binary, no runtime dependencies, ~8 MB

## Quick Start

### Install

```bash
go install github.com/yourusername/ais@latest
```

Or build from source:

```bash
git clone https://github.com/BG4JTS/AISwitch.git
cd AISwitch/ais
go build -o ais .
```

### Save your key once

```bash
ais config set mykey --provider deepseek --key sk-xxx --model deepseek-chat
ais config use mykey
```

### Start the proxy

```bash
ais serve
# → AI Switch started on port 8080
```

### Send a request

```bash
curl http://localhost:8080/v1/chat/completions   \
  -H "Content-Type: application/json"            \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Streaming:**

```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json"             \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "Tell me a story"}],
    "stream": true
  }'
```

## CLI Reference

### `ais serve`

Starts the proxy server.

| Flag | Default | Description |
|------|---------|-------------|
| `--provider` | `openai` | Provider name (`openai`, `anthropic`, `deepseek`) |
| `--key` | *(from config)* | API key |
| `--model` | *(from config)* | Model name |
| `--port` | `8080` | Listen port |
| `--base-url` | *(provider default)* | Custom upstream URL |
| `--profile` | *(default)* | Which saved profile to use |
| `--verbose` | `false` | Print debug information |

### `ais config`

Manages saved profiles (`~/.ais/config.json`).

```bash
ais config set    <name> --provider <p> --key <k> --model <m>
ais config use    <name>      # set as default
ais config list               # list all profiles (with -t for table)
ais config show   [name]      # show config / single profile
ais config delete <name>      # remove a profile
```

**Key is masked** (`sk-abc123***`) in `list` and `show` output.

## Supported Providers

| Provider | Default URL | Auth | Status |
|----------|-------------|------|--------|
| **OpenAI** | `https://api.openai.com/v1/chat/completions` | `Bearer` | ✅ |
| **Anthropic** | `https://api.anthropic.com/v1/messages` | `x-api-key` | ✅ auto-convert |
| **DeepSeek** | `https://api.deepseek.com/v1/chat/completions` | `Bearer` | ✅ |
| Any OpenAI-compatible | custom `--base-url` | `Bearer` | ✅ |

## Pricing Tables

Cost is calculated automatically for these models (more can be added in
`internal/logger/logger.go`):

```
gpt-4o           ·   $0.0025 / $0.01   per 1K prompt / completion
gpt-4o-mini      ·   $0.00015 / $0.0006
claude-3-sonnet  ·   $0.003 / $0.015
claude-3-haiku   ·   $0.00025 / $0.00125
deepseek-chat    ·   $0.00014 / $0.00028
gemini-1.5-flash ·   $0.000075 / $0.0003
...
```

Unknown models fall back to a default of `$0.001 / $0.002`.

## Logging

Every request prints a **single JSON line** to stdout:

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

Pipe to `jq`, aggregate, or ship to your logging stack.

## Architecture

```
Client (OpenAI SDK)           AI Switch              Upstream
       │                         │                      │
       │  POST /v1/chat/         │                      │
       │  completions            │                      │
       │ ──────────────────────▶ │                      │
       │                         │ ① parse & validate   │
       │                         │ ② translate if needed │
       │                         │ ③ add auth headers   │
       │                         │ ────────────────────▶ │
       │                         │                      │ ④ upstream model
       │                         │ ◀──────────────────── │
       │                         │ ⑤ translate response  │
       │ ◀────────────────────── │ ⑥ log & track cost   │
       │                         │                      │
```

```
ais/
├── main.go
├── cmd/
│   ├── root.go        # CLI root
│   ├── serve.go       # serve subcommand
│   └── config.go      # config subcommand
└── internal/
    ├── proxy/
    │   └── handler.go  # HTTP proxy + SSE streaming
    ├── convert/
    │   └── convert.go  # OpenAI ↔ Anthropic translation
    ├── logger/
    │   └── logger.go   # JSON logging + cost calculation
    └── config/
        └── config.go   # Persistent config (~/.ais/config.json)
```

## Testing

```bash
# Unit tests
go test ./internal/convert/ -v   # 6 tests
go test ./internal/logger/ -v    # 4 tests
go test ./internal/config/ -v    # 5 tests

# End-to-end (requires API key)
./test_e2e.sh <DEEPSEEK_API_KEY>
```

## License

MIT

---

<p align="center">
  Built with Go · Powered by Cobra<br>
  Inspired by the need for a <i>single API surface</i> across AI providers.
</p>
