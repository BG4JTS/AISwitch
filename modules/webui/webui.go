//go:build webui
// +build webui

// Package webui 提供可选的 Web 仪表板模块（编译标签：webui）。
//
// 使用 //go:embed 将静态文件编译进二进制。
// 路由：GET /dashboard/* → 嵌入式静态文件服务。
//
// 编译标签：//go:build webui
package webui

import (
	"context"
	"embed"
	"io/fs"
	"net/http"

	"github.com/yourusername/ais/core"
	"github.com/yourusername/ais/pkg/module"
)

//go:embed dist/*
var staticFiles embed.FS

type WebUI struct{}

func init() {
	core.RegisterModule(&WebUI{})
}

func (m *WebUI) Name() string     { return "webui" }
func (m *WebUI) Requires() []string { return nil }
func (m *WebUI) Enabled() bool    { return true }

func (m *WebUI) Init(ctx *module.CoreContext) error { return nil }
func (m *WebUI) Stop() error                        { return nil }

func (m *WebUI) Start(ctx context.Context) error {
	sub, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		// If dist/ is empty or missing, serve nothing.
		return nil
	}
	http.Handle("/dashboard/", http.StripPrefix("/dashboard/", http.FileServer(http.FS(sub))))
	return nil
}
