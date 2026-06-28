//go:build !keymgr
// +build !keymgr

package keymgr

import (
	"context"
	"fmt"

	"github.com/yourusername/ais/core"
	"github.com/yourusername/ais/pkg/module"
)

type stub struct{}

func init()                                    { core.RegisterModule(&stub{}) }
func (s *stub) Name() string                   { return "keymgr" }
func (s *stub) Requires() []string             { return nil }
func (s *stub) Enabled() bool                  { return false }
func (s *stub) Init(*module.CoreContext) error { return nil }
func (s *stub) Start(_ context.Context) error  { return nil }
func (s *stub) Stop() error                    { return nil }

// CliAdd returns an error — keymgr module is not compiled in.
func CliAdd(provider, key string) error {
	return fmt.Errorf("keymgr module not loaded (build with -tags keymgr)")
}

// CliList returns nil when the module is not compiled.
func CliList() interface{} { return nil }

// CliDelete returns an error.
func CliDelete(provider string) error {
	return fmt.Errorf("keymgr module not loaded (build with -tags keymgr)")
}
