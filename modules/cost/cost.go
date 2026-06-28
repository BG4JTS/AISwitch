//go:build cost
// +build cost

// Package cost 提供可选的费用控制模块（编译标签：cost）。
//
// 基于 pkg/price 的价格表追踪每日/每月 API 花费。
// 超预算时可选拒绝新请求（HTTP 429）。
//
// 特性：每日/每月双轨预算、文件持久化（~/.ais/budget.json）、
// 80% 告警阈值、block_on_exceed 熔断、日期变更自动重置。
//
// 编译标签：//go:build cost
package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yourusername/ais/core"
	"github.com/yourusername/ais/pkg/module"
	"github.com/yourusername/ais/pkg/price"
	"github.com/yourusername/ais/pkg/types"
)

// Budget tracks daily/monthly spending.
type Budget struct {
	Daily   float64 `json:"daily"`
	Monthly float64 `json:"monthly"`
	Day     string  `json:"day"`   // YYYY-MM-DD
	Month   string  `json:"month"` // YYYY-MM
}

// Controller monitors and enforces spending limits.
type Controller struct {
	mu             sync.Mutex
	config         types.CostConfig
	budget         Budget
	savePath       string
}

// Cost is the cost-control module.
type Cost struct {
	ctrl *Controller
}

func init() {
	core.RegisterModule(&Cost{})
}

func (m *Cost) Name() string                { return "cost" }
func (m *Cost) Requires() []string          { return nil }
func (m *Cost) Enabled() bool               { return true }

func (m *Cost) Init(ctx *module.CoreContext) error {
	cfg := types.CostConfig{
		Enabled:        true,
		BudgetDaily:    10.0,
		BudgetMonthly:  100.0,
		AlertThreshold: 0.8,
		BlockOnExceed:  true,
	}
	if ctx.Config != nil {
		if ctx.Config.Modules.Cost.Enabled {
			cfg = ctx.Config.Modules.Cost
		}
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ais")
	os.MkdirAll(dir, 0700)

	m.ctrl = &Controller{
		config:   cfg,
		savePath: filepath.Join(dir, "budget.json"),
	}
	m.ctrl.load()
	return nil
}

func (m *Cost) Start(ctx context.Context) error { return nil }
func (m *Cost) Stop() error                      { return nil }

// BeforeRequest checks the budget and returns an error if blocked.
// Returns the cost in USD for logging purposes.
func (m *Cost) BeforeRequest(model string, promptTokens, completionTokens int) (costUSD float64, err error) {
	if m == nil || m.ctrl == nil || !m.ctrl.config.Enabled {
		return price.Global().Calculate(model, promptTokens, completionTokens), nil
	}

	costUSD = price.Global().Calculate(model, promptTokens, completionTokens)

	m.ctrl.mu.Lock()
	defer m.ctrl.mu.Unlock()

	// Reset counters if the day or month changed
	today := time.Now().Format("2006-01-02")
	thisMonth := time.Now().Format("2006-01")
	if m.ctrl.budget.Day != today {
		m.ctrl.budget.Daily = 0
		m.ctrl.budget.Day = today
	}
	if m.ctrl.budget.Month != thisMonth {
		m.ctrl.budget.Monthly = 0
		m.ctrl.budget.Month = thisMonth
	}

	// Check before adding
	if m.ctrl.config.BlockOnExceed {
		if m.ctrl.config.BudgetDaily > 0 && m.ctrl.budget.Daily >= m.ctrl.config.BudgetDaily {
			return costUSD, fmt.Errorf("daily budget exceeded ($%.4f used, $%.2f limit)",
				m.ctrl.budget.Daily, m.ctrl.config.BudgetDaily)
		}
		if m.ctrl.config.BudgetMonthly > 0 && m.ctrl.budget.Monthly >= m.ctrl.config.BudgetMonthly {
			return costUSD, fmt.Errorf("monthly budget exceeded ($%.4f used, $%.2f limit)",
				m.ctrl.budget.Monthly, m.ctrl.config.BudgetMonthly)
		}
	}

	// Add cost
	m.ctrl.budget.Daily += costUSD
	m.ctrl.budget.Monthly += costUSD

	// Alert check
	threshold := m.ctrl.config.AlertThreshold
	if m.ctrl.config.BudgetDaily > 0 && threshold > 0 {
		ratio := m.ctrl.budget.Daily / m.ctrl.config.BudgetDaily
		if ratio >= threshold {
			log.Printf("[COST] ⚠️  daily budget at %.0f%% ($%.4f / $%.2f)",
				math.Round(ratio*100), m.ctrl.budget.Daily, m.ctrl.config.BudgetDaily)
		}
	}

	m.ctrl.save()
	return costUSD, nil
}

// Summary returns the current spending summary.
func (m *Cost) Summary() Budget {
	if m == nil || m.ctrl == nil {
		return Budget{}
	}
	m.ctrl.mu.Lock()
	defer m.ctrl.mu.Unlock()
	return m.ctrl.budget
}

func (c *Controller) load() {
	data, err := os.ReadFile(c.savePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &c.budget)
}

func (c *Controller) save() {
	data, _ := json.Marshal(c.budget)
	os.WriteFile(c.savePath, data, 0600)
}

// Singleton shortcut
var defaultCost *Cost

// SetDefault registers the global cost controller.
func SetDefault(m *Cost) { defaultCost = m }

// BeforeRequest is a convenience function usable from handler.go.
func BeforeRequest(model string, promptTokens, completionTokens int) (float64, error) {
	if defaultCost == nil {
		return price.Global().Calculate(model, promptTokens, completionTokens), nil
	}
	return defaultCost.BeforeRequest(model, promptTokens, completionTokens)
}

// Summary returns current spending.
func Summary() Budget {
	if defaultCost == nil {
		return Budget{}
	}
	return defaultCost.Summary()
}
