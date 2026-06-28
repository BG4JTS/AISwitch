//go:build !tui
// +build !tui

package tui

import (
	"context"

	"github.com/BG4JTS/AISwitch/core"
	"github.com/BG4JTS/AISwitch/pkg/module"
	"github.com/BG4JTS/AISwitch/pkg/price"
)

// Budget is a stub for the no-tui build.
type Budget struct{}

// Tui is a disabled tui-control module.
type Tui struct{}

func init()                                    { core.RegisterModule(&Tui{}) }
func (c *Tui) Name() string                   { return "tui" }
func (c *Tui) Requires() []string             { return nil }
func (c *Tui) Enabled() bool                  { return false }
func (c *Tui) Init(*module.CoreContext) error { return nil }
func (c *Tui) Start(_ context.Context) error  { return nil }
func (c *Tui) Stop() error                    { return nil }

// BeforeRequest always allows and returns the tui.
func BeforeRequest(model string, promptTokens, completionTokens int) (float64, error) {
	return price.Global().Calculate(model, promptTokens, completionTokens), nil
}

// Summary returns an empty budget.
func Summary() Budget { return Budget{} }
