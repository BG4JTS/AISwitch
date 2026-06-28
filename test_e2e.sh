#!/bin/bash
# ============================================================
# AI Switch 端到端测试脚本
# 使用 DeepSeek API 进行真实测试
# ============================================================
# 用法:
#   ./test_e2e.sh <DEEPSEEK_API_KEY> [PORT]
# ============================================================

set -euo pipefail

API_KEY="${1:?用法: $0 <DEEPSEEK_API_KEY> [PORT]}"
PORT="${2:-9090}"
BASE_URL="http://localhost:${PORT}"
PASS=0
FAIL=0

green()  { printf "\033[32m%s\033[0m\n" "$1"; }
red()    { printf "\033[31m%s\033[0m\n" "$1"; }
yellow() { printf "\033[33m%s\033[0m\n" "$1"; }
header() { printf "\n\033[36m━━━ %s ━━━\033[0m\n" "$1"; }

assert_eq() {
    local desc="$1" actual="$2" expected="$3"
    if [ "$actual" = "$expected" ]; then
        green "  ✅ $desc"
        PASS=$((PASS + 1))
    else
        red "  ❌ $desc"
        red "     期望: $expected"
        red "     实际: $actual"
        FAIL=$((FAIL + 1))
    fi
}

assert_contains() {
    local desc="$1" haystack="$2" needle="$3"
    if echo "$haystack" | grep -q "$needle"; then
        green "  ✅ $desc"
        PASS=$((PASS + 1))
    else
        red "  ❌ $desc"
        red "     未找到: $needle"
        FAIL=$((FAIL + 1))
    fi
}

# ============================================================
# 0. 前置检查：ais 是否已编译
# ============================================================
header "0. 前置检查"
if [ ! -f "./ais.exe" ]; then
    red "  ❌ 找不到 ais.exe，请先运行 go build -o ais.exe ."
    exit 1
fi
green "  ✅ ais.exe 存在"

# ============================================================
# 1. 启动服务器
# ============================================================
header "1. 启动 AI Switch 服务器 (端口 $PORT)"
./ais.exe serve \
    --provider deepseek \
    --key "$API_KEY" \
    --model deepseek-chat \
    --port "$PORT" &
SERVER_PID=$!
sleep 2

# 检查服务器是否启动成功
if curl -s --max-time 2 "$BASE_URL/" > /dev/null 2>&1; then
    green "  ✅ 服务器已启动 (PID: $SERVER_PID)"
else
    red "  ❌ 服务器启动失败"
    kill $SERVER_PID 2>/dev/null || true
    exit 1
fi

# ============================================================
# 2. 测试 404 - 未知路径返回 404
# ============================================================
header "2. 测试 404 - 未知路径"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/")
assert_eq "GET / 返回 404" "$STATUS" "404"

# ============================================================
# 3. 测试 405 - 只允许 POST
# ============================================================
header "3. 测试 405 - 方法限制"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/v1/chat/completions")
assert_eq "GET 返回 405" "$STATUS" "405"

# ============================================================
# 4. 测试 400 - 无效 JSON
# ============================================================
header "4. 测试 400 - 无效请求体"
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d 'not json')
STATUS=$(echo "$RESP" | tail -1)
assert_eq "无效 JSON 返回 400" "$STATUS" "400"

# ============================================================
# 5. 测试非流式请求 (DeepSeek)
# ============================================================
header "5. 测试非流式请求 (stream=false)"
RESP=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "deepseek-chat",
        "messages": [{"role": "user", "content": "请用一句话介绍你自己"}],
        "stream": false,
        "max_tokens": 100
    }')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_CODE:" | sed 's/HTTP_CODE://')
BODY=$(echo "$RESP" | grep -v "HTTP_CODE:")

assert_eq "HTTP 状态码为 200" "$HTTP_CODE" "200"
assert_contains "响应包含 id 字段" "$BODY" '"id"'
assert_contains "响应包含 object: chat.completion" "$BODY" '"object":"chat.completion"'
assert_contains "响应包含 model 字段" "$BODY" '"model"'
assert_contains "响应包含 choices 数组" "$BODY" '"choices"'
assert_contains "响应包含 usage 字段" "$BODY" '"usage"'
assert_contains "usage 包含 prompt_tokens" "$BODY" '"prompt_tokens"'
assert_contains "usage 包含 completion_tokens" "$BODY" '"completion_tokens"'
assert_contains "usage 包含 total_tokens" "$BODY" '"total_tokens"'
assert_contains "choices 包含 message" "$BODY" '"message"'
assert_contains "message 包含 assistant role" "$BODY" '"role":"assistant"'
assert_contains "message 包含 content" "$BODY" '"content"'
assert_contains "created 是非零值" "$BODY" '"created":[1-9]'
assert_contains "cost_usd > 0" "$BODY" '"cost_usd":0'
assert_not_contains "cost_usd 不是 0" "$BODY" '"cost_usd":0,'

# ============================================================
# 6. 测试流式请求 (DeepSeek)
# ============================================================
header "6. 测试流式请求 (stream=true)"
STREAM_RESP=$(curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "deepseek-chat",
        "messages": [{"role": "user", "content": "说一个数字"}],
        "stream": true,
        "max_tokens": 20
    }' --max-time 15)

assert_contains "流式响应包含 data: 行" "$STREAM_RESP" "data: "
assert_contains "流式响应包含 chat.completion.chunk" "$STREAM_RESP" '"chat.completion.chunk"'
assert_contains "流式响应包含 choices 数组" "$STREAM_RESP" '"choices"'
assert_contains "流式响应包含 delta" "$STREAM_RESP" '"delta"'
assert_contains "流式响应包含 data: [DONE]" "$STREAM_RESP" "data: \[DONE\]"

# ============================================================
# 7. 测试简短请求验证回复内容合理
# ============================================================
header "7. 验证回复内容"
RESP2=$(curl -s -X POST "$BASE_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "deepseek-chat",
        "messages": [{"role": "system", "content": "你只回复数字1，不要说别的"}],
        "messages": [{"role": "user", "content": "你好"}],
        "stream": false,
        "max_tokens": 10
    }')
assert_contains "回复包含实际内容 (非空)" "$RESP2" '"content":"'

# ============================================================
# 8. 清理
# ============================================================
header "8. 清理"
kill $SERVER_PID 2>/dev/null && green "  ✅ 服务器已停止" || yellow "  ⚠️  服务器可能已停止"

# ============================================================
# 汇总
# ============================================================
header "测试结果汇总"
TOTAL=$((PASS + FAIL))
green "  通过: $PASS / $TOTAL"
if [ "$FAIL" -gt 0 ]; then
    red "  失败: $FAIL / $TOTAL"
    exit 1
else
    green "  🎉 全部通过！"
fi