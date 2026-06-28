//go:build !cost
// +build !cost

package cost

import (
	"context"

	"github.com/yourusername/ais/core"
	"github.com/yourusername/ais/pkg/module"
	"github.com/yourusername/ais/pkg/price"
)

// Budget is a stub for the no-cost build.
type Budget struct{}

// Cost is a disabled cost-control module.
type Cost struct{}

func init()                                    { core.RegisterModule(&Cost{}) }
func (c *Cost) Name() string                   { return "cost" }
func (c *Cost) Requires() []string             { return nil }
func (c *Cost) Enabled() bool                  { return false }
func (c *Cost) Init(*module.CoreContext) error { return nil }
func (c *Cost) Start(_ context.Context) error  { return nil }
func (c *Cost) Stop() error                    { return nil }

// BeforeRequest always allows and returns the cost.
func BeforeRequest(model string, promptTokens, completionTokens int) (float64, error) {
	return price.Global().Calculate(model, promptTokens, completionTokens), nil
}

// Summary returns an empty budget.
func Summary() Budget { return Budget{} }
