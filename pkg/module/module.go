package module

import (
	"context"

	"github.com/yourusername/ais/pkg/types"
)

// CoreContext provides shared dependencies to all modules.
type CoreContext struct {
	Config *types.Config
	// Additional dependencies can be injected here as the codebase evolves.
}

// Module is the interface every pluggable module must implement.
type Module interface {
	Name() string
	Init(ctx *CoreContext) error
	Start(ctx context.Context) error
	Stop() error
	Requires() []string
	Enabled() bool
}
