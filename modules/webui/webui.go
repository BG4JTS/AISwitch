//go:build webui
// +build webui

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
