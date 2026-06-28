// AI Switch — 通用 AI 对话 API 代理服务器。
//
// 接收 OpenAI 兼容格式的聊天请求，转发到 DeepSeek、Anthropic、OpenAI
// 等任意厂商，并自动完成格式转换、流式处理、费用追踪和结构化日志。
//
// 编译后为单一二进制文件，无运行时依赖。
//
// 使用方式：
//
//	ais serve --provider deepseek --key sk-xxx --model deepseek-chat
//	ais config set mykey --provider deepseek --key sk-xxx --model deepseek-chat
//	ais config use mykey && ais serve
package main

import "github.com/yourusername/ais/cmd"

func main() {
	cmd.Execute()
}