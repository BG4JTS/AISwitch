//go:build tui
// +build tui

package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/BG4JTS/AISwitch/core"
	"github.com/BG4JTS/AISwitch/pkg/module"
)

// Dashboard 是 TUI 模块的公开 API。
type Dashboard struct {
	collector *StatsCollector
	provider  string
	modelName string
	port      int
}

func init() {
	core.RegisterModule(&Dashboard{})
}

func (d *Dashboard) Name() string       { return "tui" }
func (d *Dashboard) Requires() []string { return nil }
func (d *Dashboard) Enabled() bool      { return true }

func (d *Dashboard) Init(ctx *module.CoreContext) error {
	d.collector = NewStatsCollector(200)
	return nil
}

func (d *Dashboard) Start(ctx context.Context) error { return nil }
func (d *Dashboard) Stop() error                      { return nil }

// SetInfo configures the provider/model/port displayed in the dashboard.
func (d *Dashboard) SetInfo(provider, modelName string, port int) {
	d.provider = provider
	d.modelName = modelName
	d.port = port
}

// Collector returns the stats collector so the proxy handler can report to it.
func (d *Dashboard) Collector() *StatsCollector { return d.collector }

// Run runs the bubbletea TUI. Blocks until the user presses q or Ctrl+C.
func (d *Dashboard) Run() error {
	m := model{
		collector: d.collector,
		provider:  d.provider,
		modelName: d.modelName,
		port:      d.port,
		running:   true,
		viewport:  viewport.New(80, 24),
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// ── Global singleton ────────────────────────────────────────────────────

var defaultDashboard *Dashboard

// SetDefault stores the dashboard instance.
func SetDefault(d *Dashboard) { defaultDashboard = d }

// DefaultCollector returns the global stats collector.
func DefaultCollector() *StatsCollector {
	if defaultDashboard == nil {
		return nil
	}
	return defaultDashboard.collector
}

// DefaultRun starts the global TUI.
func DefaultRun(provider, modelName string, port int) error {
	if defaultDashboard == nil {
		return fmt.Errorf("tui module not initialized")
	}
	defaultDashboard.SetInfo(provider, modelName, port)
	return defaultDashboard.Run()
}
